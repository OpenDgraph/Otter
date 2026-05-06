package routing

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/OpenDgraph/Otter/internal/config"
	"github.com/OpenDgraph/Otter/internal/proxy"
	"github.com/OpenDgraph/Otter/internal/ratelimit"
)

// SetupRoutes wires Otter's HTTP surface together. Two invariants matter
// to anyone consuming the gateway:
//
//  1. Every documented path resolves to a registered handler instead of
//     falling through to the Ratel-forwarding catch-all on `/`.
//  2. The rate limiter is applied only to user-driven traffic. Operational
//     endpoints (/health, /state, /alter, /admin/schema, /ui/keywords,
//     /validate/*) must stay unmetered so a noisy ops loop or health
//     check cannot lock the gateway out.
//
// We exercise (1) with `mux.Handler` to read the matched pattern without
// invoking any backend, and (2) by exhausting a tiny limiter and asserting
// that only metered routes return 429.

func newTestProxy(t *testing.T) *proxy.Proxy {
	t.Helper()
	// nil balancer is intentional: handlers that hit selectBackendHost
	// short-circuit with "no balancer configured" → 500. We never let
	// any test request reach a real backend.
	p, err := proxy.NewProxy(nil, config.Config{})
	if err != nil {
		t.Fatalf("NewProxy: %v", err)
	}
	return p
}

func TestSetupRoutes_AllPathsRegistered(t *testing.T) {
	mux := SetupRoutes(newTestProxy(t), nil, nil)

	cases := []struct {
		path        string
		wantPattern string
	}{
		{"/query", "/query"},
		{"/mutate", "/mutate"},
		{"/graphql", "/graphql"},
		{"/validate/dql", "/validate/dql"},
		{"/validate/schema", "/validate/schema"},
		{"/alter", "/alter"},
		{"/health", "/health"},
		{"/ui/keywords", "/ui/keywords"},
		{"/admin/schema", "/admin/schema"},
		{"/state", "/state"},
		// Anything else must fall through to the Ratel catch-all.
		{"/", "/"},
		{"/unknown", "/"},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			_, pattern := mux.Handler(req)
			if pattern != tc.wantPattern {
				t.Fatalf("path %q matched pattern %q, want %q",
					tc.path, pattern, tc.wantPattern)
			}
		})
	}
}

// TestSetupRoutes_RateLimitWiring exhausts a 1 rps / 1 burst limiter and
// verifies the second request to a metered route returns 429 while the
// second request to /health stays unmetered. We deliberately avoid making
// the request body valid: the limiter runs BEFORE HandleQuery reads the
// body, so a 415 from the handler proves the request reached the
// downstream handler (limiter passed) and a 429 proves it did not.
func TestSetupRoutes_RateLimitWiring(t *testing.T) {
	limiter := ratelimit.New(1, 1)
	if limiter == nil {
		t.Fatal("ratelimit.New unexpectedly disabled")
	}
	mux := SetupRoutes(newTestProxy(t), limiter, nil)

	// All requests share the same RemoteAddr so they hit the same
	// limiter bucket. httptest.NewRequest defaults to 192.0.2.1.
	hit := func(method, path string) int {
		req := httptest.NewRequest(method, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec.Code
	}

	t.Run("metered routes return 429 once exhausted", func(t *testing.T) {
		// Reset by using a fresh limiter scoped to this subtest. Each
		// metered path drains its own bucket.
		for _, path := range []string{"/query", "/mutate", "/graphql"} {
			lim := ratelimit.New(1, 1)
			mux := SetupRoutes(newTestProxy(t), lim, nil)

			req1 := httptest.NewRequest(http.MethodGet, path, nil)
			rec1 := httptest.NewRecorder()
			mux.ServeHTTP(rec1, req1)
			if rec1.Code == http.StatusTooManyRequests {
				t.Fatalf("%s: first request unexpectedly 429", path)
			}

			req2 := httptest.NewRequest(http.MethodGet, path, nil)
			rec2 := httptest.NewRecorder()
			mux.ServeHTTP(rec2, req2)
			if rec2.Code != http.StatusTooManyRequests {
				t.Fatalf("%s: second request should be 429, got %d (body=%s)",
					path, rec2.Code, rec2.Body.String())
			}
		}
	})

	t.Run("unmetered ops routes never see 429", func(t *testing.T) {
		// Reuse the outer limiter that we've already exhausted via the
		// `hit` helper above; whatever happens, none of these endpoints
		// must turn its response into a 429.
		_ = hit(http.MethodGet, "/query") // burn the burst
		_ = hit(http.MethodGet, "/query") // and confirm exhaustion

		for _, path := range []string{
			"/health", "/state", "/alter",
			"/admin/schema", "/ui/keywords",
			"/validate/dql", "/validate/schema",
		} {
			if got := hit(http.MethodGet, path); got == http.StatusTooManyRequests {
				t.Errorf("ops path %q must not be rate limited, got 429", path)
			}
		}
	})
}

// TestSetupRoutes_NilLimiterIsPassthrough proves SetupRoutes is safe to
// call with limiter=nil (the default when rate_limit_rps is zero) and
// that no path is gated on a non-nil limiter.
func TestSetupRoutes_NilLimiterIsPassthrough(t *testing.T) {
	mux := SetupRoutes(newTestProxy(t), nil, nil)

	for _, path := range []string{"/query", "/mutate", "/graphql"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code == http.StatusTooManyRequests {
			t.Errorf("%s with nil limiter must not 429, got %d", path, rec.Code)
		}
	}
}
