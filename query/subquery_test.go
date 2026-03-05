package query

import (
	"strings"
	"testing"
)

func TestSubquery_SQL_And_Args(t *testing.T) {
	sq := RawSubquery("SELECT id FROM posts WHERE user_id = $1", 42)
	if sq.SQL() != "SELECT id FROM posts WHERE user_id = $1" {
		t.Errorf("SQL() = %q", sq.SQL())
	}
	if len(sq.Args()) != 1 {
		t.Errorf("Args() len = %d, want 1", len(sq.Args()))
	}
}

func TestRawSubquery(t *testing.T) {
	sq := RawSubquery("SELECT 1", "a", "b")
	if sq.SQL() != "SELECT 1" {
		t.Errorf("SQL = %q", sq.SQL())
	}
	if len(sq.Args()) != 2 {
		t.Errorf("Args len = %d, want 2", len(sq.Args()))
	}
}

func TestSubqueryBuilder_Build(t *testing.T) {
	b := newTestBuilder().
		Where(Condition{Field: "status", Operator: OpEq, Value: "active"})

	sb := b.Subquery()
	sql, args := sb.Build()

	if !strings.Contains(sql, `SELECT "id" FROM "users"`) {
		t.Errorf("Build() SQL = %q, should contain SELECT id FROM users", sql)
	}
	if !strings.Contains(sql, "WHERE") {
		t.Errorf("Build() SQL should contain WHERE, got %q", sql)
	}
	if len(args) != 1 {
		t.Errorf("Build() args len = %d, want 1", len(args))
	}
}

func TestSubqueryBuilder_Column(t *testing.T) {
	b := newTestBuilder()
	sb := b.Subquery().Column("email")
	sql, _ := sb.Build()

	if !strings.Contains(sql, `SELECT "email"`) {
		t.Errorf("Build() SQL = %q, should contain SELECT email", sql)
	}
}

func TestSubqueryBuilder_ToSubquery(t *testing.T) {
	b := newTestBuilder().
		Where(Condition{Field: "active", Operator: OpEq, Value: true})

	sq := b.Subquery().ToSubquery()
	if sq.SQL() == "" {
		t.Error("ToSubquery() should not return empty SQL")
	}
}

func TestSubqueryBuilder_WithOrderAndLimit(t *testing.T) {
	b := newTestBuilder().
		OrderByDesc("created_at").
		Limit(5).
		Offset(10)

	sb := b.Subquery()
	sql, _ := sb.Build()

	if !strings.Contains(sql, "ORDER BY") {
		t.Errorf("SQL should contain ORDER BY, got %q", sql)
	}
	if !strings.Contains(sql, "LIMIT 5") {
		t.Errorf("SQL should contain LIMIT 5, got %q", sql)
	}
	if !strings.Contains(sql, "OFFSET 10") {
		t.Errorf("SQL should contain OFFSET 10, got %q", sql)
	}
}

func TestExists(t *testing.T) {
	sq := RawSubquery("SELECT 1 FROM posts WHERE user_id = $1", 1)
	cond := Exists(sq)
	if cond.Not {
		t.Error("Exists should set Not = false")
	}
	if cond.Subquery != sq {
		t.Error("Exists should set Subquery")
	}
}

func TestNotExists(t *testing.T) {
	sq := RawSubquery("SELECT 1 FROM posts")
	cond := NotExists(sq)
	if !cond.Not {
		t.Error("NotExists should set Not = true")
	}
}

func TestExtractSubquery(t *testing.T) {
	sq := RawSubquery("SELECT 1")
	result := extractSubquery(sq)
	if result != sq {
		t.Error("extractSubquery should return same Subquery for *Subquery input")
	}

	// Non-Subquery input
	result2 := extractSubquery("not a subquery")
	if result2 == nil {
		t.Error("extractSubquery should return empty Subquery for unknown type")
	}
}

func TestCreateSubqueryCondition(t *testing.T) {
	sq := RawSubquery("SELECT id FROM posts")
	cond := createSubqueryCondition("user_id", OpIn, sq)
	if cond.Field != "user_id" {
		t.Errorf("Field = %q, want 'user_id'", cond.Field)
	}
	if cond.Operator != OpIn {
		t.Errorf("Operator = %v, want OpIn", cond.Operator)
	}
}

func TestCreateSubqueryCondition_UnknownType(t *testing.T) {
	cond := createSubqueryCondition("user_id", OpIn, "unknown")
	if cond.Subquery == nil {
		t.Error("should create empty subquery for unknown type")
	}
}

func TestBuilder_BuildSubqueryCondition(t *testing.T) {
	b := newTestBuilder()
	sq := RawSubquery("SELECT id FROM posts WHERE active = $1", true)
	cond := SubqueryCondition{
		Field:    "user_id",
		Operator: OpIn,
		Subquery: sq,
	}

	idx := 1
	sql, args := b.buildSubqueryCondition(cond, &idx)
	if !strings.Contains(sql, "user_id IN (") {
		t.Errorf("SQL = %q, should contain 'user_id IN ('", sql)
	}
	if len(args) != 1 {
		t.Errorf("args len = %d, want 1", len(args))
	}
}

func TestBuilder_BuildSubqueryCondition_NotIn(t *testing.T) {
	b := newTestBuilder()
	sq := RawSubquery("SELECT id FROM posts")
	cond := SubqueryCondition{
		Field:    "user_id",
		Operator: OpNotIn,
		Subquery: sq,
	}

	idx := 1
	sql, _ := b.buildSubqueryCondition(cond, &idx)
	if !strings.Contains(sql, "NOT IN") {
		t.Errorf("SQL = %q, should contain 'NOT IN'", sql)
	}
}

func TestBuilder_BuildExistsCondition(t *testing.T) {
	b := newTestBuilder()
	sq := RawSubquery("SELECT 1 FROM posts WHERE user_id = $1", 1)

	idx := 1
	sql, args := b.buildExistsCondition(ExistsCondition{Subquery: sq, Not: false}, &idx)
	if !strings.HasPrefix(sql, "EXISTS (") {
		t.Errorf("SQL = %q, should start with 'EXISTS ('", sql)
	}
	if len(args) != 1 {
		t.Errorf("args len = %d, want 1", len(args))
	}
}

func TestBuilder_BuildExistsCondition_Not(t *testing.T) {
	b := newTestBuilder()
	sq := RawSubquery("SELECT 1 FROM posts")

	idx := 1
	sql, _ := b.buildExistsCondition(ExistsCondition{Subquery: sq, Not: true}, &idx)
	if !strings.HasPrefix(sql, "NOT EXISTS (") {
		t.Errorf("SQL = %q, should start with 'NOT EXISTS ('", sql)
	}
}

func TestBuilder_RewritePlaceholders_MySQL(t *testing.T) {
	b := newTestBuilderMySQL()
	idx := 3
	result := b.rewritePlaceholders("SELECT * WHERE a = ? AND b = ?", &idx, 2)
	// MySQL doesn't rewrite placeholders
	if !strings.Contains(result, "?") {
		t.Errorf("MySQL should keep ? placeholders, got %q", result)
	}
	if idx != 5 {
		t.Errorf("argIndex should be 5, got %d", idx)
	}
}

func TestBuilder_RewritePlaceholders_Postgres(t *testing.T) {
	b := newTestBuilder()
	idx := 3
	result := b.rewritePlaceholders("SELECT * WHERE a = $1 AND b = $2", &idx, 2)
	if strings.Contains(result, "$1") || strings.Contains(result, "$2") {
		t.Errorf("Postgres should rewrite $1/$2, got %q", result)
	}
}

func TestBuilder_WhereInSubquery(t *testing.T) {
	b := newTestBuilder()
	sq := RawSubquery("SELECT id FROM posts")
	b2 := b.WhereInSubquery("user_id", sq)

	if len(b2.conditions) != 1 {
		t.Errorf("conditions len = %d, want 1", len(b2.conditions))
	}
	if len(b.conditions) != 0 {
		t.Error("original builder should not be modified")
	}
}

func TestBuilder_WhereNotInSubquery(t *testing.T) {
	b := newTestBuilder()
	sq := RawSubquery("SELECT id FROM posts")
	b2 := b.WhereNotInSubquery("user_id", sq)

	if len(b2.conditions) != 1 {
		t.Errorf("conditions len = %d, want 1", len(b2.conditions))
	}
}

func TestBuilder_WhereExists(t *testing.T) {
	b := newTestBuilder()
	sq := RawSubquery("SELECT 1 FROM posts WHERE posts.user_id = users.id")
	b2 := b.WhereExists(sq)

	if len(b2.conditions) != 1 {
		t.Errorf("conditions len = %d, want 1", len(b2.conditions))
	}
}

func TestBuilder_WhereNotExists(t *testing.T) {
	b := newTestBuilder()
	sq := RawSubquery("SELECT 1 FROM posts")
	b2 := b.WhereNotExists(sq)

	if len(b2.conditions) != 1 {
		t.Errorf("conditions len = %d, want 1", len(b2.conditions))
	}
}

func TestBuilder_GetDialect(t *testing.T) {
	b := newTestBuilder()
	d := b.GetDialect()
	if d == nil {
		t.Error("GetDialect should not return nil")
	}
}

func TestBuilder_BuildConditionWithSubquery(t *testing.T) {
	b := newTestBuilder()
	sq := RawSubquery("SELECT id FROM posts")

	idx := 1
	// SubqueryCondition
	sql1, _ := b.buildConditionWithSubquery(SubqueryCondition{
		Field:    "user_id",
		Operator: OpIn,
		Subquery: sq,
	}, &idx)
	if sql1 == "" {
		t.Error("buildConditionWithSubquery should handle SubqueryCondition")
	}

	// ExistsCondition
	idx = 1
	sql2, _ := b.buildConditionWithSubquery(ExistsCondition{
		Subquery: sq,
		Not:      false,
	}, &idx)
	if sql2 == "" {
		t.Error("buildConditionWithSubquery should handle ExistsCondition")
	}

	// Unknown type
	idx = 1
	sql3, _ := b.buildConditionWithSubquery("unknown", &idx)
	if sql3 != "" {
		t.Error("buildConditionWithSubquery should return empty for unknown type")
	}
}

func TestBuilder_BuildWhereClause_WithSubqueryConditions(t *testing.T) {
	b := newTestBuilder()
	sq := RawSubquery("SELECT id FROM posts WHERE active = $1", true)

	b2 := b.WhereInSubquery("user_id", sq).
		WhereExists(RawSubquery("SELECT 1"))

	sql, args := b2.buildSelectQuery()
	if !strings.Contains(sql, "WHERE") {
		t.Errorf("SQL should contain WHERE, got %q", sql)
	}
	if len(args) != 1 {
		t.Errorf("args len = %d, want 1", len(args))
	}
}

func TestScalarSubquery(t *testing.T) {
	b := newTestBuilder().
		Where(Condition{Field: "category", Operator: OpEq, Value: "electronics"})

	scalar := b.ScalarSubquery("AVG(price)").ToScalar()
	sql := scalar.SQL()
	if !strings.Contains(sql, "AVG(price)") {
		t.Errorf("SQL = %q, should contain AVG(price)", sql)
	}
	if !strings.HasPrefix(sql, "(") || !strings.HasSuffix(sql, ")") {
		t.Errorf("ScalarSubquery SQL should be wrapped in parens, got %q", sql)
	}
}

func TestCorrelatedSubquery(t *testing.T) {
	b := newTestBuilder()
	sq := b.CorrelatedSubquery("id").
		Correlate("users.id = posts.user_id").
		ToSubquery()

	sql := sq.SQL()
	if !strings.Contains(sql, "users.id = posts.user_id") {
		t.Errorf("SQL = %q, should contain correlation", sql)
	}
	if !strings.Contains(sql, `SELECT "id"`) {
		t.Errorf("SQL = %q, should contain SELECT id", sql)
	}
}

func TestCorrelatedSubquery_WithConditions(t *testing.T) {
	b := newTestBuilder().
		Where(Condition{Field: "active", Operator: OpEq, Value: true})

	sq := b.CorrelatedSubquery("id").
		Correlate("users.id = posts.user_id").
		ToSubquery()

	sql := sq.SQL()
	if !strings.Contains(sql, "AND") {
		t.Errorf("SQL should contain AND for correlation + conditions, got %q", sql)
	}
}

func TestCorrelatedSubquery_NoCorrelation(t *testing.T) {
	b := newTestBuilder().
		Where(Condition{Field: "active", Operator: OpEq, Value: true})

	sq := b.CorrelatedSubquery("id").ToSubquery()
	sql := sq.SQL()
	if !strings.Contains(sql, "WHERE") {
		t.Errorf("SQL should contain WHERE, got %q", sql)
	}
}

func TestInSubquery_Fields(t *testing.T) {
	sq := RawSubquery("SELECT id FROM posts")

	// Int64Field
	cond1 := NewInt64Field("id").InSubquery(sq)
	if cond1.Field != "id" || cond1.Operator != OpIn {
		t.Errorf("Int64Field.InSubquery unexpected: %+v", cond1)
	}
	cond2 := NewInt64Field("id").NotInSubquery(sq)
	if cond2.Operator != OpNotIn {
		t.Errorf("Int64Field.NotInSubquery operator = %v", cond2.Operator)
	}

	// StringField
	cond3 := NewStringField("name").InSubquery(sq)
	if cond3.Field != "name" || cond3.Operator != OpIn {
		t.Errorf("StringField.InSubquery unexpected: %+v", cond3)
	}
	cond4 := NewStringField("name").NotInSubquery(sq)
	if cond4.Operator != OpNotIn {
		t.Errorf("StringField.NotInSubquery operator = %v", cond4.Operator)
	}

	// IntField
	cond5 := NewIntField("age").InSubquery(sq)
	if cond5.Field != "age" || cond5.Operator != OpIn {
		t.Errorf("IntField.InSubquery unexpected: %+v", cond5)
	}
	cond6 := NewIntField("age").NotInSubquery(sq)
	if cond6.Operator != OpNotIn {
		t.Errorf("IntField.NotInSubquery operator = %v", cond6.Operator)
	}
}
