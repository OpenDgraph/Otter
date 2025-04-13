package parsing

import (
	"fmt"
	"strings"

	"github.com/alecthomas/participle/v2"
)

var (
	matchParser  = mustBuildParser[MatchClause]()
	whereParser  = mustBuildParser[WhereClause]()
	returnParser = mustBuildParser[ReturnClause]()
)

func mustBuildParser[T any]() *participle.Parser[T] {
	p, err := participle.Build[T](
		participle.Lexer(myLexer),
		participle.Elide("Whitespace"),
		participle.Unquote("String"),
	)
	if err != nil {
		panic(fmt.Errorf("failed to build parser for %T: %w", *new(T), err))
	}
	return p
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

// Full dispatcher for breaking a query into parts and parsing them
func ParseQueryParts(query string) (*MatchClause, *WhereClause, *ReturnClause, error) {
	query = strings.TrimSpace(query)

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
