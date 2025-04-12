package parsing

import (
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

	require.Equal(t, "n", ast.Match.Node.Variable)
	require.Equal(t, "Person", ast.Match.Node.Label)

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
	require.NoError(t, err)

	require.Equal(t, "a", ast.Match.Node.Variable)
	require.Equal(t, "Person", ast.Match.Node.Label)
	require.Equal(t, "FRIEND", ast.Match.Relations[0].Edge)
	require.Equal(t, "b", ast.Match.Relations[0].Node.Variable)
	require.Equal(t, "Person", ast.Match.Relations[0].Node.Label)

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

	require.Equal(t, "n", ast.Match.Node.Variable)
	require.Equal(t, "Person", ast.Match.Node.Label)
	require.Equal(t, "n", ast.Where.LeftObj)
	require.Equal(t, "name", ast.Where.LeftKey)
	require.Equal(t, "=", ast.Where.Operator)
	require.Equal(t, "Alice", ast.Where.Right)
}

// func TestBasic(t *testing.T) {
// 	def := lexer.MustSimple([]lexer.SimpleRule{
// 		{"Ident", `[a-zA-Z_][a-zA-Z0-9_]*`},
// 		{"String", `"[^"]*"`},
// 		{"Punct", `[-:\[\]\(\),>]`},
// 		{"Whitespace", `\s+`},
// 	})

// 	src := `MATCH (n:Person)-[:FRIEND]->(b:Person) RETURN a, b`
// 	reader := strings.NewReader(src)

// 	scanner, err := def.Lex("", reader)
// 	if err != nil {
// 		panic(err)
// 	}

// 	for {
// 		tok, err := scanner.Next()
// 		if err != nil {
// 			panic(err)
// 		}
// 		if tok.EOF() {
// 			break
// 		}

// 		// Aqui fazemos a comparação correta convertendo para string
// 		if fmt.Sprintf("%v", tok.Type) != "Whitespace" {
// 			fmt.Printf("Type: %-10v  Value: %q\n", tok.Type, tok.Value)
// 		}
// 	}
// }

// func TestMatchWithRelation(t *testing.T) {
// 	parser, err := participle.Build[Query](
// 		participle.Lexer(myLexer),
// 		participle.Unquote("String"),   // opcional, remove aspas do valor
// 		participle.Elide("Whitespace"), // ignora whitespace
// 	)
// 	require.NoError(t, err)

// 	src := `MATCH (a:Person)-[:FRIEND]->(b:Person) RETURN a, b`
// 	ast, err := parser.ParseString("", src)
// 	require.NoError(t, err)

// 	// require.Equal(t, "a", ast.Match.Pattern.StartNode.Variable)
// 	// require.Equal(t, "FRIEND", ast.Match.Pattern.Relations[0].Out.Type)
// 	// require.Equal(t, "b", ast.Match.Pattern.Relations[0].Node.Variable)
// 	require.Equal(t, []string{"n"}, ast.Return.Fields)

// }
