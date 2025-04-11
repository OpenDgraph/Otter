package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
)

const baseURL = "http://localhost:8084"

func TestValidateDQL(t *testing.T) {
	body := []byte(`{ me(func: has(name)) { uid name } }`)
	resp, err := postValidateRequest("/validate/dql", body)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	var res map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if res["type"] != "dql" {
		t.Errorf("Expected type 'dql', got '%s'", res["type"])
	}
}

func TestValidateSchema(t *testing.T) {
	body := []byte(`name: string @index(term) .`)
	resp, err := postValidateRequest("/validate/schema", body)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	var res map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if res["type"] != "schema" {
		t.Errorf("Expected type 'schema', got '%s'", res["type"])
	}
}

func postValidateRequest(path string, body []byte) (*http.Response, error) {
	return http.Post(
		baseURL+path,
		"application/dql",
		bytes.NewBuffer(body),
	)
}

func TestValidateMutations(t *testing.T) {
	tests := []struct {
		name           string
		mutation       string
		expectedStatus int
		expectedType   string
	}{
		{
			name: "Simple Set Mutation",
			mutation: `
			    {
					set {
						<0x1> <name> "Alice" .
					}
				}`,
			expectedStatus: http.StatusOK,
			expectedType:   "mutation",
		},
		{
			name: "Set with Blank Node",
			mutation: `
			    {
					set {
						_:user <name> "Bob" .
					}
				}`,
			expectedStatus: http.StatusOK,
			expectedType:   "mutation",
		},
		{
			name: "Delete Mutation",
			mutation: `
			    {
					delete {
						<0x1> <name> * .
					}
				}`,
			expectedStatus: http.StatusOK,
			expectedType:   "mutation",
		},
		{
			name: "Combined Set and Delete",
			mutation: `
			    {
					set {
						<0x2> <name> "Charlie" .
					}
					delete {
						<0x2> <name> * .
					}
				}`,
			expectedStatus: http.StatusOK,
			expectedType:   "mutation",
		},
		{
			name: "Invalid Mutation Syntax",
			mutation: `
			 mutation   {
					set {
						<0x1> name = "bad syntax"
					}
				}`,
			expectedStatus: http.StatusBadRequest,
			expectedType:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := postValidateRequest("/validate/dql", []byte(tt.mutation))
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.expectedStatus {
				t.Fatalf("Expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}

			if tt.expectedStatus == http.StatusOK {
				var res map[string]string
				if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}

				if res["type"] != tt.expectedType {
					t.Errorf("Expected type '%s', got '%s'", tt.expectedType, res["type"])
				}
			}
		})
	}
}
