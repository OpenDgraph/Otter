package websocket

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/OpenDgraph/Otter/internal/proxy"
	"github.com/dgraph-io/dgo/v240/protos/api"
	"github.com/gorilla/websocket"
)

// wsWriteTimeout caps how long Otter is willing to block on a single
// frame write before treating the peer as dead. Without this, a slow
// or stalled client could pin a goroutine and the connection's TCP
// send buffer indefinitely.
const wsWriteTimeout = 10 * time.Second

// wsBufPool reuses bytes.Buffers across WSResponse marshal calls so the
// per-message allocation footprint stays flat instead of scaling with
// payload size.
var wsBufPool = sync.Pool{
	New: func() any { return new(bytes.Buffer) },
}

// wsWriteText writes a text frame with a per-write deadline. Errors
// short-circuit the read loop because once a write fails the connection
// is unrecoverable.
func wsWriteText(conn *websocket.Conn, data []byte) error {
	if err := conn.SetWriteDeadline(time.Now().Add(wsWriteTimeout)); err != nil {
		return err
	}
	return conn.WriteMessage(websocket.TextMessage, data)
}

// wsWriteJSON marshals v into a pooled buffer and writes it as a text
// frame. We rely on json.NewEncoder's default HTML-escape behaviour
// (true), which matches the previous json.Marshal callers byte-for-byte.
func wsWriteJSON(conn *websocket.Conn, v any) error {
	buf := wsBufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer wsBufPool.Put(buf)

	if err := json.NewEncoder(buf).Encode(v); err != nil {
		return err
	}
	// json.Encoder appends a trailing newline; trim before writing the
	// frame so the wire format matches the previous json.Marshal path.
	out := buf.Bytes()
	if n := len(out); n > 0 && out[n-1] == '\n' {
		out = out[:n-1]
	}
	return wsWriteText(conn, out)
}

// newUpgrader builds a WebSocket upgrader whose CheckOrigin honours the
// supplied allow-list. When allowedOrigins is empty AND devMode is true, the
// upgrader accepts any origin and logs a single warning on startup; this is
// the explicit dev-only behaviour. When allowedOrigins is empty and devMode
// is false, every connection is rejected (validateSecurity should already
// have prevented this case, but we fail closed defensively).
func newUpgrader(allowedOrigins []string, devMode bool) websocket.Upgrader {
	return websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if len(allowedOrigins) == 0 {
				if devMode {
					return true
				}
				return false
			}
			if origin == "" {
				// Same-origin WS upgrades from non-browser clients commonly
				// omit the header. In strict mode we reject to keep the
				// decision boundary obvious.
				return false
			}
			return MatchOrigin(origin, allowedOrigins)
		},
	}
}

func checkAuth(authenticated bool, conn *websocket.Conn) bool {
	if !authenticated {
		_ = wsWriteText(conn, []byte(`{"error":"papers please!"}`))
		return false
	}
	return true
}

func HandleWebSocketWithProxy(p *proxy.Proxy) http.HandlerFunc {
	cfg := p.Config()
	devMode := cfg.DevMode != nil && *cfg.DevMode
	upgrader := newUpgrader(cfg.WSAllowedOrigins, devMode)

	expectedToken := cfg.WSToken
	tokenValid := func(t string) bool {
		return ConstantTimeTokenEqual(t, expectedToken)
	}

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
		applyReadLimit(conn, cfg.WSMaxMessageBytes)

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
				_ = wsWriteText(conn, fmt.Appendf(nil, `{"error":"invalid JSON: %v"}`, err))
				continue
			}

			// só valida estrutura
			if err := msg.validate(conn); err != nil {
				continue
			}

			switch msg.Type {
			case TypePing:
				_ = wsWriteText(conn, []byte(`{"status":"pong"}`))

			case TypeAuth:
				if tokenValid(msg.Token) {
					authenticated = true
					_ = wsWriteText(conn, []byte(`{"status":"authenticated"}`))
				} else {
					_ = wsWriteText(conn, []byte(`{"error":"invalid token"}`))
				}
				continue

			case TypeQuery:
				isAuthorized := checkAuth(authenticated, conn)
				if !isAuthorized {
					continue
				}

				_, client, err := p.SelectClientAuto("query")
				if err != nil {
					_ = wsWriteText(conn, fmt.Appendf(nil, `{"error":"%v"}`, err))
					continue
				}

				resp, err := client.Query(context.Background(), msg.Query)
				if err != nil {
					_ = wsWriteText(conn, fmt.Appendf(nil, `{"error":"%v"}`, err))
					continue
				}

				if msg.Verbose {
					_ = wsWriteJSON(conn, WSResponse{
						Data:      resp.Json,
						LatencyNs: resp.Latency.GetTotalNs(),
					})
				} else {
					// Resposta direta, só o JSON da query
					_ = wsWriteText(conn, resp.Json)
				}

			case TypeMutation:
				isAuthorized := checkAuth(authenticated, conn)
				if !isAuthorized {
					continue
				}
				_, client, err := p.SelectClientAuto("mutation")
				if err != nil {
					_ = wsWriteText(conn, fmt.Appendf(nil, `{"error":"%v"}`, err))
					continue
				}

				m := &api.Mutation{
					SetNquads: []byte(msg.Mutation),
					CommitNow: msg.CommitNow,
				}
				resp, err := client.Mutate(context.Background(), m)
				if err != nil {
					_ = wsWriteText(conn, fmt.Appendf(nil, `{"error":"%v"}`, err))
					continue
				}

				if msg.Verbose {
					_ = wsWriteJSON(conn, WSResponse{
						Data:      resp.Json,
						Uids:      resp.Uids,
						CommitTs:  resp.Txn.GetCommitTs(),
						Preds:     resp.Txn.GetPreds(),
						LatencyNs: resp.Latency.GetTotalNs(),
					})
				} else {
					data := resp.Json
					if len(data) == 0 {
						data = []byte(`{}`)
					}
					_ = wsWriteText(conn, data)
				}

			case TypeUpsert:
				isAuthorized := checkAuth(authenticated, conn)
				if !isAuthorized {
					continue
				}
				_, client, err := p.SelectClientAuto("upsert")
				if err != nil {
					_ = wsWriteJSON(conn, WSResponse{Error: err.Error()})
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
					_ = wsWriteJSON(conn, WSResponse{Error: err.Error()})
					continue
				}

				if err := wsWriteJSON(conn, WSResponse{
					Data:      resp.Json,
					Uids:      resp.Uids,
					CommitTs:  resp.Txn.GetCommitTs(),
					Preds:     resp.Txn.GetPreds(),
					LatencyNs: resp.Latency.GetTotalNs(),
				}); err != nil {
					_ = wsWriteText(conn, []byte(`{"error":"failed to encode response"}`))
					continue
				}

			default:
				_ = wsWriteText(conn, []byte(`{"error":"unsupported type"}`))
			}
		}
	}
}

func applyReadLimit(conn *websocket.Conn, limit int64) {
	if conn == nil || limit <= 0 {
		return
	}
	conn.SetReadLimit(limit)
}
