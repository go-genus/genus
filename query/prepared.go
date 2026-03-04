package query

import (
	"context"
	"database/sql"
	"reflect"
	"strings"
	"sync"

	"github.com/GabrielOnRails/genus/core"
)

// PreparedQuery is a pre-built query that can be executed multiple times with different arguments.
// It eliminates the overhead of building the SQL string on each execution.
type PreparedQuery[T any] struct {
	sql       string
	scanFunc  ScanFunc[T]
	executor  core.Executor
	numParams int
}

// preparedQueryCache caches PreparedQuery by a key (usually table + conditions pattern).
var preparedQueryCache sync.Map

// PrepareSelectAll creates a prepared SELECT query for the given table.
// The query can be executed multiple times with different WHERE values.
func PrepareSelectAll[T any](executor core.Executor, dialect core.Dialect, tableName string, columns []string, whereColumns []string) *PreparedQuery[T] {
	var sb strings.Builder
	sb.Grow(128)

	sb.WriteString("SELECT ")
	if len(columns) > 0 {
		sb.WriteString(strings.Join(columns, ", "))
	} else {
		sb.WriteString("*")
	}
	sb.WriteString(" FROM ")
	sb.WriteString(dialect.QuoteIdentifier(tableName))

	if len(whereColumns) > 0 {
		sb.WriteString(" WHERE ")
		for i, col := range whereColumns {
			if i > 0 {
				sb.WriteString(" AND ")
			}
			sb.WriteString(col)
			sb.WriteString(" = ")
			sb.WriteString(dialect.Placeholder(i + 1))
		}
	}

	pq := &PreparedQuery[T]{
		sql:       sb.String(),
		executor:  executor,
		numParams: len(whereColumns),
	}

	// Try to get registered scan function
	var model T
	typ := reflect.TypeOf(model)
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	if fn, ok := scanFuncRegistry.Load(typ); ok {
		pq.scanFunc = fn.(ScanFunc[T])
	}

	return pq
}

// Exec executes the prepared query with the given arguments.
func (pq *PreparedQuery[T]) Exec(ctx context.Context, args ...interface{}) ([]T, error) {
	rows, err := pq.executor.QueryContext(ctx, pq.sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if pq.scanFunc != nil {
		return pq.execWithScanFunc(rows)
	}
	return pq.execWithReflection(rows)
}

func (pq *PreparedQuery[T]) execWithScanFunc(rows *sql.Rows) ([]T, error) {
	results := make([]T, 0, 64)
	for rows.Next() {
		item, err := pq.scanFunc(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, item)
	}
	return results, rows.Err()
}

func (pq *PreparedQuery[T]) execWithReflection(rows *sql.Rows) ([]T, error) {
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
	results := make([]T, 0, 64)
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

// ExecFirst executes the prepared query and returns the first result.
func (pq *PreparedQuery[T]) ExecFirst(ctx context.Context, args ...interface{}) (T, error) {
	results, err := pq.Exec(ctx, args...)
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

// SQL returns the prepared SQL string.
func (pq *PreparedQuery[T]) SQL() string {
	return pq.sql
}
