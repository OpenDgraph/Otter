package helpers_test

import (
	"encoding/json"
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
