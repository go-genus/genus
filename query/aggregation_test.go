package query

import (
	"strings"
	"testing"
)

func TestAggregateResult_Int64(t *testing.T) {
	tests := []struct {
		name     string
		values   map[string]interface{}
		key      string
		expected int64
	}{
		{"int64 value", map[string]interface{}{"count": int64(42)}, "count", 42},
		{"int value", map[string]interface{}{"count": int(42)}, "count", 42},
		{"int32 value", map[string]interface{}{"count": int32(42)}, "count", 42},
		{"float64 value", map[string]interface{}{"count": float64(42.9)}, "count", 42},
		{"float32 value", map[string]interface{}{"count": float32(42.9)}, "count", 42},
		{"missing key", map[string]interface{}{"other": int64(42)}, "count", 0},
		{"nil map", map[string]interface{}{}, "count", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewAggregateResult(tt.values)
			got := result.Int64(tt.key)
			if got != tt.expected {
				t.Errorf("Int64(%q) = %d, want %d", tt.key, got, tt.expected)
			}
		})
	}
}

func TestAggregateResult_Float64(t *testing.T) {
	tests := []struct {
		name     string
		values   map[string]interface{}
		key      string
		expected float64
	}{
		{"float64 value", map[string]interface{}{"avg": float64(3.14)}, "avg", 3.14},
		{"float32 value", map[string]interface{}{"avg": float32(3.14)}, "avg", float64(float32(3.14))},
		{"int64 value", map[string]interface{}{"avg": int64(42)}, "avg", 42.0},
		{"int value", map[string]interface{}{"avg": int(42)}, "avg", 42.0},
		{"int32 value", map[string]interface{}{"avg": int32(42)}, "avg", 42.0},
		{"missing key", map[string]interface{}{"other": float64(3.14)}, "avg", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewAggregateResult(tt.values)
			got := result.Float64(tt.key)
			if got != tt.expected {
				t.Errorf("Float64(%q) = %f, want %f", tt.key, got, tt.expected)
			}
		})
	}
}

func TestAggregateResult_String(t *testing.T) {
	tests := []struct {
		name     string
		values   map[string]interface{}
		key      string
		expected string
	}{
		{"string value", map[string]interface{}{"name": "test"}, "name", "test"},
		{"[]byte value", map[string]interface{}{"name": []byte("test")}, "name", "test"},
		{"int value formatted", map[string]interface{}{"count": 42}, "count", "42"},
		{"missing key", map[string]interface{}{"other": "test"}, "name", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NewAggregateResult(tt.values)
			got := result.String(tt.key)
			if got != tt.expected {
				t.Errorf("String(%q) = %q, want %q", tt.key, got, tt.expected)
			}
		})
	}
}

func TestAggregateResult_Has(t *testing.T) {
	result := NewAggregateResult(map[string]interface{}{
		"count": int64(42),
		"name":  "test",
	})

	if !result.Has("count") {
		t.Error("Has(count) = false, want true")
	}
	if !result.Has("name") {
		t.Error("Has(name) = false, want true")
	}
	if result.Has("missing") {
		t.Error("Has(missing) = true, want false")
	}
}

func TestAggregateResult_Keys(t *testing.T) {
	result := NewAggregateResult(map[string]interface{}{
		"count": int64(42),
		"sum":   float64(100.5),
	})

	keys := result.Keys()
	if len(keys) != 2 {
		t.Errorf("Keys() len = %d, want 2", len(keys))
	}

	// Verifica que ambas as chaves existem
	hasCount, hasSum := false, false
	for _, k := range keys {
		if k == "count" {
			hasCount = true
		}
		if k == "sum" {
			hasSum = true
		}
	}

	if !hasCount || !hasSum {
		t.Errorf("Keys() = %v, expected to contain 'count' and 'sum'", keys)
	}
}

func TestAggregateResult_Value(t *testing.T) {
	values := map[string]interface{}{
		"count": int64(42),
		"name":  "test",
	}
	result := NewAggregateResult(values)

	if result.Value("count") != int64(42) {
		t.Errorf("Value(count) = %v, want 42", result.Value("count"))
	}

	if result.Value("name") != "test" {
		t.Errorf("Value(name) = %v, want 'test'", result.Value("name"))
	}

	if result.Value("missing") != nil {
		t.Errorf("Value(missing) = %v, want nil", result.Value("missing"))
	}
}

// ===== AggregateBuilder SQL generation tests =====

func newTestAggregateBuilder() *AggregateBuilder[testUser] {
	b := newTestBuilder()
	return b.Aggregate()
}

func TestAggregateBuilder_Aggregate(t *testing.T) {
	ab := newTestAggregateBuilder()
	if ab == nil {
		t.Fatal("Aggregate returned nil")
	}
	if ab.tableName != "users" {
		t.Errorf("tableName = %q, want users", ab.tableName)
	}
}

func TestAggregateBuilder_Clone(t *testing.T) {
	ab := newTestAggregateBuilder().
		CountAll().
		Where(Condition{Field: "age", Operator: OpGt, Value: 18}).
		GroupBy("name").
		Having(Condition{Field: "COUNT(*)", Operator: OpGt, Value: 5}).
		OrderByAsc("name").
		Limit(10).
		Offset(5)

	cloned := ab.clone()

	if len(cloned.aggregates) != len(ab.aggregates) {
		t.Error("clone should copy aggregates")
	}
	if len(cloned.conditions) != len(ab.conditions) {
		t.Error("clone should copy conditions")
	}
	if len(cloned.groupByFields) != len(ab.groupByFields) {
		t.Error("clone should copy groupByFields")
	}
	if len(cloned.havingConds) != len(ab.havingConds) {
		t.Error("clone should copy havingConds")
	}
	if len(cloned.orderBy) != len(ab.orderBy) {
		t.Error("clone should copy orderBy")
	}
	if cloned.limit == nil || *cloned.limit != 10 {
		t.Error("clone should copy limit")
	}
	if cloned.offset == nil || *cloned.offset != 5 {
		t.Error("clone should copy offset")
	}
}

func TestAggregateBuilder_Where(t *testing.T) {
	ab := newTestAggregateBuilder()
	ab2 := ab.Where(Condition{Field: "age", Operator: OpGt, Value: 18})

	if len(ab.conditions) != 0 {
		t.Error("Where should not modify original")
	}
	if len(ab2.conditions) != 1 {
		t.Error("Where should add condition")
	}
}

func TestAggregateBuilder_CountAll(t *testing.T) {
	ab := newTestAggregateBuilder().CountAll()
	sql, _ := ab.buildQuery()
	if !strings.Contains(sql, "COUNT(*) AS") {
		t.Errorf("SQL should contain COUNT(*), got %q", sql)
	}
}

func TestAggregateBuilder_Count(t *testing.T) {
	ab := newTestAggregateBuilder().Count("name")
	sql, _ := ab.buildQuery()
	if !strings.Contains(sql, `COUNT("name") AS "count_name"`) {
		t.Errorf("SQL should contain COUNT(name), got %q", sql)
	}
}

func TestAggregateBuilder_Sum(t *testing.T) {
	ab := newTestAggregateBuilder().Sum("amount")
	sql, _ := ab.buildQuery()
	if !strings.Contains(sql, `SUM("amount") AS "sum_amount"`) {
		t.Errorf("SQL should contain SUM(amount), got %q", sql)
	}
}

func TestAggregateBuilder_Avg(t *testing.T) {
	ab := newTestAggregateBuilder().Avg("age")
	sql, _ := ab.buildQuery()
	if !strings.Contains(sql, `AVG("age") AS "avg_age"`) {
		t.Errorf("SQL should contain AVG(age), got %q", sql)
	}
}

func TestAggregateBuilder_Max(t *testing.T) {
	ab := newTestAggregateBuilder().Max("salary")
	sql, _ := ab.buildQuery()
	if !strings.Contains(sql, `MAX("salary") AS "max_salary"`) {
		t.Errorf("SQL should contain MAX(salary), got %q", sql)
	}
}

func TestAggregateBuilder_Min(t *testing.T) {
	ab := newTestAggregateBuilder().Min("salary")
	sql, _ := ab.buildQuery()
	if !strings.Contains(sql, `MIN("salary") AS "min_salary"`) {
		t.Errorf("SQL should contain MIN(salary), got %q", sql)
	}
}

func TestAggregateBuilder_GroupBy(t *testing.T) {
	ab := newTestAggregateBuilder().CountAll().GroupBy("department", "role")
	sql, _ := ab.buildQuery()
	if !strings.Contains(sql, "GROUP BY") {
		t.Errorf("SQL should contain GROUP BY, got %q", sql)
	}
	if !strings.Contains(sql, `"department"`) {
		t.Errorf("SQL should contain department, got %q", sql)
	}
	// Group by columns should appear in SELECT
	if !strings.HasPrefix(sql, `SELECT "department", "role"`) {
		t.Errorf("SQL should start with group columns in SELECT, got %q", sql)
	}
}

func TestAggregateBuilder_Having(t *testing.T) {
	ab := newTestAggregateBuilder().
		CountAll().
		GroupBy("department").
		Having(Condition{Field: "COUNT(*)", Operator: OpGt, Value: 5})
	sql, args := ab.buildQuery()
	if !strings.Contains(sql, "HAVING") {
		t.Errorf("SQL should contain HAVING, got %q", sql)
	}
	if len(args) != 1 {
		t.Errorf("args len = %d, want 1", len(args))
	}
}

func TestAggregateBuilder_OrderByAsc(t *testing.T) {
	ab := newTestAggregateBuilder().CountAll().OrderByAsc("name")
	sql, _ := ab.buildQuery()
	if !strings.Contains(sql, "ORDER BY name ASC") {
		t.Errorf("SQL should contain ORDER BY ASC, got %q", sql)
	}
}

func TestAggregateBuilder_OrderByDesc(t *testing.T) {
	ab := newTestAggregateBuilder().CountAll().OrderByDesc("created_at")
	sql, _ := ab.buildQuery()
	if !strings.Contains(sql, "ORDER BY created_at DESC") {
		t.Errorf("SQL should contain ORDER BY DESC, got %q", sql)
	}
}

func TestAggregateBuilder_Limit(t *testing.T) {
	ab := newTestAggregateBuilder().CountAll().Limit(10)
	sql, _ := ab.buildQuery()
	if !strings.Contains(sql, "LIMIT 10") {
		t.Errorf("SQL should contain LIMIT 10, got %q", sql)
	}
}

func TestAggregateBuilder_Offset(t *testing.T) {
	ab := newTestAggregateBuilder().CountAll().Offset(20)
	sql, _ := ab.buildQuery()
	if !strings.Contains(sql, "OFFSET 20") {
		t.Errorf("SQL should contain OFFSET 20, got %q", sql)
	}
}

func TestAggregateBuilder_BuildQuery_NoAggregates(t *testing.T) {
	ab := newTestAggregateBuilder()
	sql, _ := ab.buildQuery()
	// Default should use COUNT(*) AS count
	if !strings.Contains(sql, "COUNT(*) AS count") {
		t.Errorf("SQL without aggregates should default to COUNT(*), got %q", sql)
	}
}

func TestAggregateBuilder_BuildQuery_Full(t *testing.T) {
	ab := newTestAggregateBuilder().
		Where(Condition{Field: "active", Operator: OpEq, Value: true}).
		CountAll().
		Sum("amount").
		GroupBy("department").
		Having(Condition{Field: "COUNT(*)", Operator: OpGt, Value: 5}).
		OrderByDesc("count").
		Limit(10).
		Offset(0)

	sql, args := ab.buildQuery()

	if !strings.Contains(sql, "SELECT") {
		t.Errorf("SQL should contain SELECT, got %q", sql)
	}
	if !strings.Contains(sql, `FROM "users"`) {
		t.Errorf("SQL should contain FROM, got %q", sql)
	}
	if !strings.Contains(sql, "WHERE") {
		t.Errorf("SQL should contain WHERE, got %q", sql)
	}
	if !strings.Contains(sql, "GROUP BY") {
		t.Errorf("SQL should contain GROUP BY, got %q", sql)
	}
	if !strings.Contains(sql, "HAVING") {
		t.Errorf("SQL should contain HAVING, got %q", sql)
	}
	if !strings.Contains(sql, "ORDER BY") {
		t.Errorf("SQL should contain ORDER BY, got %q", sql)
	}
	if !strings.Contains(sql, "LIMIT 10") {
		t.Errorf("SQL should contain LIMIT, got %q", sql)
	}
	if len(args) != 2 { // active = $1, COUNT(*) > $2
		t.Errorf("args len = %d, want 2", len(args))
	}
}

func TestAggregateBuilder_BuildWhereClause_Empty(t *testing.T) {
	ab := newTestAggregateBuilder()
	sql, args := ab.buildWhereClause(nil, 1)
	if sql != "" {
		t.Errorf("empty where clause should return empty, got %q", sql)
	}
	if args != nil {
		t.Errorf("empty where clause should return nil args, got %v", args)
	}
}

func TestAggregateBuilder_BuildCondition_AllOperators(t *testing.T) {
	ab := newTestAggregateBuilder()

	// IsNull
	idx := 1
	sql, args := ab.buildCondition(Condition{Field: "deleted_at", Operator: OpIsNull}, &idx)
	if sql != "deleted_at IS NULL" {
		t.Errorf("IsNull: got %q", sql)
	}
	if len(args) != 0 {
		t.Errorf("IsNull args: %d", len(args))
	}

	// IsNotNull
	idx = 1
	sql, args = ab.buildCondition(Condition{Field: "name", Operator: OpIsNotNull}, &idx)
	if sql != "name IS NOT NULL" {
		t.Errorf("IsNotNull: got %q", sql)
	}

	// In
	idx = 1
	sql, args = ab.buildCondition(Condition{Field: "id", Operator: OpIn, Value: []int{1, 2, 3}}, &idx)
	if sql != "id IN ($1, $2, $3)" {
		t.Errorf("In: got %q", sql)
	}
	if len(args) != 3 {
		t.Errorf("In args: %d", len(args))
	}

	// NotIn
	idx = 1
	sql, args = ab.buildCondition(Condition{Field: "id", Operator: OpNotIn, Value: []int{1, 2}}, &idx)
	if !strings.Contains(sql, "NOT IN") {
		t.Errorf("NotIn: got %q", sql)
	}

	// Between
	idx = 1
	sql, args = ab.buildCondition(Condition{Field: "age", Operator: OpBetween, Value: []int{18, 65}}, &idx)
	if sql != "age BETWEEN $1 AND $2" {
		t.Errorf("Between: got %q", sql)
	}
	if len(args) != 2 {
		t.Errorf("Between args: %d", len(args))
	}

	// Between invalid
	idx = 1
	sql, args = ab.buildCondition(Condition{Field: "age", Operator: OpBetween, Value: []int{18}}, &idx)
	if sql != "" {
		t.Errorf("Between invalid: got %q", sql)
	}

	// Default (Eq)
	idx = 1
	sql, args = ab.buildCondition(Condition{Field: "name", Operator: OpEq, Value: "John"}, &idx)
	if sql != "name = $1" {
		t.Errorf("Default: got %q", sql)
	}
	if len(args) != 1 {
		t.Errorf("Default args: %d", len(args))
	}
}

func TestAggregateBuilder_BuildConditionGroup(t *testing.T) {
	ab := newTestAggregateBuilder()
	idx := 1
	group := ConditionGroup{
		Conditions: []interface{}{
			Condition{Field: "a", Operator: OpEq, Value: 1},
			Condition{Field: "b", Operator: OpEq, Value: 2},
		},
		Operator: LogicalOr,
	}
	sql, args := ab.buildConditionGroup(group, &idx)
	if !strings.Contains(sql, " OR ") {
		t.Errorf("should contain OR, got %q", sql)
	}
	if len(args) != 2 {
		t.Errorf("args len = %d, want 2", len(args))
	}
}

func TestAggregateBuilder_BuildConditionGroup_Nested(t *testing.T) {
	ab := newTestAggregateBuilder()
	idx := 1
	group := ConditionGroup{
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
	}
	sql, args := ab.buildConditionGroup(group, &idx)
	if !strings.Contains(sql, " AND ") {
		t.Errorf("should contain AND, got %q", sql)
	}
	if !strings.Contains(sql, "(") {
		t.Errorf("should contain nested group, got %q", sql)
	}
	if len(args) != 2 {
		t.Errorf("args len = %d, want 2", len(args))
	}
}

func TestAggregateBuilder_Immutability(t *testing.T) {
	ab := newTestAggregateBuilder()
	ab1 := ab.CountAll()
	ab2 := ab.Sum("amount")

	sql1, _ := ab1.buildQuery()
	sql2, _ := ab2.buildQuery()

	if strings.Contains(sql1, "SUM") {
		t.Error("ab1 should not contain ab2's SUM")
	}
	if strings.Contains(sql2, "COUNT(*)") && !strings.Contains(sql2, "AS count") {
		// sql2 will only have the default COUNT(*) if no aggregates
		// It should have SUM, not the CountAll from ab1
	}
	if !strings.Contains(sql1, "COUNT(*)") {
		t.Errorf("ab1 should contain COUNT(*), got %q", sql1)
	}
	if !strings.Contains(sql2, "SUM") {
		t.Errorf("ab2 should contain SUM, got %q", sql2)
	}
}

func TestSanitizeAlias(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"user.name", "user_name"},
		{"some-column", "some_column"},
		{"user.first-name", "user_first_name"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeAlias(tt.input)
			if got != tt.expected {
				t.Errorf("sanitizeAlias(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
