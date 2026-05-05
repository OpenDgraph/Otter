package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"strings"
	"sync"
	"testing"
)

// TestReverseProxyFor_CachesByKey verifies that two calls with the same
// (host, fixedPath) return the same *ReverseProxy instance. This is the
// invariant that makes the cache cheaper than allocating a new proxy
// per request.
func TestReverseProxyFor_CachesByKey(t *testing.T) {
	a := reverseProxyFor("example.test:1234", "/graphql")
	b := reverseProxyFor("example.test:1234", "/graphql")
	if a != b {
		t.Fatalf("expected cached proxy to be reused; got distinct instances")
	}

	c := reverseProxyFor("example.test:1234", "")
	if a == c {
		t.Fatalf("different fixedPath must yield distinct proxies")
	}

	d := reverseProxyFor("other.test:1234", "/graphql")
	if a == d {
		t.Fatalf("different host must yield distinct proxies")
	}
}

// TestReverseProxyFor_FixedPathRewrite checks that fixedPath replaces
// the inbound request path before reaching the upstream, while an empty
// fixedPath preserves it. Regression coverage for HandleGraphQL (always
// /graphql) vs HandleDirect (path-preserving).
func TestReverseProxyFor_FixedPathRewrite(t *testing.T) {
	type capture struct {
		path string
		host string
	}

	gotCh := make(chan capture, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCh <- capture{path: r.URL.Path, host: r.Host}
		_, _ = io.WriteString(w, "ok")
	}))
	defer upstream.Close()

	host := strings.TrimPrefix(upstream.URL, "http://")

	t.Run("rewrites to fixedPath", func(t *testing.T) {
		rp := reverseProxyFor(host, "/graphql")
		req := httptest.NewRequest(http.MethodPost, "/anything", strings.NewReader(""))
		rec := httptest.NewRecorder()
		rp.ServeHTTP(rec, req)

		got := <-gotCh
		if got.path != "/graphql" {
			t.Fatalf("upstream path = %q, want /graphql", got.path)
		}
		if got.host != host {
			t.Fatalf("upstream Host header = %q, want %q", got.host, host)
		}
	})

	t.Run("preserves inbound path when fixedPath empty", func(t *testing.T) {
		rp := reverseProxyFor(host, "")
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()
		rp.ServeHTTP(rec, req)

		got := <-gotCh
		if got.path != "/health" {
			t.Fatalf("upstream path = %q, want /health", got.path)
		}
	})
}

// TestReverseProxyFor_ConcurrentLookup hammers the cache from many
// goroutines to exercise the double-checked locking. All calls for the
// same key must converge on a single *ReverseProxy.
func TestReverseProxyFor_ConcurrentLookup(t *testing.T) {
	const goroutines = 64
	results := make(chan *httputil.ReverseProxy, goroutines)

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			results <- reverseProxyFor("concurrent.test:9999", "/graphql")
		}()
	}
	wg.Wait()
	close(results)

	var first *httputil.ReverseProxy
	for rp := range results {
		if first == nil {
			first = rp
			continue
		}
		if rp != first {
			t.Fatalf("concurrent reverseProxyFor returned distinct instances")
		}
	}
}
