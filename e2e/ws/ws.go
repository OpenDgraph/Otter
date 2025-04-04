package main

import (
	"encoding/json"
	"log"

	"github.com/gorilla/websocket"
)

func main() {
	wsURL := "ws://localhost:8081/ws"

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		log.Fatalf("Failed to connect to WebSocket: %v", err)
	}
	defer conn.Close()

	type WSMessage struct {
		Type      string `json:"type"`
		Query     string `json:"query,omitempty"`
		Mutation  string `json:"mutation,omitempty"`
		Cond      string `json:"cond,omitempty"`
		CommitNow bool   `json:"commitNow,omitempty"`
		Verbose   bool   `json:"verbose,omitempty"`
	}

	msg := WSMessage{
		Type:      "mutation",
		Mutation:  `<0x1> <name> "Alice Updated" .`,
		CommitNow: true,
		Verbose:   true,
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		log.Fatalf("Failed to encode mutation message: %v", err)
	}

	err = conn.WriteMessage(websocket.TextMessage, payload)
	if err != nil {
		log.Fatalf("Failed to send mutation: %v", err)
	}

	_, reply, err := conn.ReadMessage()
	if err != nil {
		log.Fatalf("Failed to read mutation reply: %v", err)
	}

	log.Printf("Mutation response: %s", string(reply))

	msg = WSMessage{
		Type:  "query",
		Query: `{ data(func: has(email)) { uid name email } }`,
	}

	payload, err = json.Marshal(msg)
	if err != nil {
		log.Fatalf("Failed to encode message: %v", err)
	}

	err = conn.WriteMessage(websocket.TextMessage, payload)
	if err != nil {
		log.Fatalf("Failed to send message: %v", err)
	}

	_, reply, err = conn.ReadMessage()
	if err != nil {
		log.Fatalf("Failed to read reply: %v", err)
	}

	log.Printf("Received: %s", string(reply))

	msg = WSMessage{
		Type: "upsert",
		Query: `
			query {
				user as var(func: eq(email, "test2@example.com"))
			}
		`,
		Mutation: `uid(user) <email> "tes2t@example.com" .`,
		// Cond:      "@if(eq(len(user), 1))",
		CommitNow: true,
	}

	payload, err = json.Marshal(msg)
	if err != nil {
		log.Fatalf("Failed to encode message: %v", err)
	}

	err = conn.WriteMessage(websocket.TextMessage, payload)
	if err != nil {
		log.Fatalf("Failed to send message: %v", err)
	}

	_, reply, err = conn.ReadMessage()
	if err != nil {
		log.Fatalf("Failed to read reply: %v", err)
	}

	log.Printf("Received: %s", string(reply))

}
