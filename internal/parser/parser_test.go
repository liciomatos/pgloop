package parser_test

import (
	"strings"
	"testing"

	"github.com/liciomatos/pgloop/internal/parser"
)

func TestParseEmpty(t *testing.T) {
	stmts, err := parser.ParseStatements("")
	if err != nil {
		t.Fatalf("empty SQL must not produce a parse error: %v", err)
	}
	if len(stmts) != 0 {
		t.Errorf("empty SQL must return 0 statements, got %d", len(stmts))
	}
}

func TestParseSyntaxError(t *testing.T) {
	_, err := parser.ParseStatements("ALTER TABLE INVALID SYNTAX %%%")
	if err == nil {
		t.Error("invalid SQL must return an error")
	}
}

func TestParseMultiStatement(t *testing.T) {
	sql := `
		SET lock_timeout = '3s';
		ALTER TABLE users ADD COLUMN score INT;
		CREATE INDEX CONCURRENTLY idx_users_score ON users(score);
	`
	stmts, err := parser.ParseStatements(sql)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(stmts) != 3 {
		t.Errorf("expected 3 statements, got %d", len(stmts))
	}
}

func TestParseStatementPositions(t *testing.T) {
	sql := "SELECT 1;\nSELECT 2;"
	stmts, err := parser.ParseStatements(sql)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(stmts) != 2 {
		t.Fatalf("expected 2 statements, got %d", len(stmts))
	}
	if stmts[0].Position != 0 {
		t.Errorf("first statement must be at position 0, got %d", stmts[0].Position)
	}
	if stmts[1].Position <= 0 {
		t.Errorf("second statement must have position > 0, got %d", stmts[1].Position)
	}
}

func TestParseRawIsCanonical(t *testing.T) {
	// Raw must be canonical SQL (deparsed), not the original with extra whitespace.
	sql := "ALTER   TABLE   users   ADD   COLUMN   score   INT"
	stmts, err := parser.ParseStatements(sql)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement")
	}
	// Canonical form has no double spaces.
	if strings.Contains(stmts[0].Raw, "  ") {
		t.Errorf("Raw must be canonical SQL without double spaces, got: %q", stmts[0].Raw)
	}
}
