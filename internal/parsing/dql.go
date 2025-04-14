package parsing

import (
	"fmt"
	"strings"

	dqlpkg "github.com/hypermodeinc/dgraph/v24/dql"
)

// ! TODO: Remove AST from returning later
func RenderQuery(query string, prefix string, returnAST bool) (string, *dqlpkg.Result) {
	AST, err := ParseQuery(query)
	if err != nil {
		fmt.Printf("error")
		return "", nil
	}

	var out strings.Builder
	for _, q := range AST.Query {
		out.WriteString(renderBlock(q, "  ", prefix+"."))
	}

	if returnAST {
		return out.String(), &AST
	}
	return out.String(), nil
}

func renderBlock(gq *dqlpkg.GraphQuery, indent string, prefix string) string {
	if gq == nil {
		return ""
	}
	var b strings.Builder

	if gq.Alias != "" {
		fmt.Fprintf(&b, "  %s", gq.Alias)
	}

	if gq.Func != nil {
		prefixed := prefix + gq.Func.Attr
		fmt.Fprintf(&b, "(func: %s(%s))", gq.Func.Name, prefixed)
	}

	b.WriteString(" {\n")
	childIndent := indent + "  "
	for _, child := range gq.Children {
		if child.Attr != "" {
			// Se tiver Attr, aplicamos o prefixo
			prefixed := prefix + child.Attr
			// Se for igual, imprime direto, senão usa alias
			if prefixed != child.Attr {
				fmt.Fprintf(&b, "%s%s : %s\n", childIndent, prefixed, child.Attr)
			} else {
				fmt.Fprintf(&b, "%s%s\n", childIndent, child.Attr)
			}
		} else {
			b.WriteString(renderBlock(child, childIndent, prefix))
		}
	}
	b.WriteString("  }\n")

	return "{\n" + b.String() + "}\n"
}
