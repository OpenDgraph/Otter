package websocket

import (
	"errors"

	"github.com/gorilla/websocket"
)

const (
	TypeQuery    = "query"
	TypeMutation = "mutation"
	TypeUpsert   = "upsert"
)

func (m *WSMessage) validate(conn *websocket.Conn) error {
	switch m.Type {
	case TypeQuery:
		if m.Query == "" {
			conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"missing query field"}`))
			return errors.New("missing query field")
		}
	case TypeMutation:
		if m.Mutation == "" {
			conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"missing mutation field"}`))
			return errors.New("missing mutation field")
		}
	case TypeUpsert:
		if m.Query == "" || m.Mutation == "" {
			conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"missing query or mutation field"}`))
			return errors.New("missing upsert fields")
		}
	case "":
		return errors.New("missing type field")
	default:
		return errors.New("unknown type field")

	}
	return nil
}
