package astneo

import (
	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
)

// ==================
// AST
// ==================

type Query struct {
	Match  *MatchClause  `parser:"\"MATCH\" @@"`
	Create *CreateClause `parser:"[ \"CREATE\" @@ ]"`
	Where  *WhereClause  `parser:"[ \"WHERE\" @@ ]"`
	Return *ReturnClause `parser:"\"RETURN\" @@"`
}

// ==================
// CREATE
// ==================

type CreateClause struct {
	Patterns []*Pattern `parser:"@@ { \",\" @@ }"`
}

// ==================
// MATCH
// ==================

type MatchClause struct {
	Patterns []*Pattern `parser:"@@ { \",\" @@ }"`
}

type Pattern struct {
	StartNode *NodePattern   `parser:"\"(\" @@ \")\""` // O padrão DEVE começar com um nó
	Segments  []*PathSegment `parser:" { @@ } "`       // Segmentos de relação/nó subsequentes
}

type NodePattern struct {
	Variable   string      `parser:"@Ident"`
	Label      string      `parser:"[ \":\" @(Ident | Keyword) ]"`
	Properties *Properties `parser:"[ @@ ]"` // Propriedades são opcionais e definidas em sua própria struct
}

type PathSegment struct {
	Relationship *RelationshipPattern `parser:"@@"`             // Detalhes da relação (setas, tipo, alias)
	EndNode      *NodePattern         `parser:"\"(\" @@ \")\""` // O nó no final deste segmento
}

type RelationshipPattern struct {
	LeftArrow  string       `parser:"(@ArrowL | @Punct)"` // Captura '<-' ou '-' (Punct aqui DEVE ser '-')
	Edge       *EdgePattern `parser:"\"[\" @@ \"]\""`     // Detalhes dentro dos colchetes
	RightArrow string       `parser:"(@ArrowR | @Punct)"` // Captura '->' ou '-' (Punct aqui DEVE ser '-')
}

type EdgePattern struct {
	Variable   string      `parser:"@Ident?"`          // Alias opcional (e.g., 'r' em [r:KNOWS])
	Type       string      `parser:"[ \":\" @Ident ]"` // Tipo da relação (e.g., 'KNOWS')
	Properties *Properties `parser:"[ @@ ]"`
}

type Properties struct {
	Entries []*Property `parser:"\"{\" @@ { \",\" @@ } \"}\""`
}

type Property struct {
	Key   string `parser:"@Ident \":\""`
	Value string `parser:"@String"` // Por agora, apenas valores string. Poderia ser estendido.
}

// ==================
// WHERE
// ==================

type WhereClause struct {
	Cond *Condition `parser:"@@"`
}

type Condition struct {
	Left     *PropertyAccess `parser:"@@"`
	Operator string          `parser:"@Operator"`
	Right    string          `parser:"@String"` // Ou outros tipos de valor
}

type PropertyAccess struct {
	Object string `parser:"@Ident"`
	Dot    string `parser:"@Punct"` // Captura o '.'
	Field  string `parser:"@Ident"`
}

// ==================
// RETURN
// ==================
type ReturnClause struct {
	Fields []string `parser:"@Ident { \",\" @Ident }"`
}

// ==================
// Custom Lexer
// ==================

var myLexer = lexer.MustSimple([]lexer.SimpleRule{
	{Name: "Keyword", Pattern: `(?i)\b(MATCH|RETURN|WHERE|AND|OR|NOT|NULL|TRUE|FALSE|IN|IS|AS|WITH|UNWIND|OPTIONAL|DETACH|DELETE|SET|CREATE|MERGE|ON|CASE|WHEN|THEN|ELSE|DISTINCT|ORDER|BY|SKIP|LIMIT|ASC|DESC)\b`},
	{Name: "ArrowL", Pattern: `<-`},
	{Name: "ArrowR", Pattern: `->`},
	{Name: "Ident", Pattern: `[a-zA-Z_][a-zA-Z0-9_]*`},
	{Name: "String", Pattern: `'[^']*'|"[^"]*"`},
	{Name: "Operator", Pattern: `<>|<=|>=|=|<|>`},
	{Name: "Punct", Pattern: `[-:\[\]\(\),\{\}.]`},
	{Name: "Whitespace", Pattern: `\s+`},
	{Name: "comment", Pattern: `/\*.*?\*/`},
	{Name: "line_comment", Pattern: `//[^\n]*`},
})

func BuildParser[T any](options ...participle.Option) *participle.Parser[T] {
	defaultOptions := []participle.Option{
		participle.Lexer(myLexer),
		participle.Unquote("String"),
		participle.Elide("Whitespace", "comment", "line_comment"),
		participle.CaseInsensitive("Keyword"),
		participle.UseLookahead(2),
	}
	return participle.MustBuild[T](append(defaultOptions, options...)...)
}

func BuildQueryParser(options ...participle.Option) *participle.Parser[Query] {
	defaultOptions := []participle.Option{
		participle.Lexer(myLexer),
		participle.Unquote("String"),
		participle.Elide("Whitespace", "comment", "line_comment"),
		participle.CaseInsensitive("Keyword"),
		participle.UseLookahead(2),
	}
	parser := participle.MustBuild[Query](append(defaultOptions, options...)...)

	return parser
}
