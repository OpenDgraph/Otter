package parsing

import "github.com/alecthomas/participle/v2/lexer"

// ==================
// AST
// ==================

type Query struct {
	Match  *MatchClause  `"MATCH" @@`
	Where  *WhereClause  `[ "WHERE" @@ ]`
	Return *ReturnClause `"RETURN" @@`
}

// ==================
// MATCH
// ==================

type MatchClause struct {
	Node      *Node          `"(" @@ ")"` // Starting node
	Relations []*PathSegment `{ @@ }`     // Zero or more path segments
}

type PathSegment struct {
	Edge *Edge `"-" "[" ":" @@ "]" "-" ">"` // Match "-" token then ">" token
	Node *Node `"(" @@ ")"`                 // Target node of the segment
}

type RelationClause struct {
	From   *Node  `"(" @@ ")"`
	Arrow1 string `@Punct` // Captures "-"
	Rel    *Edge  `"[" ":" @@ "]"`
	Arrow2 string `@Punct` // Captures "->" as two tokens
	To     *Node  `"(" @@ ")"`
}

type Node struct {
	Variable string `@Ident`
	Label    string `[ ":" @Ident ]`
}

type Edge struct {
	Type string `@Ident`
}

// ==================
// WHERE
// ==================

type WhereClause struct {
	Cond *Condition `@@`
}

type Condition struct {
	Left     *PropertyAccess `@@`
	Operator string          `@Operator`
	Right    string          `@String`
}

type PropertyAccess struct {
	Object string `@Ident`
	Dot    string `@Punct` // Captures '.'
	Field  string `@Ident`
}

// ==================
// RETURN
// ==================
type ReturnClause struct {
	Fields []string `@Ident { "," @Ident }`
}

// ==================
// Custom Lexer
// ==================

var myLexer = lexer.MustSimple([]lexer.SimpleRule{
	{Name: "Ident", Pattern: `[a-zA-Z_][a-zA-Z0-9_]*`},
	{Name: "String", Pattern: `"[^"]*"`},
	{Name: "Operator", Pattern: `<>|<=|>=|=|<|>`},
	{Name: "Punct", Pattern: `[-:\[\]\(\),>.]`},
	{Name: "Whitespace", Pattern: `\s+`},
})
