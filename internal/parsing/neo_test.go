package parsing

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/alecthomas/participle/v2"
	"github.com/stretchr/testify/require"
)

func TestMatchReturn(t *testing.T) {
	parser := BuildParser[Query]()
	src := `MATCH (n:Person) RETURN n`
	ast, err := parser.ParseString("", src)
	require.NoError(t, err)

	// Match assertions - Updated
	require.NotNil(t, ast.Match, "Match clause should not be nil")
	require.Len(t, ast.Match.Patterns, 1)
	p1 := ast.Match.Patterns[0]
	require.NotNil(t, p1.StartNode)
	require.Equal(t, "n", p1.StartNode.Variable)
	require.Equal(t, "Person", p1.StartNode.Label)
	require.Nil(t, p1.StartNode.Properties)
	require.Len(t, p1.Segments, 0, "Should have no segments")

	// Return assertions - OK
	require.NotNil(t, ast.Return, "Return clause should not be nil")
	require.Equal(t, []string{"n"}, ast.Return.Fields)
}

func TestMatchRelation(t *testing.T) {
	parser := BuildParser[Query]()
	src := `MATCH (a:Person)-[:FRIEND]->(b:Person) RETURN a`
	ast, err := parser.ParseString("", src)
	require.NoError(t, err)

	// Match assertions - Updated
	require.NotNil(t, ast.Match)
	require.Len(t, ast.Match.Patterns, 1)
	p1 := ast.Match.Patterns[0]
	require.NotNil(t, p1.StartNode)
	require.Equal(t, "a", p1.StartNode.Variable)
	require.Equal(t, "Person", p1.StartNode.Label)

	require.Len(t, p1.Segments, 1)
	seg1 := p1.Segments[0]
	require.NotNil(t, seg1.Relationship)
	require.Equal(t, "-", seg1.Relationship.LeftArrow)
	require.Equal(t, "->", seg1.Relationship.RightArrow)
	require.NotNil(t, seg1.Relationship.Edge)
	require.Equal(t, "", seg1.Relationship.Edge.Variable, "Edge variable should be empty when no alias")
	require.Equal(t, "FRIEND", seg1.Relationship.Edge.Type)

	require.NotNil(t, seg1.EndNode)
	require.Equal(t, "b", seg1.EndNode.Variable)
	require.Equal(t, "Person", seg1.EndNode.Label)

	// Return assertions - OK
	require.NotNil(t, ast.Return)
	require.Equal(t, []string{"a"}, ast.Return.Fields)
}

func TestMatchWhereReturn(t *testing.T) {
	// Precisa de Unquote para o valor de Where
	parser := BuildParser[Query]()
	src := `MATCH (n:Person) WHERE n.name = "Alice" RETURN n`
	ast, err := parser.ParseString("", src)
	require.NoError(t, err)

	// Match assertions - Updated
	require.NotNil(t, ast.Match)
	require.Len(t, ast.Match.Patterns, 1)
	p1 := ast.Match.Patterns[0]
	require.NotNil(t, p1.StartNode)
	require.Equal(t, "n", p1.StartNode.Variable)
	require.Equal(t, "Person", p1.StartNode.Label)
	require.Len(t, p1.Segments, 0)

	// Where clause assertions - OK
	require.NotNil(t, ast.Where, "Where clause should not be nil")
	require.NotNil(t, ast.Where.Cond, "Condition should not be nil")
	require.NotNil(t, ast.Where.Cond.Left, "Left side of condition should not be nil")
	require.Equal(t, "n", ast.Where.Cond.Left.Object)
	require.Equal(t, "name", ast.Where.Cond.Left.Field)
	require.Equal(t, "=", ast.Where.Cond.Operator)
	require.Equal(t, "Alice", ast.Where.Cond.Right) // Value is unquoted

	// Return assertions - OK
	require.NotNil(t, ast.Return, "Return clause should not be nil")
	require.Equal(t, []string{"n"}, ast.Return.Fields)
}
func TestMatchNodeWithoutLabel(t *testing.T) {
	parser := BuildParser[Query]()
	src := `MATCH (n) RETURN n`
	ast, err := parser.ParseString("", src)
	require.NoError(t, err)

	// Match assertions - Updated
	require.NotNil(t, ast.Match)
	require.Len(t, ast.Match.Patterns, 1)
	p1 := ast.Match.Patterns[0]
	require.NotNil(t, p1.StartNode)
	require.Equal(t, "n", p1.StartNode.Variable)
	require.Equal(t, "", p1.StartNode.Label, "Label should be empty when not provided")
	require.Len(t, p1.Segments, 0)

	// Return assertions - OK
	require.NotNil(t, ast.Return)
	require.Equal(t, []string{"n"}, ast.Return.Fields)
}

func TestMatchMultipleReturnFields(t *testing.T) {
	parser := BuildParser[Query]()
	src := `MATCH (a:User)-[:FOLLOWS]->(b:Topic) RETURN a, b`
	ast, err := parser.ParseString("", src)
	require.NoError(t, err)

	// Match assertions - Updated
	require.NotNil(t, ast.Match)
	require.Len(t, ast.Match.Patterns, 1)
	p1 := ast.Match.Patterns[0]
	require.NotNil(t, p1.StartNode)
	require.Equal(t, "a", p1.StartNode.Variable)
	require.Equal(t, "User", p1.StartNode.Label)

	require.Len(t, p1.Segments, 1)
	seg1 := p1.Segments[0]
	require.NotNil(t, seg1.Relationship)
	require.Equal(t, "-", seg1.Relationship.LeftArrow)
	require.Equal(t, "->", seg1.Relationship.RightArrow)
	require.Equal(t, "FOLLOWS", seg1.Relationship.Edge.Type)
	require.Equal(t, "", seg1.Relationship.Edge.Variable) // No alias

	require.NotNil(t, seg1.EndNode)
	require.Equal(t, "b", seg1.EndNode.Variable)
	require.Equal(t, "Topic", seg1.EndNode.Label)

	// Return assertions - OK
	require.NotNil(t, ast.Return)
	require.Equal(t, []string{"a", "b"}, ast.Return.Fields, "Should return multiple fields")
}

func TestMatchLongerPath(t *testing.T) {
	parser := BuildParser[Query]()
	src := `MATCH (a:Start)-[:REL_ONE]->(b:Middle)-[:REL_TWO]->(c:End) RETURN c`
	ast, err := parser.ParseString("", src)
	require.NoError(t, err)

	// Match assertions - Updated
	require.NotNil(t, ast.Match)
	require.Len(t, ast.Match.Patterns, 1)
	p1 := ast.Match.Patterns[0]
	require.NotNil(t, p1.StartNode)
	require.Equal(t, "a", p1.StartNode.Variable)
	require.Equal(t, "Start", p1.StartNode.Label)

	require.Len(t, p1.Segments, 2, "Should have two relation segments")

	// First segment
	seg1 := p1.Segments[0]
	require.NotNil(t, seg1.Relationship)
	require.Equal(t, "-", seg1.Relationship.LeftArrow)
	require.Equal(t, "->", seg1.Relationship.RightArrow)
	require.Equal(t, "REL_ONE", seg1.Relationship.Edge.Type)
	require.NotNil(t, seg1.EndNode)
	require.Equal(t, "b", seg1.EndNode.Variable)
	require.Equal(t, "Middle", seg1.EndNode.Label)

	// Second segment
	seg2 := p1.Segments[1]
	require.NotNil(t, seg2.Relationship)
	require.Equal(t, "-", seg2.Relationship.LeftArrow)
	require.Equal(t, "->", seg2.Relationship.RightArrow)
	require.Equal(t, "REL_TWO", seg2.Relationship.Edge.Type)
	require.NotNil(t, seg2.EndNode)
	require.Equal(t, "c", seg2.EndNode.Variable)
	require.Equal(t, "End", seg2.EndNode.Label)

	// Return - OK
	require.NotNil(t, ast.Return)
	require.Equal(t, []string{"c"}, ast.Return.Fields)
}

func TestMatchWhereDifferentOperator(t *testing.T) {
	parser := BuildParser[Query]()
	src := `MATCH (p:Product) WHERE p.stock > "0" RETURN p`
	ast, err := parser.ParseString("", src)
	require.NoError(t, err)

	// Match - Updated
	require.NotNil(t, ast.Match)
	require.Len(t, ast.Match.Patterns, 1)
	p1 := ast.Match.Patterns[0]
	require.Equal(t, "p", p1.StartNode.Variable)
	require.Equal(t, "Product", p1.StartNode.Label)
	require.Len(t, p1.Segments, 0)

	// Where - OK
	require.NotNil(t, ast.Where)
	require.NotNil(t, ast.Where.Cond)
	require.NotNil(t, ast.Where.Cond.Left)
	require.Equal(t, "p", ast.Where.Cond.Left.Object)
	require.Equal(t, "stock", ast.Where.Cond.Left.Field)
	require.Equal(t, ">", ast.Where.Cond.Operator)
	require.Equal(t, "0", ast.Where.Cond.Right)

	// Return - OK
	require.NotNil(t, ast.Return)
	require.Equal(t, []string{"p"}, ast.Return.Fields)
}

func TestMatchLongPathWithWhereAndMultipleReturn(t *testing.T) {
	parser := BuildParser[Query]()
	src := `MATCH (u:User)-[:WROTE]->(a:Article) WHERE a.status = "published" RETURN u, a`
	ast, err := parser.ParseString("", src)
	require.NoError(t, err)

	// Match - Updated
	require.NotNil(t, ast.Match)
	require.Len(t, ast.Match.Patterns, 1)
	p1 := ast.Match.Patterns[0]
	require.Equal(t, "u", p1.StartNode.Variable)
	require.Equal(t, "User", p1.StartNode.Label)

	require.Len(t, p1.Segments, 1)
	seg1 := p1.Segments[0]
	require.Equal(t, "-", seg1.Relationship.LeftArrow)
	require.Equal(t, "->", seg1.Relationship.RightArrow)
	require.Equal(t, "WROTE", seg1.Relationship.Edge.Type)
	require.Equal(t, "a", seg1.EndNode.Variable)
	require.Equal(t, "Article", seg1.EndNode.Label)

	// Where - OK
	require.NotNil(t, ast.Where)
	require.NotNil(t, ast.Where.Cond)
	require.Equal(t, "a", ast.Where.Cond.Left.Object)
	require.Equal(t, "status", ast.Where.Cond.Left.Field)
	require.Equal(t, "=", ast.Where.Cond.Operator)
	require.Equal(t, "published", ast.Where.Cond.Right)

	// Return - OK
	require.NotNil(t, ast.Return)
	require.Equal(t, []string{"u", "a"}, ast.Return.Fields)
}

func TestMatchWithVariedSpacing(t *testing.T) {
	parser := BuildParser[Query]()
	src := `
        MATCH ( a : Person ) - [ : FRIEND ] -> ( b : Person )
        RETURN a
    `
	ast, err := parser.ParseString("", src)
	require.NoError(t, err, "Parser should handle varied whitespace")

	// Match assertions - Updated (Same as TestMatchRelation)
	require.NotNil(t, ast.Match)
	require.Len(t, ast.Match.Patterns, 1)
	p1 := ast.Match.Patterns[0]
	require.NotNil(t, p1.StartNode)
	require.Equal(t, "a", p1.StartNode.Variable)
	require.Equal(t, "Person", p1.StartNode.Label)

	require.Len(t, p1.Segments, 1)
	seg1 := p1.Segments[0]
	require.NotNil(t, seg1.Relationship)
	require.Equal(t, "-", seg1.Relationship.LeftArrow)
	require.Equal(t, "->", seg1.Relationship.RightArrow)
	require.NotNil(t, seg1.Relationship.Edge)
	require.Equal(t, "", seg1.Relationship.Edge.Variable)
	require.Equal(t, "FRIEND", seg1.Relationship.Edge.Type)

	require.NotNil(t, seg1.EndNode)
	require.Equal(t, "b", seg1.EndNode.Variable)
	require.Equal(t, "Person", seg1.EndNode.Label)

	// Return assertions - OK
	require.NotNil(t, ast.Return)
	require.Equal(t, []string{"a"}, ast.Return.Fields)
}

func TestMatchWithReverseRelation(t *testing.T) {
	parser := BuildParser[Query]()
	src := `MATCH (a)<-[:FOLLOWS]-(b) RETURN a, b`
	ast, err := parser.ParseString("", src)
	// b, _ := json.MarshalIndent(ast, "", "  ") // Debug
	// fmt.Println("AST JSON (Reverse):", string(b)) // Debug
	require.NoError(t, err)

	require.NotNil(t, ast.Match)
	require.Len(t, ast.Match.Patterns, 1)
	p1 := ast.Match.Patterns[0]
	require.Equal(t, "a", p1.StartNode.Variable) // Nó inicial é 'a'
	require.Len(t, p1.Segments, 1)
	seg1 := p1.Segments[0]
	require.Equal(t, "<-", seg1.Relationship.LeftArrow) // Verifica seta esquerda
	require.Equal(t, "-", seg1.Relationship.RightArrow)
	require.Equal(t, "FOLLOWS", seg1.Relationship.Edge.Type)
	require.Equal(t, "b", seg1.EndNode.Variable)

	require.NotNil(t, ast.Return)
	require.Equal(t, []string{"a", "b"}, ast.Return.Fields)
}
func TestMatchWithMultiplePatterns(t *testing.T) {
	parser := BuildParser[Query]()
	src := `MATCH (a:User), (b:User) RETURN a, b`
	ast, err := parser.ParseString("", src)
	// b, _ := json.MarshalIndent(ast, "", "  ") // Debug
	// fmt.Println("AST JSON (MultiPattern):", string(b)) // Debug
	require.NoError(t, err)

	require.NotNil(t, ast.Match)
	require.Len(t, ast.Match.Patterns, 2) // Verifica se há dois patterns

	// Pattern 1
	p1 := ast.Match.Patterns[0]
	require.Equal(t, "a", p1.StartNode.Variable)
	require.Equal(t, "User", p1.StartNode.Label)
	require.Len(t, p1.Segments, 0)

	// Pattern 2
	p2 := ast.Match.Patterns[1]
	require.Equal(t, "b", p2.StartNode.Variable)
	require.Equal(t, "User", p2.StartNode.Label)
	require.Len(t, p2.Segments, 0)

	require.NotNil(t, ast.Return)
	require.Equal(t, []string{"a", "b"}, ast.Return.Fields)
}

func TestReturnWithPropertyAccess(t *testing.T) {
	parser, err := participle.Build[Query](
		participle.Lexer(myLexer),
		participle.Elide("Whitespace"),
	)
	require.NoError(t, err)

	src := `MATCH (n:Person) RETURN n.name`
	ast, err := parser.ParseString("", src)
	b, _ := json.MarshalIndent(ast, "", "  ")
	fmt.Println("AST JSON:", string(b))
	require.Error(t, err, "Property access in RETURN is not supported yet")
}

func TestWhereWithLogicalOperators(t *testing.T) {
	parser, err := participle.Build[Query](
		participle.Lexer(myLexer),
		participle.Elide("Whitespace"),
		participle.Unquote("String"),
	)
	require.NoError(t, err)

	src := `MATCH (a) WHERE a.name = "Alice" AND a.age > "30" RETURN a`
	ast, err := parser.ParseString("", src)
	b, _ := json.MarshalIndent(ast, "", "  ")
	fmt.Println("AST JSON:", string(b))
	require.Error(t, err, "Logical operators (AND/OR) not supported yet")
}

func TestMatchWithNodeProperties(t *testing.T) {
	parser := BuildParser[Query]() // Precisa Unquote para valor
	src := `MATCH (n:Person {name: "Alice"}) RETURN n`
	ast, err := parser.ParseString("", src)
	// b, _ := json.MarshalIndent(ast, "", "  ") // Debug
	// fmt.Println("AST JSON (NodeProps):", string(b)) // Debug
	require.NoError(t, err)

	require.NotNil(t, ast.Match)
	require.Len(t, ast.Match.Patterns, 1)
	p1 := ast.Match.Patterns[0]
	require.Equal(t, "n", p1.StartNode.Variable)
	require.Equal(t, "Person", p1.StartNode.Label)
	require.Len(t, p1.Segments, 0)

	require.NotNil(t, p1.StartNode.Properties) // Verifica se as propriedades existem
	require.Len(t, p1.StartNode.Properties.Entries, 1)
	require.Equal(t, "name", p1.StartNode.Properties.Entries[0].Key)
	require.Equal(t, "Alice", p1.StartNode.Properties.Entries[0].Value) // Valor sem aspas

	require.NotNil(t, ast.Return)
	require.Equal(t, []string{"n"}, ast.Return.Fields)
}

func TestRelationWithAlias(t *testing.T) {
	parser := BuildParser[Query]()
	src := `MATCH (a)-[r:KNOWS]->(b) RETURN r`
	ast, err := parser.ParseString("", src)
	require.NoError(t, err)

	// Match assertions - Updated
	require.NotNil(t, ast.Match)
	require.Len(t, ast.Match.Patterns, 1)
	p1 := ast.Match.Patterns[0]
	require.NotNil(t, p1.StartNode)
	require.Equal(t, "a", p1.StartNode.Variable)
	require.Equal(t, "", p1.StartNode.Label) // Sem label

	require.Len(t, p1.Segments, 1)
	seg1 := p1.Segments[0]
	require.NotNil(t, seg1.Relationship)
	require.Equal(t, "-", seg1.Relationship.LeftArrow)
	require.Equal(t, "->", seg1.Relationship.RightArrow)
	require.NotNil(t, seg1.Relationship.Edge)
	require.Equal(t, "r", seg1.Relationship.Edge.Variable, "Edge variable (alias) should be 'r'") // <<== Verifica o alias
	require.Equal(t, "KNOWS", seg1.Relationship.Edge.Type, "Edge type should be 'KNOWS'")

	require.NotNil(t, seg1.EndNode)
	require.Equal(t, "b", seg1.EndNode.Variable)
	require.Equal(t, "", seg1.EndNode.Label) // Sem label

	// Return assertions - OK
	require.NotNil(t, ast.Return)
	require.Equal(t, []string{"r"}, ast.Return.Fields, "Return field should be the alias 'r'")
}

func TestInvalidQueryMissingParenthesis(t *testing.T) {
	parser, err := participle.Build[Query](
		participle.Lexer(myLexer),
		participle.Elide("Whitespace"),
	)
	require.NoError(t, err)

	src := `MATCH a:Person) RETURN a`
	ast, err := parser.ParseString("", src)
	b, _ := json.MarshalIndent(ast, "", "  ")
	fmt.Println("AST JSON:", string(b))
	require.Error(t, err, "Missing parenthesis should result in parse error")
}

func TestInvalidQueryWhereWithoutMatch(t *testing.T) {
	parser, err := participle.Build[Query](
		participle.Lexer(myLexer),
		participle.Elide("Whitespace"),
	)
	require.NoError(t, err)

	src := `WHERE name = "Alice" RETURN n`
	ast, err := parser.ParseString("", src)
	b, _ := json.MarshalIndent(ast, "", "  ")
	fmt.Println("AST JSON:", string(b))
	require.Error(t, err, "WHERE without MATCH should fail")
}

func TestInvalidQueryReturnOnly(t *testing.T) {
	parser, err := participle.Build[Query](
		participle.Lexer(myLexer),
		participle.Elide("Whitespace"),
	)
	require.NoError(t, err)

	src := `RETURN n`
	ast, err := parser.ParseString("", src)
	b, _ := json.MarshalIndent(ast, "", "  ")
	fmt.Println("AST JSON:", string(b))
	require.Error(t, err, "RETURN without MATCH should fail")
}
