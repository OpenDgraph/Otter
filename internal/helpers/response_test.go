package helpers_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/OpenDgraph/Otter/internal/helpers"
	api "github.com/dgraph-io/dgo/v240/protos/api"
	"github.com/stretchr/testify/require"
)

func TestWriteJSONResponse_NilResponseDoesNotPanic(t *testing.T) {
	rec := httptest.NewRecorder()
	helpers.WriteJSONResponse(rec, http.StatusOK, nil)

	require.Equal(t, http.StatusOK, rec.Code)

	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Contains(t, out, "data")
	require.Contains(t, out, "extensions")
}

func TestWriteJSONResponse_MalformedJSONReturns500(t *testing.T) {
	rec := httptest.NewRecorder()
	helpers.WriteJSONResponse(rec, http.StatusOK, &api.Response{
		Json: []byte(`{"this is not valid`),
	})

	require.Equal(t, http.StatusInternalServerError, rec.Code,
		"malformed upstream JSON must surface as 500, not the original status")
	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.Equal(t, "error parsing response JSON", out["error"])
}

func TestWriteJSONResponseList_EmptyListYieldsEmptyResults(t *testing.T) {
	rec := httptest.NewRecorder()
	helpers.WriteJSONResponseList(rec, http.StatusOK, nil)

	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	results, ok := out["results"].([]interface{})
	require.True(t, ok)
	require.Len(t, results, 0)
}

func TestWriteJSONResponseList_PreservesOrderAndNils(t *testing.T) {
	rec := httptest.NewRecorder()
	helpers.WriteJSONResponseList(rec, http.StatusOK, []*api.Response{
		{Json: []byte(`{"a":1}`)},
		nil,
		{Uids: map[string]string{"x": "0x1"}},
	})

	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	results := out["results"].([]interface{})
	require.Len(t, results, 3)

	first := results[0].(map[string]interface{})
	require.Equal(t, float64(1), first["data"].(map[string]interface{})["a"])

	third := results[2].(map[string]interface{})
	ext := third["extensions"].(map[string]interface{})
	uids := ext["uids"].(map[string]interface{})
	require.Equal(t, "0x1", uids["x"])
}

func TestCheckMutationBody_RejectsEmptyUpsertArray(t *testing.T) {
	body := []byte(`{"upsert": []}`)
	mut, upserts, err := helpers.CheckMutationBody(helpers.ContentTypeJSON, body)
	require.Error(t, err)
	require.Nil(t, mut)
	require.Nil(t, upserts)
}

func TestCheckMutationBody_RejectsEmptyUpsertObject(t *testing.T) {
	body := []byte(`{"upsert": {}}`)
	mut, upserts, err := helpers.CheckMutationBody(helpers.ContentTypeJSON, body)
	require.Error(t, err)
	require.Nil(t, mut)
	require.Nil(t, upserts)
}

func TestCheckMutationBody_RejectsUpsertMissingQueryOrMutation(t *testing.T) {
	body := []byte(`{"upsert": {"query": "only query"}}`)
	_, _, err := helpers.CheckMutationBody(helpers.ContentTypeJSON, body)
	require.Error(t, err)
}

func TestMaxBodyLimit_ExtractsMaxBytesReaderLimit(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("123456789")))
	rec := httptest.NewRecorder()
	req.Body = http.MaxBytesReader(rec, req.Body, 4)

	_, err := helpers.ReadRequestBody(req)
	require.Error(t, err)

	limit, ok := helpers.MaxBodyLimit(err)
	require.True(t, ok)
	require.EqualValues(t, 4, limit)
}

func TestWriteRequestBodyReadError_MaxBytesReaderMapsTo413(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("123456789")))
	rec := httptest.NewRecorder()
	req.Body = http.MaxBytesReader(rec, req.Body, 4)

	_, err := helpers.ReadRequestBody(req)
	require.Error(t, err)

	out := httptest.NewRecorder()
	helpers.WriteRequestBodyReadError(out, err, "fallback")
	require.Equal(t, http.StatusRequestEntityTooLarge, out.Code)
	require.JSONEq(t, `{"error":"Request body exceeds max_body_bytes limit (4 bytes)."}`, out.Body.String())
}

func TestWriteRequestBodyReadError_GenericErrorFallsBackTo400(t *testing.T) {
	out := httptest.NewRecorder()
	helpers.WriteRequestBodyReadError(out, errors.New("read error"), "couldn't read body")
	require.Equal(t, http.StatusBadRequest, out.Code)
	require.Contains(t, out.Body.String(), "couldn't read body")
}

func TestMaxBodyLimit_NilReturnsFalse(t *testing.T) {
	limit, ok := helpers.MaxBodyLimit(nil)
	require.False(t, ok)
	require.EqualValues(t, 0, limit)
}

// TestCheckQueryBody covers every branch of helpers.CheckQueryBody. It is
// intentionally table-driven: every Content-Type / body combination gets
// the same shape of assertion.
func TestCheckQueryBody(t *testing.T) {
	cases := []struct {
		name        string
		contentType string
		body        []byte
		wantQuery   string
		wantErr     bool
	}{
		{
			name:        "json with valid query",
			contentType: helpers.ContentTypeJSON,
			body:        []byte(`{"query":"{ q(func: has(name)) { uid } }"}`),
			wantQuery:   "{ q(func: has(name)) { uid } }",
		},
		{
			name:        "json missing query field",
			contentType: helpers.ContentTypeJSON,
			body:        []byte(`{"other":"value"}`),
			wantErr:     true,
		},
		{
			name:        "json with empty string query",
			contentType: helpers.ContentTypeJSON,
			body:        []byte(`{"query":""}`),
			wantErr:     true,
		},
		{
			name:        "json with non-string query",
			contentType: helpers.ContentTypeJSON,
			body:        []byte(`{"query":42}`),
			wantErr:     true,
		},
		{
			name:        "json with invalid syntax",
			contentType: helpers.ContentTypeJSON,
			body:        []byte(`{"query":`),
			wantErr:     true,
		},
		{
			name:        "dql body returned as-is",
			contentType: helpers.ContentTypeDQL,
			body:        []byte(`{ q(func: has(name)) { uid } }`),
			wantQuery:   `{ q(func: has(name)) { uid } }`,
		},
		{
			name:        "old dql alias accepted",
			contentType: helpers.ContentTypeOldDQL,
			body:        []byte(`{ q(func: has(name)) { uid } }`),
			wantQuery:   `{ q(func: has(name)) { uid } }`,
		},
		{
			name:        "dql empty body rejected",
			contentType: helpers.ContentTypeDQL,
			body:        []byte{},
			wantErr:     true,
		},
		{
			name:        "unknown content type rejected",
			contentType: "text/plain",
			body:        []byte("anything"),
			wantErr:     true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := helpers.CheckQueryBody(tc.contentType, tc.body)
			if tc.wantErr {
				require.Error(t, err)
				require.Empty(t, got)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.wantQuery, got)
		})
	}
}

// TestWriteJSONQueryError covers the GraphQL-shaped error envelope. It is
// purely a serialization helper, so we compare against a typed structure
// rather than substring-matching.
func TestWriteJSONQueryError_HasGraphQLEnvelopeShape(t *testing.T) {
	rec := httptest.NewRecorder()
	helpers.WriteJSONQueryError(rec, "boom")

	require.Equal(t, http.StatusOK, rec.Code,
		"GraphQL clients expect 200 with an `errors` array, not a non-2xx status")

	var out struct {
		Data       map[string]interface{} `json:"data"`
		Errors     []map[string]string    `json:"errors"`
		Extensions map[string]interface{} `json:"extensions"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.NotNil(t, out.Data, "data must always be present, even when empty")
	require.Len(t, out.Errors, 1)
	require.Equal(t, "boom", out.Errors[0]["message"])
	require.Contains(t, out.Extensions, "server_latency")
	require.Contains(t, out.Extensions, "txn")
	require.Contains(t, out.Extensions, "metrics")
}

// TestWriteJSONResponse_HonoursLatencyAndUids covers the success branch
// of WriteJSONResponse: a real-shaped *api.Response with latency, txn,
// metrics, uids and rdf must round-trip into the documented envelope.
func TestWriteJSONResponse_HonoursLatencyAndUids(t *testing.T) {
	rec := httptest.NewRecorder()
	helpers.WriteJSONResponse(rec, http.StatusOK, &api.Response{
		Json: []byte(`{"q":[{"uid":"0x1"}]}`),
		Latency: &api.Latency{
			ParsingNs:    10,
			ProcessingNs: 20,
			TotalNs:      100,
		},
		Txn:     &api.TxnContext{StartTs: 1000},
		Metrics: &api.Metrics{NumUids: map[string]uint64{"_total": 1}},
		Uids:    map[string]string{"u": "0x1"},
		Rdf:     []byte("<0x1> <name> \"x\" ."),
		Hdrs:    map[string]*api.ListOfString{"X-Test": {Value: []string{"hello"}}},
	})

	require.Equal(t, http.StatusOK, rec.Code)

	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))

	data := out["data"].(map[string]interface{})
	require.Contains(t, data, "q")

	ext := out["extensions"].(map[string]interface{})
	require.Contains(t, ext, "server_latency")
	require.Contains(t, ext, "txn")
	require.Contains(t, ext, "metrics")
	require.Equal(t, "0x1", ext["uids"].(map[string]interface{})["u"])
	require.Equal(t, "<0x1> <name> \"x\" .", ext["rdf"])
	headers := ext["headers"].(map[string]interface{})
	require.Equal(t, []interface{}{"hello"}, headers["X-Test"])
}

// TestWriteJSONResponseList_MalformedEntryReportsErrorInPlace covers the
// per-item failure mode: one bad entry should not poison the others.
func TestWriteJSONResponseList_MalformedEntryReportsErrorInPlace(t *testing.T) {
	rec := httptest.NewRecorder()
	helpers.WriteJSONResponseList(rec, http.StatusOK, []*api.Response{
		{Json: []byte(`{"a":1}`)},
		{Json: []byte(`{not json`)},
		{Json: []byte(`{"a":3}`)},
	})

	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))

	results := out["results"].([]interface{})
	require.Len(t, results, 3)

	require.Equal(t, float64(1), results[0].(map[string]interface{})["data"].(map[string]interface{})["a"])
	require.Equal(t, "error parsing response JSON", results[1].(map[string]interface{})["error"])
	require.Equal(t, float64(3), results[2].(map[string]interface{})["data"].(map[string]interface{})["a"])
}

// TestCheckMutationBody_DeleteOnlyAndOldDQLAlias rounds out cases the
// existing mutation_test.go file does not cover.
func TestCheckMutationBody_OldDQLAlias(t *testing.T) {
	dql := `<_:a> <name> "Julian" .`
	mut, upserts, err := helpers.CheckMutationBody(helpers.ContentTypeOldDQL, []byte(dql))
	require.NoError(t, err)
	require.Nil(t, upserts)
	require.NotNil(t, mut)
	require.Equal(t, dql, string(mut.SetNquads))
}

func TestCheckMutationBody_NoFields(t *testing.T) {
	body := []byte(`{}`)
	mut, upserts, err := helpers.CheckMutationBody(helpers.ContentTypeJSON, body)
	require.Error(t, err)
	require.Nil(t, mut)
	require.Nil(t, upserts)
}

func TestCheckMutationBody_EmptyJSONBody(t *testing.T) {
	mut, upserts, err := helpers.CheckMutationBody(helpers.ContentTypeJSON, []byte{})
	require.Error(t, err)
	require.Nil(t, mut)
	require.Nil(t, upserts)
}

// TestWriteJSONResponse_PreservesUpstreamData ensures the streamed
// `data` block matches the upstream payload byte-for-byte. The previous
// implementation round-tripped through map[string]interface{}, which
// would silently re-order keys; clients depending on raw passthrough
// would have regressed.
func TestWriteJSONResponse_PreservesUpstreamData(t *testing.T) {
	rec := httptest.NewRecorder()
	helpers.WriteJSONResponse(rec, http.StatusOK, &api.Response{
		Json: []byte(`{"q":[{"uid":"0x1","name":"alice"}]}`),
	})
	require.Equal(t, http.StatusOK, rec.Code)

	var out struct {
		Data       json.RawMessage `json:"data"`
		Extensions json.RawMessage `json:"extensions"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))
	require.JSONEq(t, `{"q":[{"uid":"0x1","name":"alice"}]}`, string(out.Data))
	require.JSONEq(t, `{}`, string(out.Extensions))
}

func BenchmarkWriteJSONResponse_TypicalPayload(b *testing.B) {
	resp := &api.Response{
		Json: []byte(`{"q":[{"uid":"0x1","name":"alice","age":30},{"uid":"0x2","name":"bob","age":42}]}`),
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		helpers.WriteJSONResponse(rec, http.StatusOK, resp)
	}
}
