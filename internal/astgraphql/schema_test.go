package astgraphql

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// updateGolden lets the developer regenerate the golden JSON with:
//
//	go test ./internal/astgraphql -run TestSchemaToJSON_Golden -update
//
// Without the flag, the test asserts byte-for-byte equality against the
// committed golden file. Unlike the previous SaveASTSnapshot helper this
// does NOT mutate the working tree on every run.
var updateGolden = flag.Bool("update", false, "rewrite golden files in testdata/")

// sdlFixture is the single fixture used by the tests in this file. It
// intentionally exercises objects, enums, interfaces, unions, input
// objects, and a self-referencing edge.
const sdlFixture = `
	type TodoList {
		id: ID!
		title: String! @search(by: [term])
		todos: [TodoItem] @hasInverse(field: list)

		createdAt: DateTime
		updatedAt: DateTime
	}

	type TodoItem {
		id: ID!
		text: String! @search(by: [term])
		done: Boolean!
		dueDate: DateTime
		list: TodoList!

		createdAt: DateTime
		updatedAt: DateTime
	}

	enum Role {
		ADMIN
		USER
		GUEST
	}

	interface Entity {
		id: ID!
	}

	type User implements Entity {
		id: ID!
		name: String!
		role: Role
		friend: User
	}

	type Admin implements Entity {
		id: ID!
		powers: [String!]!
	}

	union Account = User | Admin

	input NewUserInput {
		name: String!
		role: Role!
	}
`

// collectNames returns the set of top-level node names in the JSON
// output of SchemaToJSON.
func collectNames(t *testing.T, jsonData []byte) map[string]bool {
	t.Helper()
	var nodes []GraphNode
	if err := json.Unmarshal(jsonData, &nodes); err != nil {
		t.Fatalf("unmarshal SchemaToJSON output: %v", err)
	}
	out := map[string]bool{}
	for _, n := range nodes {
		out[n.Name] = true
	}
	return out
}

// findNode returns the top-level node with the given name, or nil.
func findNode(t *testing.T, jsonData []byte, name string) *GraphNode {
	t.Helper()
	var nodes []GraphNode
	if err := json.Unmarshal(jsonData, &nodes); err != nil {
		t.Fatalf("unmarshal SchemaToJSON output: %v", err)
	}
	for i := range nodes {
		if nodes[i].Name == name {
			return &nodes[i]
		}
	}
	return nil
}

func TestParseSchema_Valid(t *testing.T) {
	schema, err := ParseSchema(sdlFixture)
	if err != nil {
		t.Fatalf("ParseSchema returned error on valid SDL: %v", err)
	}
	if schema.Types["TodoList"] == nil {
		t.Errorf("expected user-defined type TodoList to be present")
	}
	if schema.Types["Role"] == nil {
		t.Errorf("expected user-defined enum Role to be present")
	}
	// The handler must have synthesized the operation roots.
	if schema.Types["Query"] == nil {
		t.Errorf("expected synthesized Query operation root")
	}
}

func TestParseSchema_Invalid(t *testing.T) {
	cases := []struct {
		name, sdl string
	}{
		{"empty", ""},
		{"garbage", "this is not graphql"},
		{"undefined type", `type A { ref: DoesNotExist }`},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			if _, err := ParseSchema(c.sdl); err == nil {
				t.Fatalf("expected error for %q, got nil", c.name)
			}
		})
	}
}

func TestSchemaToJSON_IncludesUserTypes(t *testing.T) {
	schema, err := ParseSchema(sdlFixture)
	if err != nil {
		t.Fatalf("ParseSchema: %v", err)
	}
	data, err := SchemaToJSON(schema)
	if err != nil {
		t.Fatalf("SchemaToJSON: %v", err)
	}
	names := collectNames(t, data)

	mustInclude := []string{"TodoList", "TodoItem", "User", "Admin", "Role", "Entity", "Account"}
	for _, n := range mustInclude {
		if !names[n] {
			t.Errorf("expected user-defined type %q in output, got keys=%v", n, keys(names))
		}
	}
}

func TestSchemaToJSON_ExcludesGeneratedAndOperationRoots(t *testing.T) {
	schema, err := ParseSchema(sdlFixture)
	if err != nil {
		t.Fatalf("ParseSchema: %v", err)
	}
	data, err := SchemaToJSON(schema)
	if err != nil {
		t.Fatalf("SchemaToJSON: %v", err)
	}
	names := collectNames(t, data)

	// Operation roots must be filtered, both when user-declared and
	// when Dgraph-synthesized.
	for _, n := range []string{"Query", "Mutation", "Subscription"} {
		if names[n] {
			t.Errorf("operation root %q must not appear at top level", n)
		}
	}

	// Dgraph-generated plumbing must not leak.
	forbiddenPrefixes := []string{"Add", "Update", "Delete"}
	forbiddenSuffixes := []string{"Payload", "AggregateResult", "HasFilter", "Orderable", "Ref", "Patch"}
	for name := range names {
		for _, p := range forbiddenPrefixes {
			if strings.HasPrefix(name, p) {
				t.Errorf("generated type with prefix %q leaked: %s", p, name)
			}
		}
		for _, s := range forbiddenSuffixes {
			if strings.HasSuffix(name, s) {
				t.Errorf("generated type with suffix %q leaked: %s", s, name)
			}
		}
		if strings.HasPrefix(name, "__") {
			t.Errorf("introspection type leaked: %s", name)
		}
	}

	// Union discriminator enum must not appear. `AccountType` is
	// Dgraph's synthetic enum for `union Account = User | Admin`.
	if names["AccountType"] {
		t.Errorf("union discriminator enum AccountType must be filtered")
	}
}

func TestSchemaToJSON_EdgesAndProperties(t *testing.T) {
	schema, err := ParseSchema(sdlFixture)
	if err != nil {
		t.Fatalf("ParseSchema: %v", err)
	}
	data, err := SchemaToJSON(schema)
	if err != nil {
		t.Fatalf("SchemaToJSON: %v", err)
	}

	todo := findNode(t, data, "TodoList")
	if todo == nil {
		t.Fatalf("TodoList not found")
	}
	// Scalar fields become properties.
	if !containsAll(todo.Properties, "id", "title", "createdAt", "updatedAt") {
		t.Errorf("TodoList scalar fields missing: got %v", todo.Properties)
	}
	// `todos: [TodoItem]` must become an edge.
	if !hasEdge(todo, "TodoItem") {
		t.Errorf("TodoList should have an edge to TodoItem, got edges=%v", edgeNames(todo))
	}

	role := findNode(t, data, "Role")
	if role == nil {
		t.Fatalf("Role not found")
	}
	if !containsAll(role.Properties, "ADMIN", "USER", "GUEST") {
		t.Errorf("Role enum values missing: got %v", role.Properties)
	}

	// Interface must expand to its implementers.
	entity := findNode(t, data, "Entity")
	if entity == nil {
		t.Fatalf("Entity not found")
	}
	if !hasEdge(entity, "User") || !hasEdge(entity, "Admin") {
		t.Errorf("Entity interface should expand to User+Admin, got edges=%v", edgeNames(entity))
	}

	// Union must expand to its members.
	account := findNode(t, data, "Account")
	if account == nil {
		t.Fatalf("Account not found")
	}
	if !hasEdge(account, "User") || !hasEdge(account, "Admin") {
		t.Errorf("Account union should expand to User+Admin, got edges=%v", edgeNames(account))
	}
}

// TestSchemaToJSON_Deterministic verifies that two calls return
// byte-identical output so the serialization is stable across runs.
func TestSchemaToJSON_Deterministic(t *testing.T) {
	schema, err := ParseSchema(sdlFixture)
	if err != nil {
		t.Fatalf("ParseSchema: %v", err)
	}
	a, err := SchemaToJSON(schema)
	if err != nil {
		t.Fatalf("SchemaToJSON (1): %v", err)
	}
	b, err := SchemaToJSON(schema)
	if err != nil {
		t.Fatalf("SchemaToJSON (2): %v", err)
	}
	if !bytes.Equal(a, b) {
		t.Fatalf("SchemaToJSON output is non-deterministic")
	}
}

// TestExampleSchemaFileIsValid parses the schema shipped in
// `examples/cluster/graphql/schema.graphql` through the same pipeline a
// real caller would use. This guards against accidental regressions in
// the example (which is the first thing a new contributor runs) and
// confirms that the example exercises the intended code path.
func TestExampleSchemaFileIsValid(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "examples", "cluster", "graphql", "schema.graphql"))
	if err != nil {
		t.Fatalf("read example schema: %v", err)
	}
	s, err := ParseSchema(string(b))
	if err != nil {
		t.Fatalf("ParseSchema(example): %v", err)
	}
	data, err := SchemaToJSON(s)
	if err != nil {
		t.Fatalf("SchemaToJSON(example): %v", err)
	}
	names := collectNames(t, data)
	for _, must := range []string{"TodoList", "TodoItem", "User", "Admin", "Role", "Entity", "Account"} {
		if !names[must] {
			t.Errorf("example schema output missing %q (keys=%v)", must, keys(names))
		}
	}
	for _, forbidden := range []string{"Query", "Mutation", "Subscription", "AccountType"} {
		if names[forbidden] {
			t.Errorf("example schema output unexpectedly contains %q", forbidden)
		}
	}
}

// TestSchemaToJSON_Golden pins the full JSON tree to a committed golden
// file. Regenerate with `go test -run TestSchemaToJSON_Golden -update`.
func TestSchemaToJSON_Golden(t *testing.T) {
	schema, err := ParseSchema(sdlFixture)
	if err != nil {
		t.Fatalf("ParseSchema: %v", err)
	}
	got, err := SchemaToJSON(schema)
	if err != nil {
		t.Fatalf("SchemaToJSON: %v", err)
	}

	goldenPath := filepath.Join("testdata", "schema_to_json.golden.json")
	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("golden file rewritten: %s", goldenPath)
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden (run with -update to create): %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("schema JSON mismatch (run with -update to accept):\n--- want ---\n%s\n--- got ---\n%s", want, got)
	}
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func containsAll(hay []string, needles ...string) bool {
	have := map[string]bool{}
	for _, h := range hay {
		have[h] = true
	}
	for _, n := range needles {
		if !have[n] {
			return false
		}
	}
	return true
}

func hasEdge(n *GraphNode, name string) bool {
	for _, e := range n.Edge {
		if e.Name == name {
			return true
		}
	}
	return false
}

func edgeNames(n *GraphNode) []string {
	out := make([]string, 0, len(n.Edge))
	for _, e := range n.Edge {
		out = append(out, e.Name)
	}
	return out
}
