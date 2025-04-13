package parsing

import "testing"

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
}
