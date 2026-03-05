package query

import (
	"strings"
	"testing"
)

func TestPrepareSelectAll_Basic(t *testing.T) {
	pq := PrepareSelectAll[testUser](
		&mockExecutor{},
		&mockDialect{},
		"users",
		nil,
		nil,
	)

	if pq == nil {
		t.Fatal("PrepareSelectAll returned nil")
	}

	sql := pq.SQL()
	if !strings.Contains(sql, "SELECT * FROM") {
		t.Errorf("SQL = %q, should contain SELECT * FROM", sql)
	}
	if !strings.Contains(sql, `"users"`) {
		t.Errorf("SQL = %q, should contain table name", sql)
	}
}

func TestPrepareSelectAll_WithColumns(t *testing.T) {
	pq := PrepareSelectAll[testUser](
		&mockExecutor{},
		&mockDialect{},
		"users",
		[]string{"name", "email"},
		nil,
	)

	sql := pq.SQL()
	if !strings.Contains(sql, "SELECT name, email") {
		t.Errorf("SQL = %q, should contain SELECT name, email", sql)
	}
}

func TestPrepareSelectAll_WithWhere(t *testing.T) {
	pq := PrepareSelectAll[testUser](
		&mockExecutor{},
		&mockDialect{},
		"users",
		nil,
		[]string{"name", "age"},
	)

	sql := pq.SQL()
	if !strings.Contains(sql, "WHERE") {
		t.Errorf("SQL = %q, should contain WHERE", sql)
	}
	if !strings.Contains(sql, "name = $1") {
		t.Errorf("SQL = %q, should contain name = $1", sql)
	}
	if !strings.Contains(sql, "age = $2") {
		t.Errorf("SQL = %q, should contain age = $2", sql)
	}
	if pq.numParams != 2 {
		t.Errorf("numParams = %d, want 2", pq.numParams)
	}
}

func TestPreparedQuery_SQL(t *testing.T) {
	pq := PrepareSelectAll[testUser](
		&mockExecutor{},
		&mockDialect{},
		"users",
		nil,
		nil,
	)

	sql := pq.SQL()
	if sql == "" {
		t.Error("SQL() should not return empty")
	}
}
