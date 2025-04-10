package ws

import (
	"encoding/json"
	"log"
	"math/rand"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestPersistentConnection(t *testing.T) {
	wsURL := "ws://localhost:8089/ws"

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		log.Fatalf("Failed to connect to WebSocket: %v", err)
	}
	defer conn.Close()
	log.Println("Connected to WebSocket")

	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				_, msg, err := conn.ReadMessage()
				if err != nil {
					log.Printf("Connection closed or error: %v", err)
					return
				}
				log.Printf("Server says: %s", string(msg))
			}
		}
	}()

	// Envia pings aleatórios
	for i := range 10 {
		delay := time.Duration(rand.Intn(5)+1) * time.Second
		time.Sleep(delay)

		ping := map[string]interface{}{
			"type": "ping",
		}
		payload, _ := json.Marshal(ping)
		log.Printf("Sending ping (%d)", i+1)
		conn.WriteMessage(websocket.TextMessage, payload)
	}

	log.Println("Finished sending pings. Waiting 5s before closing...")
	time.Sleep(5 * time.Second)
}
