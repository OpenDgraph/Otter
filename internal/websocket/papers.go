package websocket

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

const maxAuthAttempts = 8

// ValidatePapers runs a bounded auth challenge loop against the supplied
// expectedToken. It is kept for callers that still use the pre-main
// authentication flow; the live handler in HandleWebSocketWithProxy uses a
// per-message check driven by the same expected token.
func (m *WSMessage) ValidatePapers(conn *websocket.Conn, expectedToken string) bool {
	authAttempts := 0

	for authAttempts < maxAuthAttempts {
		if !ConstantTimeTokenEqual(m.Token, expectedToken) {
			authAttempts++
			conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"papers please!"}`))
			if authAttempts >= maxAuthAttempts {
				time.Sleep(3 * time.Second)
				conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "Go away! bye!"))
				conn.Close()
				return false
			}

			_, msgBytes, err := conn.ReadMessage()
			if err != nil {
				log.Printf("| Error reading auth retry: %v\n", err)
				return false
			}
			err = json.Unmarshal(msgBytes, m)
			if err != nil {
				conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"invalid JSON"}`))
				continue
			}
		} else {
			conn.WriteMessage(websocket.TextMessage, []byte(`{"status":"authenticated"}`))
			return true
		}
	}

	return false
}
