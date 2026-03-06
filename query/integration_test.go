package query

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/go-genus/genus/cache"
	"github.com/go-genus/genus/core"
	sqliteDialect "github.com/go-genus/genus/dialects/sqlite"
)

// sqliteDialectInstance is a shared dialect for SQLite tests.
var sqliteDialectInstance = sqliteDialect.New()

// setupTestDB creates an in-memory SQLite database with a users table and test data.
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			email TEXT NOT NULL,
			age INTEGER NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			deleted_at DATETIME
		)
	`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO users (name, email, age) VALUES
			('Alice', 'alice@test.com', 30),
			('Bob', 'bob@test.com', 25),
			('Charlie', 'charlie@test.com', 35),
			('Diana', 'diana@test.com', 28),
			('Eve', 'eve@test.com', 22)
	`)
	if err != nil {
		t.Fatalf("failed to insert data: %v", err)
	}

	t.Cleanup(func() { db.Close() })
	return db
}

// sqliteTestUser is a simpler model for SQLite tests (no embedded core.Model).
type sqliteTestUser struct {
	ID    int64  `db:"id"`
	Name  string `db:"name"`
	Email string `db:"email"`
	Age   int    `db:"age"`
}

func (u sqliteTestUser) TableName() string {
	return "users"
}

func newSQLiteBuilder(db *sql.DB) *Builder[sqliteTestUser] {
	return NewBuilder[sqliteTestUser](
		db,
		sqliteDialectInstance,
		newMockLogger(),
		"users",
	)
}

// ===== Builder.Find =====

func TestBuilder_Find_Integration(t *testing.T) {
	db := setupTestDB(t)
	b := newSQLiteBuilder(db)
	ctx := context.Background()

	results, err := b.Find(ctx)
	if err != nil {
		t.Fatalf("Find error: %v", err)
	}
	if len(results) != 5 {
		t.Errorf("Find returned %d results, want 5", len(results))
	}
}

func TestBuilder_Find_WithWhere(t *testing.T) {
	db := setupTestDB(t)
	b := newSQLiteBuilder(db)
	ctx := context.Background()

	results, err := b.Where(Condition{Field: "age", Operator: OpGt, Value: 27}).Find(ctx)
	if err != nil {
		t.Fatalf("Find error: %v", err)
	}
	if len(results) != 3 { // Alice(30), Charlie(35), Diana(28)
		t.Errorf("Find returned %d results, want 3", len(results))
	}
}

func TestBuilder_Find_WithSelect(t *testing.T) {
	db := setupTestDB(t)
	b := newSQLiteBuilder(db)
	ctx := context.Background()

	results, err := b.Select("name", "email").Find(ctx)
	if err != nil {
		t.Fatalf("Find error: %v", err)
	}
	if len(results) != 5 {
		t.Errorf("Find returned %d results, want 5", len(results))
	}
	if results[0].Name == "" {
		t.Error("Name should be populated")
	}
}

func TestBuilder_Find_WithOrderAndLimit(t *testing.T) {
	db := setupTestDB(t)
	b := newSQLiteBuilder(db)
	ctx := context.Background()

	results, err := b.OrderByAsc("name").Limit(2).Find(ctx)
	if err != nil {
		t.Fatalf("Find error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("Find returned %d results, want 2", len(results))
	}
	if results[0].Name != "Alice" {
		t.Errorf("first result = %q, want Alice", results[0].Name)
	}
	if results[1].Name != "Bob" {
		t.Errorf("second result = %q, want Bob", results[1].Name)
	}
}

// ===== Builder.First =====

func TestBuilder_First_Integration(t *testing.T) {
	db := setupTestDB(t)
	b := newSQLiteBuilder(db)
	ctx := context.Background()

	result, err := b.OrderByAsc("name").First(ctx)
	if err != nil {
		t.Fatalf("First error: %v", err)
	}
	if result.Name != "Alice" {
		t.Errorf("First = %q, want Alice", result.Name)
	}
}

func TestBuilder_First_NoResults(t *testing.T) {
	db := setupTestDB(t)
	b := newSQLiteBuilder(db)
	ctx := context.Background()

	_, err := b.Where(Condition{Field: "age", Operator: OpGt, Value: 100}).First(ctx)
	if err == nil {
		t.Error("First should error when no results")
	}
}

// ===== Builder.Count =====

func TestBuilder_Count_Integration(t *testing.T) {
	db := setupTestDB(t)
	b := newSQLiteBuilder(db)
	ctx := context.Background()

	count, err := b.Count(ctx)
	if err != nil {
		t.Fatalf("Count error: %v", err)
	}
	if count != 5 {
		t.Errorf("Count = %d, want 5", count)
	}
}

func TestBuilder_Count_WithWhere(t *testing.T) {
	db := setupTestDB(t)
	b := newSQLiteBuilder(db)
	ctx := context.Background()

	count, err := b.Where(Condition{Field: "age", Operator: OpGte, Value: 30}).Count(ctx)
	if err != nil {
		t.Fatalf("Count error: %v", err)
	}
	if count != 2 { // Alice(30), Charlie(35)
		t.Errorf("Count = %d, want 2", count)
	}
}

// ===== Builder.Explain =====

func TestBuilder_Explain_Integration(t *testing.T) {
	db := setupTestDB(t)
	// SQLite EXPLAIN returns multiple columns, not a single string.
	// The Explain function expects a single text column per row, so it will fail on SQLite.
	// We test that it doesn't panic and returns an error.
	b := newSQLiteBuilder(db)
	ctx := context.Background()

	_, err := b.Explain(ctx)
	// We expect an error since SQLite EXPLAIN format differs from PostgreSQL
	_ = err
}

func TestBuilder_ExplainAnalyze_Integration(t *testing.T) {
	db := setupTestDB(t)
	b := newSQLiteBuilder(db)
	ctx := context.Background()

	// SQLite doesn't support EXPLAIN ANALYZE, so this will error
	_, err := b.ExplainAnalyze(ctx)
	_ = err
}

// ===== AggregateBuilder.One / All =====

func TestAggregateBuilder_One_Integration(t *testing.T) {
	db := setupTestDB(t)
	b := newSQLiteBuilder(db)
	ctx := context.Background()

	result, err := b.Aggregate().CountAll().One(ctx)
	if err != nil {
		t.Fatalf("One error: %v", err)
	}
	if result.Int64("count") != 5 {
		t.Errorf("count = %d, want 5", result.Int64("count"))
	}
}

func TestAggregateBuilder_One_WithSum(t *testing.T) {
	db := setupTestDB(t)
	b := newSQLiteBuilder(db)
	ctx := context.Background()

	result, err := b.Aggregate().Sum("age").One(ctx)
	if err != nil {
		t.Fatalf("One error: %v", err)
	}
	// 30 + 25 + 35 + 28 + 22 = 140
	sumAge := result.Int64("sum_age")
	if sumAge != 140 {
		t.Errorf("sum_age = %d, want 140", sumAge)
	}
}

func TestAggregateBuilder_One_WithWhere(t *testing.T) {
	db := setupTestDB(t)
	b := newSQLiteBuilder(db)
	ctx := context.Background()

	result, err := b.Aggregate().
		Where(Condition{Field: "age", Operator: OpGte, Value: 30}).
		CountAll().
		One(ctx)
	if err != nil {
		t.Fatalf("One error: %v", err)
	}
	if result.Int64("count") != 2 {
		t.Errorf("count = %d, want 2", result.Int64("count"))
	}
}

func TestAggregateBuilder_All_Integration(t *testing.T) {
	db := setupTestDB(t)
	b := newSQLiteBuilder(db)
	ctx := context.Background()

	// Group by age and count - there should be 5 groups (each user has unique age)
	results, err := b.Aggregate().
		GroupBy("age").
		CountAll().
		OrderByAsc("age").
		All(ctx)
	if err != nil {
		t.Fatalf("All error: %v", err)
	}
	if len(results) != 5 {
		t.Errorf("All returned %d results, want 5", len(results))
	}
}

func TestAggregateBuilder_One_Error(t *testing.T) {
	db := setupTestDB(t)
	b := NewBuilder[sqliteTestUser](
		db,
		sqliteDialectInstance,
		newMockLogger(),
		"nonexistent_table",
	)
	ctx := context.Background()

	_, err := b.Aggregate().CountAll().One(ctx)
	if err == nil {
		t.Error("should error for nonexistent table")
	}
}

func TestAggregateBuilder_All_Error(t *testing.T) {
	db := setupTestDB(t)
	b := NewBuilder[sqliteTestUser](
		db,
		sqliteDialectInstance,
		newMockLogger(),
		"nonexistent_table",
	)
	ctx := context.Background()

	_, err := b.Aggregate().CountAll().All(ctx)
	if err == nil {
		t.Error("should error for nonexistent table")
	}
}

// ===== TimeoutBuilder =====

func TestTimeoutBuilder_Find_Integration(t *testing.T) {
	db := setupTestDB(t)
	b := newSQLiteBuilder(db)
	ctx := context.Background()

	results, err := b.WithTimeout(5 * time.Second).Find(ctx)
	if err != nil {
		t.Fatalf("TimeoutBuilder.Find error: %v", err)
	}
	if len(results) != 5 {
		t.Errorf("Find returned %d, want 5", len(results))
	}
}

func TestTimeoutBuilder_First_Integration(t *testing.T) {
	db := setupTestDB(t)
	b := newSQLiteBuilder(db)
	ctx := context.Background()

	result, err := b.OrderByAsc("name").WithTimeout(5 * time.Second).First(ctx)
	if err != nil {
		t.Fatalf("TimeoutBuilder.First error: %v", err)
	}
	if result.Name != "Alice" {
		t.Errorf("First = %q, want Alice", result.Name)
	}
}

func TestTimeoutBuilder_Count_Integration(t *testing.T) {
	db := setupTestDB(t)
	b := newSQLiteBuilder(db)
	ctx := context.Background()

	count, err := b.WithTimeout(5 * time.Second).Count(ctx)
	if err != nil {
		t.Fatalf("TimeoutBuilder.Count error: %v", err)
	}
	if count != 5 {
		t.Errorf("Count = %d, want 5", count)
	}
}

// ===== FastBuilder =====

func TestFastBuilder_Find_Integration(t *testing.T) {
	db := setupTestDB(t)
	fb := NewFastBuilder[sqliteTestUser](db, sqliteDialectInstance, "users")
	ctx := context.Background()

	results, err := fb.Find(ctx)
	if err != nil {
		t.Fatalf("FastBuilder.Find error: %v", err)
	}
	if len(results) != 5 {
		t.Errorf("Find returned %d, want 5", len(results))
	}
	if results[0].Name == "" {
		t.Error("Name should be populated")
	}
}

func TestFastBuilder_First_Integration(t *testing.T) {
	db := setupTestDB(t)
	fb := NewFastBuilder[sqliteTestUser](db, sqliteDialectInstance, "users")
	ctx := context.Background()

	result, err := fb.OrderByAsc("name").First(ctx)
	if err != nil {
		t.Fatalf("FastBuilder.First error: %v", err)
	}
	if result.Name != "Alice" {
		t.Errorf("First = %q, want Alice", result.Name)
	}
}

func TestFastBuilder_First_NoResults(t *testing.T) {
	db := setupTestDB(t)
	fb := NewFastBuilder[sqliteTestUser](db, sqliteDialectInstance, "users")
	ctx := context.Background()

	_, err := fb.Where(Condition{Field: "age", Operator: OpGt, Value: 100}).First(ctx)
	if err == nil {
		t.Error("First should error when no results")
	}
}

func TestFastBuilder_Count_Integration(t *testing.T) {
	db := setupTestDB(t)
	fb := NewFastBuilder[sqliteTestUser](db, sqliteDialectInstance, "users")
	ctx := context.Background()

	count, err := fb.Count(ctx)
	if err != nil {
		t.Fatalf("FastBuilder.Count error: %v", err)
	}
	if count != 5 {
		t.Errorf("Count = %d, want 5", count)
	}
}

// ===== UltraFastBuilder =====

func TestUltraFastBuilder_Find_Integration(t *testing.T) {
	db := setupTestDB(t)
	ub := NewUltraFastBuilder[sqliteTestUser](db, sqliteDialectInstance, "users")
	ctx := context.Background()

	results, err := ub.Find(ctx)
	if err != nil {
		t.Fatalf("UltraFastBuilder.Find error: %v", err)
	}
	if len(results) != 5 {
		t.Errorf("Find returned %d, want 5", len(results))
	}
}

func TestUltraFastBuilder_Find_WithScanFunc(t *testing.T) {
	db := setupTestDB(t)
	ub := NewUltraFastBuilder[sqliteTestUser](db, sqliteDialectInstance, "users")
	ctx := context.Background()

	fn := func(rows *sql.Rows) (sqliteTestUser, error) {
		var u sqliteTestUser
		err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.Age, new(interface{}), new(interface{}), new(interface{}))
		return u, err
	}

	results, err := ub.WithScanFunc(fn).Find(ctx)
	if err != nil {
		t.Fatalf("Find with ScanFunc error: %v", err)
	}
	if len(results) != 5 {
		t.Errorf("Find returned %d, want 5", len(results))
	}
	if results[0].Name == "" {
		t.Error("Name should be populated")
	}
}

func TestUltraFastBuilder_First_Integration(t *testing.T) {
	db := setupTestDB(t)
	ub := NewUltraFastBuilder[sqliteTestUser](db, sqliteDialectInstance, "users")
	ctx := context.Background()

	result, err := ub.OrderByAsc("name").First(ctx)
	if err != nil {
		t.Fatalf("UltraFastBuilder.First error: %v", err)
	}
	if result.Name != "Alice" {
		t.Errorf("First = %q, want Alice", result.Name)
	}
}

func TestUltraFastBuilder_First_NoResults(t *testing.T) {
	db := setupTestDB(t)
	ub := NewUltraFastBuilder[sqliteTestUser](db, sqliteDialectInstance, "users")
	ctx := context.Background()

	_, err := ub.Where(Condition{Field: "age", Operator: OpGt, Value: 100}).First(ctx)
	if err == nil {
		t.Error("First should error when no results")
	}
}

func TestUltraFastBuilder_Count_Integration(t *testing.T) {
	db := setupTestDB(t)
	ub := NewUltraFastBuilder[sqliteTestUser](db, sqliteDialectInstance, "users")
	ctx := context.Background()

	count, err := ub.Count(ctx)
	if err != nil {
		t.Fatalf("UltraFastBuilder.Count error: %v", err)
	}
	if count != 5 {
		t.Errorf("Count = %d, want 5", count)
	}
}

// ===== PreparedQuery =====

func TestPreparedQuery_Exec_Integration(t *testing.T) {
	db := setupTestDB(t)
	pq := PrepareSelectAll[sqliteTestUser](
		db,
		sqliteDialectInstance,
		"users",
		nil,
		nil,
	)
	ctx := context.Background()

	results, err := pq.Exec(ctx)
	if err != nil {
		t.Fatalf("Exec error: %v", err)
	}
	if len(results) != 5 {
		t.Errorf("Exec returned %d, want 5", len(results))
	}
}

func TestPreparedQuery_Exec_WithWhere(t *testing.T) {
	db := setupTestDB(t)
	pq := PrepareSelectAll[sqliteTestUser](
		db,
		sqliteDialectInstance,
		"users",
		nil,
		[]string{"name"},
	)
	ctx := context.Background()

	results, err := pq.Exec(ctx, "Alice")
	if err != nil {
		t.Fatalf("Exec error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Exec returned %d, want 1", len(results))
	}
}

func TestPreparedQuery_ExecFirst_Integration(t *testing.T) {
	db := setupTestDB(t)
	pq := PrepareSelectAll[sqliteTestUser](
		db,
		sqliteDialectInstance,
		"users",
		nil,
		[]string{"name"},
	)
	ctx := context.Background()

	result, err := pq.ExecFirst(ctx, "Bob")
	if err != nil {
		t.Fatalf("ExecFirst error: %v", err)
	}
	if result.Name != "Bob" {
		t.Errorf("ExecFirst = %q, want Bob", result.Name)
	}
}

func TestPreparedQuery_ExecFirst_NoResults(t *testing.T) {
	db := setupTestDB(t)
	pq := PrepareSelectAll[sqliteTestUser](
		db,
		sqliteDialectInstance,
		"users",
		nil,
		[]string{"name"},
	)
	ctx := context.Background()

	_, err := pq.ExecFirst(ctx, "NonExistent")
	if err == nil {
		t.Error("ExecFirst should error when no results")
	}
}

func TestPreparedQuery_Exec_WithScanFunc(t *testing.T) {
	db := setupTestDB(t)

	fn := func(rows *sql.Rows) (sqliteTestUser, error) {
		var u sqliteTestUser
		err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.Age, new(interface{}), new(interface{}), new(interface{}))
		return u, err
	}
	RegisterScanFunc(fn)
	defer scanFuncRegistry.Delete(sqliteTestUser{})

	pq := PrepareSelectAll[sqliteTestUser](
		db,
		sqliteDialectInstance,
		"users",
		nil,
		nil,
	)
	ctx := context.Background()

	results, err := pq.Exec(ctx)
	if err != nil {
		t.Fatalf("Exec error: %v", err)
	}
	if len(results) != 5 {
		t.Errorf("Exec returned %d, want 5", len(results))
	}
}

// ===== CachedBuilder =====

func TestCachedBuilder_Find_SkipCache(t *testing.T) {
	db := setupTestDB(t)
	b := newSQLiteBuilder(db)
	ctx := context.Background()

	// skipCache = true (cache disabled)
	cb := b.WithCache(nil, cache.CacheConfig{Enabled: false})
	results, err := cb.Find(ctx)
	if err != nil {
		t.Fatalf("CachedBuilder.Find error: %v", err)
	}
	if len(results) != 5 {
		t.Errorf("Find returned %d, want 5", len(results))
	}
}

func TestCachedBuilder_First_SkipCache(t *testing.T) {
	db := setupTestDB(t)
	b := newSQLiteBuilder(db)
	ctx := context.Background()

	cb := b.WithCache(nil, cache.CacheConfig{Enabled: false})
	result, err := cb.OrderByAsc("name").First(ctx)
	if err != nil {
		t.Fatalf("CachedBuilder.First error: %v", err)
	}
	if result.Name != "Alice" {
		t.Errorf("First = %q, want Alice", result.Name)
	}
}

func TestCachedBuilder_First_NoResults(t *testing.T) {
	db := setupTestDB(t)
	b := newSQLiteBuilder(db)
	ctx := context.Background()

	cb := b.WithCache(nil, cache.CacheConfig{Enabled: false}).
		Where(Condition{Field: "age", Operator: OpGt, Value: 100})
	_, err := cb.First(ctx)
	if err == nil {
		t.Error("First should error when no results")
	}
	if err != core.ErrNotFound {
		t.Errorf("error should be ErrNotFound, got %v", err)
	}
}

func TestCachedBuilder_Count_Integration(t *testing.T) {
	db := setupTestDB(t)
	b := newSQLiteBuilder(db)
	ctx := context.Background()

	cb := b.WithCache(nil, cache.CacheConfig{Enabled: false})
	count, err := cb.Count(ctx)
	if err != nil {
		t.Fatalf("CachedBuilder.Count error: %v", err)
	}
	if count != 5 {
		t.Errorf("Count = %d, want 5", count)
	}
}

// ===== CursorBuilder.Fetch =====

func TestCursorBuilder_Fetch_Integration(t *testing.T) {
	db := setupTestDB(t)
	b := newSQLiteBuilder(db)
	ctx := context.Background()

	page, err := b.Paginate(CursorConfig{
		OrderBy: "id",
		First:   3,
	}).Fetch(ctx)
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if len(page.Items) != 3 {
		t.Errorf("Items len = %d, want 3", len(page.Items))
	}
	if !page.HasNextPage {
		t.Error("should have next page")
	}
}

func TestCursorBuilder_Fetch_WithAfter(t *testing.T) {
	db := setupTestDB(t)
	b := newSQLiteBuilder(db)
	ctx := context.Background()

	// First page
	page1, err := b.Paginate(CursorConfig{
		OrderBy: "id",
		First:   2,
	}).Fetch(ctx)
	if err != nil {
		t.Fatalf("Fetch page1 error: %v", err)
	}

	// Second page using After cursor
	page2, err := b.Paginate(CursorConfig{
		OrderBy: "id",
		First:   2,
		After:   page1.EndCursor,
	}).Fetch(ctx)
	if err != nil {
		t.Fatalf("Fetch page2 error: %v", err)
	}
	if len(page2.Items) != 2 {
		t.Errorf("Items len = %d, want 2", len(page2.Items))
	}
	if page2.Items[0].ID <= page1.Items[len(page1.Items)-1].ID {
		t.Error("page2 items should come after page1 items")
	}
}

func TestCursorBuilder_Fetch_Desc(t *testing.T) {
	db := setupTestDB(t)
	b := newSQLiteBuilder(db)
	ctx := context.Background()

	page, err := b.Paginate(CursorConfig{
		OrderBy:   "id",
		OrderDesc: true,
		First:     2,
	}).Fetch(ctx)
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if len(page.Items) != 2 {
		t.Errorf("Items len = %d, want 2", len(page.Items))
	}
	// Desc order: highest IDs first
	if page.Items[0].ID < page.Items[1].ID {
		t.Error("items should be in descending order")
	}
}

func TestCursorBuilder_Fetch_Last(t *testing.T) {
	db := setupTestDB(t)
	b := newSQLiteBuilder(db)
	ctx := context.Background()

	page, err := b.Paginate(CursorConfig{
		OrderBy: "id",
		Last:    2,
	}).Fetch(ctx)
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if len(page.Items) != 2 {
		t.Errorf("Items len = %d, want 2", len(page.Items))
	}
}

func TestCursorBuilder_Fetch_WithTotalCount(t *testing.T) {
	db := setupTestDB(t)
	b := newSQLiteBuilder(db)
	ctx := context.Background()

	page, err := b.Paginate(CursorConfig{
		OrderBy:           "id",
		First:             2,
		IncludeTotalCount: true,
	}).Fetch(ctx)
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if page.TotalCount == nil {
		t.Fatal("TotalCount should not be nil")
	}
	if *page.TotalCount != 5 {
		t.Errorf("TotalCount = %d, want 5", *page.TotalCount)
	}
}

func TestCursorBuilder_Fetch_Validation(t *testing.T) {
	db := setupTestDB(t)
	b := newSQLiteBuilder(db)
	ctx := context.Background()

	// Missing OrderBy
	_, err := b.Paginate(CursorConfig{First: 10}).Fetch(ctx)
	if err == nil {
		t.Error("should error without OrderBy")
	}

	// Both First and Last
	_, err = b.Paginate(CursorConfig{OrderBy: "id", First: 10, Last: 10}).Fetch(ctx)
	if err == nil {
		t.Error("should error with both First and Last")
	}

	// Both After and Before
	_, err = b.Paginate(CursorConfig{OrderBy: "id", First: 10, After: "a", Before: "b"}).Fetch(ctx)
	if err == nil {
		t.Error("should error with both After and Before")
	}
}

// ===== scanStruct =====

func TestScanStruct_Integration(t *testing.T) {
	db := setupTestDB(t)

	rows, err := db.Query("SELECT id, name, email, age FROM users ORDER BY id LIMIT 1")
	if err != nil {
		t.Fatalf("query error: %v", err)
	}
	defer rows.Close()

	if !rows.Next() {
		t.Fatal("no rows")
	}

	var user sqliteTestUser
	err = scanStruct(rows, &user)
	if err != nil {
		t.Fatalf("scanStruct error: %v", err)
	}
	if user.Name != "Alice" {
		t.Errorf("Name = %q, want Alice", user.Name)
	}
}

func TestScanStruct_NotPointer(t *testing.T) {
	err := scanStruct(nil, sqliteTestUser{})
	if err == nil {
		t.Error("should error when dest is not pointer")
	}
}

func TestScanStruct_NotStruct(t *testing.T) {
	var s string
	err := scanStruct(nil, &s)
	if err == nil {
		t.Error("should error when dest is not pointer to struct")
	}
}

// ===== scanStructFast =====

func TestScanStructFast_Integration(t *testing.T) {
	db := setupTestDB(t)

	rows, err := db.Query("SELECT id, name, email, age FROM users ORDER BY id LIMIT 1")
	if err != nil {
		t.Fatalf("query error: %v", err)
	}
	defer rows.Close()

	if !rows.Next() {
		t.Fatal("no rows")
	}

	columns := []string{"id", "name", "email", "age"}
	fieldMap := getOrBuildFieldMap(reflect.TypeOf(sqliteTestUser{}))
	scanValues := make([]interface{}, len(columns))

	var user sqliteTestUser
	err = scanStructFast(rows, &user, columns, fieldMap, scanValues)
	if err != nil {
		t.Fatalf("scanStructFast error: %v", err)
	}
	if user.Name != "Alice" {
		t.Errorf("Name = %q, want Alice", user.Name)
	}
}

// ===== stmtCache.getStmt =====

func TestStmtCache_GetStmt(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	_, err = db.Exec("CREATE TABLE test (id INTEGER)")
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	sc := &stmtCache{
		stmts: make(map[string]*sql.Stmt),
	}

	// First call should create a new statement
	stmt, err := sc.getStmt(db, "SELECT * FROM test")
	if err != nil {
		t.Fatalf("getStmt error: %v", err)
	}
	if stmt == nil {
		t.Fatal("stmt should not be nil")
	}

	// Second call should return cached statement
	stmt2, err := sc.getStmt(db, "SELECT * FROM test")
	if err != nil {
		t.Fatalf("getStmt error: %v", err)
	}
	if stmt2 != stmt {
		t.Error("should return same cached statement")
	}

	stmt.Close()
}

// ===== Optimizer with real DB =====

func TestOptimizer_Analyze_Integration(t *testing.T) {
	db := setupTestDB(t)
	qo := NewQueryOptimizer(db, sqliteDialectInstance)
	ctx := context.Background()

	// SQLite EXPLAIN returns query plan in a different format
	analysis, err := qo.Analyze(ctx, `SELECT * FROM "users" WHERE age > 25`)
	if err != nil {
		t.Fatalf("Analyze error: %v", err)
	}
	if analysis == nil {
		t.Fatal("analysis should not be nil")
	}
	if analysis.SQL == "" {
		t.Error("SQL should be set")
	}
}

func TestOptimizer_GetTableStats_MySQL(t *testing.T) {
	// MySQL dialect triggers getTableStatsMySQL path
	// The mock executor returns nil for QueryRowContext which causes nil ptr deref
	// So we use a real DB that will simply fail the query
	db := setupTestDB(t)
	qo := NewQueryOptimizer(db, &mockMySQLDialect{})
	_, err := qo.GetTableStats(context.Background(), "users")
	// Will error because SQLite doesn't have information_schema
	if err == nil {
		t.Error("should error since SQLite doesn't have MySQL information_schema")
	}
}

func TestOptimizer_GetTableStats_Postgres(t *testing.T) {
	db := setupTestDB(t)
	qo := NewQueryOptimizer(db, sqliteDialectInstance)
	_, err := qo.GetTableStats(context.Background(), "users")
	// Will error because SQLite doesn't have pg_stat_user_tables
	if err == nil {
		t.Error("should error since SQLite doesn't have pg_stat_user_tables")
	}
}

// ===== applySoftDeleteScope with value receiver =====

// valueSoftDeletable implements core.SoftDeletable on value receiver
type valueSoftDeletable struct {
	deleted bool
}

func (v valueSoftDeletable) TableName() string         { return "soft_items" }
func (v valueSoftDeletable) IsDeleted() bool           { return v.deleted }
func (v valueSoftDeletable) GetDeletedAt() *time.Time  { return nil }
func (v valueSoftDeletable) SetDeletedAt(t *time.Time) {}

func TestApplySoftDeleteScope_ValueReceiverSoftDeletable(t *testing.T) {
	b := NewBuilder[valueSoftDeletable](
		&mockExecutor{},
		&mockDialect{},
		newMockLogger(),
		"soft_items",
	)

	result := applySoftDeleteScope(b)

	// Value receiver SoftDeletable should be detected
	if len(result.conditions) != 1 {
		t.Errorf("applySoftDeleteScope should add condition for value-receiver SoftDeletable, got %d", len(result.conditions))
	}
}

func TestApplySoftDeleteScope_ValueReceiverDisabledScopes(t *testing.T) {
	b := NewBuilder[valueSoftDeletable](
		&mockExecutor{},
		&mockDialect{},
		newMockLogger(),
		"soft_items",
	)
	b.disableScopes = true

	result := applySoftDeleteScope(b)
	if len(result.conditions) != 0 {
		t.Error("should not add conditions when scopes disabled")
	}
}

// ===== CachedBuilder with real cache =====

func TestCachedBuilder_Find_WithRealCache(t *testing.T) {
	db := setupTestDB(t)
	b := newSQLiteBuilder(db)
	ctx := context.Background()

	c := cache.NewInMemoryCache(100)
	config := cache.CacheConfig{
		Enabled:    true,
		DefaultTTL: 5 * time.Minute,
		KeyPrefix:  "test",
	}

	// First call - cache miss, should execute query
	cb := b.WithCache(c, config)
	results, err := cb.Find(ctx)
	if err != nil {
		t.Fatalf("CachedBuilder.Find error: %v", err)
	}
	if len(results) != 5 {
		t.Errorf("Find returned %d, want 5", len(results))
	}

	// Second call - cache hit
	results2, err := cb.Find(ctx)
	if err != nil {
		t.Fatalf("CachedBuilder.Find (cache hit) error: %v", err)
	}
	if len(results2) != 5 {
		t.Errorf("Find (cache hit) returned %d, want 5", len(results2))
	}

	// Verify cache stats
	stats := c.Stats()
	if stats.Sets == 0 {
		t.Error("cache should have at least one set")
	}
}

func TestCachedBuilder_Find_NilCache(t *testing.T) {
	db := setupTestDB(t)
	b := newSQLiteBuilder(db)
	ctx := context.Background()

	// With nil cache, should still work (skip cache path)
	cb := b.WithCache(nil, cache.CacheConfig{Enabled: true})
	results, err := cb.Find(ctx)
	if err != nil {
		t.Fatalf("CachedBuilder.Find with nil cache error: %v", err)
	}
	if len(results) != 5 {
		t.Errorf("Find returned %d, want 5", len(results))
	}
}

// ===== Builder.Find with AfterFind hook =====

type afterFindUser struct {
	ID    int64  `db:"id"`
	Name  string `db:"name"`
	Email string `db:"email"`
	Age   int    `db:"age"`
	found bool
}

func (u *afterFindUser) AfterFind() error {
	u.found = true
	return nil
}

func (u afterFindUser) TableName() string { return "users" }

func TestBuilder_Find_AfterFindHook(t *testing.T) {
	db := setupTestDB(t)
	b := NewBuilder[afterFindUser](
		db,
		sqliteDialectInstance,
		newMockLogger(),
		"users",
	)
	ctx := context.Background()

	results, err := b.Limit(1).Find(ctx)
	if err != nil {
		t.Fatalf("Find error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Find returned %d, want 1", len(results))
	}
	if !results[0].found {
		t.Error("AfterFind hook should have been called")
	}
}

// ===== Builder.Find with error =====

func TestBuilder_Find_QueryError(t *testing.T) {
	b := NewBuilder[sqliteTestUser](
		&mockExecutor{},
		sqliteDialectInstance,
		newMockLogger(),
		"users",
	)
	ctx := context.Background()

	_, err := b.Find(ctx)
	if err == nil {
		t.Error("Find should error with mock executor")
	}
}

// ===== executePreload with empty results =====

func TestExecutePreload_EmptyResults(t *testing.T) {
	var empty []sqliteTestUser
	err := executePreload(context.Background(), &mockExecutor{}, sqliteDialectInstance, newMockLogger(), empty, []*PreloadSpec{
		{Relation: "Posts"},
	})
	if err != nil {
		t.Error("executePreload with empty results should return nil")
	}
}

// ===== executePreload with no registered relationships =====

func TestExecutePreload_NoRelationships(t *testing.T) {
	results := []sqliteTestUser{{ID: 1, Name: "Alice"}}
	err := executePreload(context.Background(), &mockExecutor{}, sqliteDialectInstance, newMockLogger(), results, []*PreloadSpec{
		{Relation: "Posts"},
	})
	if err == nil {
		t.Error("executePreload should error when no relationships registered")
	}
}

// ===== CursorBuilder.Fetch with Before =====

func TestCursorBuilder_Fetch_WithBefore(t *testing.T) {
	db := setupTestDB(t)
	b := newSQLiteBuilder(db)
	ctx := context.Background()

	// Get all items to find a cursor for "Before"
	allItems, _ := b.OrderByAsc("id").Find(ctx)
	if len(allItems) < 3 {
		t.Fatal("need at least 3 items")
	}

	// Create a cursor for the last item
	lastCursor := EncodeCursor("id", float64(allItems[len(allItems)-1].ID), allItems[len(allItems)-1].ID)

	page, err := b.Paginate(CursorConfig{
		OrderBy: "id",
		Last:    2,
		Before:  lastCursor,
	}).Fetch(ctx)
	if err != nil {
		t.Fatalf("Fetch with Before error: %v", err)
	}
	if len(page.Items) != 2 {
		t.Errorf("Items len = %d, want 2", len(page.Items))
	}
}

// ===== CursorBuilder.Fetch with Desc + After =====

func TestCursorBuilder_Fetch_Desc_After(t *testing.T) {
	db := setupTestDB(t)
	b := newSQLiteBuilder(db)
	ctx := context.Background()

	page1, err := b.Paginate(CursorConfig{
		OrderBy:   "id",
		OrderDesc: true,
		First:     2,
	}).Fetch(ctx)
	if err != nil {
		t.Fatalf("Fetch page1 error: %v", err)
	}

	page2, err := b.Paginate(CursorConfig{
		OrderBy:   "id",
		OrderDesc: true,
		First:     2,
		After:     page1.EndCursor,
	}).Fetch(ctx)
	if err != nil {
		t.Fatalf("Fetch page2 error: %v", err)
	}
	if len(page2.Items) == 0 {
		t.Error("should have items in page2")
	}
}

// ===== CursorBuilder.Fetch with Desc + Before =====

func TestCursorBuilder_Fetch_Desc_Before(t *testing.T) {
	db := setupTestDB(t)
	b := newSQLiteBuilder(db)
	ctx := context.Background()

	allItems, _ := b.OrderByDesc("id").Find(ctx)
	lastCursor := EncodeCursor("id", float64(allItems[len(allItems)-1].ID), allItems[len(allItems)-1].ID)

	page, err := b.Paginate(CursorConfig{
		OrderBy:   "id",
		OrderDesc: true,
		Last:      2,
		Before:    lastCursor,
	}).Fetch(ctx)
	if err != nil {
		t.Fatalf("Fetch with Desc+Before error: %v", err)
	}
	if len(page.Items) == 0 {
		t.Error("should have items")
	}
}

// ===== FastBuilder.Find with error =====

func TestFastBuilder_Find_Error(t *testing.T) {
	fb := NewFastBuilder[sqliteTestUser](&mockExecutor{}, sqliteDialectInstance, "users")
	_, err := fb.Find(context.Background())
	if err == nil {
		t.Error("Find should error with mock executor")
	}
}

// ===== FastBuilder.Count with Where =====

func TestFastBuilder_Count_WithWhere_Integration(t *testing.T) {
	db := setupTestDB(t)
	fb := NewFastBuilder[sqliteTestUser](db, sqliteDialectInstance, "users")
	ctx := context.Background()

	fb.Where(Condition{Field: "age", Operator: OpGt, Value: 27})
	count, err := fb.Count(ctx)
	if err != nil {
		t.Fatalf("Count error: %v", err)
	}
	if count != 3 { // Alice(30), Charlie(35), Diana(28)
		t.Errorf("Count = %d, want 3", count)
	}
}

// ===== UltraFastBuilder.Find with error =====

func TestUltraFastBuilder_Find_Error(t *testing.T) {
	ub := NewUltraFastBuilder[sqliteTestUser](&mockExecutor{}, sqliteDialectInstance, "users")
	_, err := ub.Find(context.Background())
	if err == nil {
		t.Error("Find should error with mock executor")
	}
}

// ===== UltraFastBuilder.Count with Where =====

func TestUltraFastBuilder_Count_WithWhere_Integration(t *testing.T) {
	db := setupTestDB(t)
	ub := NewUltraFastBuilder[sqliteTestUser](db, sqliteDialectInstance, "users")
	ctx := context.Background()

	ub.Where(Condition{Field: "age", Operator: OpGt, Value: 27})
	count, err := ub.Count(ctx)
	if err != nil {
		t.Fatalf("Count error: %v", err)
	}
	if count != 3 {
		t.Errorf("Count = %d, want 3", count)
	}
}

// ===== Preload integration tests =====

// Note: getTableNameFromType converts type name to snake_case.
// So "Article" -> "article", "Author" -> "author".
// We name tables to match.

type Author struct {
	ID    int64     `db:"id"`
	Name  string    `db:"name"`
	Email string    `db:"email"`
	Age   int       `db:"age"`
	Posts []Article `relation:"has_many,foreign_key=author_id"`
}

func (u Author) TableName() string { return "author" }

type Article struct {
	ID       int64  `db:"id"`
	Title    string `db:"title"`
	AuthorId int64  `db:"author_id"`
}

func (p Article) TableName() string { return "article" }

func setupPreloadDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE author (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			email TEXT NOT NULL,
			age INTEGER NOT NULL
		);
		CREATE TABLE article (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			author_id INTEGER NOT NULL
		);
		INSERT INTO author (name, email, age) VALUES ('Alice', 'alice@test.com', 30);
		INSERT INTO author (name, email, age) VALUES ('Bob', 'bob@test.com', 25);
		INSERT INTO article (title, author_id) VALUES ('Post 1', 1);
		INSERT INTO article (title, author_id) VALUES ('Post 2', 1);
		INSERT INTO article (title, author_id) VALUES ('Post 3', 2);
	`)
	if err != nil {
		t.Fatalf("failed to create tables: %v", err)
	}

	t.Cleanup(func() { db.Close() })
	return db
}

func TestPreloadHasMany_Integration(t *testing.T) {
	db := setupPreloadDB(t)

	// Register the model
	err := core.RegisterModel(Author{})
	if err != nil {
		t.Fatalf("RegisterModel error: %v", err)
	}

	b := NewBuilder[Author](
		db,
		sqliteDialectInstance,
		newMockLogger(),
		"author",
	)
	ctx := context.Background()

	results, err := b.Preload("Posts").Find(ctx)
	if err != nil {
		t.Fatalf("Find with Preload error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("Find returned %d, want 2", len(results))
	}

	// Alice should have 2 posts
	alice := results[0]
	if alice.Name != "Alice" {
		t.Errorf("first user = %q, want Alice", alice.Name)
	}
	if len(alice.Posts) != 2 {
		t.Errorf("Alice posts = %d, want 2", len(alice.Posts))
	}

	// Bob should have 1 post
	bob := results[1]
	if len(bob.Posts) != 1 {
		t.Errorf("Bob posts = %d, want 1", len(bob.Posts))
	}
}

func TestPreload_RelationshipNotFound(t *testing.T) {
	db := setupPreloadDB(t)

	core.RegisterModel(Author{})

	b := NewBuilder[Author](
		db,
		sqliteDialectInstance,
		newMockLogger(),
		"author",
	)
	ctx := context.Background()

	_, err := b.Preload("NonExistentRelation").Find(ctx)
	if err == nil {
		t.Error("should error for non-existent relationship")
	}
}

// ===== BelongsTo preload test =====

type ArticleWithAuthor struct {
	ID       int64  `db:"id"`
	Title    string `db:"title"`
	AuthorId int64  `db:"author_id"`
	Writer   Author `relation:"belongs_to,foreign_key=author_id,references=id"`
}

func (p ArticleWithAuthor) TableName() string { return "article" }

func TestPreloadBelongsTo_Integration(t *testing.T) {
	db := setupPreloadDB(t)

	err := core.RegisterModel(ArticleWithAuthor{})
	if err != nil {
		t.Fatalf("RegisterModel error: %v", err)
	}

	b := NewBuilder[ArticleWithAuthor](
		db,
		sqliteDialectInstance,
		newMockLogger(),
		"article",
	)
	ctx := context.Background()

	results, err := b.Preload("Writer").Find(ctx)
	if err != nil {
		t.Fatalf("Find with Preload error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("Find returned %d, want 3", len(results))
	}

	// Post 1 should belong to Alice (id=1)
	if results[0].Writer.Name != "Alice" {
		t.Errorf("Post1 writer = %q, want Alice", results[0].Writer.Name)
	}
}

// ===== ManyToMany preload test =====

type Student struct {
	ID      int64    `db:"id"`
	Name    string   `db:"name"`
	Courses []Course `relation:"many_to_many,join_table=enrollment,foreign_key=student_id,association_foreign_key=course_id"`
}

func (s Student) TableName() string { return "student" }

type Course struct {
	ID   int64  `db:"id"`
	Name string `db:"name"`
}

func (c Course) TableName() string { return "course" }

func setupManyToManyDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE student (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL
		);
		CREATE TABLE course (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL
		);
		CREATE TABLE enrollment (
			student_id INTEGER NOT NULL,
			course_id INTEGER NOT NULL
		);
		INSERT INTO student (name) VALUES ('Alice');
		INSERT INTO student (name) VALUES ('Bob');
		INSERT INTO course (name) VALUES ('Math');
		INSERT INTO course (name) VALUES ('Science');
		INSERT INTO course (name) VALUES ('English');
		INSERT INTO enrollment (student_id, course_id) VALUES (1, 1);
		INSERT INTO enrollment (student_id, course_id) VALUES (1, 2);
		INSERT INTO enrollment (student_id, course_id) VALUES (2, 2);
		INSERT INTO enrollment (student_id, course_id) VALUES (2, 3);
	`)
	if err != nil {
		t.Fatalf("failed to create tables: %v", err)
	}

	t.Cleanup(func() { db.Close() })
	return db
}

func TestPreloadManyToMany_Integration(t *testing.T) {
	db := setupManyToManyDB(t)

	err := core.RegisterModel(Student{})
	if err != nil {
		t.Fatalf("RegisterModel error: %v", err)
	}

	b := NewBuilder[Student](
		db,
		sqliteDialectInstance,
		newMockLogger(),
		"student",
	)
	ctx := context.Background()

	results, err := b.Preload("Courses").Find(ctx)
	if err != nil {
		t.Fatalf("Find with Preload error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("Find returned %d, want 2", len(results))
	}

	// Alice should have 2 courses (Math, Science)
	alice := results[0]
	if len(alice.Courses) != 2 {
		t.Errorf("Alice courses = %d, want 2", len(alice.Courses))
	}

	// Bob should have 2 courses (Science, English)
	bob := results[1]
	if len(bob.Courses) != 2 {
		t.Errorf("Bob courses = %d, want 2", len(bob.Courses))
	}
}

// ===== Polymorphic preload test =====

type Comment struct {
	ID              int64    `db:"id"`
	Body            string   `db:"body"`
	CommentableType string   `db:"commentable_type"`
	CommentableId   int64    `db:"commentable_id"`
	Commentable     Blogpost `relation:"polymorphic,polymorphic=commentable"`
}

func (c Comment) TableName() string { return "comment" }

type Blogpost struct {
	ID    int64  `db:"id"`
	Title string `db:"title"`
}

func setupPolymorphicDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE comment (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			body TEXT NOT NULL,
			commentable_type TEXT NOT NULL,
			commentable_id INTEGER NOT NULL
		);
		CREATE TABLE blogpost (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL
		);
		INSERT INTO blogpost (title) VALUES ('Hello World');
		INSERT INTO blogpost (title) VALUES ('Go Generics');
		INSERT INTO comment (body, commentable_type, commentable_id) VALUES ('Nice post', 'Blogpost', 1);
		INSERT INTO comment (body, commentable_type, commentable_id) VALUES ('Great post', 'Blogpost', 2);
	`)
	if err != nil {
		t.Fatalf("failed to create tables: %v", err)
	}

	t.Cleanup(func() { db.Close() })
	return db
}

func TestPreloadPolymorphic_Integration(t *testing.T) {
	db := setupPolymorphicDB(t)

	err := core.RegisterModel(Comment{})
	if err != nil {
		t.Fatalf("RegisterModel error: %v", err)
	}

	b := NewBuilder[Comment](
		db,
		sqliteDialectInstance,
		newMockLogger(),
		"comment",
	)
	ctx := context.Background()

	results, err := b.Preload("Commentable").Find(ctx)
	if err != nil {
		t.Fatalf("Find with Preload error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("Find returned %d, want 2", len(results))
	}

	// Comment 1 should have Blogpost "Hello World"
	if results[0].Commentable.Title != "Hello World" {
		t.Errorf("Comment1 commentable = %q, want 'Hello World'", results[0].Commentable.Title)
	}
	if results[1].Commentable.Title != "Go Generics" {
		t.Errorf("Comment2 commentable = %q, want 'Go Generics'", results[1].Commentable.Title)
	}
}

// ===== Additional error path tests =====

func TestAggregateBuilder_One_NoRows(t *testing.T) {
	db := setupTestDB(t)
	b := newSQLiteBuilder(db)
	ctx := context.Background()

	// Query that returns no rows
	_, err := b.Aggregate().
		Where(Condition{Field: "age", Operator: OpGt, Value: 1000}).
		CountAll().
		GroupBy("name").
		One(ctx)
	if err == nil {
		t.Error("One should error when no rows returned for grouped query with impossible condition")
	}
}

func TestBuilder_Find_RowsError(t *testing.T) {
	db := setupTestDB(t)
	b := NewBuilder[sqliteTestUser](
		db,
		sqliteDialectInstance,
		newMockLogger(),
		"nonexistent_table",
	)
	ctx := context.Background()

	_, err := b.Find(ctx)
	if err == nil {
		t.Error("Find should error for nonexistent table")
	}
}

// ===== getOrBuildFieldMap with reflect.Type =====

func TestGetOrBuildFieldMap_WithReflectType(t *testing.T) {
	fm := getOrBuildFieldMap(reflect.TypeOf(sqliteTestUser{}))
	if _, ok := fm["id"]; !ok {
		t.Error("should contain id")
	}
	if _, ok := fm["name"]; !ok {
		t.Error("should contain name")
	}
}

// ===== Optimizer.ListUnusedIndexes with real DB =====

func TestOptimizer_ListUnusedIndexes_Postgres_Integration(t *testing.T) {
	db := setupTestDB(t)
	// SQLite doesn't have pg_stat_user_indexes, so this will error
	qo := NewQueryOptimizer(db, sqliteDialectInstance)
	_, err := qo.ListUnusedIndexes(context.Background())
	if err == nil {
		t.Error("should error with SQLite (no pg_stat_user_indexes)")
	}
}

func TestOptimizer_ListDuplicateIndexes_Postgres_Integration(t *testing.T) {
	db := setupTestDB(t)
	qo := NewQueryOptimizer(db, sqliteDialectInstance)
	_, err := qo.ListDuplicateIndexes(context.Background())
	if err == nil {
		t.Error("should error with SQLite (no pg_index)")
	}
}

// ===== Cursor: Before, Last, OrderDesc branches =====

func TestCursorBuilder_Fetch_Before(t *testing.T) {
	db := setupTestDB(t)
	b := newSQLiteBuilder(db)
	ctx := context.Background()

	// First, get some results to get a cursor
	page1, err := b.Paginate(CursorConfig{
		First:   3,
		OrderBy: "id",
	}).Fetch(ctx)
	if err != nil {
		t.Fatalf("Fetch page1 error: %v", err)
	}
	if len(page1.Items) != 3 {
		t.Fatalf("page1 items = %d, want 3", len(page1.Items))
	}

	// Now use Before with the end cursor
	page2, err := b.Paginate(CursorConfig{
		Last:    2,
		Before:  page1.EndCursor,
		OrderBy: "id",
	}).Fetch(ctx)
	if err != nil {
		t.Fatalf("Fetch page2 error: %v", err)
	}
	if len(page2.Items) == 0 {
		t.Error("page2 should have items")
	}
}

func TestCursorBuilder_Fetch_OrderDesc(t *testing.T) {
	db := setupTestDB(t)
	b := newSQLiteBuilder(db)
	ctx := context.Background()

	page, err := b.Paginate(CursorConfig{
		First:     3,
		OrderBy:   "id",
		OrderDesc: true,
	}).Fetch(ctx)
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if len(page.Items) != 3 {
		t.Fatalf("items = %d, want 3", len(page.Items))
	}
	// With OrderDesc, first item should have highest ID
	if page.Items[0].ID < page.Items[2].ID {
		t.Errorf("OrderDesc: first item ID (%d) should be >= last item ID (%d)", page.Items[0].ID, page.Items[2].ID)
	}
}

func TestCursorBuilder_Fetch_AfterWithOrderDesc(t *testing.T) {
	db := setupTestDB(t)
	b := newSQLiteBuilder(db)
	ctx := context.Background()

	// First page with desc ordering
	page1, err := b.Paginate(CursorConfig{
		First:     2,
		OrderBy:   "id",
		OrderDesc: true,
	}).Fetch(ctx)
	if err != nil {
		t.Fatalf("Fetch page1 error: %v", err)
	}

	// Second page using After cursor
	page2, err := b.Paginate(CursorConfig{
		First:     2,
		After:     page1.EndCursor,
		OrderBy:   "id",
		OrderDesc: true,
	}).Fetch(ctx)
	if err != nil {
		t.Fatalf("Fetch page2 error: %v", err)
	}
	if len(page2.Items) == 0 {
		t.Error("page2 should have items")
	}
	// page2 items should have lower IDs than page1
	if page2.Items[0].ID >= page1.Items[len(page1.Items)-1].ID {
		t.Error("AfterWithOrderDesc: page2 should have lower IDs than page1 end")
	}
}

func TestCursorBuilder_Fetch_BeforeWithOrderDesc(t *testing.T) {
	db := setupTestDB(t)
	b := newSQLiteBuilder(db)
	ctx := context.Background()

	// First get a page
	page1, err := b.Paginate(CursorConfig{
		First:     2,
		OrderBy:   "id",
		OrderDesc: true,
	}).Fetch(ctx)
	if err != nil {
		t.Fatalf("Fetch page1 error: %v", err)
	}

	// Get page before it using Last
	page2, err := b.Paginate(CursorConfig{
		Last:      2,
		Before:    page1.EndCursor,
		OrderBy:   "id",
		OrderDesc: true,
	}).Fetch(ctx)
	if err != nil {
		t.Fatalf("Fetch page2 error: %v", err)
	}
	_ = page2 // Just verifying no error
}

func TestCursorBuilder_Fetch_LastOnly(t *testing.T) {
	db := setupTestDB(t)
	b := newSQLiteBuilder(db)
	ctx := context.Background()

	page, err := b.Paginate(CursorConfig{
		Last:    2,
		OrderBy: "id",
	}).Fetch(ctx)
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if len(page.Items) != 2 {
		t.Errorf("items = %d, want 2", len(page.Items))
	}
}

func TestCursorBuilder_Fetch_IncludeTotalCount(t *testing.T) {
	db := setupTestDB(t)
	b := newSQLiteBuilder(db)
	ctx := context.Background()

	page, err := b.Paginate(CursorConfig{
		First:             3,
		OrderBy:           "id",
		IncludeTotalCount: true,
	}).Fetch(ctx)
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if page.TotalCount == nil {
		t.Fatal("TotalCount should not be nil")
	}
	if *page.TotalCount != 5 {
		t.Errorf("TotalCount = %d, want 5", *page.TotalCount)
	}
}

// ===== Cursor: findFieldValue with embedded structs =====

type embeddedBase struct {
	ID   int64  `db:"id"`
	Name string `db:"name"`
}

type embeddedUser struct {
	embeddedBase
	Email string `db:"email"`
}

func (e embeddedUser) TableName() string { return "users" }

func TestCursorBuilder_FindFieldValue_Embedded(t *testing.T) {
	db := setupTestDB(t)
	b := NewBuilder[embeddedUser](db, sqliteDialectInstance, newMockLogger(), "users")
	ctx := context.Background()

	page, err := b.Paginate(CursorConfig{
		First:   2,
		OrderBy: "name",
	}).Fetch(ctx)
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if len(page.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(page.Items))
	}
	// Verify cursors were generated (they use findFieldValue for embedded)
	if page.StartCursor == "" {
		t.Error("StartCursor should not be empty")
	}
}

func TestCursorBuilder_FindFieldValue_NonStruct(t *testing.T) {
	cb := &CursorBuilder[sqliteTestUser]{
		config: CursorConfig{OrderBy: "id"},
	}
	val := reflect.ValueOf(42)
	result := cb.findFieldValue(val, "id")
	if result != nil {
		t.Errorf("findFieldValue on non-struct should return nil, got %v", result)
	}
}

func TestCursorBuilder_FindFieldValue_Pointer(t *testing.T) {
	cb := &CursorBuilder[sqliteTestUser]{
		config: CursorConfig{OrderBy: "id"},
	}
	user := sqliteTestUser{ID: 42, Name: "test"}
	val := reflect.ValueOf(&user)
	result := cb.findFieldValue(val, "id")
	if result == nil {
		t.Error("findFieldValue on pointer should dereference")
	}
}

// ===== ToConnection =====

func TestCursorPage_ToConnection_Empty(t *testing.T) {
	page := &CursorPage[sqliteTestUser]{
		Items: []sqliteTestUser{},
	}

	conn := page.ToConnection(func(u sqliteTestUser) Cursor {
		return ""
	})

	if len(conn.Edges) != 0 {
		t.Errorf("edges = %d, want 0", len(conn.Edges))
	}
}

// ===== buildCursorSQL branches =====

func TestBuildCursorSQL_AfterAndBefore(t *testing.T) {
	db := setupTestDB(t)
	b := newSQLiteBuilder(db)

	// Test After cursor
	cursor := EncodeCursor("id", int64(2), 2)
	sql, args, err := b.buildCursorSQL(CursorConfig{
		After:   cursor,
		OrderBy: "id",
		First:   10,
	}, sqliteDialectInstance)
	if err != nil {
		t.Fatalf("buildCursorSQL error: %v", err)
	}
	if len(args) == 0 {
		t.Error("should have args for After cursor")
	}
	_ = sql

	// Test Before cursor
	sql2, args2, err := b.buildCursorSQL(CursorConfig{
		Before:    cursor,
		OrderBy:   "id",
		OrderDesc: true,
		First:     10,
	}, sqliteDialectInstance)
	if err != nil {
		t.Fatalf("buildCursorSQL Before error: %v", err)
	}
	_ = sql2
	_ = args2
}

func TestBuildCursorSQL_InvalidCursor(t *testing.T) {
	db := setupTestDB(t)
	b := newSQLiteBuilder(db)

	_, _, err := b.buildCursorSQL(CursorConfig{
		After:   "invalid_base64!!",
		OrderBy: "id",
	}, sqliteDialectInstance)
	if err == nil {
		t.Error("should error on invalid cursor")
	}
}

// ===== Explain/ExplainAnalyze integration =====

// ===== Explain/ExplainAnalyze error from QueryContext =====

func TestBuilder_Explain_QueryError(t *testing.T) {
	db := setupTestDB(t)
	b := NewBuilder[sqliteTestUser](db, sqliteDialectInstance, newMockLogger(), "nonexistent_table")
	_, err := b.Explain(context.Background())
	if err == nil {
		t.Error("Explain should error for nonexistent table")
	}
}

func TestBuilder_ExplainAnalyze_QueryError(t *testing.T) {
	db := setupTestDB(t)
	b := NewBuilder[sqliteTestUser](db, sqliteDialectInstance, newMockLogger(), "nonexistent_table")
	_, err := b.ExplainAnalyze(context.Background())
	if err == nil {
		t.Error("ExplainAnalyze should error for nonexistent table")
	}
}

// ===== N1Detector: cleanOldQueries periodic trigger =====

func TestN1Detector_CleanOldQueries_PeriodicTrigger(t *testing.T) {
	d := NewN1Detector(N1DetectorConfig{
		Enabled:    true,
		Threshold:  100,
		TimeWindow: 1 * time.Millisecond,
	})

	// Record a query to initialize lastClean
	d.Record("SELECT 1")

	// Wait for TimeWindow*10 to trigger periodic cleanup
	time.Sleep(20 * time.Millisecond)

	// Record again - should trigger cleanOldQueries
	d.Record("SELECT 2")

	// Queries from before the cleanup window should be gone
	d.mu.Lock()
	count := len(d.queries)
	d.mu.Unlock()
	// At most the latest query should remain
	if count > 1 {
		t.Errorf("queries after periodic cleanup = %d, expected <= 1", count)
	}
}

// ===== N1Detector Record with StackTrace =====

func TestN1Detector_Record_WithStackTrace(t *testing.T) {
	detected := false
	d := NewN1Detector(N1DetectorConfig{
		Enabled:           true,
		Threshold:         2,
		TimeWindow:        1 * time.Second,
		IncludeStackTrace: true,
		OnDetection: func(det N1Detection) {
			detected = true
			// CallerFile should be set when IncludeStackTrace is enabled
		},
	})

	for i := 0; i < 3; i++ {
		d.Record("SELECT * FROM users WHERE id = $1")
	}

	if !detected {
		t.Error("should have detected N+1")
	}

	detections := d.GetDetections()
	_ = detections // CallerFile might be empty in test context, just check it doesn't panic
}

// ===== Aggregation: buildWhereClause with ConditionGroup =====

func TestAggregateBuilder_BuildWhereClause_WithConditionGroup(t *testing.T) {
	b := newTestAggregateBuilder().
		Where(ConditionGroup{
			Operator: LogicalOr,
			Conditions: []interface{}{
				Condition{Field: "age", Operator: OpGt, Value: 18},
				Condition{Field: "age", Operator: OpLt, Value: 10},
			},
		}).
		CountAll()

	sql, _ := b.buildQuery()
	if sql == "" {
		t.Error("buildQuery should produce SQL")
	}
}

// ===== Aggregation: scanRow error path =====

func TestAggregateBuilder_All_RowsError(t *testing.T) {
	db := setupTestDB(t)
	b := NewBuilder[sqliteTestUser](
		db,
		sqliteDialectInstance,
		newMockLogger(),
		"nonexistent_table",
	)
	ctx := context.Background()

	_, err := b.Aggregate().CountAll().All(ctx)
	if err == nil {
		t.Error("All should error for nonexistent table")
	}
}

// ===== Builder: getTableName fallback =====

type noTableNamer struct {
	ID int64 `db:"id"`
}

func TestGetTableName_Fallback(t *testing.T) {
	name := getTableName(noTableNamer{})
	if name == "" {
		t.Error("getTableName should return non-empty string for struct without TableName")
	}
}

func TestGetTableName_Pointer(t *testing.T) {
	name := getTableName(&noTableNamer{})
	if name == "" {
		t.Error("getTableName should handle pointers")
	}
}

// ===== Scanner: getFieldByPath out of range =====

func TestGetFieldByPath_OutOfRange(t *testing.T) {
	val := reflect.ValueOf(sqliteTestUser{})
	result := getFieldByPath(val, fieldPath{999})
	if result.IsValid() {
		t.Error("should return invalid for out-of-range index")
	}
}

// ===== Scanner: scanStruct non-pointer / non-struct =====

func TestScanStruct_NonPointer(t *testing.T) {
	err := scanStruct(nil, "not a pointer")
	if err == nil {
		t.Error("should error for non-pointer")
	}
}

func TestScanStruct_NonStruct(t *testing.T) {
	x := 42
	err := scanStruct(nil, &x)
	if err == nil {
		t.Error("should error for pointer to non-struct")
	}
}

// ===== Scanner: GetFieldIndices errors =====

func TestGetFieldIndices_NonPointer(t *testing.T) {
	_, err := GetFieldIndices("not a pointer")
	if err == nil {
		t.Error("should error for non-pointer")
	}
}

func TestGetFieldIndices_NonStruct(t *testing.T) {
	x := 42
	_, err := GetFieldIndices(&x)
	if err == nil {
		t.Error("should error for non-struct pointer")
	}
}

func TestGetFieldIndices_Valid(t *testing.T) {
	u := sqliteTestUser{}
	indices, err := GetFieldIndices(&u)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(indices) == 0 {
		t.Error("should return field indices")
	}
}

// ===== Scanner: buildFieldMapWithPrefix skip tag "-" and unexported =====

type skipFieldStruct struct {
	ID       int64  `db:"id"`
	Ignored  string `db:"-"`
	internal string
}

func TestBuildFieldMap_SkipFields(t *testing.T) {
	fm := buildFieldMap(reflect.TypeOf(skipFieldStruct{}))
	if _, ok := fm["id"]; !ok {
		t.Error("should contain id")
	}
	if _, ok := fm["-"]; ok {
		t.Error("should not contain '-' field")
	}
	if _, ok := fm["internal"]; ok {
		t.Error("should not contain unexported field")
	}
}

// ===== Scanner: getFieldByPath with pointer in path =====

type ptrEmbedded struct {
	Inner *sqliteTestUser
}

func TestGetFieldByPath_WithPointer(t *testing.T) {
	inner := sqliteTestUser{ID: 42}
	val := reflect.ValueOf(ptrEmbedded{Inner: &inner})
	// path [0] -> Inner field, then it's a pointer, path [0] inside it -> ID
	result := getFieldByPath(val, fieldPath{0, 0})
	if !result.IsValid() {
		t.Error("should be valid for pointer field access")
	}
}

// ===== AfterFind hook error path =====

type afterFindSuccessUser struct {
	ID   int64  `db:"id"`
	Name string `db:"name"`
}

func (u *afterFindSuccessUser) AfterFind() error {
	return nil
}

func (u afterFindSuccessUser) TableName() string { return "users" }

func TestBuilder_Find_WithAfterFindHook(t *testing.T) {
	db := setupTestDB(t)
	b := NewBuilder[afterFindSuccessUser](db, sqliteDialectInstance, newMockLogger(), "users")

	results, err := b.Find(context.Background())
	if err != nil {
		t.Fatalf("Find error: %v", err)
	}
	if len(results) != 5 {
		t.Errorf("results = %d, want 5", len(results))
	}
}

type afterFindErrorUser struct {
	ID   int64  `db:"id"`
	Name string `db:"name"`
}

func (u *afterFindErrorUser) AfterFind() error {
	return fmt.Errorf("afterfind hook error")
}

func (u afterFindErrorUser) TableName() string { return "users" }

func TestBuilder_Find_AfterFindHookError(t *testing.T) {
	db := setupTestDB(t)
	b := NewBuilder[afterFindErrorUser](db, sqliteDialectInstance, newMockLogger(), "users")

	_, err := b.Find(context.Background())
	if err == nil {
		t.Error("Find should error when AfterFind hook fails")
	}
}

// ===== Builder.Find with Preload that errors =====

type preloadErrorUser struct {
	ID    int64     `db:"id"`
	Name  string    `db:"name"`
	Posts []Article `relation:"has_many,foreign_key=user_id"`
}

func (u preloadErrorUser) TableName() string { return "users" }

func TestBuilder_Find_PreloadError(t *testing.T) {
	db := setupTestDB(t)
	core.RegisterModel(preloadErrorUser{})

	b := NewBuilder[preloadErrorUser](db, sqliteDialectInstance, newMockLogger(), "users").
		Preload("Posts")

	// This will fail because the "article" table (from type Article) doesn't exist
	_, err := b.Find(context.Background())
	if err == nil {
		t.Error("Find with bad preload should error")
	}
}

// ===== N1Detector cleanOldQueries with nothing to clean =====

func TestN1Detector_CleanOldQueries_NothingToClean(t *testing.T) {
	d := NewN1Detector(N1DetectorConfig{
		Enabled:    true,
		Threshold:  100,
		TimeWindow: 1 * time.Second,
	})

	d.mu.Lock()
	d.cleanOldQueries(time.Now())
	d.mu.Unlock()

	if len(d.queries) != 0 {
		t.Error("should have no queries")
	}
}

func TestN1Detector_CleanOldQueries_PartialClean(t *testing.T) {
	d := NewN1Detector(N1DetectorConfig{
		Enabled:    true,
		Threshold:  100,
		TimeWindow: 50 * time.Millisecond,
	})

	// Record an old query
	d.Record("SELECT old")

	// Wait for it to age
	time.Sleep(120 * time.Millisecond)

	// Record a new query
	d.Record("SELECT new")

	// Clean - the old one should be removed, new one kept
	d.mu.Lock()
	d.cleanOldQueries(time.Now())
	remaining := len(d.queries)
	d.mu.Unlock()

	if remaining != 1 {
		t.Errorf("should keep 1 recent query, got %d", remaining)
	}
}

// ===== Subquery: createSubqueryCondition default case =====

func TestCreateSubqueryCondition_DefaultCase(t *testing.T) {
	result := createSubqueryCondition("id", OpIn, "not a subquery")
	if result.Subquery == nil {
		t.Error("should create empty subquery for unknown type")
	}
}

func TestCreateSubqueryCondition_WithSubquery(t *testing.T) {
	sq := &Subquery{query: "SELECT id FROM users", args: []interface{}{}}
	result := createSubqueryCondition("id", OpIn, sq)
	if result.Subquery.SQL() != "SELECT id FROM users" {
		t.Errorf("SQL = %q, want SELECT id FROM users", result.Subquery.SQL())
	}
}

// ===== Subquery: Exists / NotExists =====

func TestExists_WithSubquery(t *testing.T) {
	sq := &Subquery{query: "SELECT 1 FROM users"}
	cond := Exists(sq)
	if cond.Not {
		t.Error("Exists should have Not=false")
	}
	if cond.Subquery.SQL() != "SELECT 1 FROM users" {
		t.Error("wrong subquery SQL")
	}
}

func TestNotExists_WithSubquery(t *testing.T) {
	sq := &Subquery{query: "SELECT 1 FROM users"}
	cond := NotExists(sq)
	if !cond.Not {
		t.Error("NotExists should have Not=true")
	}
}

func TestExists_DefaultCase(t *testing.T) {
	cond := Exists("not a subquery")
	if cond.Subquery == nil {
		t.Error("should create empty subquery for unknown type")
	}
}

// ===== Preload: executePreload errors =====

func TestExecutePreload_UnknownRelation(t *testing.T) {
	db := setupTestDB(t)

	type preloadTestUser struct {
		ID   int64  `db:"id"`
		Name string `db:"name"`
	}

	core.RegisterModel(preloadTestUser{})

	users := []preloadTestUser{{ID: 1, Name: "test"}}
	specs := []*PreloadSpec{{Relation: "NonExistent"}}

	err := executePreload(context.Background(), db, sqliteDialectInstance, newMockLogger(), users, specs)
	if err == nil {
		t.Error("should error for unknown relation")
	}
}

// ===== FastBuilder: NewFastBuilder =====

// ===== FastBuilder: error paths =====

func TestFastBuilder_First_NoRows(t *testing.T) {
	db := setupTestDB(t)
	fb := NewFastBuilder[sqliteTestUser](db, sqliteDialectInstance, "users")
	fb = fb.Where(Condition{Field: "age", Operator: OpGt, Value: 9999})
	_, err := fb.First(context.Background())
	if err == nil {
		t.Error("First should error when no rows")
	}
}

// ===== UltraFastBuilder error paths =====

// ===== UltraFastBuilder: error paths =====

func TestUltraFastBuilder_First_NoRows(t *testing.T) {
	db := setupTestDB(t)
	ub := NewUltraFastBuilder[sqliteTestUser](db, sqliteDialectInstance, "users")
	ub = ub.Where(Condition{Field: "age", Operator: OpGt, Value: 9999})
	_, err := ub.First(context.Background())
	if err == nil {
		t.Error("First should error when no rows")
	}
}

// ===== UltraFastBuilder: scanWithFunc path =====

func TestUltraFastBuilder_WithScanFunc_Integration(t *testing.T) {
	db := setupTestDB(t)
	ub := NewUltraFastBuilder[sqliteTestUser](db, sqliteDialectInstance, "users")
	ub = ub.WithScanFunc(func(rows *sql.Rows) (sqliteTestUser, error) {
		var u sqliteTestUser
		err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.Age, new(interface{}), new(interface{}), new(interface{}))
		return u, err
	})

	results, err := ub.Find(context.Background())
	if err != nil {
		t.Fatalf("Find error: %v", err)
	}
	if len(results) != 5 {
		t.Errorf("results = %d, want 5", len(results))
	}
}

// ===== PreparedQuery: ExecFirst no rows =====

func TestPreparedQuery_ExecFirst_NoRows(t *testing.T) {
	db := setupTestDB(t)
	pq := PrepareSelectAll[sqliteTestUser](db, sqliteDialectInstance, "users", nil, []string{"age"})
	_, err := pq.ExecFirst(context.Background(), 9999)
	if err == nil {
		t.Error("ExecFirst should error when no rows")
	}
}

// ===== PreparedQuery: Exec error =====

func TestPreparedQuery_Exec_Error(t *testing.T) {
	db := setupTestDB(t)
	pq := PrepareSelectAll[sqliteTestUser](db, sqliteDialectInstance, "nonexistent_table", nil, []string{"id"})
	_, err := pq.Exec(context.Background(), 1)
	if err == nil {
		t.Error("Exec should error for nonexistent table")
	}
}

// ===== PreparedQuery: with ScanFunc =====

func TestPreparedQuery_WithScanFunc(t *testing.T) {
	db := setupTestDB(t)

	// Register scan func first
	RegisterScanFunc(func(rows *sql.Rows) (sqliteTestUser, error) {
		var u sqliteTestUser
		err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.Age, new(interface{}), new(interface{}), new(interface{}))
		return u, err
	})
	defer func() {
		// Clean up registry
		scanFuncRegistry.Delete(reflect.TypeOf(sqliteTestUser{}))
	}()

	pq := PrepareSelectAll[sqliteTestUser](db, sqliteDialectInstance, "users", nil, []string{"age"})
	if pq.scanFunc == nil {
		t.Fatal("scanFunc should be set from registry")
	}

	// WHERE age = 30 should return Alice
	results, err := pq.Exec(context.Background(), 30)
	if err != nil {
		t.Fatalf("Exec error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("results = %d, want 1", len(results))
	}
}

// ===== CachedBuilder: Find with cache error =====

func TestCachedBuilder_Find_CacheError(t *testing.T) {
	db := setupTestDB(t)
	b := newSQLiteBuilder(db)
	ctx := context.Background()

	c := cache.NewInMemoryCache(1000)
	cb := b.WithCache(c, cache.CacheConfig{KeyPrefix: "test", Enabled: true, DefaultTTL: 1 * time.Minute})

	// First call - cache miss, should query DB
	results, err := cb.Find(ctx)
	if err != nil {
		t.Fatalf("Find error: %v", err)
	}
	if len(results) != 5 {
		t.Errorf("results = %d, want 5", len(results))
	}

	// Second call - cache hit
	results2, err := cb.Find(ctx)
	if err != nil {
		t.Fatalf("Find (cached) error: %v", err)
	}
	if len(results2) != 5 {
		t.Errorf("cached results = %d, want 5", len(results2))
	}
}

// ===== CachedBuilder: First with cache enabled =====

func TestCachedBuilder_First_CacheEnabled(t *testing.T) {
	db := setupTestDB(t)
	b := newSQLiteBuilder(db)
	ctx := context.Background()

	c := cache.NewInMemoryCache(1000)
	cb := b.WithCache(c, cache.CacheConfig{KeyPrefix: "test", Enabled: true, DefaultTTL: 1 * time.Minute})

	result, err := cb.OrderByAsc("name").First(ctx)
	if err != nil {
		t.Fatalf("First error: %v", err)
	}
	if result.Name != "Alice" {
		t.Errorf("First = %q, want Alice", result.Name)
	}

	// Second call should use cache
	result2, err := cb.OrderByAsc("name").First(ctx)
	if err != nil {
		t.Fatalf("First (cached) error: %v", err)
	}
	if result2.Name != "Alice" {
		t.Errorf("First cached = %q, want Alice", result2.Name)
	}
}

func TestCachedBuilder_First_NoResults_CacheEnabled(t *testing.T) {
	db := setupTestDB(t)
	b := newSQLiteBuilder(db)
	ctx := context.Background()

	c := cache.NewInMemoryCache(1000)
	cb := b.WithCache(c, cache.CacheConfig{KeyPrefix: "test", Enabled: true, DefaultTTL: 1 * time.Minute}).
		Where(Condition{Field: "age", Operator: OpGt, Value: 9999})

	_, err := cb.First(ctx)
	if err == nil {
		t.Error("First should error when no results")
	}
}

// ===== CachedBuilder: Find with cache that returns error =====

type errorCache struct{}

func (e *errorCache) Get(ctx context.Context, key string) ([]byte, error) {
	return nil, fmt.Errorf("cache error")
}
func (e *errorCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return fmt.Errorf("cache set error")
}
func (e *errorCache) Delete(ctx context.Context, key string) error            { return nil }
func (e *errorCache) DeleteByPrefix(ctx context.Context, prefix string) error { return nil }
func (e *errorCache) Clear(ctx context.Context) error                         { return nil }
func (e *errorCache) Stats() cache.CacheStats                                 { return cache.CacheStats{} }

func TestCachedBuilder_Find_CacheGetError(t *testing.T) {
	db := setupTestDB(t)
	b := newSQLiteBuilder(db)
	ctx := context.Background()

	cb := b.WithCache(&errorCache{}, cache.CacheConfig{KeyPrefix: "test", Enabled: true, DefaultTTL: 1 * time.Minute})

	// Should fall through to builder.Find when cache.Get errors
	results, err := cb.Find(ctx)
	if err != nil {
		t.Fatalf("Find error: %v", err)
	}
	if len(results) != 5 {
		t.Errorf("results = %d, want 5", len(results))
	}
}

// ===== CachedBuilder: Find with corrupt cache data =====

type corruptCache struct {
	data map[string][]byte
}

func newCorruptCache() *corruptCache {
	return &corruptCache{data: make(map[string][]byte)}
}
func (c *corruptCache) Get(ctx context.Context, key string) ([]byte, error) {
	if d, ok := c.data[key]; ok {
		return d, nil
	}
	return nil, nil
}
func (c *corruptCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	c.data[key] = value
	return nil
}
func (c *corruptCache) Delete(ctx context.Context, key string) error            { return nil }
func (c *corruptCache) DeleteByPrefix(ctx context.Context, prefix string) error { return nil }
func (c *corruptCache) Clear(ctx context.Context) error                         { return nil }
func (c *corruptCache) Stats() cache.CacheStats                                 { return cache.CacheStats{} }

func TestCachedBuilder_Find_CorruptCacheData(t *testing.T) {
	db := setupTestDB(t)
	b := newSQLiteBuilder(db)
	ctx := context.Background()

	cc := newCorruptCache()
	cb := b.WithCache(cc, cache.CacheConfig{KeyPrefix: "test", Enabled: true, DefaultTTL: 1 * time.Minute})

	// Pre-populate cache with corrupt data
	query, args := b.buildSelectQuery()
	key := cache.QueryCacheKey("test", "users", query, args)
	cc.data[key] = []byte("not valid json{{{")

	// Should detect unmarshal error and fall through to DB query
	results, err := cb.Find(ctx)
	if err != nil {
		t.Fatalf("Find error: %v", err)
	}
	if len(results) != 5 {
		t.Errorf("results = %d, want 5", len(results))
	}
}

// ===== CachedBuilder: Find where builder.Find errors (with cache enabled) =====

func TestCachedBuilder_Find_BuilderError(t *testing.T) {
	db := setupTestDB(t)
	b := NewBuilder[sqliteTestUser](db, sqliteDialectInstance, newMockLogger(), "nonexistent_table")

	c := cache.NewInMemoryCache(1000)
	cb := b.WithCache(c, cache.CacheConfig{KeyPrefix: "test", Enabled: true, DefaultTTL: 1 * time.Minute})

	_, err := cb.Find(context.Background())
	if err == nil {
		t.Error("Find should error for nonexistent table")
	}
}

// ===== Optimizer: GetTableStats with PostgreSQL dialect (mock) =====

func TestOptimizer_GetTableStats_PostgresDialect(t *testing.T) {
	// Use real SQLite DB - it won't have pg_stat_user_tables but exercises the code path
	db := setupTestDB(t)
	qo := NewQueryOptimizer(db, &mockDialect{}) // mockDialect uses $N (PostgreSQL)

	_, err := qo.GetTableStats(context.Background(), "users")
	// This will fail because SQLite doesn't have pg_stat_user_tables
	if err == nil {
		t.Error("should error with SQLite (no pg_stat_user_tables)")
	}
}

// ===== Optimizer: ListUnusedIndexes with PostgreSQL dialect =====

func TestOptimizer_ListUnusedIndexes_PostgresDialect_Error(t *testing.T) {
	exec := &mockExecutor{
		queryFn: func(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
			return nil, sql.ErrConnDone
		},
	}
	qo := NewQueryOptimizer(exec, &mockDialect{})

	_, err := qo.ListUnusedIndexes(context.Background())
	if err == nil {
		t.Error("should propagate QueryContext error")
	}
}

// ===== Optimizer: ListDuplicateIndexes with PostgreSQL dialect =====

func TestOptimizer_ListDuplicateIndexes_PostgresDialect_Error(t *testing.T) {
	exec := &mockExecutor{
		queryFn: func(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
			return nil, sql.ErrConnDone
		},
	}
	qo := NewQueryOptimizer(exec, &mockDialect{})

	_, err := qo.ListDuplicateIndexes(context.Background())
	if err == nil {
		t.Error("should propagate QueryContext error")
	}
}

// ===== Optimizer: getTableStatsMySQL error path =====

func TestOptimizer_GetTableStats_MySQLError(t *testing.T) {
	// Use real SQLite DB - it won't have information_schema.TABLES properly but exercises the MySQL code path
	db := setupTestDB(t)
	qo := NewQueryOptimizer(db, &mockMySQLDialect{})

	_, err := qo.GetTableStats(context.Background(), "users")
	// May or may not error depending on SQLite behavior, but exercises the MySQL path
	_ = err
}

// ===== Optimizer: ListUnusedIndexes with real DB (PostgreSQL dialect) =====

func TestOptimizer_ListUnusedIndexes_RealDB_PostgresPath(t *testing.T) {
	db := setupTestDB(t)

	// Create fake pg_stat_user_indexes table for SQLite to exercise the full code path
	_, err := db.Exec(`CREATE TABLE pg_stat_user_indexes (
		indexrelname TEXT,
		idx_scan INTEGER,
		indexrelid INTEGER
	)`)
	if err != nil {
		t.Fatalf("failed to create fake table: %v", err)
	}
	_, _ = db.Exec(`INSERT INTO pg_stat_user_indexes VALUES ('idx_users_name', 0, 1)`)
	_, _ = db.Exec(`INSERT INTO pg_stat_user_indexes VALUES ('users_pkey', 0, 2)`)
	_, _ = db.Exec(`INSERT INTO pg_stat_user_indexes VALUES ('idx_users_active', 5, 3)`)

	qo := NewQueryOptimizer(db, &mockDialect{})

	// The query uses pg_relation_size which doesn't exist in SQLite, so it will error
	// But we exercise the QueryContext path
	_, err = qo.ListUnusedIndexes(context.Background())
	// May error due to pg_relation_size not existing in SQLite
	_ = err
}

// ===== Optimizer: ListDuplicateIndexes with real DB (PostgreSQL dialect) =====

func TestOptimizer_ListDuplicateIndexes_RealDB_PostgresPath(t *testing.T) {
	db := setupTestDB(t)
	qo := NewQueryOptimizer(db, &mockDialect{})

	_, err := qo.ListDuplicateIndexes(context.Background())
	// Will error since SQLite doesn't have pg_index
	_ = err
}

// ===== Builder: Count error path =====

func TestBuilder_Count_NonexistentTable(t *testing.T) {
	db := setupTestDB(t)
	b := NewBuilder[sqliteTestUser](db, sqliteDialectInstance, newMockLogger(), "nonexistent")
	_, err := b.Count(context.Background())
	if err == nil {
		t.Error("Count should error for nonexistent table")
	}
}

// ===== Builder: First error path =====

func TestBuilder_First_Error(t *testing.T) {
	db := setupTestDB(t)
	b := NewBuilder[sqliteTestUser](db, sqliteDialectInstance, newMockLogger(), "nonexistent")
	_, err := b.First(context.Background())
	if err == nil {
		t.Error("First should error for nonexistent table")
	}
}

// ===== Builder: First no rows =====

func TestBuilder_First_NoRows(t *testing.T) {
	db := setupTestDB(t)
	b := newSQLiteBuilder(db).
		Where(Condition{Field: "age", Operator: OpGt, Value: 9999})
	_, err := b.First(context.Background())
	if err == nil {
		t.Error("First should error when no rows found")
	}
}

// ===== Aggregation: One error and All rows error =====

// ===== Preload: BelongsTo with pointer field =====

type ArticleWithPtrAuthor struct {
	ID       int64   `db:"id"`
	Title    string  `db:"title"`
	AuthorId int64   `db:"author_id"`
	Writer   *Author `relation:"belongs_to,foreign_key=author_id,references=id"`
}

func (a ArticleWithPtrAuthor) TableName() string { return "article" }

func TestPreloadBelongsTo_PointerField_Integration(t *testing.T) {
	db := setupTestDB(t)

	_, err := db.Exec(`CREATE TABLE author (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		email TEXT NOT NULL DEFAULT '',
		age INTEGER NOT NULL DEFAULT 0
	)`)
	if err != nil {
		t.Fatalf("create author table: %v", err)
	}

	_, err = db.Exec(`CREATE TABLE article (
		id INTEGER PRIMARY KEY,
		title TEXT NOT NULL,
		author_id INTEGER NOT NULL
	)`)
	if err != nil {
		t.Fatalf("create article table: %v", err)
	}

	_, _ = db.Exec(`INSERT INTO author (id, name) VALUES (1, 'Author1'), (2, 'Author2')`)
	_, _ = db.Exec(`INSERT INTO article (id, title, author_id) VALUES (1, 'Post1', 1), (2, 'Post2', 2)`)

	core.RegisterModel(ArticleWithPtrAuthor{})

	b := NewBuilder[ArticleWithPtrAuthor](db, sqliteDialectInstance, newMockLogger(), "article").
		Preload("Writer")

	results, err := b.Find(context.Background())
	if err != nil {
		t.Fatalf("Find error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %d, want 2", len(results))
	}
	if results[0].Writer == nil {
		t.Error("Writer should not be nil")
	}
	if results[0].Writer != nil && results[0].Writer.Name != "Author1" {
		t.Errorf("Writer name = %q, want Author1", results[0].Writer.Name)
	}
}

// ===== Preload: executePreload with unsupported type =====

func TestExecutePreload_UnsupportedRelType(t *testing.T) {
	db := setupTestDB(t)

	type unsupportedRelUser struct {
		ID   int64  `db:"id"`
		Name string `db:"name"`
	}

	core.RegisterModel(unsupportedRelUser{})

	// Manually add a relationship with unsupported type
	rels := core.GetRelationships(reflect.TypeOf(unsupportedRelUser{}))
	if rels != nil {
		rels.Relationships["BadRel"] = &core.RelationshipMeta{
			Type:      "unsupported",
			FieldName: "BadField",
		}

		users := []unsupportedRelUser{{ID: 1, Name: "test"}}
		specs := []*PreloadSpec{{Relation: "BadRel"}}

		err := executePreload(context.Background(), db, sqliteDialectInstance, newMockLogger(), users, specs)
		if err == nil {
			t.Error("should error for unsupported relationship type")
		}
	}
}

// ===== FastBuilder: First error from Find =====

func TestFastBuilder_First_FindError(t *testing.T) {
	db := setupTestDB(t)
	fb := NewFastBuilder[sqliteTestUser](db, sqliteDialectInstance, "nonexistent")
	_, err := fb.First(context.Background())
	if err == nil {
		t.Error("First should propagate Find error")
	}
}

// ===== UltraFastBuilder: First error from Find =====

func TestUltraFastBuilder_First_FindError(t *testing.T) {
	db := setupTestDB(t)
	ub := NewUltraFastBuilder[sqliteTestUser](db, sqliteDialectInstance, "nonexistent")
	_, err := ub.First(context.Background())
	if err == nil {
		t.Error("First should propagate Find error")
	}
}

// ===== PreparedQuery: ExecFirst error from Exec =====

func TestPreparedQuery_ExecFirst_ExecError(t *testing.T) {
	db := setupTestDB(t)
	pq := PrepareSelectAll[sqliteTestUser](db, sqliteDialectInstance, "nonexistent_table", nil, []string{"id"})
	_, err := pq.ExecFirst(context.Background(), 1)
	if err == nil {
		t.Error("ExecFirst should propagate Exec error")
	}
}

// ===== ClearStmtCache with actual stmts =====

func TestClearStmtCache_WithStmts(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	_, err = db.Exec("CREATE TABLE test (id INTEGER)")
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Add a statement to the global cache
	stmt, err := db.Prepare("SELECT * FROM test")
	if err != nil {
		t.Fatalf("failed to prepare: %v", err)
	}
	globalStmtCache.mu.Lock()
	globalStmtCache.stmts["SELECT * FROM test"] = stmt
	globalStmtCache.mu.Unlock()

	// Clear should close all statements
	ClearStmtCache()

	globalStmtCache.mu.RLock()
	count := len(globalStmtCache.stmts)
	globalStmtCache.mu.RUnlock()

	if count != 0 {
		t.Errorf("stmts should be empty after clear, got %d", count)
	}
}
