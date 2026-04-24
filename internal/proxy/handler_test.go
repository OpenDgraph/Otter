package proxy

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleQuery_BodyTooLargeReturns413(t *testing.T) {
	body := bytes.NewReader([]byte(`{"query":"{ q(func: has(name)) { uid } }"}`))
	req := httptest.NewRequest(http.MethodPost, "/query", body)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	req.Body = http.MaxBytesReader(rec, req.Body, 8)

	out := httptest.NewRecorder()
	(&Proxy{}).HandleQuery(out, req)

	if out.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("HandleQuery status = %d, want %d", out.Code, http.StatusRequestEntityTooLarge)
	}
	if !strings.Contains(out.Body.String(), "max_body_bytes limit (8 bytes)") {
		t.Fatalf("expected 413 body to mention configured limit, got %s", out.Body.String())
	}
}

func TestHandleMutation_BodyTooLargeReturns413(t *testing.T) {
	body := bytes.NewReader([]byte(`{"set":{"name":"alice"}}`))
	req := httptest.NewRequest(http.MethodPost, "/mutate", body)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	req.Body = http.MaxBytesReader(rec, req.Body, 8)

	out := httptest.NewRecorder()
	(&Proxy{}).HandleMutation(out, req)

	if out.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("HandleMutation status = %d, want %d", out.Code, http.StatusRequestEntityTooLarge)
	}
	if !strings.Contains(out.Body.String(), "max_body_bytes limit (8 bytes)") {
		t.Fatalf("expected 413 body to mention configured limit, got %s", out.Body.String())
	}
}
