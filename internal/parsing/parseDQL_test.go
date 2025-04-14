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
	AST, err := ParseQuery(query)
	if err != nil {
		t.Errorf("Parse(%v) = %v; want nil", AST, err)
	}
	newquery := RenderQuery(AST, "user")
	SaveDQLSnapshotFile("basic query", query, AST, "snapshots/dql_ast.json", newquery)
	if err != nil {
		log.Fatalf("Erro Saving the snapshot: %v", err)
	}

}
