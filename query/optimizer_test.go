package query

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
)

func TestNewQueryOptimizer(t *testing.T) {
	qo := NewQueryOptimizer(&mockExecutor{}, &mockDialect{})
	if qo == nil {
		t.Fatal("NewQueryOptimizer returned nil")
	}
}

func TestIsNumeric(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"123", true},
		{"0", true},
		{"abc", false},
		{"12a", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if isNumeric(tt.input) != tt.expected {
				t.Errorf("isNumeric(%q) = %v, want %v", tt.input, !tt.expected, tt.expected)
			}
		})
	}
}

func TestIsKeyword(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"and", true},
		{"or", true},
		{"not", true},
		{"null", true},
		{"true", true},
		{"false", true},
		{"is", true},
		{"AND", true}, // case insensitive
		{"name", false},
		{"users", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if isKeyword(tt.input) != tt.expected {
				t.Errorf("isKeyword(%q) = %v, want %v", tt.input, !tt.expected, tt.expected)
			}
		})
	}
}

func TestAnalyzePlan(t *testing.T) {
	qo := NewQueryOptimizer(&mockExecutor{}, &mockDialect{})
	analysis := &QueryAnalysis{
		ExecutionPlan: "Seq Scan on users  (cost=0.00..1234.56 rows=100 width=128)",
	}

	qo.analyzePlan(analysis)

	if len(analysis.SeqScans) == 0 {
		t.Error("should detect seq scan on users")
	}
	if analysis.SeqScans[0] != "users" {
		t.Errorf("SeqScans[0] = %q, want 'users'", analysis.SeqScans[0])
	}
	if analysis.EstimatedCost == 0 {
		t.Error("should extract estimated cost")
	}
	if analysis.EstimatedRows == 0 {
		t.Error("should extract estimated rows")
	}
	if len(analysis.Warnings) == 0 {
		t.Error("should have warnings for seq scan")
	}
}

func TestAnalyzePlan_IndexScan(t *testing.T) {
	qo := NewQueryOptimizer(&mockExecutor{}, &mockDialect{})
	analysis := &QueryAnalysis{
		ExecutionPlan: "Index Scan using idx_users_email on users  (cost=0.00..10.00 rows=1 width=64)",
	}

	qo.analyzePlan(analysis)

	if len(analysis.IndexScans) == 0 {
		t.Error("should detect index scan")
	}
}

func TestSuggestIndexes(t *testing.T) {
	qo := NewQueryOptimizer(&mockExecutor{}, &mockDialect{})
	analysis := &QueryAnalysis{
		SeqScans: []string{"users"},
	}

	qo.suggestIndexes(analysis, "SELECT * FROM users WHERE name = 'John' AND age > 18")

	if len(analysis.MissingIndexes) == 0 {
		t.Error("should suggest indexes for seq scan with WHERE")
	}
}

func TestSuggestIndexes_OrderBy(t *testing.T) {
	qo := NewQueryOptimizer(&mockExecutor{}, &mockDialect{})
	analysis := &QueryAnalysis{
		SeqScans: []string{"users"},
	}

	qo.suggestIndexes(analysis, "SELECT * FROM users ORDER BY created_at")

	found := false
	for _, idx := range analysis.MissingIndexes {
		if idx.Columns[0] == "created_at" {
			found = true
			break
		}
	}
	if !found {
		t.Error("should suggest index for ORDER BY column")
	}
}

func TestSuggestIndexes_NoTable(t *testing.T) {
	qo := NewQueryOptimizer(&mockExecutor{}, &mockDialect{})
	analysis := &QueryAnalysis{}

	qo.suggestIndexes(analysis, "INVALID SQL")

	if len(analysis.MissingIndexes) != 0 {
		t.Error("should not suggest indexes for invalid SQL")
	}
}

func TestAddRecommendations(t *testing.T) {
	qo := NewQueryOptimizer(&mockExecutor{}, &mockDialect{})

	tests := []struct {
		name    string
		sql     string
		wantRec string
	}{
		{"select star", "SELECT * FROM users", "SELECT *"},
		{"leading wildcard", "SELECT name FROM users WHERE name LIKE '%john'", "wildcard"},
		{"multiple OR", "SELECT * FROM t WHERE a = 1 OR b = 2 OR c = 3 OR d = 4", "OR"},
		{"subquery", "SELECT * FROM users WHERE id IN (SELECT user_id FROM posts)", "Subquery"},
		{"not in", "SELECT * FROM users WHERE id NOT IN (1,2,3)", "NOT IN"},
		{"distinct", "SELECT DISTINCT name FROM users", "DISTINCT"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis := &QueryAnalysis{}
			qo.addRecommendations(analysis, tt.sql)
			found := false
			for _, rec := range analysis.Recommendations {
				if strings.Contains(rec, tt.wantRec) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("should have recommendation containing %q for SQL: %q, got %v", tt.wantRec, tt.sql, analysis.Recommendations)
			}
		})
	}
}

func TestOptimizer_AutoIndex_DryRun(t *testing.T) {
	qo := NewQueryOptimizer(&mockExecutor{}, &mockDialect{})
	analysis := &QueryAnalysis{
		MissingIndexes: []IndexSuggestion{
			{
				Table:     "users",
				Columns:   []string{"name"},
				CreateSQL: `CREATE INDEX "idx_users_name" ON "users" ("name")`,
			},
		},
	}

	executed, err := qo.AutoIndex(context.Background(), analysis, true)
	if err != nil {
		t.Fatalf("AutoIndex dry run error: %v", err)
	}
	if len(executed) != 1 {
		t.Errorf("executed len = %d, want 1", len(executed))
	}
	if !strings.Contains(executed[0], "CREATE INDEX") {
		t.Errorf("should contain CREATE INDEX, got %q", executed[0])
	}
}

func TestOptimizer_AutoIndex_Execute(t *testing.T) {
	executed := false
	exec := &mockExecutor{
		execFn: func(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
			executed = true
			return nil, nil
		},
	}
	qo := NewQueryOptimizer(exec, &mockDialect{})
	analysis := &QueryAnalysis{
		MissingIndexes: []IndexSuggestion{
			{CreateSQL: `CREATE INDEX idx_test ON test (col)`},
		},
	}

	_, err := qo.AutoIndex(context.Background(), analysis, false)
	if err != nil {
		t.Fatalf("AutoIndex execute error: %v", err)
	}
	if !executed {
		t.Error("should have called ExecContext")
	}
}

func TestOptimizer_AutoIndex_AlreadyExists(t *testing.T) {
	exec := &mockExecutor{
		execFn: func(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
			return nil, fmt.Errorf("index already exists")
		},
	}
	qo := NewQueryOptimizer(exec, &mockDialect{})
	analysis := &QueryAnalysis{
		MissingIndexes: []IndexSuggestion{
			{CreateSQL: `CREATE INDEX idx_test ON test (col)`},
		},
	}

	executed, err := qo.AutoIndex(context.Background(), analysis, false)
	if err != nil {
		t.Fatalf("AutoIndex should not error on 'already exists': %v", err)
	}
	if len(executed) != 1 {
		t.Error("should still add to executed list")
	}
}

func TestOptimizer_AutoIndex_Error(t *testing.T) {
	exec := &mockExecutor{
		execFn: func(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
			return nil, fmt.Errorf("permission denied")
		},
	}
	qo := NewQueryOptimizer(exec, &mockDialect{})
	analysis := &QueryAnalysis{
		MissingIndexes: []IndexSuggestion{
			{CreateSQL: `CREATE INDEX idx_test ON test (col)`},
		},
	}

	_, err := qo.AutoIndex(context.Background(), analysis, false)
	if err == nil {
		t.Error("should return error for non-'already exists' errors")
	}
}

func TestOptimizer_ListUnusedIndexes_MySQL(t *testing.T) {
	qo := NewQueryOptimizer(&mockExecutor{}, &mockMySQLDialect{})
	_, err := qo.ListUnusedIndexes(context.Background())
	if err == nil {
		t.Error("should error for MySQL dialect")
	}
}

func TestOptimizer_ListDuplicateIndexes_MySQL(t *testing.T) {
	qo := NewQueryOptimizer(&mockExecutor{}, &mockMySQLDialect{})
	_, err := qo.ListDuplicateIndexes(context.Background())
	if err == nil {
		t.Error("should error for MySQL dialect")
	}
}

func TestBuilder_Optimize(t *testing.T) {
	// Optimize calls Analyze which needs a real executor to run EXPLAIN
	// We test it returns an error with mock executor
	b := newTestBuilder().Where(Condition{Field: "age", Operator: OpGt, Value: 18})
	_, err := b.Optimize(context.Background())
	if err == nil {
		t.Error("should error with mock executor")
	}
}

func TestAddRecommendations_HighCost(t *testing.T) {
	qo := NewQueryOptimizer(&mockExecutor{}, &mockDialect{})
	analysis := &QueryAnalysis{EstimatedCost: 20000}
	qo.addRecommendations(analysis, "SELECT id FROM users")
	found := false
	for _, rec := range analysis.Recommendations {
		if strings.Contains(rec, "High estimated cost") {
			found = true
			break
		}
	}
	if !found {
		t.Error("should warn about high cost")
	}
}
