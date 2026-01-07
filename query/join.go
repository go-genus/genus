package query

import (
	"fmt"

	"github.com/GabrielOnRails/genus/core"
)

// JoinType representa os tipos de JOIN SQL.
type JoinType string

const (
	InnerJoinType JoinType = "INNER JOIN"
	LeftJoinType  JoinType = "LEFT JOIN"
	RightJoinType JoinType = "RIGHT JOIN"
)

// JoinClause representa uma cláusula JOIN em SQL.
type JoinClause struct {
	Type      JoinType
	Table     string
	Alias     string
	Condition JoinCondition
}

// JoinCondition representa a condição ON do JOIN.
type JoinCondition struct {
	LeftColumn  string
	RightColumn string
	Operator    Operator // Reusa Operator de Condition (=, !=, etc.)
}

// On cria uma condição de JOIN básica com operador =.
// Exemplo: On("users.id", "posts.user_id") gera: users.id = posts.user_id
func On(leftCol, rightCol string) JoinCondition {
	return JoinCondition{
		LeftColumn:  leftCol,
		RightColumn: rightCol,
		Operator:    OpEq,
	}
}

// OnCustom cria uma condição de JOIN com operador customizado.
// Exemplo: OnCustom("users.id", "posts.user_id", OpGt) gera: users.id > posts.user_id
func OnCustom(leftCol, rightCol string, op Operator) JoinCondition {
	return JoinCondition{
		LeftColumn:  leftCol,
		RightColumn: rightCol,
		Operator:    op,
	}
}

// BuildSQL gera o SQL para a cláusula JOIN.
func (j *JoinClause) BuildSQL(dialect core.Dialect) string {
	tableRef := dialect.QuoteIdentifier(j.Table)
	if j.Alias != "" {
		tableRef = fmt.Sprintf("%s AS %s", tableRef, dialect.QuoteIdentifier(j.Alias))
	}

	onClause := fmt.Sprintf("%s %s %s",
		j.Condition.LeftColumn,
		j.Condition.Operator,
		j.Condition.RightColumn,
	)

	return fmt.Sprintf("%s %s ON %s", j.Type, tableRef, onClause)
}
