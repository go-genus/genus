package query

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/go-genus/genus/core"
)

// UltraFastBuilder is a zero-reflection query builder for maximum performance.
// It achieves this by using generated scan functions instead of reflection.
type UltraFastBuilder[T any] struct {
	executor   core.Executor
	dialect    core.Dialect
	tableName  string
	columns    string
	conditions []string
	args       []interface{}
	orderBy    string
	limit      int
	offset     int
	scanFunc   func(*sql.Rows) (T, error)
}

// ScanFunc is the type signature for generated scanner functions.
type ScanFunc[T any] func(*sql.Rows) (T, error)

// scanFuncRegistry stores registered scan functions by type.
var scanFuncRegistry sync.Map // map[reflect.Type]interface{}

// RegisterScanFunc registers a scan function for a type.
// Call this at init time for each model with a generated scanner.
func RegisterScanFunc[T any](fn ScanFunc[T]) {
	var model T
	typ := reflect.TypeOf(model)
	scanFuncRegistry.Store(typ, fn)
}

// NewUltraFastBuilder creates a new ultra-fast query builder.
// If a scan function is registered for type T, it will be used instead of reflection.
func NewUltraFastBuilder[T any](executor core.Executor, dialect core.Dialect, tableName string) *UltraFastBuilder[T] {
	var model T
	typ := reflect.TypeOf(model)
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	builder := &UltraFastBuilder[T]{
		executor:  executor,
		dialect:   dialect,
		tableName: tableName,
		columns:   "*",
	}

	// Try to get registered scan function
	if fn, ok := scanFuncRegistry.Load(typ); ok {
		builder.scanFunc = fn.(ScanFunc[T])
	}

	return builder
}

// WithScanFunc sets a custom scan function for this query.
func (b *UltraFastBuilder[T]) WithScanFunc(fn ScanFunc[T]) *UltraFastBuilder[T] {
	b.scanFunc = fn
	return b
}

// Select specifies columns to select.
func (b *UltraFastBuilder[T]) Select(columns ...string) *UltraFastBuilder[T] {
	b.columns = strings.Join(columns, ", ")
	return b
}

// Where adds a condition.
func (b *UltraFastBuilder[T]) Where(condition interface{}) *UltraFastBuilder[T] {
	switch c := condition.(type) {
	case Condition:
		sql, condArgs := b.buildCond(c)
		b.conditions = append(b.conditions, sql)
		b.args = append(b.args, condArgs...)
	case ConditionGroup:
		sql, condArgs := b.buildCondGroup(c)
		b.conditions = append(b.conditions, "("+sql+")")
		b.args = append(b.args, condArgs...)
	}
	return b
}

func (b *UltraFastBuilder[T]) buildCond(cond Condition) (string, []interface{}) {
	var args []interface{}
	switch cond.Operator {
	case OpIsNull:
		return cond.Field + " IS NULL", args
	case OpIsNotNull:
		return cond.Field + " IS NOT NULL", args
	default:
		idx := len(b.args) + 1
		sql := cond.Field + " " + string(cond.Operator) + " " + b.dialect.Placeholder(idx)
		return sql, []interface{}{cond.Value}
	}
}

func (b *UltraFastBuilder[T]) buildCondGroup(group ConditionGroup) (string, []interface{}) {
	var parts []string
	var args []interface{}
	for _, cond := range group.Conditions {
		switch c := cond.(type) {
		case Condition:
			sql, condArgs := b.buildCond(c)
			parts = append(parts, sql)
			args = append(args, condArgs...)
		case ConditionGroup:
			sql, condArgs := b.buildCondGroup(c)
			parts = append(parts, "("+sql+")")
			args = append(args, condArgs...)
		}
	}
	operator := " AND "
	if group.Operator == LogicalOr {
		operator = " OR "
	}
	return strings.Join(parts, operator), args
}

// OrderByAsc adds ascending order.
func (b *UltraFastBuilder[T]) OrderByAsc(column string) *UltraFastBuilder[T] {
	b.orderBy = b.dialect.QuoteIdentifier(column) + " ASC"
	return b
}

// OrderByDesc adds descending order.
func (b *UltraFastBuilder[T]) OrderByDesc(column string) *UltraFastBuilder[T] {
	b.orderBy = b.dialect.QuoteIdentifier(column) + " DESC"
	return b
}

// Limit sets the limit.
func (b *UltraFastBuilder[T]) Limit(n int) *UltraFastBuilder[T] {
	b.limit = n
	return b
}

// Offset sets the offset.
func (b *UltraFastBuilder[T]) Offset(n int) *UltraFastBuilder[T] {
	b.offset = n
	return b
}

// buildSQL builds the SQL query.
func (b *UltraFastBuilder[T]) buildSQL() string {
	var sb strings.Builder
	sb.Grow(128)

	sb.WriteString("SELECT ")
	sb.WriteString(b.columns)
	sb.WriteString(" FROM ")
	sb.WriteString(b.dialect.QuoteIdentifier(b.tableName))

	if len(b.conditions) > 0 {
		sb.WriteString(" WHERE ")
		sb.WriteString(strings.Join(b.conditions, " AND "))
	}

	if b.orderBy != "" {
		sb.WriteString(" ORDER BY ")
		sb.WriteString(b.orderBy)
	}

	if b.limit > 0 {
		sb.WriteString(" LIMIT ")
		sb.WriteString(itoa(b.limit))
	}

	if b.offset > 0 {
		sb.WriteString(" OFFSET ")
		sb.WriteString(itoa(b.offset))
	}

	return sb.String()
}

// Find executes the query and returns results.
func (b *UltraFastBuilder[T]) Find(ctx context.Context) ([]T, error) {
	query := b.buildSQL()

	rows, err := b.executor.QueryContext(ctx, query, b.args...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	// Use registered scan function if available (zero-reflection)
	if b.scanFunc != nil {
		return b.scanWithFunc(rows)
	}

	// Fall back to reflection-based scanning
	return b.scanWithReflection(rows)
}

// scanWithFunc uses the registered scan function (zero-reflection).
func (b *UltraFastBuilder[T]) scanWithFunc(rows *sql.Rows) ([]T, error) {
	results := make([]T, 0, 32)
	for rows.Next() {
		item, err := b.scanFunc(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, item)
	}
	return results, rows.Err()
}

// scanWithReflection uses reflection-based scanning as fallback.
func (b *UltraFastBuilder[T]) scanWithReflection(rows *sql.Rows) ([]T, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var model T
	typ := reflect.TypeOf(model)
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	fieldMap := GetCachedFieldMap(typ)
	results := make([]T, 0, 32)
	numCols := len(columns)
	scanValues := make([]interface{}, numCols)
	var placeholder interface{}

	for rows.Next() {
		var item T
		itemVal := reflect.ValueOf(&item).Elem()

		for i, colName := range columns {
			if path, ok := fieldMap[colName]; ok {
				field := getFieldByPath(itemVal, path)
				if field.IsValid() && field.CanAddr() {
					scanValues[i] = field.Addr().Interface()
				} else {
					scanValues[i] = &placeholder
				}
			} else {
				scanValues[i] = &placeholder
			}
		}

		if err := rows.Scan(scanValues...); err != nil {
			return nil, err
		}
		results = append(results, item)
	}

	return results, rows.Err()
}

// First returns the first result.
func (b *UltraFastBuilder[T]) First(ctx context.Context) (T, error) {
	b.limit = 1
	results, err := b.Find(ctx)
	if err != nil {
		var zero T
		return zero, err
	}
	if len(results) == 0 {
		var zero T
		return zero, sql.ErrNoRows
	}
	return results[0], nil
}

// Count returns the count of matching rows.
func (b *UltraFastBuilder[T]) Count(ctx context.Context) (int64, error) {
	var sb strings.Builder
	sb.WriteString("SELECT COUNT(*) FROM ")
	sb.WriteString(b.dialect.QuoteIdentifier(b.tableName))

	if len(b.conditions) > 0 {
		sb.WriteString(" WHERE ")
		sb.WriteString(strings.Join(b.conditions, " AND "))
	}

	var count int64
	err := b.executor.QueryRowContext(ctx, sb.String(), b.args...).Scan(&count)
	return count, err
}
