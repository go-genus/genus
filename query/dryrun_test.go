package query

import (
	"strings"
	"testing"
)

func TestDryRunResult_String(t *testing.T) {
	r := DryRunResult{
		SQL:          "SELECT * FROM users WHERE age > $1",
		Args:         []interface{}{18},
		FormattedSQL: "SELECT * FROM users WHERE age > 18",
		Operation:    "SELECT",
		Table:        "users",
	}

	s := r.String()
	if !strings.Contains(s, "Operation: SELECT") {
		t.Errorf("String() should contain Operation, got %q", s)
	}
	if !strings.Contains(s, "Table: users") {
		t.Errorf("String() should contain Table, got %q", s)
	}
	if !strings.Contains(s, "SQL:") {
		t.Errorf("String() should contain SQL, got %q", s)
	}
	if !strings.Contains(s, "Args:") {
		t.Errorf("String() should contain Args, got %q", s)
	}
	if !strings.Contains(s, "Formatted:") {
		t.Errorf("String() should contain Formatted, got %q", s)
	}
}

func TestDryRunResult_String_NoArgs(t *testing.T) {
	r := DryRunResult{
		SQL:       "SELECT * FROM users",
		Operation: "SELECT",
		Table:     "users",
	}
	s := r.String()
	if strings.Contains(s, "Args:") {
		t.Error("String() should not contain Args when empty")
	}
}

func TestDryRunBuilder_Select(t *testing.T) {
	b := newTestBuilder().
		Where(Condition{Field: "age", Operator: OpGt, Value: 18}).
		Limit(10)

	result := b.DryRun().Select()

	if result.Operation != "SELECT" {
		t.Errorf("Operation = %q, want SELECT", result.Operation)
	}
	if result.Table != "users" {
		t.Errorf("Table = %q, want users", result.Table)
	}
	if !strings.Contains(result.SQL, "SELECT * FROM") {
		t.Errorf("SQL should contain SELECT, got %q", result.SQL)
	}
	if !strings.Contains(result.SQL, "LIMIT 10") {
		t.Errorf("SQL should contain LIMIT, got %q", result.SQL)
	}
	if len(result.Args) != 1 {
		t.Errorf("args len = %d, want 1", len(result.Args))
	}
}

func TestDryRunBuilder_Count(t *testing.T) {
	b := newTestBuilder().
		Where(Condition{Field: "active", Operator: OpEq, Value: true}).
		OrderByDesc("name").
		Limit(10)

	result := b.DryRun().Count()

	if result.Operation != "COUNT" {
		t.Errorf("Operation = %q, want COUNT", result.Operation)
	}
	if !strings.Contains(result.SQL, "COUNT(*)") {
		t.Errorf("SQL should contain COUNT(*), got %q", result.SQL)
	}
	// Count should not have ORDER BY or LIMIT
	if strings.Contains(result.SQL, "ORDER BY") {
		t.Errorf("Count SQL should not have ORDER BY, got %q", result.SQL)
	}
	if strings.Contains(result.SQL, "LIMIT") {
		t.Errorf("Count SQL should not have LIMIT, got %q", result.SQL)
	}
}

func TestDryRunBuilder_First(t *testing.T) {
	b := newTestBuilder()
	result := b.DryRun().First()

	if !strings.Contains(result.SQL, "LIMIT 1") {
		t.Errorf("First() SQL should contain LIMIT 1, got %q", result.SQL)
	}
}

func TestDryRunBuilder_Explain(t *testing.T) {
	b := newTestBuilder()
	result := b.DryRun().Explain()

	if result.Operation != "EXPLAIN" {
		t.Errorf("Operation = %q, want EXPLAIN", result.Operation)
	}
	if !strings.HasPrefix(result.SQL, "EXPLAIN ") {
		t.Errorf("SQL should start with EXPLAIN, got %q", result.SQL)
	}
}

func TestDryRunBuilder_ExplainAnalyze(t *testing.T) {
	b := newTestBuilder()
	result := b.DryRun().ExplainAnalyze()

	if result.Operation != "EXPLAIN ANALYZE" {
		t.Errorf("Operation = %q, want EXPLAIN ANALYZE", result.Operation)
	}
	if !strings.HasPrefix(result.SQL, "EXPLAIN ANALYZE ") {
		t.Errorf("SQL should start with EXPLAIN ANALYZE, got %q", result.SQL)
	}
}

func TestFormatSQL(t *testing.T) {
	tests := []struct {
		name     string
		sql      string
		args     []interface{}
		contains string
	}{
		{"no args", "SELECT * FROM users", nil, "SELECT * FROM users"},
		{"string arg", "SELECT * FROM users WHERE name = $1", []interface{}{"John"}, "'John'"},
		{"int arg", "SELECT * FROM users WHERE age = $1", []interface{}{18}, "18"},
		{"nil arg", "SELECT * FROM users WHERE deleted_at = $1", []interface{}{nil}, "NULL"},
		{"mysql placeholder", "SELECT * FROM users WHERE name = ?", []interface{}{"John"}, "'John'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatSQL(tt.sql, tt.args)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("formatSQL() = %q, should contain %q", result, tt.contains)
			}
		})
	}
}

func TestFormatSQL_StringEscaping(t *testing.T) {
	result := formatSQL("SELECT * WHERE name = $1", []interface{}{"O'Brien"})
	if !strings.Contains(result, "O''Brien") {
		t.Errorf("formatSQL should escape single quotes, got %q", result)
	}
}

func TestBuilder_ToSQL(t *testing.T) {
	b := newTestBuilder().
		Where(Condition{Field: "name", Operator: OpEq, Value: "John"})

	sql, args := b.ToSQL()
	if !strings.Contains(sql, "SELECT") {
		t.Errorf("ToSQL() SQL should contain SELECT, got %q", sql)
	}
	if len(args) != 1 {
		t.Errorf("ToSQL() args len = %d, want 1", len(args))
	}
}

func TestBuilder_BuildDryRunSelectQuery_WithOrderOffset(t *testing.T) {
	b := newTestBuilder().
		Where(Condition{Field: "age", Operator: OpGt, Value: 18}).
		OrderByAsc("name").
		OrderByDesc("age").
		Limit(10).
		Offset(20)

	sql, args := b.buildDryRunSelectQuery()
	if !strings.Contains(sql, "ORDER BY name ASC, age DESC") {
		t.Errorf("SQL should contain ORDER BY, got %q", sql)
	}
	if !strings.Contains(sql, "LIMIT 10") {
		t.Errorf("SQL should contain LIMIT, got %q", sql)
	}
	if !strings.Contains(sql, "OFFSET 20") {
		t.Errorf("SQL should contain OFFSET, got %q", sql)
	}
	if len(args) != 1 {
		t.Errorf("args len = %d, want 1", len(args))
	}
}

func TestBuilder_BuildDryRunSelectQuery_WithSelectCols(t *testing.T) {
	b := newTestBuilder().Select("name", "email")
	sql, _ := b.buildDryRunSelectQuery()
	if !strings.HasPrefix(sql, "SELECT name, email FROM") {
		t.Errorf("SQL should start with SELECT name, email, got %q", sql)
	}
}

func TestBuilder_BuildDryRunSelectQuery_WithJoins(t *testing.T) {
	b := newTestBuilder()
	b.joins = append(b.joins, JoinClause{
		Type:  LeftJoinType,
		Table: "posts",
		Alias: "p",
		Condition: JoinCondition{
			LeftColumn:  "users.id",
			RightColumn: "p.user_id",
			Operator:    OpEq,
		},
	})

	sql, _ := b.buildDryRunSelectQuery()
	if !strings.Contains(sql, "LEFT JOIN") {
		t.Errorf("SQL should contain LEFT JOIN, got %q", sql)
	}
	if !strings.Contains(sql, "AS p") {
		t.Errorf("SQL should contain AS p, got %q", sql)
	}
}
