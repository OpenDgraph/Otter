package parsing

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dgraph-io/dgo/v240/protos/api"
	dqlpkg "github.com/hypermodeinc/dgraph/v24/dql"

	schemapkg "github.com/hypermodeinc/dgraph/v24/schema"
)

type DQLSnapshot struct {
	Test  string                 `json:"test"`
	Input string                 `json:"input"`
	AST   map[string]interface{} `json:"ast"`
	NewQ  string                 `json:"newq"`
}

func ParseQuery(query string) (dqlpkg.Result, error) {

	AST, err := dqlpkg.Parse(dqlpkg.Request{Str: query})
	if err != nil {
		return dqlpkg.Result{}, fmt.Errorf("failed to parse DQL query: %w", err)
	}

	// fmt.Println("Parsed DQL Query:", AST)
	return AST, nil
}
func ParseSchema(schema string) (schemapkg.ParsedSchema, error) {

	AST, err := schemapkg.Parse(schema)
	if err != nil {
		return schemapkg.ParsedSchema{}, fmt.Errorf("failed to parse DQL schema: %w", err)
	}

	// fmt.Println("Parsed DQL schema:", AST)
	return *AST, nil
}

func ParseMutation(mutation string) (*api.Request, error) {
	AST, err := dqlpkg.ParseMutation(mutation)
	if err != nil {
		return nil, fmt.Errorf("failed to parse DQL mutation: %w", err)
	}
	return AST, nil
}

func SaveDQLSnapshotFile(testName string, inputQuery string, ast dqlpkg.Result, snapshotPath string, newquery string) error {

	rawJSON, _ := json.Marshal(ast)
	var parsed map[string]interface{}
	_ = json.Unmarshal(rawJSON, &parsed)

	cleaned := cleanNulls(parsed)
	cleanedMap, ok := cleaned.(map[string]interface{})
	if !ok {
		return fmt.Errorf("expected cleaned AST to be a map, got %T", cleaned)
	}
	removeEmptyVars(cleanedMap)

	snapshot := DQLSnapshot{
		Test:  testName,
		Input: inputQuery,
		AST:   cleanedMap,
		NewQ:  newquery,
	}

	dir := filepath.Dir(snapshotPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	var snapshots []DQLSnapshot
	if data, err := os.ReadFile(snapshotPath); err == nil {
		_ = json.Unmarshal(data, &snapshots)
	}

	snapshots = append(snapshots, snapshot)

	data, err := json.MarshalIndent(snapshots, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal snapshots: %w", err)
	}

	if err := os.WriteFile(snapshotPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write snapshot file: %w", err)
	}

	return nil
}
