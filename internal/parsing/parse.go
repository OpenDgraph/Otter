package parsing

import (
	"fmt"

	dqlpkg "github.com/hypermodeinc/dgraph/v24/dql"
	// _ "github.com/hypermodeinc/dgraph/v24/dql"
	schemapkg "github.com/hypermodeinc/dgraph/v24/schema"
)

func ParseQuery(query string) (dqlpkg.Result, error) {

	AST, err := dqlpkg.Parse(dqlpkg.Request{Str: query})
	if err != nil {
		return dqlpkg.Result{}, fmt.Errorf("failed to parse DQL query: %w", err)
	}

	// fmt.Println("Parsed DQL Query:", AST)
	return AST, nil
}
func ParseSchema(schema string) (schemapkg.ParsedSchema, error) {

	AST, err := schemapkg.Parse(schema)
	if err != nil {
		return schemapkg.ParsedSchema{}, fmt.Errorf("failed to parse DQL schema: %w", err)
	}

	// fmt.Println("Parsed DQL schema:", AST)
	return *AST, nil
}

func ParseMutation(mutation string) (dqlpkg.Result, error) {
	AST, err := dqlpkg.Parse(dqlpkg.Request{Str: mutation})
	if err != nil {
		return dqlpkg.Result{}, fmt.Errorf("failed to parse DQL mutation: %w", err)
	}

	// fmt.Println("Parsed DQL mutation:", AST)
	return AST, nil
}
