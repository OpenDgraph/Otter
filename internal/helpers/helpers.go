package helpers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"

	api "github.com/dgraph-io/dgo/v240/protos/api"
)

// responseBufPool amortises the bytes.Buffer used to assemble the JSON
// envelope ({"data":..., "extensions":...}) returned by WriteJSONResponse
// and WriteJSONResponseList. The buffer holds intermediate output; we
// flush it to the ResponseWriter at the end so the entire payload lands
// in a single write.
var responseBufPool = sync.Pool{
	New: func() any { return new(bytes.Buffer) },
}

// Typed extension envelopes mirror the legacy map[string]interface{}
// shape so existing clients see byte-identical output (modulo key order
// inside the upstream `data` block, which we now stream verbatim instead
// of round-tripping through map[string]interface{}). Using structs avoids
// per-request map allocations and the dynamic-typing tax of interface{}
// JSON encoding.

type latencyOut struct {
	ParsingNs         uint64 `json:"parsing_ns"`
	ProcessingNs      uint64 `json:"processing_ns"`
	EncodingNs        uint64 `json:"encoding_ns"`
	AssignTimestampNs uint64 `json:"assign_timestamp_ns"`
	TotalNs           uint64 `json:"total_ns"`
}

type txnOut struct {
	StartTs  uint64 `json:"start_ts"`
	CommitTs uint64 `json:"commit_ts,omitempty"`
}

type metricsOut struct {
	NumUids map[string]uint64 `json:"num_uids"`
}

type extensionsOut struct {
	ServerLatency *latencyOut         `json:"server_latency,omitempty"`
	Txn           *txnOut             `json:"txn,omitempty"`
	Metrics       *metricsOut         `json:"metrics,omitempty"`
	Uids          map[string]string   `json:"uids,omitempty"`
	Rdf           string              `json:"rdf,omitempty"`
	Headers       map[string][]string `json:"headers,omitempty"`
}

// buildExtensions translates the protobuf api.Response into the typed
// envelope. includeCommitTs preserves a small behavioural difference
// between the single-response and list paths: the single-response
// helper historically emitted only start_ts; the list helper emitted
// both start_ts and commit_ts.
func buildExtensions(resp *api.Response, includeCommitTs bool) extensionsOut {
	var ext extensionsOut
	if resp == nil {
		return ext
	}
	if resp.Latency != nil {
		ext.ServerLatency = &latencyOut{
			ParsingNs:         resp.Latency.GetParsingNs(),
			ProcessingNs:      resp.Latency.GetProcessingNs(),
			EncodingNs:        resp.Latency.GetEncodingNs(),
			AssignTimestampNs: resp.Latency.GetAssignTimestampNs(),
			TotalNs:           resp.Latency.GetTotalNs(),
		}
	}
	if resp.Txn != nil {
		t := &txnOut{StartTs: resp.Txn.GetStartTs()}
		if includeCommitTs {
			t.CommitTs = resp.Txn.GetCommitTs()
		}
		ext.Txn = t
	}
	if resp.Metrics != nil {
		ext.Metrics = &metricsOut{NumUids: resp.Metrics.GetNumUids()}
	}
	if len(resp.Uids) > 0 {
		ext.Uids = resp.Uids
	}
	if len(resp.Rdf) > 0 {
		ext.Rdf = string(resp.Rdf)
	}
	if len(resp.Hdrs) > 0 {
		hdrs := make(map[string][]string, len(resp.Hdrs))
		for key, val := range resp.Hdrs {
			hdrs[key] = val.GetValue()
		}
		ext.Headers = hdrs
	}
	return ext
}

const (
	ContentTypeJSON   = "application/json"
	ContentTypeDQL    = "application/dql"
	ContentTypeOldDQL = "application/graphql+-"
)

// maxBodyPrealloc bounds how much we are willing to preallocate based on
// a Content-Length hint. Larger bodies still read fine, they just start
// at this size and grow with append. The cap protects against a hostile
// Content-Length that would otherwise make us materialise a huge slice
// before the MaxBytesReader (configured upstream) gets a chance to
// reject the request.
const maxBodyPrealloc = 1 << 20 // 1 MiB

// ReadRequestBody reads the entire request body and closes it.
//
// The previous implementation delegated to io.ReadAll, which starts at
// 512 bytes and amortizes growth via append; for typical query bodies
// (a few KiB to a few hundred KiB) that meant 5-10 reallocations per
// request. We seed the destination slice with Content-Length when the
// header is present and within maxBodyPrealloc, dropping the typical
// hot-path body read down to a single allocation.
func ReadRequestBody(r *http.Request) ([]byte, error) {
	defer r.Body.Close()

	initial := 512
	if cl := r.ContentLength; cl > 0 && cl < int64(maxBodyPrealloc) {
		initial = int(cl) + 1 // +1 so the final read at EOF doesn't trigger a regrow
	}

	b := make([]byte, 0, initial)
	for {
		if len(b) == cap(b) {
			// Out of room: hand back to append to pick a growth factor.
			b = append(b, 0)[:len(b)]
		}
		n, err := r.Body.Read(b[len(b):cap(b)])
		b = b[:len(b)+n]
		if err != nil {
			if err == io.EOF {
				return b, nil
			}
			return nil, fmt.Errorf("failed to read request body: %w", err)
		}
	}
}

// MaxBodyLimit extracts the configured MaxBytesReader limit from an error
// returned by ReadRequestBody.
func MaxBodyLimit(err error) (int64, bool) {
	if err == nil {
		return 0, false
	}
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		return maxErr.Limit, true
	}
	return 0, false
}

// WriteRequestBodyReadError maps MaxBytesReader failures to HTTP 413 and
// falls back to the supplied bad-request message for every other read error.
func WriteRequestBodyReadError(w http.ResponseWriter, err error, fallback string) {
	if limit, ok := MaxBodyLimit(err); ok {
		WriteJSONError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("Request body exceeds max_body_bytes limit (%d bytes).", limit))
		return
	}
	WriteJSONError(w, http.StatusBadRequest, fallback)
}

func CheckQueryBody(contentType string, body []byte) (string, error) {
	switch contentType {
	case ContentTypeJSON:
		var data map[string]interface{}
		if err := json.Unmarshal(body, &data); err != nil {
			return "", fmt.Errorf("| Invalid JSON payload: %w", err)
		}
		query, ok := data["query"].(string)
		if !ok || query == "" {
			return "", fmt.Errorf("| Missing or empty 'query' field in JSON payload")
		}
		return query, nil

	case ContentTypeDQL, ContentTypeOldDQL:
		if len(body) == 0 {
			return "", fmt.Errorf("| Empty request body for %s or %s", ContentTypeDQL, ContentTypeOldDQL)
		}
		return string(body), nil

	default:
		return "", fmt.Errorf("| Unsupported Content-Type for query: %s", contentType)
	}
}

type UpsertBlock struct {
	Query    string `json:"query"`
	Mutation string `json:"mutation"`
	Cond     string `json:"cond,omitempty"`
}

// mutationPayload is the typed view of an Otter mutation request body.
// Fields are kept as json.RawMessage so we can:
//   - skip the cost of decoding `set` / `delete` into map[string]interface{}
//     and re-marshaling them back out for the gRPC api.Mutation;
//   - accept either a single object or an array under `upsert` without
//     a separate untyped pre-pass.
type mutationPayload struct {
	Upsert json.RawMessage `json:"upsert"`
	Set    json.RawMessage `json:"set"`
	Delete json.RawMessage `json:"delete"`
}

// firstNonSpace returns the first byte in raw that is not ASCII
// whitespace, or 0 if raw is empty/all-whitespace. Used to discriminate
// between an upsert object and an upsert array without invoking the
// JSON parser twice.
func firstNonSpace(raw []byte) byte {
	for i := 0; i < len(raw); i++ {
		switch raw[i] {
		case ' ', '\t', '\n', '\r':
			continue
		}
		return raw[i]
	}
	return 0
}

func CheckMutationBody(contentType string, body []byte) (*api.Mutation, []*UpsertBlock, error) {
	switch contentType {
	case ContentTypeDQL, ContentTypeOldDQL:
		if len(body) == 0 {
			return nil, nil, fmt.Errorf("| Empty request body for %s", contentType)
		}
		return &api.Mutation{SetNquads: body, CommitNow: true}, nil, nil

	case ContentTypeJSON:
		if len(body) == 0 {
			return nil, nil, fmt.Errorf("| Empty request body for %s", contentType)
		}

		// Single typed decode replaces the previous map[string]any + per-field
		// re-marshal cycle. RawMessage lets us pass `set` / `delete` through
		// to the gRPC mutation untouched, which is what the upstream Dgraph
		// expects anyway.
		var payload mutationPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, nil, fmt.Errorf("| Invalid JSON payload: %w", err)
		}

		if len(payload.Upsert) > 0 {
			switch firstNonSpace(payload.Upsert) {
			case '{':
				var blk UpsertBlock
				if err := json.Unmarshal(payload.Upsert, &blk); err != nil {
					return nil, nil, fmt.Errorf("| Invalid upsert block: %w", err)
				}
				if blk.Query == "" || blk.Mutation == "" {
					// Distinguish "missing fields" from "empty object" so the
					// existing tests for both shapes keep their messages.
					if blk.Query == "" && blk.Mutation == "" && blk.Cond == "" {
						return nil, nil, fmt.Errorf("| 'upsert' object is empty")
					}
					return nil, nil, fmt.Errorf("| upsert block missing 'query' or 'mutation'")
				}
				return nil, []*UpsertBlock{&blk}, nil
			case '[':
				var blocks []*UpsertBlock
				if err := json.Unmarshal(payload.Upsert, &blocks); err != nil {
					return nil, nil, fmt.Errorf("| Invalid upsert block in array: %w", err)
				}
				if len(blocks) == 0 {
					return nil, nil, fmt.Errorf("| 'upsert' array is empty")
				}
				for _, blk := range blocks {
					if blk == nil || blk.Query == "" || blk.Mutation == "" {
						return nil, nil, fmt.Errorf("| upsert block missing 'query' or 'mutation'")
					}
				}
				return nil, blocks, nil
			default:
				return nil, nil, fmt.Errorf("| Unsupported 'upsert' format")
			}
		}

		// Simple mutation: pass the RawMessage bytes straight through to
		// gRPC. The previous implementation re-Marshaled `set` / `delete`
		// from a decoded map[string]interface{}, which doubled allocations
		// for every mutation. The shared underlying array (`body`) outlives
		// `mut` because the handler holds `body` until its synchronous
		// upstream call returns, so no defensive copy is required.
		mut := &api.Mutation{CommitNow: true}
		if len(payload.Set) > 0 {
			mut.SetJson = payload.Set
		}
		if len(payload.Delete) > 0 {
			mut.DeleteJson = payload.Delete
		}

		if len(mut.SetJson) == 0 && len(mut.DeleteJson) == 0 {
			return nil, nil, fmt.Errorf("| No valid mutation fields found")
		}

		return mut, nil, nil
	default:
		return nil, nil, fmt.Errorf("| Unsupported Content-Type for mutation: %s", contentType)
	}
}

func WriteJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func WriteJSONQueryError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	response := map[string]interface{}{
		"data": map[string]interface{}{},
		"errors": []map[string]interface{}{
			{
				"message": msg,
			},
		},
		"extensions": map[string]interface{}{
			"server_latency": map[string]interface{}{},
			"txn":            map[string]interface{}{},
			"metrics": map[string]interface{}{
				"num_uids": map[string]interface{}{},
			},
		},
	}

	_ = json.NewEncoder(w).Encode(response)
}

// WriteJSONResponse writes the standard {"data":..., "extensions":...}
// envelope. The upstream `data` payload is streamed verbatim from
// resp.Json after a single json.Valid() check; we no longer Unmarshal
// into map[string]interface{} and re-Marshal, which was the largest
// allocator on the response path.
//
// Validation runs BEFORE committing the status header so a malformed
// JSON response surfaces as 500 instead of leaking through with the
// originally requested status.
func WriteJSONResponse(w http.ResponseWriter, status int, resp *api.Response) {
	var raw []byte
	if resp != nil {
		raw = resp.Json
	}
	if len(raw) > 0 && !json.Valid(raw) {
		WriteJSONError(w, http.StatusInternalServerError, "error parsing response JSON")
		return
	}

	buf := responseBufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer responseBufPool.Put(buf)

	buf.WriteString(`{"data":`)
	if len(raw) == 0 {
		buf.WriteString(`{}`)
	} else {
		buf.Write(raw)
	}
	buf.WriteString(`,"extensions":`)

	// Marshal a typed struct: with no optional fields populated this
	// emits `{}`, matching the legacy behaviour which the test suite
	// relies on.
	extBytes, err := json.Marshal(buildExtensions(resp, false))
	if err != nil {
		// Should be unreachable for our typed struct; fall back rather
		// than corrupt the response.
		WriteJSONError(w, http.StatusInternalServerError, "error encoding response extensions")
		return
	}
	buf.Write(extBytes)
	buf.WriteByte('}')

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)
}

// WriteJSONResponseList serializes multiple Dgraph responses (for example the
// result of a multi-block upsert) into a single JSON envelope. Nil entries
// become {"data":{},"extensions":{}} so callers can correlate by index.
//
// Like WriteJSONResponse, this version streams resp.Json verbatim into
// the output buffer instead of decoding into map[string]interface{}.
// For an N-block upsert that drops O(N) Unmarshal+Marshal round-trips.
func WriteJSONResponseList(w http.ResponseWriter, status int, resps []*api.Response) {
	buf := responseBufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer responseBufPool.Put(buf)

	buf.WriteString(`{"results":[`)

	for i, resp := range resps {
		if i > 0 {
			buf.WriteByte(',')
		}
		if resp == nil {
			buf.WriteString(`{"data":{},"extensions":{}}`)
			continue
		}

		raw := resp.Json
		if len(raw) > 0 && !json.Valid(raw) {
			// Per-item failure: preserve index alignment but flag the
			// offending entry. This matches the legacy contract.
			buf.WriteString(`{"error":"error parsing response JSON"}`)
			continue
		}

		buf.WriteString(`{"data":`)
		if len(raw) == 0 {
			buf.WriteString(`{}`)
		} else {
			buf.Write(raw)
		}
		buf.WriteString(`,"extensions":`)
		extBytes, err := json.Marshal(buildExtensions(resp, true))
		if err != nil {
			// Same fallback as WriteJSONResponse; should be unreachable.
			buf.WriteString(`{}`)
		} else {
			buf.Write(extBytes)
		}
		buf.WriteByte('}')
	}

	buf.WriteString(`]}`)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)
}
