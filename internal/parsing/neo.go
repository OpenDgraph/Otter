package parsing

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alecthomas/participle/v2"
)

var (
	matchParser  = mustBuildParser[MatchClause]()
	whereParser  = mustBuildParser[WhereClause]()
	returnParser = mustBuildParser[ReturnClause]()
	createParser = mustBuildParser[CreateClause]()
)

func mustBuildParser[T any]() *participle.Parser[T] {
	return BuildParser[T]()
}

func ParseMatchClause(src string) (*MatchClause, error) {
	return matchParser.ParseString("", src)
}

func ParseWhereClause(src string) (*WhereClause, error) {
	return whereParser.ParseString("", src)
}

func ParseReturnClause(src string) (*ReturnClause, error) {
	return returnParser.ParseString("", src)
}

func ParseCreateClause(src string) (*CreateClause, error) {
	return createParser.ParseString("", src)
}

// Full dispatcher for breaking a query into parts and parsing them
func ParseQueryParts(query string) (*MatchClause, *WhereClause, *ReturnClause, error) {
	query = strings.TrimSpace(query)
	upper := strings.ToUpper(query)

	switch {
	case strings.HasPrefix(upper, "CREATE"):
		createClause, err := ParseCreateClause(query)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("create parse error: %w", err)
		}
		// Se quiser devolver CREATE como resultado exclusivo:
		return nil, nil, nil, fmt.Errorf("CREATE clause parsed: %+v", createClause) // ou outro retorno que faça sentido

	default:

		matchIndex := strings.Index(query, "MATCH")
		whereIndex := strings.Index(query, "WHERE")
		returnIndex := strings.Index(query, "RETURN")

		if matchIndex == -1 {
			return nil, nil, nil, fmt.Errorf("invalid query: must start with MATCH")
		}

		if returnIndex == -1 {
			return nil, nil, nil, fmt.Errorf("invalid query: must contain RETURN")
		}

		matchEnd := len(query)
		if whereIndex != -1 {
			matchEnd = whereIndex
		} else if returnIndex != -1 {
			matchEnd = returnIndex
		}

		matchPart := query[matchIndex+len("MATCH") : matchEnd]

		var wherePart string
		if whereIndex != -1 {
			whereEnd := len(query)
			if returnIndex != -1 && returnIndex > whereIndex {
				whereEnd = returnIndex
			} else if returnIndex != -1 && returnIndex < whereIndex {
				return nil, nil, nil, fmt.Errorf("invalid query: RETURN cannot come before WHERE")
			}
			wherePart = query[whereIndex+len("WHERE") : whereEnd]
		}

		var returnPart string
		if returnIndex != -1 {
			returnPart = query[returnIndex+len("RETURN"):]
		} else {
			return nil, nil, nil, fmt.Errorf("invalid query: must contain RETURN")
		}

		matchClause, err := ParseMatchClause(matchPart)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("match parse error: %w", err)
		}

		var whereClause *WhereClause
		if wherePart != "" {
			whereClause, err = ParseWhereClause(wherePart)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("where parse error: %w", err)
			}
		}

		returnClause, err := ParseReturnClause(returnPart)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("return parse error: %w", err)
		}

		return matchClause, whereClause, returnClause, nil
	}
}

type ASTSnapshot struct {
	Test  string      `json:"test"`
	Input string      `json:"input"`
	AST   interface{} `json:"ast"`
}

func saveASTSnapshot(t *testing.T, snapshot ASTSnapshot, path string) {
	t.Helper()

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create directory %s: %v", dir, err)
	}

	var snapshots []ASTSnapshot
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &snapshots)
	}

	snapshots = append(snapshots, snapshot)

	data, err := json.MarshalIndent(snapshots, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal snapshot: %v", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write snapshot file: %v", err)
	}
}
