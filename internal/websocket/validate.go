package websocket

import (
	"errors"
	"fmt"

	"github.com/gorilla/websocket"
)

const (
	TypeQuery    = "query"
	TypeMutation = "mutation"
	TypeUpsert   = "upsert"
	TypeAuth     = "auth"
	TypeLogin    = "login"
	TypeLogout   = "logout"
	TypeState    = "state"
	TypePing     = "ping"
)

func (m *WSMessage) validate(conn *websocket.Conn) error {
	send := func(msg string) error {
		conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(`{"error":"%s"}`, msg)))
		return errors.New(msg)
	}

	switch m.Type {
	case "":
		return send("missing type field")
	case TypeAuth, TypeLogin:
		if m.Token == "" {
			return send("missing token field")
		}
	case TypeLogout, TypeState, TypePing:
		return nil
	case TypeQuery:
		if m.Query == "" {
			return send("missing query field")
		}
	case TypeMutation:
		if m.Mutation == "" {
			return send("missing mutation field")
		}
	case TypeUpsert:
		if m.Query == "" || m.Mutation == "" {
			return send("missing query or mutation field")
		}
	default:
		return send("unknown type field")
	}
	return nil
}
