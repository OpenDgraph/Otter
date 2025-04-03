package main

import (
	"log"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	wsURL := "ws://localhost:8080/ws"

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		log.Fatalf("Failed to connect to WebSocket: %v", err)
	}
	defer conn.Close()

	for {
		msg := "Ping from client"
		err := conn.WriteMessage(websocket.TextMessage, []byte(msg))
		if err != nil {
			log.Printf("Failed to send message: %v", err)
			break
		}
		log.Printf("Sent: %s", msg)

		_, reply, err := conn.ReadMessage()
		if err != nil {
			log.Printf("Failed to read message: %v", err)
			break
		}
		log.Printf("Received: %s", string(reply))

		time.Sleep(2 * time.Second)
	}
}
