package proxy

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/OpenDgraph/Otter/internal/config"
)

// newGraphQLProxy stitches a Proxy with a stub balancer pointing at an
// httptest upstream. We pin the gRPC→HTTP mapping explicitly via
// DgraphHTTPEndpoints so resolveHTTPEndpoint stays away from the
// arithmetic fallback (which would not match the test server's port).
func newGraphQLProxy(upstream *httptest.Server) *Proxy {
	upstreamHost := strings.TrimPrefix(upstream.URL, "http://")
	dev := true
	return &Proxy{
		balancer: stubBalancer{ep: "fake:9999"},
		configs: config.Config{
			DevMode: &dev,
			DgraphHTTPEndpoints: map[string]string{
				"fake:9999": upstreamHost,
			},
		},
	}
}

// TestForwardGraphQL_ProxiesBodyAndStatus exercises the happy path:
// inbound headers reach the upstream, response status + body are
// streamed back. The Content-Type override is documented behaviour
// (Otter always emits application/json regardless of upstream).
func TestForwardGraphQL_ProxiesBodyAndStatus(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("upstream method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/graphql" {
			t.Errorf("upstream path = %s, want /graphql", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"query":"{ ok }"}` {
			t.Errorf("upstream body = %q", body)
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"data":{"ok":1}}`))
	}))
	defer upstream.Close()

	p := newGraphQLProxy(upstream)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader([]byte(`{"query":"{ ok }"}`)))
	p.forwardGraphQL([]byte(`{"query":"{ ok }"}`), rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}
	if got := rec.Body.String(); got != `{"data":{"ok":1}}` {
		t.Fatalf("body = %q", got)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("Content-Type = %q", ct)
	}
}

// TestForwardGraphQL_GzipDecodes confirms decompressIfGzip is on the
// streaming path: an upstream that emits gzip with the right header
// must reach the client as plain JSON.
func TestForwardGraphQL_GzipDecodes(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		_, _ = gz.Write([]byte(`{"compressed":true}`))
		_ = gz.Close()
	}))
	defer upstream.Close()

	p := newGraphQLProxy(upstream)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader([]byte(`{}`)))
	p.forwardGraphQL([]byte(`{}`), rec, req)

	if got := rec.Body.String(); got != `{"compressed":true}` {
		t.Fatalf("expected gzip to be decoded; got %q", got)
	}
}

// TestForwardGraphQL_GzipHeaderWithoutGzipBodyDoesNotPanic is a defensive
// case. decompressIfGzip falls back to the raw body when gzip.NewReader
// fails; we want to confirm the handler simply streams whatever bytes
// are there instead of failing the request.
func TestForwardGraphQL_GzipHeaderWithoutGzipBodyDoesNotPanic(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		_, _ = w.Write([]byte(`not gzip`))
	}))
	defer upstream.Close()

	p := newGraphQLProxy(upstream)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader([]byte(`{}`)))

	// Just assert the handler returns; the body content depends on the
	// upstream and the gzip fallback path. The important property is
	// "no panic, no infinite loop".
	p.forwardGraphQL([]byte(`{}`), rec, req)
}

// TestForwardGraphQL_NoBalancerReturns500 covers the early-exit branch
// when selectBackendHost cannot pick an endpoint.
func TestForwardGraphQL_NoBalancerReturns500(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewReader([]byte(`{}`)))
	(&Proxy{}).forwardGraphQL([]byte(`{}`), rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

// TestDecompressIfGzip covers the helper directly: a gzip body returns
// a wrapper that yields the decompressed content; a non-gzip body is
// returned as-is.
func TestDecompressIfGzip(t *testing.T) {
	t.Run("gzip body", func(t *testing.T) {
		var buf bytes.Buffer
		gz := gzip.NewWriter(&buf)
		_, _ = gz.Write([]byte("hello"))
		_ = gz.Close()

		resp := &http.Response{
			Header: http.Header{"Content-Encoding": []string{"gzip"}},
			Body:   io.NopCloser(&buf),
		}
		out, err := io.ReadAll(decompressIfGzip(resp))
		if err != nil {
			t.Fatalf("read decompressed: %v", err)
		}
		if string(out) != "hello" {
			t.Fatalf("decompressed = %q", out)
		}
	})

	t.Run("non-gzip body passthrough", func(t *testing.T) {
		resp := &http.Response{
			Header: http.Header{},
			Body:   io.NopCloser(bytes.NewReader([]byte("plain"))),
		}
		out, err := io.ReadAll(decompressIfGzip(resp))
		if err != nil {
			t.Fatal(err)
		}
		if string(out) != "plain" {
			t.Fatalf("body = %q", out)
		}
	})

	t.Run("gzip header but invalid body falls back to raw", func(t *testing.T) {
		resp := &http.Response{
			Header: http.Header{"Content-Encoding": []string{"gzip"}},
			Body:   io.NopCloser(bytes.NewReader([]byte("not gzip"))),
		}
		// gzip.NewReader fails, decompressIfGzip returns resp.Body.
		// We only assert it returns *something* readable.
		_, _ = io.ReadAll(decompressIfGzip(resp))
	})
}

// TestGraphQLAllowed_AndVerbose covers the two tiny config-flag readers
// in helper.go that the rest of the suite never touches directly.
func TestGraphQLAllowed_AndVerbose(t *testing.T) {
	cases := []struct {
		name        string
		graphQL     *bool
		devMode     *bool
		wantAllowed bool
		wantVerbose bool
	}{
		{"both nil", nil, nil, false, false},
		{"graphQL false", boolPtr(false), boolPtr(false), false, false},
		{"graphQL true, verbose true", boolPtr(true), boolPtr(true), true, true},
		{"graphQL true, verbose false", boolPtr(true), boolPtr(false), true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := &Proxy{configs: config.Config{
				GraphQL: tc.graphQL,
				DevMode: tc.devMode,
			}}
			if got := p.graphQLAllowed(); got != tc.wantAllowed {
				t.Errorf("graphQLAllowed = %v, want %v", got, tc.wantAllowed)
			}
			if got := p.verbose(); got != tc.wantVerbose {
				t.Errorf("verbose = %v, want %v", got, tc.wantVerbose)
			}
		})
	}
}

func boolPtr(b bool) *bool { return &b }
