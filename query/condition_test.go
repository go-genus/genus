package query

import "testing"

func TestAnd(t *testing.T) {
	c1 := Condition{Field: "name", Operator: OpEq, Value: "John"}
	c2 := Condition{Field: "age", Operator: OpGt, Value: 18}

	group := And(c1, c2)

	if group.Operator != LogicalAnd {
		t.Errorf("And() operator = %v, want %v", group.Operator, LogicalAnd)
	}
	if len(group.Conditions) != 2 {
		t.Errorf("And() conditions len = %d, want 2", len(group.Conditions))
	}
}

func TestOr(t *testing.T) {
	c1 := Condition{Field: "status", Operator: OpEq, Value: "active"}
	c2 := Condition{Field: "status", Operator: OpEq, Value: "pending"}

	group := Or(c1, c2)

	if group.Operator != LogicalOr {
		t.Errorf("Or() operator = %v, want %v", group.Operator, LogicalOr)
	}
	if len(group.Conditions) != 2 {
		t.Errorf("Or() conditions len = %d, want 2", len(group.Conditions))
	}
}

func TestNot(t *testing.T) {
	tests := []struct {
		name     string
		input    Operator
		expected Operator
	}{
		{"Eq -> Ne", OpEq, OpNe},
		{"Ne -> Eq", OpNe, OpEq},
		{"IsNull -> IsNotNull", OpIsNull, OpIsNotNull},
		{"IsNotNull -> IsNull", OpIsNotNull, OpIsNull},
		{"In -> NotIn", OpIn, OpNotIn},
		{"NotIn -> In", OpNotIn, OpIn},
		{"Gt unchanged", OpGt, OpGt},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cond := Condition{Field: "f", Operator: tt.input, Value: "v"}
			result := Not(cond)
			if result.Operator != tt.expected {
				t.Errorf("Not(%v) = %v, want %v", tt.input, result.Operator, tt.expected)
			}
		})
	}
}
