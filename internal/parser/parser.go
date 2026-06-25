package parser

import (
	"fmt"

	pg_query "github.com/pganalyze/pg_query_go/v6"
)

type Statement struct {
	Raw      string
	Node     *pg_query.Node
	Position int
}

func ParseStatements(sql string) ([]Statement, error) {
	result, err := pg_query.Parse(sql)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	stmts := make([]Statement, 0, len(result.Stmts))
	for _, raw := range result.Stmts {
		stmts = append(stmts, Statement{
			Raw:      deparse(raw.Stmt),
			Node:     raw.Stmt,
			Position: int(raw.StmtLocation),
		})
	}
	return stmts, nil
}

func deparse(node *pg_query.Node) string {
	result, err := pg_query.Deparse(&pg_query.ParseResult{
		Stmts: []*pg_query.RawStmt{{Stmt: node}},
	})
	if err != nil {
		return "<unparseable>"
	}
	return result
}
