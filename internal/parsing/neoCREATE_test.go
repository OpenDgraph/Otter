package parsing

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateSingleNode(t *testing.T) {
	parser := BuildParser[Query]()
	src := `MATCH (a:Dummy) CREATE (n:Person { name: "Charles" }) RETURN n`
	ast, err := parser.ParseString("", src)
	require.NoError(t, err)

	// Verificar Match (mínimo)
	require.NotNil(t, ast.Match)
	require.Len(t, ast.Match.Patterns, 1)
	require.Equal(t, "a", ast.Match.Patterns[0].StartNode.Variable)

	// Verificar Where (deve ser nil)
	require.Nil(t, ast.Where)

	// Verificar Create
	require.NotNil(t, ast.Create)
	require.Len(t, ast.Create.Patterns, 1)
	p1_c := ast.Create.Patterns[0]
	require.Equal(t, "n", p1_c.StartNode.Variable)
	require.Equal(t, "Person", p1_c.StartNode.Label)
	require.Len(t, p1_c.Segments, 0)
	require.NotNil(t, p1_c.StartNode.Properties)
	require.Len(t, p1_c.StartNode.Properties.Entries, 1)
	require.Equal(t, "name", p1_c.StartNode.Properties.Entries[0].Key)
	require.Equal(t, "Charles", p1_c.StartNode.Properties.Entries[0].Value)

	// Verificar Return
	require.NotNil(t, ast.Return)
	require.Equal(t, []string{"n"}, ast.Return.Fields)
}

func TestMatchAndCreate(t *testing.T) {
	parser := BuildParser[Query]()
	src := `
        MATCH (user:User { email: "test@example.com" })
        CREATE (user)-[r:LOGGED_IN { timestamp: "1234567890" }]->(session:Session)
        RETURN user, r, session
    `
	ast, err := parser.ParseString("", src)
	require.NoError(t, err)

	// Verificar Match
	require.NotNil(t, ast.Match)
	require.Len(t, ast.Match.Patterns, 1)
	p1_m := ast.Match.Patterns[0]
	require.Equal(t, "user", p1_m.StartNode.Variable)
	require.Equal(t, "User", p1_m.StartNode.Label)
	require.NotNil(t, p1_m.StartNode.Properties) // Verifica props no MATCH
	require.Len(t, p1_m.StartNode.Properties.Entries, 1)
	require.Equal(t, "email", p1_m.StartNode.Properties.Entries[0].Key)
	require.Equal(t, "test@example.com", p1_m.StartNode.Properties.Entries[0].Value)

	// Verificar Where (nil)
	require.Nil(t, ast.Where)

	// Verificar Create
	require.NotNil(t, ast.Create)
	require.Len(t, ast.Create.Patterns, 1)
	p1_c := ast.Create.Patterns[0]
	require.Equal(t, "user", p1_c.StartNode.Variable)
	require.Equal(t, "", p1_c.StartNode.Label)
	require.Nil(t, p1_c.StartNode.Properties)

	require.Len(t, p1_c.Segments, 1)
	seg1_c := p1_c.Segments[0]
	require.Equal(t, "-", seg1_c.Relationship.LeftArrow)
	require.Equal(t, "->", seg1_c.Relationship.RightArrow)
	require.Equal(t, "r", seg1_c.Relationship.Edge.Variable) // Alias 'r'
	require.Equal(t, "LOGGED_IN", seg1_c.Relationship.Edge.Type)
	// TODO: Adicionar verificação de propriedades da relação se suportado

	require.Equal(t, "session", seg1_c.EndNode.Variable)
	require.Equal(t, "Session", seg1_c.EndNode.Label)
	require.Nil(t, seg1_c.EndNode.Properties)

	require.NotNil(t, ast.Return)
	require.Equal(t, []string{"user", "r", "session"}, ast.Return.Fields)

}

func TestInvalidClauseOrder_CreateBeforeMatch(t *testing.T) {
	parser := BuildParser[Query]()

	src := `CREATE (n:Node) MATCH (n) RETURN n`
	_, err := parser.ParseString("", src)
	require.Error(t, err, "CREATE before MATCH should fail with the full query parser")
}
