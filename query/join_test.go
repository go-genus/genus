package query

import (
	"strings"
	"testing"
)

func TestOn(t *testing.T) {
	cond := On("users.id", "posts.user_id")
	if cond.LeftColumn != "users.id" {
		t.Errorf("LeftColumn = %q, want %q", cond.LeftColumn, "users.id")
	}
	if cond.RightColumn != "posts.user_id" {
		t.Errorf("RightColumn = %q, want %q", cond.RightColumn, "posts.user_id")
	}
	if cond.Operator != OpEq {
		t.Errorf("Operator = %v, want %v", cond.Operator, OpEq)
	}
}

func TestOnCustom(t *testing.T) {
	cond := OnCustom("a.id", "b.ref", OpGt)
	if cond.Operator != OpGt {
		t.Errorf("Operator = %v, want %v", cond.Operator, OpGt)
	}
}

func TestJoinClause_BuildSQL(t *testing.T) {
	d := &mockDialect{}

	tests := []struct {
		name     string
		join     JoinClause
		contains []string
	}{
		{
			"inner join",
			JoinClause{
				Type:  InnerJoinType,
				Table: "posts",
				Condition: JoinCondition{
					LeftColumn:  "users.id",
					RightColumn: "posts.user_id",
					Operator:    OpEq,
				},
			},
			[]string{"INNER JOIN", `"posts"`, "ON", "users.id = posts.user_id"},
		},
		{
			"left join with alias",
			JoinClause{
				Type:  LeftJoinType,
				Table: "posts",
				Alias: "p",
				Condition: JoinCondition{
					LeftColumn:  "users.id",
					RightColumn: "p.user_id",
					Operator:    OpEq,
				},
			},
			[]string{"LEFT JOIN", `"posts"`, "AS", `"p"`, "ON"},
		},
		{
			"right join",
			JoinClause{
				Type:  RightJoinType,
				Table: "orders",
				Condition: JoinCondition{
					LeftColumn:  "users.id",
					RightColumn: "orders.user_id",
					Operator:    OpEq,
				},
			},
			[]string{"RIGHT JOIN", `"orders"`, "ON"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql := tt.join.BuildSQL(d)
			for _, want := range tt.contains {
				if !strings.Contains(sql, want) {
					t.Errorf("BuildSQL() = %q, should contain %q", sql, want)
				}
			}
		})
	}
}
