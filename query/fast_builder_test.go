package query

import (
	"strings"
	"testing"
)

func TestNewFastBuilder(t *testing.T) {
	b := NewFastBuilder[testUser](&mockExecutor{}, &mockDialect{}, "users")
	if b == nil {
		t.Fatal("NewFastBuilder returned nil")
	}
	if b.columns != "*" {
		t.Errorf("columns = %q, want '*'", b.columns)
	}
	if b.fieldMap == nil {
		t.Error("fieldMap should not be nil")
	}
}

func TestFastBuilder_Select(t *testing.T) {
	b := NewFastBuilder[testUser](&mockExecutor{}, &mockDialect{}, "users")
	b.Select("name", "email")
	if b.columns != "name, email" {
		t.Errorf("columns = %q, want 'name, email'", b.columns)
	}
}

func TestFastBuilder_Where(t *testing.T) {
	b := NewFastBuilder[testUser](&mockExecutor{}, &mockDialect{}, "users")
	b.Where(Condition{Field: "name", Operator: OpEq, Value: "John"})
	if len(b.conditions) != 1 {
		t.Errorf("conditions len = %d, want 1", len(b.conditions))
	}
	if len(b.args) != 1 {
		t.Errorf("args len = %d, want 1", len(b.args))
	}
}

func TestFastBuilder_Where_ConditionGroup(t *testing.T) {
	b := NewFastBuilder[testUser](&mockExecutor{}, &mockDialect{}, "users")
	b.Where(ConditionGroup{
		Conditions: []interface{}{
			Condition{Field: "a", Operator: OpEq, Value: 1},
			Condition{Field: "b", Operator: OpEq, Value: 2},
		},
		Operator: LogicalOr,
	})
	if len(b.conditions) != 1 {
		t.Errorf("conditions len = %d, want 1", len(b.conditions))
	}
}

func TestFastBuilder_Where_IsNull(t *testing.T) {
	b := NewFastBuilder[testUser](&mockExecutor{}, &mockDialect{}, "users")
	b.Where(Condition{Field: "deleted_at", Operator: OpIsNull})
	if len(b.conditions) != 1 {
		t.Errorf("conditions len = %d, want 1", len(b.conditions))
	}
	if !strings.Contains(b.conditions[0], "IS NULL") {
		t.Errorf("condition should contain IS NULL, got %q", b.conditions[0])
	}
}

func TestFastBuilder_Where_IsNotNull(t *testing.T) {
	b := NewFastBuilder[testUser](&mockExecutor{}, &mockDialect{}, "users")
	b.Where(Condition{Field: "name", Operator: OpIsNotNull})
	if !strings.Contains(b.conditions[0], "IS NOT NULL") {
		t.Errorf("condition should contain IS NOT NULL, got %q", b.conditions[0])
	}
}

func TestFastBuilder_Where_In(t *testing.T) {
	b := NewFastBuilder[testUser](&mockExecutor{}, &mockDialect{}, "users")
	b.Where(Condition{Field: "id", Operator: OpIn, Value: []int{1, 2, 3}})
	if !strings.Contains(b.conditions[0], "IN (") {
		t.Errorf("condition should contain IN, got %q", b.conditions[0])
	}
}

func TestFastBuilder_Where_NotIn(t *testing.T) {
	b := NewFastBuilder[testUser](&mockExecutor{}, &mockDialect{}, "users")
	b.Where(Condition{Field: "id", Operator: OpNotIn, Value: []int{1, 2}})
	if !strings.Contains(b.conditions[0], "NOT IN (") {
		t.Errorf("condition should contain NOT IN, got %q", b.conditions[0])
	}
}

func TestFastBuilder_Where_Between(t *testing.T) {
	b := NewFastBuilder[testUser](&mockExecutor{}, &mockDialect{}, "users")
	b.Where(Condition{Field: "age", Operator: OpBetween, Value: []int{18, 65}})
	if !strings.Contains(b.conditions[0], "BETWEEN") {
		t.Errorf("condition should contain BETWEEN, got %q", b.conditions[0])
	}
}

func TestFastBuilder_Where_Between_Invalid(t *testing.T) {
	b := NewFastBuilder[testUser](&mockExecutor{}, &mockDialect{}, "users")
	b.Where(Condition{Field: "age", Operator: OpBetween, Value: []int{18}})
	_ = b.conditions // Between with single value may add empty condition
}

func TestFastBuilder_OrderByAsc(t *testing.T) {
	b := NewFastBuilder[testUser](&mockExecutor{}, &mockDialect{}, "users")
	b.OrderByAsc("name")
	sql := b.buildSQL()
	if !strings.Contains(sql, "ASC") {
		t.Errorf("SQL = %q, should contain ASC", sql)
	}
}

func TestFastBuilder_OrderByDesc(t *testing.T) {
	b := NewFastBuilder[testUser](&mockExecutor{}, &mockDialect{}, "users")
	b.OrderByDesc("name")
	sql := b.buildSQL()
	if !strings.Contains(sql, "DESC") {
		t.Errorf("SQL = %q, should contain DESC", sql)
	}
}

func TestFastBuilder_Limit(t *testing.T) {
	b := NewFastBuilder[testUser](&mockExecutor{}, &mockDialect{}, "users")
	b.Limit(10)
	sql := b.buildSQL()
	if !strings.Contains(sql, "LIMIT 10") {
		t.Errorf("SQL = %q, should contain LIMIT 10", sql)
	}
}

func TestFastBuilder_Offset(t *testing.T) {
	b := NewFastBuilder[testUser](&mockExecutor{}, &mockDialect{}, "users")
	b.Offset(20)
	sql := b.buildSQL()
	if !strings.Contains(sql, "OFFSET 20") {
		t.Errorf("SQL = %q, should contain OFFSET 20", sql)
	}
}

func TestFastBuilder_BuildSQL(t *testing.T) {
	b := NewFastBuilder[testUser](&mockExecutor{}, &mockDialect{}, "users")
	b.Select("name", "email").
		Where(Condition{Field: "age", Operator: OpGt, Value: 18}).
		OrderByAsc("name").
		Limit(10).
		Offset(5)

	sql := b.buildSQL()
	if !strings.HasPrefix(sql, "SELECT name, email FROM") {
		t.Errorf("SQL prefix wrong: %q", sql)
	}
	if !strings.Contains(sql, "WHERE") {
		t.Errorf("SQL should contain WHERE: %q", sql)
	}
	if !strings.Contains(sql, "LIMIT 10") {
		t.Errorf("SQL should contain LIMIT: %q", sql)
	}
	if !strings.Contains(sql, "OFFSET 5") {
		t.Errorf("SQL should contain OFFSET: %q", sql)
	}
}

func TestFastInterfaceSlice(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected int
	}{
		{"[]string", []string{"a", "b"}, 2},
		{"[]int", []int{1, 2, 3}, 3},
		{"[]int64", []int64{1, 2}, 2},
		{"[]interface{}", []interface{}{1, "a"}, 2},
		{"single value", 42, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fastInterfaceSlice(tt.input)
			if len(result) != tt.expected {
				t.Errorf("fastInterfaceSlice() len = %d, want %d", len(result), tt.expected)
			}
		})
	}
}

func TestItoa(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{100, "100"},
		{1000, "1000"},
		{1001, "1001"}, // Above cache limit
		{-1, "-1"},
	}

	for _, tt := range tests {
		result := itoa(tt.input)
		if result != tt.expected {
			t.Errorf("itoa(%d) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestUnsafeString(t *testing.T) {
	b := []byte("hello")
	s := unsafeString(b)
	if s != "hello" {
		t.Errorf("unsafeString = %q, want 'hello'", s)
	}
}

func TestUnsafeBytes(t *testing.T) {
	s := "hello"
	b := unsafeBytes(s)
	if string(b) != "hello" {
		t.Errorf("unsafeBytes = %q, want 'hello'", string(b))
	}
}

func TestClearStmtCache(t *testing.T) {
	// Just verify it doesn't panic
	ClearStmtCache()
}

func TestFastBuilder_BuildConditionGroup_Nested(t *testing.T) {
	b := NewFastBuilder[testUser](&mockExecutor{}, &mockDialect{}, "users")
	b.Where(ConditionGroup{
		Conditions: []interface{}{
			Condition{Field: "a", Operator: OpEq, Value: 1},
			ConditionGroup{
				Conditions: []interface{}{
					Condition{Field: "b", Operator: OpEq, Value: 2},
					Condition{Field: "c", Operator: OpEq, Value: 3},
				},
				Operator: LogicalOr,
			},
		},
		Operator: LogicalAnd,
	})
	if len(b.conditions) != 1 {
		t.Errorf("conditions len = %d, want 1", len(b.conditions))
	}
}
