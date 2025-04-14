package parsing

import (
	"fmt"

	"github.com/dgraph-io/dgo/v240/protos/api"
	dqlpkg "github.com/hypermodeinc/dgraph/v24/dql"

	schemapkg "github.com/hypermodeinc/dgraph/v24/schema"
)

type DQLSnapshot struct {
	Test  string                 `json:"test"`
	Input string                 `json:"input"`
	AST   map[string]interface{} `json:"ast"`
	NewQ  string                 `json:"newq"`
}

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

func ParseMutation(mutation string) (*api.Request, error) {
	AST, err := dqlpkg.ParseMutation(mutation)
	if err != nil {
		return nil, fmt.Errorf("failed to parse DQL mutation: %w", err)
	}
	return AST, nil
}
