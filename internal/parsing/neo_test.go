package parsing

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/alecthomas/participle/v2"
	"github.com/stretchr/testify/require"
)

func TestMatchReturn(t *testing.T) {
	parser, err := participle.Build[Query](
		participle.Lexer(myLexer),
		participle.Elide("Whitespace"),
	)
	require.NoError(t, err)

	src := `MATCH (n:Person) RETURN n`
	ast, err := parser.ParseString("", src)
	require.NoError(t, err)

	// Match assertions
	require.NotNil(t, ast.Match, "Match clause should not be nil")
	require.NotNil(t, ast.Match.Node, "Match node should not be nil")
	require.Equal(t, "n", ast.Match.Node.Variable)
	require.Equal(t, "Person", ast.Match.Node.Label)
	require.Empty(t, ast.Match.Relations, "Match relations should be empty")

	// Return assertions
	require.NotNil(t, ast.Return, "Return clause should not be nil")
	require.Equal(t, []string{"n"}, ast.Return.Fields)
}

func TestMatchRelation(t *testing.T) {
	parser, err := participle.Build[Query](
		participle.Lexer(myLexer),
		participle.Elide("Whitespace"),
	)
	require.NoError(t, err)

	src := `MATCH (a:Person)-[:FRIEND]->(b:Person) RETURN a`
	ast, err := parser.ParseString("", src)
	b, _ := json.MarshalIndent(ast, "", "  ")
	fmt.Println("AST JSON:", string(b))

	require.NoError(t, err)

	// Match assertions - start node
	require.NotNil(t, ast.Match, "Match clause should not be nil")
	require.NotNil(t, ast.Match.Node, "Match node should not be nil")
	require.Equal(t, "a", ast.Match.Node.Variable)
	require.Equal(t, "Person", ast.Match.Node.Label)

	// Match assertions - relation segment
	require.Len(t, ast.Match.Relations, 1, "Should have exactly one relation segment")
	require.NotNil(t, ast.Match.Relations[0].Edge, "Relation edge should not be nil")
	require.Equal(t, "FRIEND", ast.Match.Relations[0].Edge.Type)

	require.NotNil(t, ast.Match.Relations[0].Node, "Relation destination node should not be nil")
	require.Equal(t, "b", ast.Match.Relations[0].Node.Variable)
	require.Equal(t, "Person", ast.Match.Relations[0].Node.Label)

	// Return assertions
	require.NotNil(t, ast.Return, "Return clause should not be nil")
	require.Equal(t, []string{"a"}, ast.Return.Fields)
}

func TestMatchWhereReturn(t *testing.T) {
	parser, err := participle.Build[Query](
		participle.Lexer(myLexer),
		participle.Elide("Whitespace"),
		participle.Unquote("String"),
	)
	require.NoError(t, err)

	src := `MATCH (n:Person) WHERE n.name = "Alice" RETURN n`
	ast, err := parser.ParseString("", src)
	require.NoError(t, err)

	// Match assertions
	require.NotNil(t, ast.Match, "Match clause should not be nil")
	require.NotNil(t, ast.Match.Node, "Match node should not be nil")
	require.Equal(t, "n", ast.Match.Node.Variable)
	require.Equal(t, "Person", ast.Match.Node.Label)

	// Where clause assertions
	require.NotNil(t, ast.Where, "Where clause should not be nil")
	require.NotNil(t, ast.Where.Cond, "Condition should not be nil")
	require.NotNil(t, ast.Where.Cond.Left, "Left side of condition should not be nil")
	require.Equal(t, "n", ast.Where.Cond.Left.Object)
	require.Equal(t, "name", ast.Where.Cond.Left.Field)
	require.Equal(t, "=", ast.Where.Cond.Operator)
	require.Equal(t, "Alice", ast.Where.Cond.Right)

	// Return assertions
	require.NotNil(t, ast.Return, "Return clause should not be nil")
	require.Equal(t, []string{"n"}, ast.Return.Fields)
}

func TestMatchNodeWithoutLabel(t *testing.T) {
	parser, err := participle.Build[Query](
		participle.Lexer(myLexer),
		participle.Elide("Whitespace"),
	)
	require.NoError(t, err)

	src := `MATCH (n) RETURN n`
	ast, err := parser.ParseString("", src)
	require.NoError(t, err)

	// Match assertions
	require.NotNil(t, ast.Match)
	require.NotNil(t, ast.Match.Node)
	require.Equal(t, "n", ast.Match.Node.Variable)
	require.Equal(t, "", ast.Match.Node.Label, "Label should be empty when not provided")
	require.Empty(t, ast.Match.Relations)

	// Return assertions
	require.NotNil(t, ast.Return)
	require.Equal(t, []string{"n"}, ast.Return.Fields)
}

func TestMatchMultipleReturnFields(t *testing.T) {
	parser, err := participle.Build[Query](
		participle.Lexer(myLexer),
		participle.Elide("Whitespace"),
	)
	require.NoError(t, err)

	src := `MATCH (a:User)-[:FOLLOWS]->(b:Topic) RETURN a, b`
	ast, err := parser.ParseString("", src)
	require.NoError(t, err)

	// Match assertions
	require.NotNil(t, ast.Match)
	require.Equal(t, "a", ast.Match.Node.Variable)
	require.Len(t, ast.Match.Relations, 1)
	require.Equal(t, "b", ast.Match.Relations[0].Node.Variable)

	// Return assertions
	require.NotNil(t, ast.Return)
	require.Equal(t, []string{"a", "b"}, ast.Return.Fields, "Should return multiple fields")
}

func TestMatchLongerPath(t *testing.T) {
	parser, err := participle.Build[Query](
		participle.Lexer(myLexer),
		participle.Elide("Whitespace"),
	)
	require.NoError(t, err)

	src := `MATCH (a:Start)-[:REL_ONE]->(b:Middle)-[:REL_TWO]->(c:End) RETURN c`
	ast, err := parser.ParseString("", src)
	require.NoError(t, err)

	// Start node
	require.NotNil(t, ast.Match)
	require.NotNil(t, ast.Match.Node)
	require.Equal(t, "a", ast.Match.Node.Variable)
	require.Equal(t, "Start", ast.Match.Node.Label)

	// Two path segments
	require.Len(t, ast.Match.Relations, 2, "Should have two relation segments")

	// First relation
	require.NotNil(t, ast.Match.Relations[0])
	require.NotNil(t, ast.Match.Relations[0].Edge)
	require.Equal(t, "REL_ONE", ast.Match.Relations[0].Edge.Type)
	require.NotNil(t, ast.Match.Relations[0].Node)
	require.Equal(t, "b", ast.Match.Relations[0].Node.Variable)
	require.Equal(t, "Middle", ast.Match.Relations[0].Node.Label)

	// Second relation
	require.NotNil(t, ast.Match.Relations[1])
	require.NotNil(t, ast.Match.Relations[1].Edge)
	require.Equal(t, "REL_TWO", ast.Match.Relations[1].Edge.Type)
	require.NotNil(t, ast.Match.Relations[1].Node)
	require.Equal(t, "c", ast.Match.Relations[1].Node.Variable)
	require.Equal(t, "End", ast.Match.Relations[1].Node.Label)

	// Return
	require.NotNil(t, ast.Return)
	require.Equal(t, []string{"c"}, ast.Return.Fields)
}

func TestMatchWhereDifferentOperator(t *testing.T) {
	parser, err := participle.Build[Query](
		participle.Lexer(myLexer),
		participle.Elide("Whitespace"),
		participle.Unquote("String"),
	)
	require.NoError(t, err)

	src := `MATCH (p:Product) WHERE p.stock > "0" RETURN p`
	ast, err := parser.ParseString("", src)
	require.NoError(t, err)

	// Match
	require.NotNil(t, ast.Match)
	require.Equal(t, "p", ast.Match.Node.Variable)
	require.Equal(t, "Product", ast.Match.Node.Label)

	// Where
	require.NotNil(t, ast.Where)
	require.NotNil(t, ast.Where.Cond)
	require.NotNil(t, ast.Where.Cond.Left)
	require.Equal(t, "p", ast.Where.Cond.Left.Object)
	require.Equal(t, "stock", ast.Where.Cond.Left.Field)
	require.Equal(t, ">", ast.Where.Cond.Operator)
	require.Equal(t, "0", ast.Where.Cond.Right)

	// Return
	require.NotNil(t, ast.Return)
	require.Equal(t, []string{"p"}, ast.Return.Fields)
}

func TestMatchLongPathWithWhereAndMultipleReturn(t *testing.T) {
	parser, err := participle.Build[Query](
		participle.Lexer(myLexer),
		participle.Elide("Whitespace"),
		participle.Unquote("String"),
	)
	require.NoError(t, err)

	src := `MATCH (u:User)-[:WROTE]->(a:Article) WHERE a.status = "published" RETURN u, a`
	ast, err := parser.ParseString("", src)
	require.NoError(t, err)

	// Match
	require.NotNil(t, ast.Match)
	require.Equal(t, "u", ast.Match.Node.Variable)
	require.Equal(t, "User", ast.Match.Node.Label)
	require.Len(t, ast.Match.Relations, 1)
	require.Equal(t, "WROTE", ast.Match.Relations[0].Edge.Type)
	require.Equal(t, "a", ast.Match.Relations[0].Node.Variable)
	require.Equal(t, "Article", ast.Match.Relations[0].Node.Label)

	// Where
	require.NotNil(t, ast.Where)
	require.NotNil(t, ast.Where.Cond)
	require.Equal(t, "a", ast.Where.Cond.Left.Object)
	require.Equal(t, "status", ast.Where.Cond.Left.Field)
	require.Equal(t, "=", ast.Where.Cond.Operator)
	require.Equal(t, "published", ast.Where.Cond.Right)

	// Return
	require.NotNil(t, ast.Return)
	require.Equal(t, []string{"u", "a"}, ast.Return.Fields)
}

func TestMatchWithVariedSpacing(t *testing.T) {
	parser, err := participle.Build[Query](
		participle.Lexer(myLexer),
		participle.Elide("Whitespace"),
	)
	require.NoError(t, err)

	src := `
        MATCH ( a : Person ) - [ : FRIEND ] -> ( b : Person )
        RETURN a
    `
	ast, err := parser.ParseString("", src)
	require.NoError(t, err, "Parser should handle varied whitespace")

	require.NotNil(t, ast.Match)
	require.NotNil(t, ast.Match.Node)
	require.Equal(t, "a", ast.Match.Node.Variable)
	require.Equal(t, "Person", ast.Match.Node.Label)
	require.Len(t, ast.Match.Relations, 1)
	require.NotNil(t, ast.Match.Relations[0])
	require.NotNil(t, ast.Match.Relations[0].Edge)
	require.Equal(t, "FRIEND", ast.Match.Relations[0].Edge.Type)
	require.NotNil(t, ast.Match.Relations[0].Node)
	require.Equal(t, "b", ast.Match.Relations[0].Node.Variable)
	require.Equal(t, "Person", ast.Match.Relations[0].Node.Label)
	require.NotNil(t, ast.Return)
	require.Equal(t, []string{"a"}, ast.Return.Fields)
