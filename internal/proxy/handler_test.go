package proxy

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/OpenDgraph/Otter/internal/config"
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

// TestHandleQuery_UnsupportedContentTypeReturns415 covers the
// CheckQueryBody error path. The handler must reject before reaching the
// balancer so the request is cheap to fail.
func TestHandleQuery_UnsupportedContentTypeReturns415(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/query",
		bytes.NewReader([]byte(`{"query":"{ a(func: has(b)) { c } }"}`)))
	req.Header.Set("Content-Type", "text/plain")

	rec := httptest.NewRecorder()
	(&Proxy{}).HandleQuery(rec, req)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnsupportedMediaType)
	}
	if !strings.Contains(rec.Body.String(), "Unsupported Content-Type") {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

// TestHandleMutation_InvalidJSONReturnsGraphQLEnvelope covers the
// pre-balancer rejection branch when CheckMutationBody can't parse the
// payload. The error envelope is GraphQL-shaped (200 OK + errors[]) by
// design — that's documented behaviour.
func TestHandleMutation_InvalidJSONReturnsGraphQLEnvelope(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/mutate",
		bytes.NewReader([]byte(`{not json`)))
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	(&Proxy{}).HandleMutation(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (GraphQL envelope)", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), `"errors"`) {
		t.Fatalf("response should embed `errors`: %s", rec.Body.String())
	}
}

// TestHandleDirect_NoBalancerReturns500 reaches the balancer step with
// neither balancer configured; the handler must surface that as 500
// rather than panicking or 503.
func TestHandleDirect_NoBalancerReturns500(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	(&Proxy{}).HandleDirect(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(rec.Body.String(), "no balancer configured") {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

// TestHandleDirect_OptionsReturnsCORSHeadersAndShortCircuits keeps the
// preflight contract honest: OPTIONS must not consult the balancer at
// all and must emit CORS headers per the configured allow-list.
func TestHandleDirect_OptionsReturnsCORSHeadersAndShortCircuits(t *testing.T) {
	dev := true
	p := &Proxy{configs: config.Config{
		DevMode:            &dev,
		CORSAllowedOrigins: []string{"https://app.example.com"},
	}}

	req := httptest.NewRequest(http.MethodOptions, "/health", nil)
	req.Header.Set("Origin", "https://app.example.com")

	rec := httptest.NewRecorder()
	p.HandleDirect(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("OPTIONS status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Fatalf("ACAO = %q, want exact-match origin", got)
	}
}

// TestHandleDirect_DisallowedPathReturns403 covers the allowedPaths
// check. selectBackendHost succeeds (we provide a fake balancer via the
// proxy's `balancer` field) so the next gate — the path allow-list — is
// the one we exercise.
func TestHandleDirect_DisallowedPathReturns403(t *testing.T) {
	p := &Proxy{
		balancer: stubBalancer{ep: "alpha:9081"},
	}

	req := httptest.NewRequest(http.MethodGet, "/something-arbitrary", nil)
	rec := httptest.NewRecorder()
	p.HandleDirect(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d (disallowed path)", rec.Code, http.StatusForbidden)
	}
}

// TestHandleGraphQL_NoBalancerReturns500 is the graphql-side parallel of
// TestHandleDirect_NoBalancerReturns500.
func TestHandleGraphQL_NoBalancerReturns500(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/graphql",
		bytes.NewReader([]byte(`{"query":"{me{name}}"}`)))
	rec := httptest.NewRecorder()
	(&Proxy{}).HandleGraphQL(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

// TestHandleFrontend_OptionsShortCircuits proves the Ratel proxy honours
// preflight OPTIONS without forwarding to the configured Ratel host.
// This protects test runs that don't have a Ratel up.
func TestHandleFrontend_OptionsShortCircuits(t *testing.T) {
	dev := true
	p := &Proxy{configs: config.Config{DevMode: &dev}}

	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "https://anywhere")
	rec := httptest.NewRecorder()
	p.HandleFrontend(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}
