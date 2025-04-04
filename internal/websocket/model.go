package websocket

import "encoding/json"

type WSMessage struct {
	Type      string `json:"type"` // "query", "mutation", "upsert"
	Query     string `json:"query,omitempty"`
	Mutation  string `json:"mutation,omitempty"`
	Cond      string `json:"cond,omitempty"` // Optional for upsert
	CommitNow bool   `json:"commitNow,omitempty"`
	Verbose   bool   `json:"verbose,omitempty"`
}

type WSResponse struct {
	Data      json.RawMessage   `json:"data,omitempty"`
	Uids      map[string]string `json:"uids,omitempty"`
	CommitTs  uint64            `json:"commitTs,omitempty"`
	Preds     []string          `json:"predicates,omitempty"`
	LatencyNs uint64            `json:"latencyNs,omitempty"`
	Error     string            `json:"error,omitempty"`
}
