package query

import "testing"

// --- StringField ---

func TestStringField(t *testing.T) {
	f := NewStringField("name")

	if f.ColumnName() != "name" {
		t.Errorf("ColumnName() = %q, want %q", f.ColumnName(), "name")
	}

	tests := []struct {
		name    string
		cond    Condition
		wantOp  Operator
		wantVal interface{}
	}{
		{"Eq", f.Eq("John"), OpEq, "John"},
		{"Ne", f.Ne("John"), OpNe, "John"},
		{"IsNull", f.IsNull(), OpIsNull, nil},
		{"IsNotNull", f.IsNotNull(), OpIsNotNull, nil},
		{"Like", f.Like("%john%"), OpLike, "%john%"},
		{"NotLike", f.NotLike("%john%"), OpNotLike, "%john%"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cond.Operator != tt.wantOp {
				t.Errorf("Operator = %v, want %v", tt.cond.Operator, tt.wantOp)
			}
			if tt.cond.Field != "name" {
				t.Errorf("Field = %q, want %q", tt.cond.Field, "name")
			}
		})
	}

	// In/NotIn
	inCond := f.In("a", "b", "c")
	if inCond.Operator != OpIn {
		t.Errorf("In() operator = %v, want %v", inCond.Operator, OpIn)
	}
	vals, ok := inCond.Value.([]string)
	if !ok || len(vals) != 3 {
		t.Errorf("In() value = %v, want []string with 3 elements", inCond.Value)
	}

	notInCond := f.NotIn("a", "b")
	if notInCond.Operator != OpNotIn {
		t.Errorf("NotIn() operator = %v, want %v", notInCond.Operator, OpNotIn)
	}
}

// --- IntField ---

func TestIntField(t *testing.T) {
	f := NewIntField("age")

	if f.ColumnName() != "age" {
		t.Errorf("ColumnName() = %q, want %q", f.ColumnName(), "age")
	}

	tests := []struct {
		name   string
		cond   Condition
		wantOp Operator
	}{
		{"Eq", f.Eq(18), OpEq},
		{"Ne", f.Ne(18), OpNe},
		{"Gt", f.Gt(18), OpGt},
		{"Gte", f.Gte(18), OpGte},
		{"Lt", f.Lt(18), OpLt},
		{"Lte", f.Lte(18), OpLte},
		{"IsNull", f.IsNull(), OpIsNull},
		{"IsNotNull", f.IsNotNull(), OpIsNotNull},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cond.Operator != tt.wantOp {
				t.Errorf("Operator = %v, want %v", tt.cond.Operator, tt.wantOp)
			}
		})
	}

	// In/NotIn
	inCond := f.In(1, 2, 3)
	if inCond.Operator != OpIn {
		t.Errorf("In() operator = %v, want %v", inCond.Operator, OpIn)
	}

	notInCond := f.NotIn(1, 2)
	if notInCond.Operator != OpNotIn {
		t.Errorf("NotIn() operator = %v, want %v", notInCond.Operator, OpNotIn)
	}

	// Between
	betweenCond := f.Between(10, 20)
	if betweenCond.Operator != OpBetween {
		t.Errorf("Between() operator = %v, want %v", betweenCond.Operator, OpBetween)
	}
}

// --- Int64Field ---

func TestInt64Field(t *testing.T) {
	f := NewInt64Field("id")

	if f.ColumnName() != "id" {
		t.Errorf("ColumnName() = %q, want %q", f.ColumnName(), "id")
	}

	tests := []struct {
		name   string
		cond   Condition
		wantOp Operator
	}{
		{"Eq", f.Eq(1), OpEq},
		{"Ne", f.Ne(1), OpNe},
		{"Gt", f.Gt(1), OpGt},
		{"Gte", f.Gte(1), OpGte},
		{"Lt", f.Lt(1), OpLt},
		{"Lte", f.Lte(1), OpLte},
		{"IsNull", f.IsNull(), OpIsNull},
		{"IsNotNull", f.IsNotNull(), OpIsNotNull},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cond.Operator != tt.wantOp {
				t.Errorf("Operator = %v, want %v", tt.cond.Operator, tt.wantOp)
			}
		})
	}

	inCond := f.In(1, 2, 3)
	if inCond.Operator != OpIn {
		t.Errorf("In() operator = %v, want %v", inCond.Operator, OpIn)
	}

	notInCond := f.NotIn(1, 2)
	if notInCond.Operator != OpNotIn {
		t.Errorf("NotIn() operator = %v, want %v", notInCond.Operator, OpNotIn)
	}

	betweenCond := f.Between(10, 20)
	if betweenCond.Operator != OpBetween {
		t.Errorf("Between() operator = %v, want %v", betweenCond.Operator, OpBetween)
	}
}

// --- BoolField ---

func TestBoolField(t *testing.T) {
	f := NewBoolField("active")

	if f.ColumnName() != "active" {
		t.Errorf("ColumnName() = %q, want %q", f.ColumnName(), "active")
	}

	tests := []struct {
		name   string
		cond   Condition
		wantOp Operator
	}{
		{"Eq", f.Eq(true), OpEq},
		{"Ne", f.Ne(false), OpNe},
		{"IsNull", f.IsNull(), OpIsNull},
		{"IsNotNull", f.IsNotNull(), OpIsNotNull},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cond.Operator != tt.wantOp {
				t.Errorf("Operator = %v, want %v", tt.cond.Operator, tt.wantOp)
			}
		})
	}

	inCond := f.In(true, false)
	if inCond.Operator != OpIn {
		t.Errorf("In() operator = %v, want %v", inCond.Operator, OpIn)
	}

	notInCond := f.NotIn(true)
	if notInCond.Operator != OpNotIn {
		t.Errorf("NotIn() operator = %v, want %v", notInCond.Operator, OpNotIn)
	}
}

// --- Float64Field ---

func TestFloat64Field(t *testing.T) {
	f := NewFloat64Field("price")

	if f.ColumnName() != "price" {
		t.Errorf("ColumnName() = %q, want %q", f.ColumnName(), "price")
	}

	tests := []struct {
		name   string
		cond   Condition
		wantOp Operator
	}{
		{"Eq", f.Eq(9.99), OpEq},
		{"Ne", f.Ne(9.99), OpNe},
		{"Gt", f.Gt(9.99), OpGt},
		{"Gte", f.Gte(9.99), OpGte},
		{"Lt", f.Lt(9.99), OpLt},
		{"Lte", f.Lte(9.99), OpLte},
		{"IsNull", f.IsNull(), OpIsNull},
		{"IsNotNull", f.IsNotNull(), OpIsNotNull},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cond.Operator != tt.wantOp {
				t.Errorf("Operator = %v, want %v", tt.cond.Operator, tt.wantOp)
			}
		})
	}

	inCond := f.In(1.1, 2.2)
	if inCond.Operator != OpIn {
		t.Errorf("In() operator = %v, want %v", inCond.Operator, OpIn)
	}

	notInCond := f.NotIn(1.1)
	if notInCond.Operator != OpNotIn {
		t.Errorf("NotIn() operator = %v, want %v", notInCond.Operator, OpNotIn)
	}

	betweenCond := f.Between(1.0, 10.0)
	if betweenCond.Operator != OpBetween {
		t.Errorf("Between() operator = %v, want %v", betweenCond.Operator, OpBetween)
	}
}

// --- Optional fields ---

func TestOptionalStringField(t *testing.T) {
	f := NewOptionalStringField("bio")

	if f.ColumnName() != "bio" {
		t.Errorf("ColumnName() = %q, want %q", f.ColumnName(), "bio")
	}

	tests := []struct {
		name   string
		cond   Condition
		wantOp Operator
	}{
		{"Eq", f.Eq("test"), OpEq},
		{"Ne", f.Ne("test"), OpNe},
		{"IsNull", f.IsNull(), OpIsNull},
		{"IsNotNull", f.IsNotNull(), OpIsNotNull},
		{"Like", f.Like("%test%"), OpLike},
		{"NotLike", f.NotLike("%test%"), OpNotLike},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cond.Operator != tt.wantOp {
				t.Errorf("Operator = %v, want %v", tt.cond.Operator, tt.wantOp)
			}
		})
	}

	inCond := f.In("a", "b")
	if inCond.Operator != OpIn {
		t.Errorf("In() operator = %v, want %v", inCond.Operator, OpIn)
	}
	notInCond := f.NotIn("a")
	if notInCond.Operator != OpNotIn {
		t.Errorf("NotIn() operator = %v, want %v", notInCond.Operator, OpNotIn)
	}
}

func TestOptionalIntField(t *testing.T) {
	f := NewOptionalIntField("score")

	if f.ColumnName() != "score" {
		t.Errorf("ColumnName() = %q, want %q", f.ColumnName(), "score")
	}

	tests := []struct {
		name   string
		cond   Condition
		wantOp Operator
	}{
		{"Eq", f.Eq(10), OpEq},
		{"Ne", f.Ne(10), OpNe},
		{"Gt", f.Gt(10), OpGt},
		{"Gte", f.Gte(10), OpGte},
		{"Lt", f.Lt(10), OpLt},
		{"Lte", f.Lte(10), OpLte},
		{"IsNull", f.IsNull(), OpIsNull},
		{"IsNotNull", f.IsNotNull(), OpIsNotNull},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cond.Operator != tt.wantOp {
				t.Errorf("Operator = %v, want %v", tt.cond.Operator, tt.wantOp)
			}
		})
	}

	inCond := f.In(1, 2)
	if inCond.Operator != OpIn {
		t.Errorf("In() operator = %v, want %v", inCond.Operator, OpIn)
	}
	notInCond := f.NotIn(1)
	if notInCond.Operator != OpNotIn {
		t.Errorf("NotIn() operator = %v, want %v", notInCond.Operator, OpNotIn)
	}
	betweenCond := f.Between(1, 10)
	if betweenCond.Operator != OpBetween {
		t.Errorf("Between() operator = %v, want %v", betweenCond.Operator, OpBetween)
	}
}

func TestOptionalInt64Field(t *testing.T) {
	f := NewOptionalInt64Field("ref_id")

	if f.ColumnName() != "ref_id" {
		t.Errorf("ColumnName() = %q, want %q", f.ColumnName(), "ref_id")
	}

	tests := []struct {
		name   string
		cond   Condition
		wantOp Operator
	}{
		{"Eq", f.Eq(1), OpEq},
		{"Ne", f.Ne(1), OpNe},
		{"Gt", f.Gt(1), OpGt},
		{"Gte", f.Gte(1), OpGte},
		{"Lt", f.Lt(1), OpLt},
		{"Lte", f.Lte(1), OpLte},
		{"IsNull", f.IsNull(), OpIsNull},
		{"IsNotNull", f.IsNotNull(), OpIsNotNull},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cond.Operator != tt.wantOp {
				t.Errorf("Operator = %v, want %v", tt.cond.Operator, tt.wantOp)
			}
		})
	}

	inCond := f.In(1, 2)
	if inCond.Operator != OpIn {
		t.Errorf("In() operator = %v, want %v", inCond.Operator, OpIn)
	}
	notInCond := f.NotIn(1)
	if notInCond.Operator != OpNotIn {
		t.Errorf("NotIn() operator = %v, want %v", notInCond.Operator, OpNotIn)
	}
	betweenCond := f.Between(1, 10)
	if betweenCond.Operator != OpBetween {
		t.Errorf("Between() operator = %v, want %v", betweenCond.Operator, OpBetween)
	}
}

func TestOptionalBoolField(t *testing.T) {
	f := NewOptionalBoolField("verified")

	if f.ColumnName() != "verified" {
		t.Errorf("ColumnName() = %q, want %q", f.ColumnName(), "verified")
	}

	tests := []struct {
		name   string
		cond   Condition
		wantOp Operator
	}{
		{"Eq", f.Eq(true), OpEq},
		{"Ne", f.Ne(false), OpNe},
		{"IsNull", f.IsNull(), OpIsNull},
		{"IsNotNull", f.IsNotNull(), OpIsNotNull},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cond.Operator != tt.wantOp {
				t.Errorf("Operator = %v, want %v", tt.cond.Operator, tt.wantOp)
			}
		})
	}

	inCond := f.In(true, false)
	if inCond.Operator != OpIn {
		t.Errorf("In() operator = %v, want %v", inCond.Operator, OpIn)
	}
	notInCond := f.NotIn(true)
	if notInCond.Operator != OpNotIn {
		t.Errorf("NotIn() operator = %v, want %v", notInCond.Operator, OpNotIn)
	}
}

func TestOptionalFloat64Field(t *testing.T) {
	f := NewOptionalFloat64Field("rating")

	if f.ColumnName() != "rating" {
		t.Errorf("ColumnName() = %q, want %q", f.ColumnName(), "rating")
	}

	tests := []struct {
		name   string
		cond   Condition
		wantOp Operator
	}{
		{"Eq", f.Eq(4.5), OpEq},
		{"Ne", f.Ne(4.5), OpNe},
		{"Gt", f.Gt(4.5), OpGt},
		{"Gte", f.Gte(4.5), OpGte},
		{"Lt", f.Lt(4.5), OpLt},
		{"Lte", f.Lte(4.5), OpLte},
		{"IsNull", f.IsNull(), OpIsNull},
		{"IsNotNull", f.IsNotNull(), OpIsNotNull},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cond.Operator != tt.wantOp {
				t.Errorf("Operator = %v, want %v", tt.cond.Operator, tt.wantOp)
			}
		})
	}

	inCond := f.In(1.1, 2.2)
	if inCond.Operator != OpIn {
		t.Errorf("In() operator = %v, want %v", inCond.Operator, OpIn)
	}
	notInCond := f.NotIn(1.1)
	if notInCond.Operator != OpNotIn {
		t.Errorf("NotIn() operator = %v, want %v", notInCond.Operator, OpNotIn)
	}
	betweenCond := f.Between(1.0, 5.0)
	if betweenCond.Operator != OpBetween {
		t.Errorf("Between() operator = %v, want %v", betweenCond.Operator, OpBetween)
	}
}
