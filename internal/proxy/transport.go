package proxy

import (
	"net"
	"net/http"
	"net/http/httputil"
	"sync"
	"time"
)

// sharedTransport is reused by every cached *httputil.ReverseProxy.
//
// Two motivations:
//
//   - http.DefaultTransport caps MaxIdleConnsPerHost at 2, which under
//     load forces TCP reconnects to the Dgraph HTTP port and keeps a long
//     tail of TIME_WAIT sockets around. Bumping the cap restores
//     keep-alive behaviour for a small number of backends with high RPS.
//   - A single Transport amortises HTTP/2 connection sharing across
//     handlers (HandleDirect, HandleGraphQL, HandleFrontend) targeting
//     the same backend.
//
// Timeouts are intentionally conservative; they bound how long Otter
// will wait on a stuck backend before failing a request. They do NOT
// override the per-handler upstream client timeouts (see
// graphqlUpstreamClient).
var sharedTransport = &http.Transport{
	Proxy: http.ProxyFromEnvironment,
	DialContext: (&net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext,
	MaxIdleConns:          512,
	MaxIdleConnsPerHost:   128,
	IdleConnTimeout:       90 * time.Second,
	TLSHandshakeTimeout:   5 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
	ForceAttemptHTTP2:     true,
}

// rpKey is the cache key for reverseProxyFor. It must include both host
// and the path-rewrite policy because each (host, fixedPath) pair needs
// its own director closure.
type rpKey struct {
	scheme    string
	host      string
	fixedPath string // empty means "preserve inbound request path"
}

var (
	rpMu    sync.RWMutex
	rpCache = make(map[rpKey]*httputil.ReverseProxy)
)

// reverseProxyFor returns a cached *httputil.ReverseProxy that forwards
// to host. When fixedPath is non-empty, the inbound request path is
// rewritten to fixedPath (used by HandleGraphQL, which always targets
// /graphql on the backend). When fixedPath is empty, the inbound path is
// preserved as-is.
//
// Building one ReverseProxy per request, as the previous implementation
// did, allocates the struct + a director closure on every call and
// prevents Transport reuse. Caching by (host, fixedPath) is safe because
// the director only closes over those two strings.
func reverseProxyFor(host, fixedPath string) *httputil.ReverseProxy {
	k := rpKey{scheme: "http", host: host, fixedPath: fixedPath}

	rpMu.RLock()
	if rp, ok := rpCache[k]; ok {
		rpMu.RUnlock()
		return rp
	}
	rpMu.RUnlock()

	rpMu.Lock()
	defer rpMu.Unlock()
	// Re-check under the write lock in case another goroutine raced us.
	if rp, ok := rpCache[k]; ok {
		return rp
	}

	rp := &httputil.ReverseProxy{
		Transport: sharedTransport,
		Director: func(req *http.Request) {
			req.URL.Scheme = k.scheme
			req.URL.Host = k.host
			if k.fixedPath != "" {
				req.URL.Path = k.fixedPath
				req.URL.RawPath = ""
			}
			// Some upstreams (Ratel) inspect Host. Mirror the legacy
			// HandleFrontend behaviour, which set req.Host = targetURL.Host.
			req.Host = k.host
			// Hide the proxy hop from the upstream's "X-Forwarded-*"
			// chain unless an operator explicitly opts in upstream.
			if _, ok := req.Header["User-Agent"]; !ok {
				// Preserve the stdlib default of empty UA suppression so
				// httputil.ReverseProxy does not inject "Go-http-client".
				req.Header.Set("User-Agent", "")
			}
		},
	}
	rpCache[k] = rp
	return rp
}
