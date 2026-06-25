package parser_test

import (
	"strings"
	"testing"

	"github.com/liciomatos/pgloop/internal/parser"
)

func TestParseEmpty(t *testing.T) {
	stmts, err := parser.ParseStatements("")
	if err != nil {
		t.Fatalf("SQL vazio não deve gerar erro: %v", err)
	}
	if len(stmts) != 0 {
		t.Errorf("SQL vazio deve retornar 0 statements, got %d", len(stmts))
	}
}

func TestParseSyntaxError(t *testing.T) {
	_, err := parser.ParseStatements("ALTER TABLE INVALID SYNTAX %%%")
	if err == nil {
		t.Error("SQL inválido deve retornar erro")
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
		t.Errorf("esperado 3 statements, got %d", len(stmts))
	}
}

func TestParseStatementPositions(t *testing.T) {
	sql := "SELECT 1;\nSELECT 2;"
	stmts, err := parser.ParseStatements(sql)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(stmts) != 2 {
		t.Fatalf("esperado 2 statements, got %d", len(stmts))
	}
	if stmts[0].Position != 0 {
		t.Errorf("primeiro statement deve estar na posição 0, got %d", stmts[0].Position)
	}
	if stmts[1].Position <= 0 {
		t.Errorf("segundo statement deve ter posição > 0, got %d", stmts[1].Position)
	}
}

func TestParseRawIsCanonical(t *testing.T) {
	// Raw deve ser SQL canônico (deparsed), não o original com formatação.
	sql := "ALTER   TABLE   users   ADD   COLUMN   score   INT"
	stmts, err := parser.ParseStatements(sql)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(stmts) != 1 {
		t.Fatalf("esperado 1 statement")
	}
	// Canonical form não tem espaços duplos.
	if strings.Contains(stmts[0].Raw, "  ") {
		t.Errorf("Raw deve ser SQL canônico sem espaços duplos, got: %q", stmts[0].Raw)
	}
}
