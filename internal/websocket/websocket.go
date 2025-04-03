package websocket

import (
	"log"
	"net/http"

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

// HandleWebSocket handles a WebSocket connection, echoing all received messages.
func HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Upgrade the incoming HTTP connection to a WebSocket
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
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			// Log only unexpected errors (close 1000 is normal, 1001 is client going away, etc.)
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("| Unexpected close error: %v\n", err)
			} else {
				log.Printf("| Connection closed: %v\n", err)
			}
			return
		}

		log.Printf("| Received from %s: %s\n", conn.RemoteAddr(), message)

		// Echo the message back to the client
		if err := conn.WriteMessage(messageType, message); err != nil {
			log.Printf("| Failed to send message: %v\n", err)
			return
		}
	}
}
