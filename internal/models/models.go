package models

type MutationPayload struct {
	Set       string `json:"set"`
	Delete    string `json:"delete"`
	Mutation  string `json:"mutation"` // raw mutation fallback
	CommitNow bool   `json:"commitNow"`
}
