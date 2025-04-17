package proxy

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/url"

	"github.com/OpenDgraph/Otter/internal/helpers"
)

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
	req2, err := http.NewRequest("POST", reqURL.String(), bytes.NewReader(body))
	if err != nil {
		helpers.WriteJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	req2.Header = r.Header.Clone()

	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		helpers.WriteJSONError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	defer resp2.Body.Close()

	reader := decompressIfGzip(resp2)
	raw, err := io.ReadAll(reader)
	if err != nil {
		helpers.WriteJSONError(w, http.StatusInternalServerError, "error reading GraphQL response")
		return
	}

	writeRawJSON(w, raw, resp2.StatusCode)
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
