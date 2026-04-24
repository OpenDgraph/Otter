package websocket

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	gws "github.com/gorilla/websocket"
)

func TestApplyReadLimit_RejectsOversizedMessages(t *testing.T) {
	upgrader := gws.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade failed: %v", err)
		}
		defer conn.Close()

		applyReadLimit(conn, 8)
		if _, _, err := conn.ReadMessage(); err == nil {
			t.Fatal("expected oversized message to fail")
		}
	}))
	defer srv.Close()

	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := gws.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteMessage(gws.TextMessage, []byte("0123456789")); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Fatal("expected server to close oversized message connection")
	}
}

func TestApplyReadLimit_NoLimitDoesNothing(t *testing.T) {
	upgrader := gws.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade failed: %v", err)
		}
		defer conn.Close()

		applyReadLimit(conn, 0)
		mt, msg, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("unexpected read error: %v", err)
		}
		if err := conn.WriteMessage(mt, msg); err != nil {
			t.Fatalf("echo failed: %v", err)
		}
	}))
	defer srv.Close()

	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := gws.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	payload := []byte(strings.Repeat("x", 32))
	if err := conn.WriteMessage(gws.TextMessage, payload); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("unexpected read error: %v", err)
	}
	if string(msg) != string(payload) {
		t.Fatalf("echo mismatch: got %q want %q", msg, payload)
	}
}
