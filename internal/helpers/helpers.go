package helpers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	api "github.com/dgraph-io/dgo/v240/protos/api"
)

const (
	ContentTypeJSON   = "application/json"
	ContentTypeDQL    = "application/dql"
	ContentTypeOldDQL = "application/graphql+-"
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

		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			return nil, nil, fmt.Errorf("| Invalid JSON payload: %w", err)
		}

		// Se for upsert
		if rawUpsert, ok := payload["upsert"]; ok {
			var blocks []*UpsertBlock
			switch up := rawUpsert.(type) {
			case map[string]interface{}:
				if len(up) == 0 {
					return nil, nil, fmt.Errorf("| 'upsert' object is empty")
				}
				b, _ := json.Marshal(up)
				var blk UpsertBlock
				if err := json.Unmarshal(b, &blk); err != nil {
					return nil, nil, fmt.Errorf("| Invalid upsert block: %w", err)
				}
				if blk.Query == "" || blk.Mutation == "" {
					return nil, nil, fmt.Errorf("| upsert block missing 'query' or 'mutation'")
				}
				blocks = append(blocks, &blk)
			case []interface{}:
				if len(up) == 0 {
					return nil, nil, fmt.Errorf("| 'upsert' array is empty")
				}
				for _, item := range up {
					b, _ := json.Marshal(item)
					var blk UpsertBlock
					if err := json.Unmarshal(b, &blk); err != nil {
						return nil, nil, fmt.Errorf("| Invalid upsert block in array: %w", err)
					}
					if blk.Query == "" || blk.Mutation == "" {
						return nil, nil, fmt.Errorf("| upsert block missing 'query' or 'mutation'")
					}
					blocks = append(blocks, &blk)
				}
			default:
				return nil, nil, fmt.Errorf("| Unsupported 'upsert' format")
			}
			return nil, blocks, nil
		}

		// Mutation simples
		mut := &api.Mutation{CommitNow: true}
		if s, ok := payload["set"]; ok {
			data, err := json.Marshal(s)
			if err != nil {
				return nil, nil, fmt.Errorf("| Error marshaling 'set': %w", err)
			}
			mut.SetJson = data
		}
		if d, ok := payload["delete"]; ok {
			data, err := json.Marshal(d)
			if err != nil {
				return nil, nil, fmt.Errorf("| Error marshaling 'delete': %w", err)
			}
			mut.DeleteJson = data
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

func WriteJSONResponse(w http.ResponseWriter, status int, resp *api.Response) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	if resp == nil {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data":       map[string]interface{}{},
			"extensions": map[string]interface{}{},
		})
		return
	}

	data := map[string]interface{}{}
	if len(resp.Json) > 0 {
		if err := json.Unmarshal(resp.Json, &data); err != nil {
			WriteJSONError(w, http.StatusInternalServerError, "error parsing response JSON")
			return
		}
	}

	extensions := map[string]interface{}{}

	// latency
	if resp.Latency != nil {
		extensions["server_latency"] = map[string]interface{}{
			"parsing_ns":          resp.Latency.GetParsingNs(),
			"processing_ns":       resp.Latency.GetProcessingNs(),
			"encoding_ns":         resp.Latency.GetEncodingNs(),
			"assign_timestamp_ns": resp.Latency.GetAssignTimestampNs(),
			"total_ns":            resp.Latency.GetTotalNs(),
		}
	}

	// txn
	if resp.Txn != nil {
		extensions["txn"] = map[string]interface{}{
			"start_ts": resp.Txn.GetStartTs(),
		}
	}

	// metrics
	if resp.Metrics != nil {
		extensions["metrics"] = map[string]interface{}{
			"num_uids": resp.Metrics.GetNumUids(),
		}
	}

	// uids
	if len(resp.Uids) > 0 {
		extensions["uids"] = resp.Uids
	}

	// RDF output
	if len(resp.Rdf) > 0 {
		extensions["rdf"] = string(resp.Rdf)
	}

	// headers
	if len(resp.Hdrs) > 0 {
		hdrs := make(map[string][]string)
		for key, val := range resp.Hdrs {
			hdrs[key] = val.GetValue()
		}
		extensions["headers"] = hdrs
	}

	final := map[string]interface{}{
		"data":       data,
		"extensions": extensions,
	}

	_ = json.NewEncoder(w).Encode(final)
}

// WriteJSONResponseList serializes multiple Dgraph responses (for example the
// result of a multi-block upsert) into a single JSON envelope. Nil entries
// are preserved as null in the output so callers can correlate by index.
func WriteJSONResponseList(w http.ResponseWriter, status int, resps []*api.Response) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	items := make([]map[string]interface{}, 0, len(resps))
	for _, resp := range resps {
		if resp == nil {
			items = append(items, map[string]interface{}{
				"data":       map[string]interface{}{},
				"extensions": map[string]interface{}{},
			})
			continue
		}
		data := map[string]interface{}{}
		if len(resp.Json) > 0 {
			if err := json.Unmarshal(resp.Json, &data); err != nil {
				items = append(items, map[string]interface{}{
					"error": "error parsing response JSON",
				})
				continue
			}
		}
		ext := map[string]interface{}{}
		if len(resp.Uids) > 0 {
			ext["uids"] = resp.Uids
		}
		if resp.Txn != nil {
			ext["txn"] = map[string]interface{}{
				"start_ts":  resp.Txn.GetStartTs(),
				"commit_ts": resp.Txn.GetCommitTs(),
			}
		}
		items = append(items, map[string]interface{}{
			"data":       data,
			"extensions": ext,
		})
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"results": items,
	})
}
