package query

import (
	"database/sql"
	"reflect"
	"strings"
	"testing"
)

func TestRegisterScanFunc(t *testing.T) {
	type TestModel struct {
		ID   int64  `db:"id"`
		Name string `db:"name"`
	}

	fn := func(rows *sql.Rows) (TestModel, error) {
		var m TestModel
		err := rows.Scan(&m.ID, &m.Name)
		return m, err
	}

	RegisterScanFunc(fn)

	// Verify it was stored
	typ := reflect.TypeOf(TestModel{})
	if _, ok := scanFuncRegistry.Load(typ); !ok {
		t.Error("scan func should be registered")
	}

	// Cleanup
	scanFuncRegistry.Delete(typ)
}

func TestNewUltraFastBuilder(t *testing.T) {
	b := NewUltraFastBuilder[testUser](&mockExecutor{}, &mockDialect{}, "users")
	if b == nil {
		t.Fatal("NewUltraFastBuilder returned nil")
	}
	if b.columns != "*" {
		t.Errorf("columns = %q, want '*'", b.columns)
	}
}

func TestUltraFastBuilder_WithScanFunc(t *testing.T) {
	b := NewUltraFastBuilder[testUser](&mockExecutor{}, &mockDialect{}, "users")
	fn := func(rows *sql.Rows) (testUser, error) {
		return testUser{}, nil
	}
	b2 := b.WithScanFunc(fn)
	if b2.scanFunc == nil {
		t.Error("scanFunc should be set")
	}
}

func TestUltraFastBuilder_Select(t *testing.T) {
	b := NewUltraFastBuilder[testUser](&mockExecutor{}, &mockDialect{}, "users")
	b.Select("name", "email")
	if b.columns != "name, email" {
		t.Errorf("columns = %q, want 'name, email'", b.columns)
	}
}

func TestUltraFastBuilder_Where(t *testing.T) {
	b := NewUltraFastBuilder[testUser](&mockExecutor{}, &mockDialect{}, "users")
	b.Where(Condition{Field: "name", Operator: OpEq, Value: "John"})
	if len(b.conditions) != 1 {
		t.Errorf("conditions len = %d, want 1", len(b.conditions))
	}
}

func TestUltraFastBuilder_Where_ConditionGroup(t *testing.T) {
	b := NewUltraFastBuilder[testUser](&mockExecutor{}, &mockDialect{}, "users")
	b.Where(ConditionGroup{
		Conditions: []interface{}{
			Condition{Field: "a", Operator: OpEq, Value: 1},
		},
		Operator: LogicalOr,
	})
	if len(b.conditions) != 1 {
		t.Errorf("conditions len = %d, want 1", len(b.conditions))
	}
}

func TestUltraFastBuilder_Where_IsNull(t *testing.T) {
	b := NewUltraFastBuilder[testUser](&mockExecutor{}, &mockDialect{}, "users")
	b.Where(Condition{Field: "deleted_at", Operator: OpIsNull})
	if !strings.Contains(b.conditions[0], "IS NULL") {
		t.Errorf("condition = %q, should contain IS NULL", b.conditions[0])
	}
}

func TestUltraFastBuilder_Where_IsNotNull(t *testing.T) {
	b := NewUltraFastBuilder[testUser](&mockExecutor{}, &mockDialect{}, "users")
	b.Where(Condition{Field: "name", Operator: OpIsNotNull})
	if !strings.Contains(b.conditions[0], "IS NOT NULL") {
		t.Errorf("condition = %q, should contain IS NOT NULL", b.conditions[0])
	}
}

func TestUltraFastBuilder_OrderByAsc(t *testing.T) {
	b := NewUltraFastBuilder[testUser](&mockExecutor{}, &mockDialect{}, "users")
	b.OrderByAsc("name")
	sql := b.buildSQL()
	if !strings.Contains(sql, "ASC") {
		t.Errorf("SQL = %q, should contain ASC", sql)
	}
}

func TestUltraFastBuilder_OrderByDesc(t *testing.T) {
	b := NewUltraFastBuilder[testUser](&mockExecutor{}, &mockDialect{}, "users")
	b.OrderByDesc("name")
	sql := b.buildSQL()
	if !strings.Contains(sql, "DESC") {
		t.Errorf("SQL = %q, should contain DESC", sql)
	}
}

func TestUltraFastBuilder_Limit(t *testing.T) {
	b := NewUltraFastBuilder[testUser](&mockExecutor{}, &mockDialect{}, "users")
	b.Limit(10)
	sql := b.buildSQL()
	if !strings.Contains(sql, "LIMIT 10") {
		t.Errorf("SQL = %q, should contain LIMIT 10", sql)
	}
}

func TestUltraFastBuilder_Offset(t *testing.T) {
	b := NewUltraFastBuilder[testUser](&mockExecutor{}, &mockDialect{}, "users")
	b.Offset(20)
	sql := b.buildSQL()
	if !strings.Contains(sql, "OFFSET 20") {
		t.Errorf("SQL = %q, should contain OFFSET 20", sql)
	}
}

func TestUltraFastBuilder_BuildSQL(t *testing.T) {
	b := NewUltraFastBuilder[testUser](&mockExecutor{}, &mockDialect{}, "users")
	b.Select("name").
		Where(Condition{Field: "age", Operator: OpGt, Value: 18}).
		OrderByDesc("age").
		Limit(5).
		Offset(10)

	sql := b.buildSQL()
	if !strings.HasPrefix(sql, "SELECT name FROM") {
		t.Errorf("SQL prefix: %q", sql)
	}
	if !strings.Contains(sql, "WHERE") {
		t.Errorf("SQL should contain WHERE: %q", sql)
	}
	if !strings.Contains(sql, "ORDER BY") {
		t.Errorf("SQL should contain ORDER BY: %q", sql)
	}
	if !strings.Contains(sql, "LIMIT 5") {
		t.Errorf("SQL should contain LIMIT: %q", sql)
	}
	if !strings.Contains(sql, "OFFSET 10") {
		t.Errorf("SQL should contain OFFSET: %q", sql)
	}
}

func TestUltraFastBuilder_BuildCondGroup_Nested(t *testing.T) {
	b := NewUltraFastBuilder[testUser](&mockExecutor{}, &mockDialect{}, "users")
	b.Where(ConditionGroup{
		Conditions: []interface{}{
			Condition{Field: "a", Operator: OpEq, Value: 1},
			ConditionGroup{
				Conditions: []interface{}{
					Condition{Field: "b", Operator: OpEq, Value: 2},
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
