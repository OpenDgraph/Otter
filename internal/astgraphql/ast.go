package astgraphql

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/hypermodeinc/dgraph/v24/graphql/schema"
	"github.com/pkg/errors"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
	"github.com/vektah/gqlparser/v2/validator"
)

// ParseSchema expands a user GraphQL SDL through Dgraph's schema handler
// (which injects Query, Mutation, filter/order/payload types, etc.) and
// then parses and validates the expanded document with gqlparser.
//
// The returned schema is therefore the Dgraph-augmented view of the user
// SDL, not the literal user SDL. See SchemaToJSON for the filter that
// reduces the augmented schema back down to "what the user wrote".
func ParseSchema(input string) (*ast.Schema, error) {
	trimmed := strings.TrimSpace(input)
	schHandler, err := schema.NewHandler(trimmed, false)
	if err != nil {
		return nil, err
	}

	expanded := schHandler.GQLSchema()

	doc, gqlErr := parser.ParseSchemas(validator.Prelude, &ast.Source{Input: expanded})
	if gqlErr != nil {
		return nil, errors.Wrap(gqlErr, "while parsing GraphQL schema")
	}
	gqlSchema, gqlErr := validator.ValidateSchemaDocument(doc)
	if gqlErr != nil {
		return nil, errors.Wrap(gqlErr, "while validating GraphQL schema")
	}
	return gqlSchema, nil
}

// GraphNode is the serialized form of a single type in the schema graph.
type GraphNode struct {
	UID        string      `json:"uid,omitempty"`
	Name       string      `json:"name"`
	Properties []string    `json:"properties,omitempty"`
	Edge       []GraphNode `json:"edge,omitempty"`
	Childs     []GraphNode `json:"childs,omitempty"`
}

// isUserDefined filters out gqlparser's built-in types (Prelude).
// User-defined (or Dgraph-augmented) types have a non-nil Position.Src.
func isUserDefined(t *ast.Definition) bool {
	return t != nil && t.Position != nil && t.Position.Src != nil
}

// isOperationRoot filters the three GraphQL operation roots. Dgraph's
// schema handler auto-generates Query (and optionally Subscription) from
// the user SDL even when the user did not declare them explicitly; we do
// not want those to surface as top-level nodes in the visualization.
func isOperationRoot(name string) bool {
	return name == "Query" || name == "Mutation" || name == "Subscription"
}

func isInputObject(t *ast.Definition) bool {
	return t != nil && t.Kind == ast.InputObject
}

func isScalar(t *ast.Definition) bool {
	return t != nil && t.Kind == ast.Scalar
}

func isIntrospectionType(name string) bool {
	return strings.HasPrefix(name, "__")
}

// isDgraphGenerated matches the suffixes and prefixes Dgraph's schema
// handler uses for auto-generated plumbing: update/delete payload inputs,
// *Ref inputs, *Patch inputs, *Filter types, *Order / *Orderable enums,
// *AggregateResult types, *HasFilter enums, and the geo helper types.
//
// These checks intentionally live in one place. See unifiedFilter.
func isDgraphGenerated(name string) bool {
	if strings.HasPrefix(name, "Add") || strings.HasPrefix(name, "Update") || strings.HasPrefix(name, "Delete") {
		return true
	}
	if strings.HasSuffix(name, "Payload") ||
		strings.HasSuffix(name, "Orderable") ||
		strings.HasSuffix(name, "HasFilter") ||
		strings.HasSuffix(name, "AggregateResult") {
		return true
	}
	switch name {
	case "Point", "PointList", "Polygon", "MultiPolygon", "PointRef", "PolygonRef", "MultiPolygonRef", "PointListRef":
		return true
	case "DgraphIndex", "Mode", "HTTPMethod":
		return true
	}
	return false
}

// isUnionDiscriminatorEnum catches the synthetic `<UnionName>Type` enum
// that Dgraph emits for every GraphQL union (e.g. `enum AccountType` for
// `union Account = User | Admin`). It is pure plumbing for the union
// return type and should not appear in the visualization.
func isUnionDiscriminatorEnum(schema *ast.Schema, t *ast.Definition) bool {
	if t == nil || t.Kind != ast.Enum {
		return false
	}
	if !strings.HasSuffix(t.Name, "Type") {
		return false
	}
	base := strings.TrimSuffix(t.Name, "Type")
	candidate, ok := schema.Types[base]
	if !ok || candidate == nil {
		return false
	}
	return candidate.Kind == ast.Union
}

// shouldIncludeType is the single source of truth for "does this type
// belong in the visualization output?". Used both at the top-level
// iteration and when expanding edges inside buildNode.
func shouldIncludeType(schema *ast.Schema, t *ast.Definition) bool {
	if t == nil {
		return false
	}
	if !isUserDefined(t) {
		return false
	}
	if isIntrospectionType(t.Name) {
		return false
	}
	if isOperationRoot(t.Name) {
		return false
	}
	if isInputObject(t) || isScalar(t) {
		return false
	}
	if isDgraphGenerated(t.Name) {
		return false
	}
	if isUnionDiscriminatorEnum(schema, t) {
		return false
	}
	return true
}

// SchemaToJSON walks the validated schema and produces a deterministic,
// indented JSON tree of user-facing types.
func SchemaToJSON(s *ast.Schema) ([]byte, error) {
	names := make([]string, 0, len(s.Types))
	for name, typ := range s.Types {
		if !shouldIncludeType(s, typ) {
			continue
		}
		names = append(names, name)
	}
	// Deterministic ordering makes golden-file comparison stable.
	sort.Strings(names)

	nodes := make([]GraphNode, 0, len(names))
	for _, name := range names {
		visited := map[string]GraphNode{}
		nodes = append(nodes, buildNode(s, name, visited))
	}

	return json.MarshalIndent(nodes, "", "    ")
}

func buildNode(s *ast.Schema, typeName string, visited map[string]GraphNode) GraphNode {
	if node, ok := visited[typeName]; ok {
		return node
	}

	t := s.Types[typeName]
	if t == nil {
		return GraphNode{Name: typeName}
	}

	node := GraphNode{
		UID:  "_:" + t.Name,
		Name: t.Name,
	}
	visited[typeName] = node // register before expansion to break cycles

	switch t.Kind {
	case ast.Object, ast.InputObject:
		for _, f := range t.Fields {
			target := s.Types[f.Type.Name()]
			if shouldIncludeType(s, target) {
				child := buildNode(s, f.Type.Name(), visited)
				node.Edge = append(node.Edge, child)
			} else {
				node.Properties = append(node.Properties, f.Name)
			}
		}
	case ast.Enum:
		for _, val := range t.EnumValues {
			node.Properties = append(node.Properties, val.Name)
		}
	case ast.Interface, ast.Union:
		for _, impl := range s.GetPossibleTypes(t) {
			target := s.Types[impl.Name]
			if shouldIncludeType(s, target) {
				child := buildNode(s, impl.Name, visited)
				node.Edge = append(node.Edge, child)
			}
		}
	}

	visited[typeName] = node // update with real properties/edges
	return node
}
