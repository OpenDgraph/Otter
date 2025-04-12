package parsing

import "github.com/alecthomas/participle/v2/lexer"

// ==================
// AST (com relacionamentos)
// ==================

type Query struct {
	Match  *MatchClause  `"MATCH" @@`
	Where  *WhereClause  `[ "WHERE" @@ ]`
	Return *ReturnClause `"RETURN" @@`
}

type MatchClause struct {
	Node      *Node          `"(" @@ ")"`
	Relations []*PathSegment `{ @@ }`
}

// type PathSegment struct {
// 	Arrow1 string `@Punct` // -
// 	Open   string `@Punct` // [
// 	Colon  string `@Punct` // :
// 	Edge   string `@Ident` // FRIEND
// 	Close  string `@Punct` // ]
// 	Arrow2 string `@Punct` // ->
// 	Node   *Node  `"(" @@ ")"`
// }

type PathSegment struct {
	Edge string `"-" "[" ":" @Ident "]" "->"`
	Node *Node  `"(" @@ ")"`
}

type RelationClause struct {
	From   *Node  `"(" @@ ")"`
	Arrow1 string `@Punct` // captura "-"
	Rel    *Edge  `"[" ":" @@ "]"`
	Arrow2 string `@Punct` // captura "->" com dois tokens
	To     *Node  `"(" @@ ")"`
}

type Node struct {
	Variable string `@Ident`
	Label    string `[ ":" @Ident ]`
}

type ReturnClause struct {
	Fields []string `@Ident { "," @Ident }`
}

type WhereClause struct {
	LeftObj  string     `@Ident`
	Dot      string     `@Punct`
	LeftKey  string     `@Ident`
	Operator string     `@Operator`
	Right    string     `@String`
	Cond     *Condition `@@`
}

type Edge struct {
	Type string `@Ident`
}

type Relation struct {
	From *Node `"(" @@ ")"`
	Edge *Edge `"-" "[" ":" @Ident "]" "->"`
	To   *Node `"(" @@ ")"`
}

type Condition struct {
	Left     *PropertyAccess `@@`
	Operator string          `@Operator`
	Right    string          `@String`
}

type PropertyAccess struct {
	Object string `@Ident`
	Dot    string `@Punct`
	Field  string `@Ident`
}

// ==================
// Lexer customizado
// ==================

var myLexer = lexer.MustSimple([]lexer.SimpleRule{
	{Name: "Ident", Pattern: `[a-zA-Z_][a-zA-Z0-9_]*`},
	{Name: "String", Pattern: `"[^"]*"`},
	{Name: "Operator", Pattern: `<>|<=|>=|=|<|>`},
	{Name: "Punct", Pattern: `[-:\[\]\(\),>.]`},
	{Name: "Whitespace", Pattern: `\s+`},
})

// ==================
// Main
// ==================

// func Parsi() {
// 	parser, err := participle.Build[Query](
// 		participle.Lexer(myLexer),
// 		participle.Unquote("String"),
// 		participle.Elide("Whitespace"),
// 	)
// 	if err != nil {
// 		panic(err)
// 	}

// 	src := `MATCH (a:Person)-[:FRIEND]->(b:Person) WHERE name = "Alice" RETURN a, b`
// 	ast, err := parser.ParseString("", src)
// 	if err != nil {
// 		fmt.Println("Erro ao parsear:", err)
// 		os.Exit(1)
// 	}

// 	fmt.Printf("Parsed AST:\n%+v\n", ast)
// }
