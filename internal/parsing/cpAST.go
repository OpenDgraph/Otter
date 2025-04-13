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
	Patterns []*Pattern `@@ { "," @@ }`
}

type Pattern struct {
	StartNode *NodePattern   `"(" @@ ")"` // O padrão DEVE começar com um nó
	Segments  []*PathSegment ` { @@ } `   // Segmentos de relação/nó subsequentes
}

type NodePattern struct {
	Variable   string      `@Ident`
	Label      string      `[ ":" @Ident ]`
	Properties *Properties `[ @@ ]` // Propriedades são opcionais e definidas em sua própria struct
}

type PathSegment struct {
	Relationship *RelationshipPattern `@@`         // Detalhes da relação (setas, tipo, alias)
	EndNode      *NodePattern         `"(" @@ ")"` // O nó no final deste segmento
}

type RelationshipPattern struct {
	LeftArrow  string       `(@ArrowL | @Punct)` // Captura '<-' ou '-' (Punct aqui DEVE ser '-')
	Edge       *EdgePattern `"[" @@ "]"`         // Detalhes dentro dos colchetes
	RightArrow string       `(@ArrowR | @Punct)` // Captura '->' ou '-' (Punct aqui DEVE ser '-')
}

type EdgePattern struct {
	Variable string `@Ident?`    // Alias opcional (e.g., 'r' em [r:KNOWS])
	Type     string `":" @Ident` // Tipo da relação (e.g., 'KNOWS')
}

type Properties struct {
	Entries []*Property `"{" @@ { "," @@ } "}"`
}

type Property struct {
	Key   string `@Ident ":"`
	Value string `@String` // Por agora, apenas valores string. Poderia ser estendido.
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
	Right    string          `@String` // Ou outros tipos de valor
}

type PropertyAccess struct {
	Object string `@Ident`
	Dot    string `@Punct` // Captura o '.'
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
	{Name: "ArrowL", Pattern: `<-`},
	{Name: "ArrowR", Pattern: `->`},
	{Name: "Ident", Pattern: `[a-zA-Z_][a-zA-Z0-9_]*`},
	{Name: "String", Pattern: `"[^"]*"`},
	{Name: "Operator", Pattern: `<>|<=|>=|=|<|>`},
	{Name: "Punct", Pattern: `[-:\[\]\(\),\{\}.]`},
	{Name: "Whitespace", Pattern: `\s+`},
	{Name: "comment", Pattern: `/\*.*?\*/`},
	{Name: "line_comment", Pattern: `//[^\n]*`},
})
