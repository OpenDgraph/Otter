package helpers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/OpenDgraph/Otter/internal/models"
	api "github.com/dgraph-io/dgo/v240/protos/api"
)

func ReadRequestBody(r *http.Request) ([]byte, error) {
	return io.ReadAll(r.Body)
}

func ParseQueryBody(contentType string, body []byte) (string, error) {
	switch contentType {
	case "application/json":
		var data map[string]interface{}
		if err := json.Unmarshal(body, &data); err != nil {
			return "", fmt.Errorf("| Invalid JSON")
		}
		query, ok := data["query"].(string)
		if !ok || query == "" {
			return "", fmt.Errorf("| Missing or invalid 'query' field in JSON")
		}
		return query, nil
	case "application/dql":
		return string(body), nil
	default:
		return "", fmt.Errorf("| Unsupported Content-Type: %s", contentType)
	}
}

func ParseMutationBody(contentType string, body []byte) (*api.Mutation, error) {
	switch contentType {
	case "application/json":
		var payload models.MutationPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, fmt.Errorf("| Invalid JSON")
		}
		if payload.Mutation != "" {
			return &api.Mutation{SetNquads: []byte(payload.Mutation), CommitNow: payload.CommitNow}, nil
		}
		if payload.Set != "" || payload.Delete != "" {
			setJson, _ := json.Marshal(map[string]string{"set": payload.Set, "delete": payload.Delete})
			return &api.Mutation{SetJson: setJson, CommitNow: payload.CommitNow}, nil
		}
		return nil, fmt.Errorf("| Missing mutation content in JSON")
	case "application/dql":
		return &api.Mutation{SetNquads: body, CommitNow: true}, nil
	default:
		return nil, fmt.Errorf("| Unsupported Content-Type: %s", contentType)
	}
}

func WriteJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func WriteJSONResponse(w http.ResponseWriter, status int, data []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(data)
}
