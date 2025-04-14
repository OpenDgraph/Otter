package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateDQLHandler(t *testing.T) {
	newRequest := func(method, contentType, body string) *http.Request {
		req := httptest.NewRequest(method, "/validate/dql", strings.NewReader(body))
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		return req
	}

	t.Run("Invalid Method (GET)", func(t *testing.T) {
		req := newRequest(http.MethodGet, "application/dql", "")
		rec := httptest.NewRecorder()
		ValidateDQLHandler(rec, req)

		assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
		assert.JSONEq(t, `{"error":"Method not allowed. Use POST."}`, rec.Body.String())
	})

	t.Run("Invalid Content-Type", func(t *testing.T) {
		req := newRequest(http.MethodPost, "application/json", "{}")
		rec := httptest.NewRecorder()
		ValidateDQLHandler(rec, req)

		assert.Equal(t, http.StatusUnsupportedMediaType, rec.Code)
		assert.JSONEq(t, `{"error":"Invalid Content-Type. Use application/dql."}`, rec.Body.String())
	})

	t.Run("Empty Body", func(t *testing.T) {
		req := newRequest(http.MethodPost, "application/dql", "")
		rec := httptest.NewRecorder()
		ValidateDQLHandler(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Contains(t, rec.Body.String(), `"error":"Request body is empty`)
	})

	t.Run("Valid DQL Query", func(t *testing.T) {
		query := `query { q(func: has(name)) { name } }`
		req := newRequest(http.MethodPost, "application/dql", query)
		rec := httptest.NewRecorder()
		ValidateDQLHandler(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, `{"status":"valid","type":"dql"}`, rec.Body.String())
	})

	t.Run("Valid DQL Mutation", func(t *testing.T) {
		mutation := `{ set { <_:user> <name> "Alice" . } }`
		req := newRequest(http.MethodPost, "application/dql", mutation)
		rec := httptest.NewRecorder()
		ValidateDQLHandler(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, `{"status":"valid","type":"mutation"}`, rec.Body.String())
	})

	t.Run("Invalid Input (Not DQL)", func(t *testing.T) {
		invalid := `this is not valid {`
		req := newRequest(http.MethodPost, "application/dql", invalid)
		rec := httptest.NewRecorder()
		ValidateDQLHandler(rec, req)

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.True(t, strings.HasPrefix(rec.Body.String(), `{"error":"Failed to parse DQL`), "error message should start with 'Failed to parse DQL'")
	})
}
