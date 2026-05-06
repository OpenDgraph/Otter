package websocket

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gws "github.com/gorilla/websocket"
)

// validate() is the inner state machine deciding which WS messages are
// well-formed. It runs in lockstep with the connection: when validation
// fails it both writes an error frame on the wire AND returns a non-nil
// error to the caller. We exercise that contract end-to-end through a
// loopback websocket so we cover both halves at once.

func validateOverWS(t *testing.T, msg WSMessage) (errFromValidate string, framePayload string) {
	t.Helper()

	upgrader := gws.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	errCh := make(chan string, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()
		m := msg
		if err := m.validate(conn); err != nil {
			errCh <- err.Error()
		} else {
			errCh <- ""
		}
	}))
	defer srv.Close()

	url := "ws" + strings.TrimPrefix(srv.URL, "http")
	conn, _, err := gws.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// validate() may or may not send a frame. Use a deadline so the
	// success branch returns promptly when no frame is on the wire.
	conn.SetReadDeadline(timeAfterShort())
	_, payload, _ := conn.ReadMessage()
	return <-errCh, string(payload)
}

// timeAfterShort returns a deadline far enough out that a real send
// completes but short enough that a missing frame is detected as a
// timeout instead of hanging the suite.
func timeAfterShort() (deadline time.Time) {
	return time.Now().Add(150 * time.Millisecond)
}

func TestWSMessage_Validate(t *testing.T) {
	cases := []struct {
		name        string
		msg         WSMessage
		wantErr     bool
		wantErrText string
		wantFrame   string
	}{
		{
			name:    "auth without token",
			msg:     WSMessage{Type: TypeAuth},
			wantErr: true, wantErrText: "missing token field",
			wantFrame: "missing token field",
		},
		{
			name:    "auth with token passes",
			msg:     WSMessage{Type: TypeAuth, Token: "x"},
			wantErr: false,
		},
		{
			name:    "login without token",
			msg:     WSMessage{Type: TypeLogin},
			wantErr: true, wantErrText: "missing token field",
		},
		{
			name:    "missing type",
			msg:     WSMessage{},
			wantErr: true, wantErrText: "missing type field",
		},
		{
			name:    "unknown type",
			msg:     WSMessage{Type: "wat"},
			wantErr: true, wantErrText: "unknown type field",
		},
		{
			name:    "ping passes",
			msg:     WSMessage{Type: TypePing},
			wantErr: false,
		},
		{
			name:    "logout passes",
			msg:     WSMessage{Type: TypeLogout},
			wantErr: false,
		},
		{
			name:    "state passes",
			msg:     WSMessage{Type: TypeState},
			wantErr: false,
		},
		{
			name:    "query without query",
			msg:     WSMessage{Type: TypeQuery},
			wantErr: true, wantErrText: "missing query field",
		},
		{
			name:    "query with query passes",
			msg:     WSMessage{Type: TypeQuery, Query: "{ a }"},
			wantErr: false,
		},
		{
			name:    "mutation without mutation",
			msg:     WSMessage{Type: TypeMutation},
			wantErr: true, wantErrText: "missing mutation field",
		},
		{
			name:    "upsert needs query and mutation",
			msg:     WSMessage{Type: TypeUpsert, Query: "{ a }"},
			wantErr: true, wantErrText: "missing query or mutation field",
		},
		{
			name:    "upsert with both passes",
			msg:     WSMessage{Type: TypeUpsert, Query: "{ a }", Mutation: "uid(a) <b> \"c\" ."},
			wantErr: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotErr, gotFrame := validateOverWS(t, tc.msg)
			if tc.wantErr {
				if gotErr != tc.wantErrText {
					t.Errorf("err = %q, want %q", gotErr, tc.wantErrText)
				}
				if tc.wantFrame != "" && !strings.Contains(gotFrame, tc.wantFrame) {
					t.Errorf("frame = %q, want substring %q", gotFrame, tc.wantFrame)
				}
				return
			}
			if gotErr != "" {
				t.Errorf("unexpected validate error: %q", gotErr)
			}
		})
	}
}

// TestNewUpgrader_CheckOrigin pins the three documented decision branches
// of CheckOrigin: empty allow-list + dev = accept, empty + non-dev =
// reject, populated = match against the list. We invoke the closure
// directly because Upgrader.CheckOrigin is the contract we care about.
func TestNewUpgrader_CheckOrigin(t *testing.T) {
	cases := []struct {
		name           string
		allowedOrigins []string
		devMode        bool
		origin         string
		want           bool
	}{
		{"empty + dev accepts any", nil, true, "https://anywhere", true},
		{"empty + dev accepts missing origin", nil, true, "", true},
		{"empty + prod rejects any", nil, false, "https://anywhere", false},
		{"populated rejects missing origin", []string{"*"}, false, "", false},
		{"populated wildcard accepts", []string{"*"}, false, "https://anywhere", true},
		{"populated exact match", []string{"https://app.example.com"}, false, "https://app.example.com", true},
		{"populated unmatched rejects", []string{"https://app.example.com"}, false, "https://attacker.example", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			up := newUpgrader(tc.allowedOrigins, tc.devMode)
			req := httptest.NewRequest(http.MethodGet, "/ws", nil)
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			if got := up.CheckOrigin(req); got != tc.want {
				t.Fatalf("CheckOrigin = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestWSResponse_OmitsZeroFields keeps the JSON envelope honest: every
// optional field uses omitempty so clients can rely on shape stability.
func TestWSResponse_OmitsZeroFields(t *testing.T) {
	out, err := json.Marshal(WSResponse{})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(out) != "{}" {
		t.Fatalf("zero WSResponse should serialise to {}; got %s", out)
	}

	out, err = json.Marshal(WSResponse{Error: "boom"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"error":"boom"`) {
		t.Fatalf("error should be present, got %s", out)
	}
}
