package parsing

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	dqlpkg "github.com/hypermodeinc/dgraph/v24/dql"
)

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

func cleanNulls(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		clean := make(map[string]interface{})
		for k, v2 := range val {
			c := cleanNulls(v2)

			switch c := c.(type) {
			case nil:
				continue
			case string:
				if c == "" {
					continue
				}
			case bool:
				if !c {
					continue
				}
			case []interface{}:
				if len(c) == 0 {
					continue
				}
			case map[string]interface{}:
				if len(c) == 0 {
					continue
				}
			}
			clean[k] = c
		}
		return clean

	case []interface{}:
		var out []interface{}
		for _, v2 := range val {
			c := cleanNulls(v2)
			if c != nil {
				out = append(out, c)
			}
		}
		return out
	default:
		return val
	}
}

func removeEmptyVars(ast map[string]interface{}) {
	if qvRaw, ok := ast["QueryVars"]; ok {
		if qvSlice, ok := qvRaw.([]interface{}); ok {
			allEmpty := true
			for _, v := range qvSlice {
				if m, ok := v.(map[string]interface{}); !ok || len(m) > 0 {
					allEmpty = false
					break
				}
			}
			if allEmpty {
				delete(ast, "QueryVars")
			}
		}
	}
}
