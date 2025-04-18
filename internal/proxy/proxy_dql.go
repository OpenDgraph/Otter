package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/OpenDgraph/Otter/internal/helpers"
)

func (p *Proxy) runDQLQuery(query string, w http.ResponseWriter) {
	_, client, err := p.SelectClientAuto("query")
	if err != nil {
		helpers.WriteJSONError(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	resp, err := client.Query(context.Background(), query)
	if err != nil {
		helpers.WriteJSONQueryError(w, fmt.Sprintf("Error querying Dgraph: %v", err))
		return
	}

	// Checa se a query é para schema
	if strings.Contains(query, "schema {}") {
		cleaned, err := cleanSchemaResponse(resp.Json)
		if err != nil {
			helpers.WriteJSONError(w, http.StatusInternalServerError, "error parsing schema")
			return
		}

		newJson, err := json.Marshal(cleaned)

		if err != nil {
			helpers.WriteJSONError(w, http.StatusInternalServerError, "error serializing cleaned schema")
			return
		}

		resp.Json = newJson
		helpers.WriteJSONResponse(w, http.StatusOK, resp)
		return
	}

	helpers.WriteJSONResponse(w, http.StatusOK, resp)
}

func cleanSchemaResponse(data []byte) (map[string]interface{}, error) {
	var wrapper map[string]interface{}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, err
	}

	result := make(map[string]interface{})

	// --- limpar predicados do "schema"
	if rawSchema, ok := wrapper["schema"]; ok {
		if schemaList, ok := rawSchema.([]interface{}); ok {
			var cleaned []interface{}
			for _, item := range schemaList {
				pred, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				name, _ := pred["predicate"].(string)
				if strings.HasPrefix(name, "dgraph.") {
					continue
				}
				cleaned = append(cleaned, pred)
			}
			result["schema"] = cleaned
		}
	}

	// --- limpar types do "types"
	if rawTypes, ok := wrapper["types"]; ok {
		if typeList, ok := rawTypes.([]interface{}); ok {
			var cleaned []interface{}
			for _, item := range typeList {
				typ, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				name, _ := typ["name"].(string)
				if strings.HasPrefix(name, "dgraph.") {
					continue
				}
				// também pode limpar predicados internos dentro do type
				if fields, ok := typ["fields"].([]interface{}); ok {
					var filteredFields []interface{}
					for _, f := range fields {
						pname, _ := f.(map[string]interface{})["name"].(string)
						if strings.HasPrefix(pname, "dgraph.") {
							continue
						}
						filteredFields = append(filteredFields, f)
					}
					typ["fields"] = filteredFields
				}
				cleaned = append(cleaned, typ)
			}
			result["types"] = cleaned
		}
	}

	return result, nil
}
