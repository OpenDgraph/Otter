package helpers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/OpenDgraph/Otter/internal/models"
	api "github.com/dgraph-io/dgo/v240/protos/api"
)

const (
	ContentTypeJSON = "application/json"
	ContentTypeDQL  = "application/dql"
)

func ReadRequestBody(r *http.Request) ([]byte, error) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read request body: %w", err)
	}

	r.Body.Close()
	return bodyBytes, nil
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

	case ContentTypeDQL:
		if len(body) == 0 {
			return "", fmt.Errorf("| Empty request body for %s", ContentTypeDQL)
		}
		return string(body), nil

	default:
		return "", fmt.Errorf("| Unsupported Content-Type for query: %s", contentType)
	}
}

func CheckMutationBody(contentType string, body []byte) (*api.Mutation, error) {
	switch contentType {
	case ContentTypeJSON:
		var payload models.MutationPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, fmt.Errorf("| Invalid JSON payload: %w", err)
		}

		mutation := &api.Mutation{
			CommitNow: payload.CommitNow,
		}

		if payload.Mutation != "" {
			mutation.SetNquads = []byte(payload.Mutation)
		} else if payload.Set != "" || payload.Delete != "" {
			if payload.Set != "" {
				mutation.SetJson = []byte(payload.Set)
			}
			if payload.Delete != "" {
				mutation.DeleteJson = []byte(payload.Delete)
			}
		} else {
			return nil, fmt.Errorf("| Missing mutation content (mutation, set, or delete) in JSON payload")
		}
		return mutation, nil

	case ContentTypeDQL:
		if len(body) == 0 {
			return nil, fmt.Errorf("| Empty request body for %s", ContentTypeDQL)
		}
		return &api.Mutation{SetNquads: body, CommitNow: true}, nil

	default:
		return nil, fmt.Errorf("| Unsupported Content-Type for mutation: %s", contentType)
	}
}

func WriteJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func WriteJSONResponse(w http.ResponseWriter, status int, data []byte) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}
