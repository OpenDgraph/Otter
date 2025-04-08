package ws

import (
	"encoding/json"
	"log"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func containsError(b []byte) bool {
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		// Se nem deu pra fazer unmarshal, já é erro
		return true
	}
	if errVal, ok := m["error"]; ok {
		if errStr, isStr := errVal.(string); isStr && errStr != "" {
			return true
		}
	}
	return false
}

func TestWebSocket(t *testing.T) {
	wsURL := "ws://localhost:8081/ws"

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		log.Fatalf("Failed to connect to WebSocket: %v", err)
	}
	defer conn.Close()

	type WSMessage struct {
		Type      string `json:"type"`
		Token     string `json:"token,omitempty"`
		Query     string `json:"query,omitempty"`
		Mutation  string `json:"mutation,omitempty"`
		Cond      string `json:"cond,omitempty"`
		CommitNow bool   `json:"commitNow,omitempty"`
		Verbose   bool   `json:"verbose,omitempty"`
	}

	// 1. Manda mensagem inválida (sem token)
	// msg := WSMessage{
	// 	Type:      "mutation",
	// 	Mutation:  `<0x1> <name> "Alice Updated" .`,
	// 	CommitNow: true,
	// 	Verbose:   true,
	// }

	// payload, _ := json.Marshal(msg)
	// conn.WriteMessage(websocket.TextMessage, payload)

	// 2. Recebe "papers please!" algumas vezes
	// go func() {
	// 	for {
	// 		_, reply, err := conn.ReadMessage()
	// 		if err != nil {
	// 			log.Printf("Connection closed or error: %v", err)
	// 			return
	// 		}
	// 		log.Printf("Server says: %s", string(reply))
	// 	}
	// }()

	// 3. Aguarda 2 segundos
	time.Sleep(2 * time.Second)

	// 4. Agora manda o "auth"
	auth := WSMessage{
		Type:  "auth",
		Token: "banana",
	}
	payload, _ := json.Marshal(auth)
	conn.WriteMessage(websocket.TextMessage, payload)

	// 5. Aguarda confirmação de autenticação
	_, reply, _ := conn.ReadMessage()
	log.Printf("Auth response: %s", string(reply))

	// 6. Agora faz a mutation de verdade
	msg := WSMessage{
		Type:      "mutation",
		Mutation:  `<0x1> <name> "Alice Updated" .`,
		CommitNow: true,
		Verbose:   true,
	}
	payload, _ = json.Marshal(msg)
	conn.WriteMessage(websocket.TextMessage, payload)

	_, reply, _ = conn.ReadMessage()
	log.Printf("Mutation response: %s", string(reply))
	if containsError(reply) {
		t.Errorf("Mutation failed with error: %s", string(reply))
	}

	// 7. Faz a query
	msg = WSMessage{
		Type:  "query",
		Query: `{ data(func: has(email)) { uid name email } }`,
	}
	payload, _ = json.Marshal(msg)
	conn.WriteMessage(websocket.TextMessage, payload)

	_, reply, _ = conn.ReadMessage()
	log.Printf("Query response: %s", string(reply))
	if containsError(reply) {
		t.Errorf("Query failed with error: %s", string(reply))
	}
}
