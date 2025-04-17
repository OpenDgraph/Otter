package helpers

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type ASTSnapshot struct {
	Test  string      `json:"test"`
	Input string      `json:"input"`
	AST   interface{} `json:"ast"`
}

func SaveASTSnapshot(t *testing.T, snapshot ASTSnapshot, path string) {
	t.Helper()

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create directory %s: %v", dir, err)
	}

	var snapshots []ASTSnapshot
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &snapshots)
	}

	snapshots = append(snapshots, snapshot)

	data, err := json.MarshalIndent(snapshots, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal snapshot: %v", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write snapshot file: %v", err)
	}
}
