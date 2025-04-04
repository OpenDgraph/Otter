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

			switch msg.Type {
			case "query":
				if msg.Query == "" {
					conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"missing query field"}`))
					continue
				}
				_, client, err := p.SelectClient()
				if err != nil {
					conn.WriteMessage(websocket.TextMessage, fmt.Appendf(nil, `{"error":"%v"}`, err))
					continue
				}
				resp, err := client.Query(context.Background(), msg.Query)
				if err != nil {
					conn.WriteMessage(websocket.TextMessage, fmt.Appendf(nil, `{"error":"%v"}`, err))
					continue
				}
				conn.WriteMessage(websocket.TextMessage, resp.Json)

			case "mutation":
				if msg.Mutation == "" {
					conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"missing mutation field"}`))
					continue
				}
				_, client, err := p.SelectClient()
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
				conn.WriteMessage(websocket.TextMessage, resp.Json)

			case "upsert":
				if msg.Query == "" || msg.Mutation == "" {
					conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"missing query or mutation field"}`))
					continue
				}
				_, client, err := p.SelectClient()
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
					conn.WriteMessage(websocket.TextMessage, []byte(`{""error":"%v"}`))
					continue
				}
				conn.WriteMessage(websocket.TextMessage, resp.Json)

			default:
				conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"unsupported type"}`))
			}
		}
	}
}
