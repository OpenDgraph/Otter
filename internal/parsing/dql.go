package parsing

import (
	"fmt"
	"strings"

	dqlpkg "github.com/hypermodeinc/dgraph/v24/dql"
)

func RenderQuery(ast dqlpkg.Result, prefix string) string {
	var out strings.Builder
	for _, q := range ast.Query {
		out.WriteString(renderBlock(q, "  ", prefix+"."))
	}
	return out.String()
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
