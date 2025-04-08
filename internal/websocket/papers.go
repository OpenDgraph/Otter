package websocket

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

const maxAuthAttempts = 8

func (m *WSMessage) ValidatePapers(conn *websocket.Conn) bool {
	authAttempts := 0

	for authAttempts < maxAuthAttempts {
		if !IsValidToken(m.Token) {
			authAttempts++
			conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"papers please!"}`))
			// time.Sleep(3 * time.Second)
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

func IsValidToken(token string) bool {
	return token == "banana"
}
