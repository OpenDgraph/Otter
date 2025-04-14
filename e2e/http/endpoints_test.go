package http_test // Ou o nome do pacote onde seus testes HTTP ficarão

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func checkHTTPError(t *testing.T, resp *http.Response, bodyBytes []byte) {
	t.Helper()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.Errorf("HTTP request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
		return
	}

	var result map[string]interface{}

	if err := json.Unmarshal(bodyBytes, &result); err == nil {

		if errors, ok := result["errors"].([]interface{}); ok && len(errors) > 0 {

			errorBytes, err := json.MarshalIndent(errors, "", "  ")
			if err != nil {
				t.Errorf("Request succeeded (status %d) but response contains Dgraph errors: %s", resp.StatusCode, string(bodyBytes))
			} else {
				t.Errorf("Request succeeded (status %d) but response contains Dgraph errors:\n%s", resp.StatusCode, string(errorBytes))
			}
		}
	}

}

func TestHTTPAPI(t *testing.T) {

	baseURL := "http://localhost:8084"
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	t.Run("Mutation", func(t *testing.T) {
		mutationPayload := `<_:new> <name> "Alice via HTTP" .`
		mutateURL := baseURL + "/mutate?commitNow=true"

		req, err := http.NewRequest("POST", mutateURL, strings.NewReader(mutationPayload))
		if err != nil {
			t.Fatalf("Failed to create mutation request: %v", err)
		}

		req.Header.Set("Content-Type", "application/dql")

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Failed to execute mutation request: %v", err)
		}
		defer resp.Body.Close()

		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read mutation response body: %v", err)
		}

		t.Logf("Mutation Response Status: %d", resp.StatusCode)
		t.Logf("Mutation Response Body: %s", string(bodyBytes))

		checkHTTPError(t, resp, bodyBytes)
	})

	time.Sleep(500 * time.Millisecond)

	t.Run("Query", func(t *testing.T) {

		queryPayload := `{
			data(func: eq(name, "Alice via HTTP")) {
				uid
				name
			}
		}`
		queryURL := baseURL + "/query"

		req, err := http.NewRequest("POST", queryURL, strings.NewReader(queryPayload))
		if err != nil {
			t.Fatalf("Failed to create query request: %v", err)
		}

		req.Header.Set("Content-Type", "application/dql")

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("Failed to execute query request: %v", err)
		}
		defer resp.Body.Close()

		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("Failed to read query response body: %v", err)
		}

		t.Logf("Query Response Status: %d", resp.StatusCode)
		t.Logf("Query Response Body: %s", string(bodyBytes))

		checkHTTPError(t, resp, bodyBytes)

		var queryResult map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &queryResult); err != nil {
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				t.Errorf("Query response status was %d but failed to unmarshal JSON body: %v. Body: %s", resp.StatusCode, err, string(bodyBytes))
			}

		} else {

			dataSlice, ok := queryResult["data"].([]interface{})
			if !ok {
				t.Errorf("Query response JSON does not contain a 'data' array. Body: %s", string(bodyBytes))
			} else if len(dataSlice) == 0 {
				t.Errorf("Query for 'Alice via HTTP' returned no results. Body: %s", string(bodyBytes))
			} else {

				firstResult, ok := dataSlice[0].(map[string]interface{})
				if ok {
					if name, ok := firstResult["name"].(string); !ok || name != "Alice via HTTP" {
						t.Errorf("First result name mismatch. Expected 'Alice via HTTP', got '%v'", firstResult["name"])
					}
					t.Logf("Successfully found node for 'Alice via HTTP' with UID: %v", firstResult["uid"])
				}
			}
		}
	})

}
