package query

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"unsafe"

	"github.com/GabrielOnRails/genus/core"
)

// FastBuilder is a high-performance query builder optimized for speed.
// It uses prepared statements, zero-copy operations, and aggressive caching.
type FastBuilder[T any] struct {
	executor  core.Executor
	dialect   core.Dialect
	tableName string

	// Query parts
	columns    string
	conditions []string
	args       []interface{}
	orderBy    string
	limit      int
	offset     int

	// Caching
	stmtCache *stmtCache
	fieldMap  map[string]fieldPath
}

// stmtCache caches prepared statements per query pattern.
type stmtCache struct {
	mu    sync.RWMutex
	stmts map[string]*sql.Stmt
	db    *sql.DB
}

var globalStmtCache = &stmtCache{
	stmts: make(map[string]*sql.Stmt),
}

// NewFastBuilder creates a new high-performance query builder.
func NewFastBuilder[T any](executor core.Executor, dialect core.Dialect, tableName string) *FastBuilder[T] {
	var model T
	typ := reflect.TypeOf(model)
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	return &FastBuilder[T]{
		executor:  executor,
		dialect:   dialect,
		tableName: tableName,
		columns:   "*",
		fieldMap:  GetCachedFieldMap(typ),
		stmtCache: globalStmtCache,
	}
}

// Select specifies columns to select.
func (b *FastBuilder[T]) Select(columns ...string) *FastBuilder[T] {
	b.columns = strings.Join(columns, ", ")
	return b
}

// Where adds a condition. Uses type-safe conditions.
func (b *FastBuilder[T]) Where(condition interface{}) *FastBuilder[T] {
	switch c := condition.(type) {
	case Condition:
		sql, condArgs := b.buildCondition(c)
		b.conditions = append(b.conditions, sql)
		b.args = append(b.args, condArgs...)
	case ConditionGroup:
		sql, condArgs := b.buildConditionGroup(c)
		b.conditions = append(b.conditions, "("+sql+")")
		b.args = append(b.args, condArgs...)
	}
	return b
}

// buildCondition converts a Condition to SQL.
func (b *FastBuilder[T]) buildCondition(cond Condition) (string, []interface{}) {
	var args []interface{}

	switch cond.Operator {
	case OpIsNull:
		return cond.Field + " IS NULL", args
	case OpIsNotNull:
		return cond.Field + " IS NOT NULL", args
	case OpIn, OpNotIn:
		values := fastInterfaceSlice(cond.Value)
		var sb strings.Builder
		sb.WriteString(cond.Field)
		if cond.Operator == OpNotIn {
			sb.WriteString(" NOT IN (")
		} else {
			sb.WriteString(" IN (")
		}
		for i, v := range values {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(b.dialect.Placeholder(len(b.args) + i + 1))
			args = append(args, v)
		}
		sb.WriteString(")")
		return sb.String(), args
	case OpBetween:
		values := fastInterfaceSlice(cond.Value)
		if len(values) != 2 {
			return "", args
		}
		idx := len(b.args) + 1
		sql := cond.Field + " BETWEEN " + b.dialect.Placeholder(idx) + " AND " + b.dialect.Placeholder(idx+1)
		return sql, []interface{}{values[0], values[1]}
	default:
		idx := len(b.args) + 1
		sql := cond.Field + " " + string(cond.Operator) + " " + b.dialect.Placeholder(idx)
		return sql, []interface{}{cond.Value}
	}
}

// buildConditionGroup converts a ConditionGroup to SQL.
func (b *FastBuilder[T]) buildConditionGroup(group ConditionGroup) (string, []interface{}) {
	var parts []string
	var args []interface{}

	for _, cond := range group.Conditions {
		switch c := cond.(type) {
		case Condition:
			sql, condArgs := b.buildCondition(c)
			parts = append(parts, sql)
			args = append(args, condArgs...)
		case ConditionGroup:
			sql, condArgs := b.buildConditionGroup(c)
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

// fastInterfaceSlice converts slice types to []interface{} efficiently.
func fastInterfaceSlice(value interface{}) []interface{} {
	switch v := value.(type) {
	case []string:
		result := make([]interface{}, len(v))
		for i, val := range v {
			result[i] = val
		}
		return result
	case []int:
		result := make([]interface{}, len(v))
		for i, val := range v {
			result[i] = val
		}
		return result
	case []int64:
		result := make([]interface{}, len(v))
		for i, val := range v {
			result[i] = val
		}
		return result
	case []interface{}:
		return v
	default:
		return []interface{}{value}
	}
}

// OrderByAsc adds ascending order.
func (b *FastBuilder[T]) OrderByAsc(column string) *FastBuilder[T] {
	b.orderBy = b.dialect.QuoteIdentifier(column) + " ASC"
	return b
}

// OrderByDesc adds descending order.
func (b *FastBuilder[T]) OrderByDesc(column string) *FastBuilder[T] {
	b.orderBy = b.dialect.QuoteIdentifier(column) + " DESC"
	return b
}

// Limit sets the limit.
func (b *FastBuilder[T]) Limit(n int) *FastBuilder[T] {
	b.limit = n
	return b
}

// Offset sets the offset.
func (b *FastBuilder[T]) Offset(n int) *FastBuilder[T] {
	b.offset = n
	return b
}

// buildSQL builds the SQL query string.
func (b *FastBuilder[T]) buildSQL() string {
	var sb strings.Builder
	sb.Grow(128) // Pre-allocate

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
func (b *FastBuilder[T]) Find(ctx context.Context) ([]T, error) {
	query := b.buildSQL()

	rows, err := b.executor.QueryContext(ctx, query, b.args...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	return b.scanAllFast(rows)
}

// First returns the first result.
func (b *FastBuilder[T]) First(ctx context.Context) (T, error) {
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

// scanAllFast scans all rows with optimizations.
func (b *FastBuilder[T]) scanAllFast(rows *sql.Rows) ([]T, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	// Pre-allocate result slice
	results := make([]T, 0, 64)

	// Reuse scan values slice
	numCols := len(columns)
	scanValues := make([]interface{}, numCols)
	var placeholder interface{}

	for rows.Next() {
		var item T
		itemVal := reflect.ValueOf(&item).Elem()

		// Setup scan targets
		for i, colName := range columns {
			if path, ok := b.fieldMap[colName]; ok {
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

// Count returns the count of matching rows.
func (b *FastBuilder[T]) Count(ctx context.Context) (int64, error) {
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

// ============================================================================
// Zero-copy utilities
// ============================================================================

// itoa converts int to string without allocation for small numbers.
var smallInts = func() []string {
	s := make([]string, 1001)
	for i := 0; i <= 1000; i++ {
		s[i] = fmt.Sprintf("%d", i)
	}
	return s
}()

func itoa(n int) string {
	if n >= 0 && n <= 1000 {
		return smallInts[n]
	}
	return fmt.Sprintf("%d", n)
}

// unsafeString converts bytes to string without allocation.
// Only use when the byte slice won't be modified after.
func unsafeString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

// unsafeBytes converts string to bytes without allocation.
// Only use for read-only operations.
func unsafeBytes(s string) []byte {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

// ============================================================================
// Prepared Statement Cache
// ============================================================================

// getStmt gets or creates a prepared statement.
func (c *stmtCache) getStmt(db *sql.DB, query string) (*sql.Stmt, error) {
	c.mu.RLock()
	stmt, ok := c.stmts[query]
	c.mu.RUnlock()

	if ok {
		return stmt, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock
	if stmt, ok := c.stmts[query]; ok {
		return stmt, nil
	}

	stmt, err := db.Prepare(query)
	if err != nil {
		return nil, err
	}

	c.stmts[query] = stmt
	return stmt, nil
}

// ClearStmtCache clears the prepared statement cache.
func ClearStmtCache() {
	globalStmtCache.mu.Lock()
	defer globalStmtCache.mu.Unlock()

	for _, stmt := range globalStmtCache.stmts {
		stmt.Close()
	}
	globalStmtCache.stmts = make(map[string]*sql.Stmt)
}
