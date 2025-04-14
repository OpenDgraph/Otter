package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/OpenDgraph/Otter/internal/helpers"
	"github.com/OpenDgraph/Otter/internal/parsing"
)

func ValidateDQLHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		helpers.WriteJSONError(w, http.StatusMethodNotAllowed, "Method not allowed. Use POST.")
		return
	}

	if r.Header.Get("Content-Type") != "application/dql" {
		helpers.WriteJSONError(w, http.StatusUnsupportedMediaType, "Invalid Content-Type. Use application/dql.")
		return
	}

	body, err := helpers.ReadRequestBody(r)
	if err != nil || len(body) == 0 {
		helpers.WriteJSONError(w, http.StatusBadRequest, "Request body is empty or unreadable.")
		return
	}

	dqlStr := string(body)

	// Try parsing as query
	queryResult, queryErr := parsing.ParseQuery(dqlStr)
	if queryErr == nil && len(queryResult.Query) > 0 {
		writeValidationResponse(w, "dql")
		return
	}

	// Fallback: try parsing as mutation
	_, mutationErr := parsing.ParseMutation(dqlStr)
	if mutationErr == nil {
		writeValidationResponse(w, "mutation")
		return
	}

	// If both failed
	msg := fmt.Sprintf("Failed to parse DQL. Query error: %v. Mutation error: %v", queryErr, mutationErr)
	helpers.WriteJSONError(w, http.StatusBadRequest, msg)
}

func ValidateSchemaHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		helpers.WriteJSONError(w, http.StatusMethodNotAllowed, "Método não permitido. Use POST.")
		return
	}

	body, err := helpers.ReadRequestBody(r)
	if err != nil || len(body) == 0 {
		helpers.WriteJSONError(w, http.StatusBadRequest, "Corpo inválido ou vazio.")
		return
	}

	schemaAST, schemaErr := parsing.ParseSchema(string(body))
	if schemaErr == nil && len(schemaAST.Preds) == 0 {
		helpers.WriteJSONError(w, http.StatusBadRequest, "Schema sem predicados definidos.")
		return
	}

	writeValidationResponse(w, "schema")
}

func writeValidationResponse(w http.ResponseWriter, typ string) {
	resp := map[string]string{"status": "valid", "type": typ}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}
