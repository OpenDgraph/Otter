package websocket

type WSMessage struct {
	Type      string `json:"type"` // "query", "mutation", "upsert"
	Query     string `json:"query,omitempty"`
	Mutation  string `json:"mutation,omitempty"`
	Cond      string `json:"cond,omitempty"` // Optional for upsert
	CommitNow bool   `json:"commitNow,omitempty"`
}
