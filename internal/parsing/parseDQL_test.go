package parsing

import (
	"log"
	"testing"
)

func TestParse(t *testing.T) {
	query := `
	query works() {
	  q(func: has(name)) {
		name
	  }
	}
	`
	parsedQuery, ast := RenderQuery(query, "user", true)

	err := SaveDQLSnapshotFile("basic query", query, *ast, "snapshots/dql_ast.json", parsedQuery)
	if err != nil {
		log.Fatalf("Erro Saving the snapshot: %v", err)
	}

}
