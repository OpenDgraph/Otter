package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/OpenDgraph/Otter/internal/proxy"
	"github.com/dgraph-io/dgo/v240/protos/api"
	"github.com/gorilla/websocket"
)

// Upgrader is responsible for upgrading HTTP requests to WebSocket connections.
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow all connections (not safe for production)
		return true
	},
}

func checkAuth(authenticated bool, conn *websocket.Conn) bool {
	if !authenticated {
		conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"papers please!"}`))
		return false
	}
	return true
}

func HandleWebSocketWithProxy(p *proxy.Proxy) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("| Failed to upgrade connection: %v\n", err)
			http.Error(w, "WebSocket upgrade failed", http.StatusBadRequest)
			return
		}
		defer func() {
			log.Printf("| Closing connection: %s\n", conn.RemoteAddr())
			conn.Close()
		}()

		log.Printf("| Client connected: %s\n", conn.RemoteAddr())

		authenticated := false

		for {
			_, msgBytes, err := conn.ReadMessage()
			if err != nil {
				log.Printf("| Error reading message: %v\n", err)
				break
			}

			var msg WSMessage
			if err := json.Unmarshal(msgBytes, &msg); err != nil {
				conn.WriteMessage(websocket.TextMessage, fmt.Appendf(nil, `{"error":"invalid JSON: %v"}`, err))
				continue
			}

			// só valida estrutura
			if err := msg.validate(conn); err != nil {
				continue
			}

			switch msg.Type {
			case TypePing:
				conn.WriteMessage(websocket.TextMessage, []byte(`{"status":"pong"}`))

			case TypeAuth:
				if IsValidToken(msg.Token) {
					authenticated = true
					conn.WriteMessage(websocket.TextMessage, []byte(`{"status":"authenticated"}`))
				} else {
					conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"invalid token"}`))
				}
				continue

			case TypeQuery:
				isAuthorized := checkAuth(authenticated, conn)
				if !isAuthorized {
					continue
				}

				_, client, err := p.SelectClientAuto("query")
				if err != nil {
					conn.WriteMessage(websocket.TextMessage, fmt.Appendf(nil, `{"error":"%v"}`, err))
					continue
				}

				resp, err := client.Query(context.Background(), msg.Query)
				if err != nil {
					conn.WriteMessage(websocket.TextMessage, fmt.Appendf(nil, `{"error":"%v"}`, err))
					continue
				}

				if msg.Verbose {
					out := WSResponse{
						Data:      resp.Json,
						LatencyNs: resp.Latency.GetTotalNs(),
					}
					b, _ := json.Marshal(out)
					conn.WriteMessage(websocket.TextMessage, b)
				} else {
					// Resposta direta, só o JSON da query
					conn.WriteMessage(websocket.TextMessage, resp.Json)
				}

			case TypeMutation:
				isAuthorized := checkAuth(authenticated, conn)
				if !isAuthorized {
					continue
				}
				_, client, err := p.SelectClientAuto("mutation")
				if err != nil {
					conn.WriteMessage(websocket.TextMessage, fmt.Appendf(nil, `{"error":"%v"}`, err))
					continue
				}

				m := &api.Mutation{
					SetNquads: []byte(msg.Mutation),
					CommitNow: msg.CommitNow,
				}
				resp, err := client.Mutate(context.Background(), m)
				if err != nil {
					conn.WriteMessage(websocket.TextMessage, fmt.Appendf(nil, `{"error":"%v"}`, err))
					continue
				}

				if msg.Verbose {
					out := WSResponse{
						Data:      resp.Json,
						Uids:      resp.Uids,
						CommitTs:  resp.Txn.GetCommitTs(),
						Preds:     resp.Txn.GetPreds(),
						LatencyNs: resp.Latency.GetTotalNs(),
					}
					b, _ := json.Marshal(out)
					conn.WriteMessage(websocket.TextMessage, b)
				} else {
					data := resp.Json
					if len(data) == 0 {
						data = []byte(`{}`)
					}
					conn.WriteMessage(websocket.TextMessage, data)
				}

			case TypeUpsert:
				isAuthorized := checkAuth(authenticated, conn)
				if !isAuthorized {
					continue
				}
				_, client, err := p.SelectClientAuto("upsert")
				if err != nil {
					conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"%v"}`))
					continue
				}

				mu := &api.Mutation{
					SetNquads: []byte(msg.Mutation),
				}
				if msg.Cond != "" {
					mu.Cond = msg.Cond
				}

				resp, err := client.Upsert(context.Background(), msg.Query, []*api.Mutation{mu}, msg.CommitNow)
				if err != nil {
					out := WSResponse{Error: err.Error()}
					b, _ := json.Marshal(out)
					conn.WriteMessage(websocket.TextMessage, b)
					continue
				}

				out := WSResponse{
					Data:      resp.Json,
					Uids:      resp.Uids,
					CommitTs:  resp.Txn.GetCommitTs(),
					Preds:     resp.Txn.GetPreds(),
					LatencyNs: resp.Latency.GetTotalNs(),
				}

				b, err := json.Marshal(out)
				if err != nil {
					conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"failed to encode response"}`))
					continue
				}
				conn.WriteMessage(websocket.TextMessage, b)

			default:
				conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"unsupported type"}`))
			}
		}
	}
}
