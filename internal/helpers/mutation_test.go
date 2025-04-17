package helpers_test

import (
	"testing"

	"github.com/OpenDgraph/Otter/internal/helpers"
	"github.com/stretchr/testify/require"
)

func TestSimpleSetObject(t *testing.T) {
	body := []byte(`{"set": {"name": "Julian"}}`)
	mut, upserts, err := helpers.CheckMutationBody(helpers.ContentTypeJSON, body)
	require.NoError(t, err)
	require.NotNil(t, mut)
	require.Nil(t, upserts)
	require.Contains(t, string(mut.SetJson), "Julian")
	require.True(t, mut.CommitNow)
}

func TestSimpleSetArray(t *testing.T) {
	body := []byte(`{"set": [{"name": "Julian"}, {"name": "Jay"}]}`)
	mut, upserts, err := helpers.CheckMutationBody(helpers.ContentTypeJSON, body)
	require.NoError(t, err)
	require.NotNil(t, mut)
	require.Nil(t, upserts)
	require.Contains(t, string(mut.SetJson), "Jay")
	require.True(t, mut.CommitNow)
}

func TestDeleteArray(t *testing.T) {
	body := []byte(`{"delete": [{"uid": "0x123", "name": null}]}`)
	mut, upserts, err := helpers.CheckMutationBody(helpers.ContentTypeJSON, body)
	require.NoError(t, err)
	require.NotNil(t, mut)
	require.Nil(t, upserts)
	require.NotEmpty(t, mut.DeleteJson)
	require.True(t, mut.CommitNow)
}

func TestMixedSetDelete(t *testing.T) {
	body := []byte(`{
		"set": [{"name": "Julian"}],
		"delete": [{"uid": "0x123"}]
	}`)
	mut, upserts, err := helpers.CheckMutationBody(helpers.ContentTypeJSON, body)
	require.NoError(t, err)
	require.NotNil(t, mut)
	require.Nil(t, upserts)
	require.NotEmpty(t, mut.SetJson)
	require.NotEmpty(t, mut.DeleteJson)
}

func TestUpsertSingle(t *testing.T) {
	body := []byte(`{
		"upsert": {
			"query": "query { u as var(func: eq(email, \"a@a.com\")) }",
			"mutation": "uid(u) <name> \"Test\" .",
			"cond": "@if(eq(len(u), 1))"
		}
	}`)
	mut, upserts, err := helpers.CheckMutationBody(helpers.ContentTypeJSON, body)
	require.NoError(t, err)
	require.Nil(t, mut)
	require.Len(t, upserts, 1)
	require.Contains(t, upserts[0].Mutation, "<name>")
}

func TestUpsertMultiple(t *testing.T) {
	body := []byte(`{
		"upsert": [
			{
				"query": "query { u as var(func: eq(email, \"a@a.com\")) }",
				"mutation": "uid(u) <name> \"A\" .",
				"cond": "@if(eq(len(u), 1))"
			},
			{
				"query": "query { u as var(func: eq(email, \"b@b.com\")) }",
				"mutation": "uid(u) <name> \"B\" .",
				"cond": "@if(eq(len(u), 1))"
			}
		]
	}`)
	mut, upserts, err := helpers.CheckMutationBody(helpers.ContentTypeJSON, body)
	require.NoError(t, err)
	require.Nil(t, mut)
	require.Len(t, upserts, 2)
	require.Contains(t, upserts[1].Mutation, "\"B\"")
}

func TestDQLContent(t *testing.T) {
	dql := `<_:a> <name> "Julian" .`
	mut, upserts, err := helpers.CheckMutationBody(helpers.ContentTypeDQL, []byte(dql))
	require.NoError(t, err)
	require.NotNil(t, mut)
	require.Nil(t, upserts)
	require.Equal(t, dql, string(mut.SetNquads))
	require.True(t, mut.CommitNow)
}

func TestEmptyBodyDQL(t *testing.T) {
	mut, upserts, err := helpers.CheckMutationBody(helpers.ContentTypeDQL, []byte{})
	require.Error(t, err)
	require.Nil(t, mut)
	require.Nil(t, upserts)
}

func TestInvalidJSON(t *testing.T) {
	body := []byte(`{invalid}`)
	mut, upserts, err := helpers.CheckMutationBody(helpers.ContentTypeJSON, body)
	require.Error(t, err)
	require.Nil(t, mut)
	require.Nil(t, upserts)
}

func TestUnknownContentType(t *testing.T) {
	body := []byte(`{"set": {"name": "test"}}`)
	mut, upserts, err := helpers.CheckMutationBody("text/plain", body)
	require.Error(t, err)
	require.Nil(t, mut)
	require.Nil(t, upserts)
}
