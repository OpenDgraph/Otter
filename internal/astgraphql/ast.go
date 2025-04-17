package astgraphql

import (
	"encoding/json"
	"strings"

	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
)

func ParseSchema(input string) (*ast.Schema, error) {
	return gqlparser.LoadSchema(&ast.Source{
		Name:  "schema.graphql",
		Input: input,
	})
}

type GraphNode struct {
	UID        string      `json:"uid,omitempty"`
	Name       string      `json:"name"`
	Properties []string    `json:"properties,omitempty"`
	Edge       []GraphNode `json:"edge,omitempty"`
	Childs     []GraphNode `json:"childs,omitempty"`
}

func isUserDefined(t *ast.Definition) bool {
	return t.Position.Src != nil
}

func isMutation(t *ast.Definition) bool {
	return t.Kind == ast.Object && t.Name == "Mutation"
}

func isInputObject(t *ast.Definition) bool {
	return t.Kind == ast.InputObject
}

func isScalar(t *ast.Definition) bool {
	return t.Kind == ast.Scalar
}

func SchemaToJSON(schema *ast.Schema) ([]byte, error) {
	var nodes []GraphNode

	for _, typ := range schema.Types {
		if isMutation(typ) || isInputObject(typ) || isScalar(typ) || isIntrospectionType(typ.Name) || !isUserDefined(typ) {
			continue
		}

		visited := map[string]GraphNode{}

		node := buildNode(schema, typ.Name, visited)
		nodes = append(nodes, node)
	}

	return json.MarshalIndent(nodes, "", "    ")
}

func isIntrospectionType(name string) bool {
	return strings.HasPrefix(name, "__")
}

func buildNode(schema *ast.Schema, typeName string, visited map[string]GraphNode) GraphNode {
	if node, ok := visited[typeName]; ok {
		return node
	}

	t := schema.Types[typeName]
	if t == nil {
		return GraphNode{Name: typeName}
	}

	node := GraphNode{
		UID:  "_:" + t.Name,
		Name: t.Name,
	}
	visited[typeName] = node // importante: registra antes de expandir

	switch t.Kind {
	case ast.Object, ast.InputObject:
		for _, f := range t.Fields {
			target := schema.Types[f.Type.Name()]
			if target != nil && isUserDefined(target) &&
				!isMutation(target) && !isInputObject(target) &&
				!isScalar(target) && !isIntrospectionType(target.Name) {

				child := buildNode(schema, f.Type.Name(), visited)
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
		for _, impl := range schema.GetPossibleTypes(t) {
			if target := schema.Types[impl.Name]; isUserDefined(target) {
				child := buildNode(schema, impl.Name, visited)
				node.Edge = append(node.Edge, child)
			}
		}
	}

	visited[typeName] = node // atualiza com propriedades e edges reais
	return node
}
