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

// schemaWrapper / schemaTypeRaw mirror the relevant top level of a
// Dgraph `schema {}` response. Each entry stays as RawMessage so any
// fields Dgraph adds in the future round-trip untouched; we only peek
// at the discriminator (`predicate` or `name`) to decide whether to
// keep the entry.
type schemaWrapper struct {
	Schema []json.RawMessage `json:"schema,omitempty"`
	Types  []schemaTypeRaw   `json:"types,omitempty"`
}

type schemaTypeRaw struct {
	Name   string            `json:"name"`
	Fields []json.RawMessage `json:"fields,omitempty"`

	// Extras captures any other fields on the type entry so we can
	// re-emit them verbatim. Populated by UnmarshalJSON.
	Extras map[string]json.RawMessage `json:"-"`
}

// nameOnly is a minimal struct for peeking at the `name` field of an
// otherwise-opaque field entry without decoding the full object.
type nameOnly struct {
	Name string `json:"name"`
}

func (t schemaTypeRaw) MarshalJSON() ([]byte, error) {
	out := make(map[string]json.RawMessage, len(t.Extras)+2)
	for k, v := range t.Extras {
		out[k] = v
	}
	name, _ := json.Marshal(t.Name)
	out["name"] = name
	if t.Fields != nil {
		fields, _ := json.Marshal(t.Fields)
		out["fields"] = fields
	}
	return json.Marshal(out)
}

func (t *schemaTypeRaw) UnmarshalJSON(b []byte) error {
	m := map[string]json.RawMessage{}
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	if raw, ok := m["name"]; ok {
		_ = json.Unmarshal(raw, &t.Name)
		delete(m, "name")
	}
	if raw, ok := m["fields"]; ok {
		_ = json.Unmarshal(raw, &t.Fields)
		delete(m, "fields")
	}
	t.Extras = m
	return nil
}

// cleanSchemaResponse strips Dgraph-internal predicates / types / fields
// whose names start with "dgraph." from a schema response. The previous
// map[string]interface{} version walked every nested value dynamically;
// this version keeps each entry as raw bytes and only decodes the
// discriminator field, so we touch O(entries) bytes once instead of
// boxing every field into interface{}.
//
// Note: in the rare case that every entry under "schema" or "types" is
// internal, the corresponding key is omitted from the cleaned output
// (omitempty). The previous implementation emitted an explicit `null`
// in that case. Clients should treat both as "no entries".
func cleanSchemaResponse(data []byte) (*schemaWrapper, error) {
	var wrapper schemaWrapper
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, err
	}

	// Filter `schema` in place: we only need to read each entry's
	// "predicate" field, which sits in the JSON without a guaranteed
	// position, so we decode into a tiny peek struct rather than the
	// whole entry.
	if len(wrapper.Schema) > 0 {
		filtered := wrapper.Schema[:0]
		for _, raw := range wrapper.Schema {
			var peek struct {
				Predicate string `json:"predicate"`
			}
			if err := json.Unmarshal(raw, &peek); err == nil && strings.HasPrefix(peek.Predicate, "dgraph.") {
				continue
			}
			filtered = append(filtered, raw)
		}
		wrapper.Schema = filtered
	}

	if len(wrapper.Types) > 0 {
		filteredTypes := wrapper.Types[:0]
		for _, typ := range wrapper.Types {
			if strings.HasPrefix(typ.Name, "dgraph.") {
				continue
			}
			if len(typ.Fields) > 0 {
				filteredFields := typ.Fields[:0]
				for _, fraw := range typ.Fields {
					var peek nameOnly
					if err := json.Unmarshal(fraw, &peek); err == nil && strings.HasPrefix(peek.Name, "dgraph.") {
						continue
					}
					filteredFields = append(filteredFields, fraw)
				}
				typ.Fields = filteredFields
			}
			filteredTypes = append(filteredTypes, typ)
		}
		wrapper.Types = filteredTypes
	}

	return &wrapper, nil
}
