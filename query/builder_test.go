package query

import (
	"strings"
	"testing"
)

func TestNewBuilder(t *testing.T) {
	b := newTestBuilder()
	if b == nil {
		t.Fatal("NewBuilder returned nil")
	}
	if b.tableName != "users" {
		t.Errorf("tableName = %q, want %q", b.tableName, "users")
	}
}

func TestBuilder_Clone(t *testing.T) {
	b := newTestBuilder()
	b2 := b.Where(Condition{Field: "name", Operator: OpEq, Value: "John"}).
		OrderByAsc("name").
		Limit(10).
		Offset(5).
		Select("name", "email").
		Preload("Posts")

	b3 := b2.clone()

	// Verify independence
	if len(b.conditions) != 0 {
		t.Error("original builder should not be modified")
	}
	if len(b3.conditions) != len(b2.conditions) {
		t.Error("clone should have same conditions")
	}
	if b3.tableName != b2.tableName {
		t.Error("clone should have same tableName")
	}
}

func TestBuilder_Where(t *testing.T) {
	b := newTestBuilder()
	b2 := b.Where(Condition{Field: "name", Operator: OpEq, Value: "John"})

	// Original should be unchanged
	if len(b.conditions) != 0 {
		t.Error("Where should not modify original builder")
	}
	if len(b2.conditions) != 1 {
		t.Errorf("Where should add condition, got %d", len(b2.conditions))
	}
}

func TestBuilder_OrderByAsc(t *testing.T) {
	b := newTestBuilder().OrderByAsc("name")
	sql, _ := b.buildSelectQuery()
	if !strings.Contains(sql, "ORDER BY name ASC") {
		t.Errorf("SQL = %q, want ORDER BY name ASC", sql)
	}
}

func TestBuilder_OrderByDesc(t *testing.T) {
	b := newTestBuilder().OrderByDesc("created_at")
	sql, _ := b.buildSelectQuery()
	if !strings.Contains(sql, "ORDER BY created_at DESC") {
		t.Errorf("SQL = %q, want ORDER BY created_at DESC", sql)
	}
}

func TestBuilder_Limit(t *testing.T) {
	b := newTestBuilder().Limit(10)
	sql, _ := b.buildSelectQuery()
	if !strings.Contains(sql, "LIMIT 10") {
		t.Errorf("SQL = %q, want LIMIT 10", sql)
	}
}

func TestBuilder_Offset(t *testing.T) {
	b := newTestBuilder().Offset(20)
	sql, _ := b.buildSelectQuery()
	if !strings.Contains(sql, "OFFSET 20") {
		t.Errorf("SQL = %q, want OFFSET 20", sql)
	}
}

func TestBuilder_Select(t *testing.T) {
	b := newTestBuilder().Select("name", "email")
	sql, _ := b.buildSelectQuery()
	if !strings.Contains(sql, "SELECT name, email") {
		t.Errorf("SQL = %q, want SELECT name, email", sql)
	}
}

func TestBuilder_WithTrashed(t *testing.T) {
	b := newTestBuilder().WithTrashed()
	if !b.disableScopes {
		t.Error("WithTrashed should set disableScopes to true")
	}
}

func TestBuilder_OnlyTrashed(t *testing.T) {
	b := newTestBuilder().OnlyTrashed()
	if !b.disableScopes {
		t.Error("OnlyTrashed should disable scopes")
	}
	if len(b.conditions) != 1 {
		t.Errorf("OnlyTrashed should add 1 condition, got %d", len(b.conditions))
	}
}

func TestBuilder_Preload(t *testing.T) {
	b := newTestBuilder().Preload("Posts").Preload("Comments")
	if len(b.preloads) != 2 {
		t.Errorf("Preload count = %d, want 2", len(b.preloads))
	}
}

func TestBuilder_BuildSelectQuery_Basic(t *testing.T) {
	b := newTestBuilder()
	sql, args := b.buildSelectQuery()

	expectedSQL := `SELECT * FROM "users"`
	if sql != expectedSQL {
		t.Errorf("SQL = %q, want %q", sql, expectedSQL)
	}
	if len(args) != 0 {
		t.Errorf("args len = %d, want 0", len(args))
	}
}

func TestBuilder_BuildSelectQuery_WithConditions(t *testing.T) {
	b := newTestBuilder().
		Where(Condition{Field: "name", Operator: OpEq, Value: "John"}).
		Where(Condition{Field: "age", Operator: OpGt, Value: 18})

	sql, args := b.buildSelectQuery()

	if !strings.Contains(sql, "WHERE") {
		t.Error("SQL should contain WHERE")
	}
	if !strings.Contains(sql, "name = $1") {
		t.Errorf("SQL should contain name = $1, got %q", sql)
	}
	if !strings.Contains(sql, "age > $2") {
		t.Errorf("SQL should contain age > $2, got %q", sql)
	}
	if len(args) != 2 {
		t.Errorf("args len = %d, want 2", len(args))
	}
}

func TestBuilder_BuildSelectQuery_WithJoins(t *testing.T) {
	b := newTestBuilder()
	b.joins = append(b.joins, JoinClause{
		Type:  InnerJoinType,
		Table: "posts",
		Condition: JoinCondition{
			LeftColumn:  "users.id",
			RightColumn: "posts.user_id",
			Operator:    OpEq,
		},
	})

	sql, _ := b.buildSelectQuery()

	if !strings.Contains(sql, `"users".*`) {
		t.Errorf("SQL should qualify with table when joins present, got %q", sql)
	}
	if !strings.Contains(sql, "INNER JOIN") {
		t.Errorf("SQL should contain INNER JOIN, got %q", sql)
	}
}

func TestBuilder_BuildSelectQuery_Full(t *testing.T) {
	b := newTestBuilder().
		Select("name", "email").
		Where(Condition{Field: "age", Operator: OpGte, Value: 18}).
		OrderByAsc("name").
		OrderByDesc("age").
		Limit(10).
		Offset(20)

	sql, args := b.buildSelectQuery()

	if !strings.HasPrefix(sql, "SELECT name, email FROM") {
		t.Errorf("SQL prefix wrong: %q", sql)
	}
	if !strings.Contains(sql, "WHERE age >= $1") {
		t.Errorf("SQL should contain WHERE, got %q", sql)
	}
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

func TestBuilder_BuildCountQuery(t *testing.T) {
	b := newTestBuilder().
		Where(Condition{Field: "active", Operator: OpEq, Value: true})

	sql, args := b.buildCountQuery()

	expectedPrefix := `SELECT COUNT(*) FROM "users" WHERE`
	if !strings.HasPrefix(sql, expectedPrefix) {
		t.Errorf("SQL = %q, want prefix %q", sql, expectedPrefix)
	}
	if len(args) != 1 {
		t.Errorf("args len = %d, want 1", len(args))
	}
}

func TestBuilder_BuildCountQuery_NoConditions(t *testing.T) {
	b := newTestBuilder()
	sql, args := b.buildCountQuery()

	expected := `SELECT COUNT(*) FROM "users"`
	if sql != expected {
		t.Errorf("SQL = %q, want %q", sql, expected)
	}
	if len(args) != 0 {
		t.Errorf("args len = %d, want 0", len(args))
	}
}

func TestBuilder_BuildWhereClause_Empty(t *testing.T) {
	b := newTestBuilder()
	sql, args := b.buildWhereClause(nil)
	if sql != "" {
		t.Errorf("expected empty, got %q", sql)
	}
	if args != nil {
		t.Errorf("expected nil args, got %v", args)
	}
}

func TestBuilder_BuildCondition_IsNull(t *testing.T) {
	b := newTestBuilder()
	idx := 1
	sql, args := b.buildCondition(Condition{Field: "deleted_at", Operator: OpIsNull}, &idx)
	if sql != "deleted_at IS NULL" {
		t.Errorf("SQL = %q, want 'deleted_at IS NULL'", sql)
	}
	if len(args) != 0 {
		t.Errorf("args len = %d, want 0", len(args))
	}
}

func TestBuilder_BuildCondition_IsNotNull(t *testing.T) {
	b := newTestBuilder()
	idx := 1
	sql, args := b.buildCondition(Condition{Field: "deleted_at", Operator: OpIsNotNull}, &idx)
	if sql != "deleted_at IS NOT NULL" {
		t.Errorf("SQL = %q, want 'deleted_at IS NOT NULL'", sql)
	}
	if len(args) != 0 {
		t.Errorf("args len = %d, want 0", len(args))
	}
}

func TestBuilder_BuildCondition_In(t *testing.T) {
	b := newTestBuilder()
	idx := 1
	sql, args := b.buildCondition(Condition{
		Field:    "id",
		Operator: OpIn,
		Value:    []int{1, 2, 3},
	}, &idx)
	if sql != "id IN ($1, $2, $3)" {
		t.Errorf("SQL = %q, want 'id IN ($1, $2, $3)'", sql)
	}
	if len(args) != 3 {
		t.Errorf("args len = %d, want 3", len(args))
	}
}

func TestBuilder_BuildCondition_NotIn(t *testing.T) {
	b := newTestBuilder()
	idx := 1
	sql, args := b.buildCondition(Condition{
		Field:    "id",
		Operator: OpNotIn,
		Value:    []int64{1, 2},
	}, &idx)
	if sql != "id NOT IN ($1, $2)" {
		t.Errorf("SQL = %q, want 'id NOT IN ($1, $2)'", sql)
	}
	if len(args) != 2 {
		t.Errorf("args len = %d, want 2", len(args))
	}
}

func TestBuilder_BuildCondition_Between(t *testing.T) {
	b := newTestBuilder()
	idx := 1
	sql, args := b.buildCondition(Condition{
		Field:    "age",
		Operator: OpBetween,
		Value:    []int{18, 65},
	}, &idx)
	if sql != "age BETWEEN $1 AND $2" {
		t.Errorf("SQL = %q, want 'age BETWEEN $1 AND $2'", sql)
	}
	if len(args) != 2 {
		t.Errorf("args len = %d, want 2", len(args))
	}
}

func TestBuilder_BuildCondition_BetweenInvalidValues(t *testing.T) {
	b := newTestBuilder()
	idx := 1
	sql, args := b.buildCondition(Condition{
		Field:    "age",
		Operator: OpBetween,
		Value:    []int{18}, // only 1 value, need 2
	}, &idx)
	if sql != "" {
		t.Errorf("SQL should be empty for invalid BETWEEN, got %q", sql)
	}
	if len(args) != 0 {
		t.Errorf("args should be empty for invalid BETWEEN, got %v", args)
	}
}

func TestBuilder_BuildCondition_Default(t *testing.T) {
	b := newTestBuilder()
	idx := 1
	sql, args := b.buildCondition(Condition{
		Field:    "name",
		Operator: OpEq,
		Value:    "John",
	}, &idx)
	if sql != "name = $1" {
		t.Errorf("SQL = %q, want 'name = $1'", sql)
	}
	if len(args) != 1 {
		t.Errorf("args len = %d, want 1", len(args))
	}
}

func TestBuilder_BuildConditionGroup(t *testing.T) {
	b := newTestBuilder()
	idx := 1
	group := ConditionGroup{
		Conditions: []interface{}{
			Condition{Field: "name", Operator: OpEq, Value: "John"},
			Condition{Field: "age", Operator: OpGt, Value: 18},
		},
		Operator: LogicalOr,
	}
	sql, args := b.buildConditionGroup(group, &idx)
	if !strings.Contains(sql, " OR ") {
		t.Errorf("SQL should contain OR, got %q", sql)
	}
	if len(args) != 2 {
		t.Errorf("args len = %d, want 2", len(args))
	}
}

func TestBuilder_BuildConditionGroup_Nested(t *testing.T) {
	b := newTestBuilder()
	idx := 1
	group := ConditionGroup{
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
	}
	sql, args := b.buildConditionGroup(group, &idx)
	if !strings.Contains(sql, " AND ") {
		t.Errorf("SQL should contain AND, got %q", sql)
	}
	if !strings.Contains(sql, "(") {
		t.Errorf("SQL should contain nested group, got %q", sql)
	}
	if len(args) != 3 {
		t.Errorf("args len = %d, want 3", len(args))
	}
}

func TestBuilder_BuildWhereClause_WithConditionGroup(t *testing.T) {
	b := newTestBuilder()

	conditions := []interface{}{
		Condition{Field: "active", Operator: OpEq, Value: true},
		ConditionGroup{
			Conditions: []interface{}{
				Condition{Field: "name", Operator: OpEq, Value: "A"},
				Condition{Field: "name", Operator: OpEq, Value: "B"},
			},
			Operator: LogicalOr,
		},
	}
	sql, args := b.buildWhereClause(conditions)
	if !strings.Contains(sql, "AND") {
		t.Errorf("SQL should contain AND, got %q", sql)
	}
	if !strings.Contains(sql, "(") {
		t.Errorf("SQL should contain group, got %q", sql)
	}
	if len(args) != 3 {
		t.Errorf("args len = %d, want 3", len(args))
	}
}

func TestInterfaceSlice(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected int
	}{
		{"[]string", []string{"a", "b"}, 2},
		{"[]int", []int{1, 2, 3}, 3},
		{"[]int64", []int64{1, 2}, 2},
		{"[]bool", []bool{true, false}, 2},
		{"[]interface{}", []interface{}{1, "a"}, 2},
		{"single value", 42, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := interfaceSlice(tt.input)
			if len(result) != tt.expected {
				t.Errorf("interfaceSlice() len = %d, want %d", len(result), tt.expected)
			}
		})
	}
}

func TestGetTableName(t *testing.T) {
	// With TableNamer
	user := testUser{}
	name := getTableName(user)
	if name != "users" {
		t.Errorf("getTableName(testUser) = %q, want %q", name, "users")
	}

	// Without TableNamer - uses struct name to snake_case
	type SimpleModel struct{}
	name2 := getTableName(SimpleModel{})
	if name2 != "simple_model" {
		t.Errorf("getTableName(SimpleModel) = %q, want %q", name2, "simple_model")
	}
}

func TestJoin(t *testing.T) {
	b := NewBuilder[any](
		&mockExecutor{},
		&mockDialect{},
		newMockLogger(),
		"users",
	)
	result := Join[testPost](b, On("users.id", "posts.user_id"))
	if len(result.joins) != 1 {
		t.Fatalf("joins len = %d, want 1", len(result.joins))
	}
	if result.joins[0].Type != InnerJoinType {
		t.Errorf("join type = %q, want INNER", result.joins[0].Type)
	}
	if result.joins[0].Table != "posts" {
		t.Errorf("join table = %q, want posts", result.joins[0].Table)
	}
}

func TestLeftJoin(t *testing.T) {
	b := NewBuilder[any](
		&mockExecutor{},
		&mockDialect{},
		newMockLogger(),
		"users",
	)
	result := LeftJoin[testPost](b, On("users.id", "posts.user_id"))
	if len(result.joins) != 1 {
		t.Fatalf("joins len = %d, want 1", len(result.joins))
	}
	if result.joins[0].Type != LeftJoinType {
		t.Errorf("join type = %q, want LEFT", result.joins[0].Type)
	}
}

func TestRightJoin(t *testing.T) {
	b := NewBuilder[any](
		&mockExecutor{},
		&mockDialect{},
		newMockLogger(),
		"users",
	)
	result := RightJoin[testPost](b, On("users.id", "posts.user_id"))
	if len(result.joins) != 1 {
		t.Fatalf("joins len = %d, want 1", len(result.joins))
	}
	if result.joins[0].Type != RightJoinType {
		t.Errorf("join type = %q, want RIGHT", result.joins[0].Type)
	}
}

func TestGetTableName_PointerType(t *testing.T) {
	p := &testPost{}
	name := getTableName(p)
	if name != "posts" {
		t.Errorf("getTableName(&testPost) = %q, want posts", name)
	}
}

func TestBuilder_Clone_AllFields(t *testing.T) {
	b := newTestBuilder()
	b.joins = append(b.joins, JoinClause{
		Type:  InnerJoinType,
		Table: "posts",
		Condition: JoinCondition{
			LeftColumn:  "users.id",
			RightColumn: "posts.user_id",
			Operator:    OpEq,
		},
	})
	b2 := b.clone()
	if len(b2.joins) != 1 {
		t.Error("clone should copy joins")
	}
}

func TestBuilder_Immutability(t *testing.T) {
	b := newTestBuilder()

	b1 := b.Where(Condition{Field: "a", Operator: OpEq, Value: 1})
	b2 := b.Where(Condition{Field: "b", Operator: OpEq, Value: 2})

	sql1, _ := b1.buildSelectQuery()
	sql2, _ := b2.buildSelectQuery()

	if strings.Contains(sql1, "b =") {
		t.Error("b1 should not contain b2's condition")
	}
	if strings.Contains(sql2, "a =") {
		t.Error("b2 should not contain b1's condition")
	}
}
