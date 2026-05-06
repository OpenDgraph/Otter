package proxy

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestCleanSchemaResponse_FiltersInternalEntries covers the cases the
// cleaner is responsible for: predicates whose name starts with
// "dgraph.", types whose name starts with "dgraph.", and fields inside
// otherwise-kept types whose name starts with "dgraph.". Unknown fields
// on a type entry must round-trip via the Extras map.
func TestCleanSchemaResponse_FiltersInternalEntries(t *testing.T) {
	in := []byte(`{
		"schema": [
			{"predicate": "name", "type": "string"},
			{"predicate": "dgraph.type", "type": "string"},
			{"predicate": "age", "type": "int"}
		],
		"types": [
			{
				"name": "User",
				"fields": [
					{"name": "name"},
					{"name": "dgraph.internal"}
				],
				"reverse": true
			},
			{"name": "dgraph.graphql"}
		]
	}`)

	cleaned, err := cleanSchemaResponse(in)
	if err != nil {
		t.Fatalf("cleanSchemaResponse: %v", err)
	}

	out, err := json.Marshal(cleaned)
	if err != nil {
		t.Fatalf("marshal cleaned: %v", err)
	}
	s := string(out)

	if strings.Contains(s, "dgraph.type") {
		t.Errorf("internal predicate not removed: %s", s)
	}
	if strings.Contains(s, "dgraph.internal") {
		t.Errorf("internal field not removed: %s", s)
	}
	if strings.Contains(s, "dgraph.graphql") {
		t.Errorf("internal type not removed: %s", s)
	}
	// Sanity: legitimate entries kept.
	if !strings.Contains(s, `"predicate":"name"`) {
		t.Errorf("expected predicate `name` to be retained: %s", s)
	}
	if !strings.Contains(s, `"predicate":"age"`) {
		t.Errorf("expected predicate `age` to be retained: %s", s)
	}
	// Forward-compat: unknown type-level field round-trips via Extras.
	if !strings.Contains(s, `"reverse":true`) {
		t.Errorf("unknown type field `reverse` must round-trip: %s", s)
	}
}

func TestCleanSchemaResponse_InvalidJSON(t *testing.T) {
	if _, err := cleanSchemaResponse([]byte(`{not json`)); err == nil {
		t.Fatalf("expected error for malformed JSON")
	}
}
