package proxy

import (
	"bytes"
	"compress/gzip"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/OpenDgraph/Otter/internal/helpers"
)

// graphqlUpstreamClient bounds the time Otter is willing to wait for the
// backend Dgraph /graphql endpoint. A hanging backend would otherwise pin
// a handler goroutine and a reverse-proxy connection until the OS timed
// out. The client reuses sharedTransport (defined in transport.go) so
// keep-alive connections are pooled across the cached ReverseProxy
// instances and this direct client path.
var graphqlUpstreamClient = &http.Client{
	Timeout:   30 * time.Second,
	Transport: sharedTransport,
}

func (p *Proxy) forwardGraphQL(body []byte, w http.ResponseWriter, r *http.Request) {
	const purpose = "query"

	backendHost, err := p.selectBackendHost(purpose, "http")
	if err != nil {
		status := http.StatusServiceUnavailable
		if err.Error() == "no balancer configured" {
			status = http.StatusInternalServerError
		}
		helpers.WriteJSONError(w, status, err.Error())
		return
	}

	reqURL := &url.URL{Scheme: "http", Host: backendHost, Path: "/graphql"}
	// Tie the upstream call to the inbound request lifetime so a client
	// disconnect cancels the backend query instead of letting it run to
	// completion on a request nobody is waiting for.
	req2, err := http.NewRequestWithContext(r.Context(), http.MethodPost, reqURL.String(), bytes.NewReader(body))
	if err != nil {
		helpers.WriteJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// Shallow-copy r.Header rather than Clone(): Clone() deep-copies
	// every value slice, which is wasted work because the upstream
	// http.Client does not mutate the header values. The map itself is
	// new so we don't share reference identity with the inbound request.
	req2.Header = make(http.Header, len(r.Header))
	for k, v := range r.Header {
		req2.Header[k] = v
	}

	resp2, err := graphqlUpstreamClient.Do(req2)
	if err != nil {
		helpers.WriteJSONError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	defer resp2.Body.Close()

	reader := decompressIfGzip(resp2)
	if rc, ok := reader.(io.Closer); ok && rc != resp2.Body {
		// The gzip.Reader wrapper needs its own Close to release its
		// internal state; resp2.Body.Close is still handled by the
		// outer defer.
		defer rc.Close()
	}

	// Commit headers + status first, then stream the (possibly
	// decompressed) body straight through. We previously buffered the
	// entire upstream response with io.ReadAll before writing a single
	// byte, which forced O(response_size) heap usage per request.
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(resp2.StatusCode)
	if _, err := io.Copy(w, reader); err != nil {
		// Status is already on the wire; we can't fall back to a JSON
		// error here, so just log and let the connection close.
		if p.verbose() {
			log.Printf("forwardGraphQL: copy upstream body: %v", err)
		}
	}
}

func decompressIfGzip(resp *http.Response) io.ReadCloser {
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return resp.Body
		}
		return reader
	}
	return resp.Body
}
