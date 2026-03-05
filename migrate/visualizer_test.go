package migrate

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ========================================
// Tests for NewMigrationVisualizer
// ========================================

func TestNewMigrationVisualizer(t *testing.T) {
	v := NewMigrationVisualizer("/tmp/test", nil)
	if v == nil {
		t.Fatal("expected non-nil visualizer")
	}
	if v.migrationsPath != "/tmp/test" {
		t.Errorf("expected path '/tmp/test', got '%s'", v.migrationsPath)
	}
	if v.appliedSet == nil {
		t.Error("expected appliedSet to be initialized")
	}
}

// ========================================
// Tests for BuildDAG
// ========================================

func TestBuildDAG(t *testing.T) {
	t.Run("builds DAG from migration files", func(t *testing.T) {
		// Create temp dir with migration files
		dir := t.TempDir()
		createMigrationFile(t, dir, "001_create_users.go", "package migrations\n")
		createMigrationFile(t, dir, "002_create_posts.go", "package migrations\n")
		createMigrationFile(t, dir, "003_add_indexes.go", "package migrations\n")

		v := NewMigrationVisualizer(dir, nil)
		ctx := context.Background()

		dag, err := v.BuildDAG(ctx)
		if err != nil {
			t.Fatalf("BuildDAG failed: %v", err)
		}

		if len(dag.Nodes) != 3 {
			t.Fatalf("expected 3 nodes, got %d", len(dag.Nodes))
		}

		// Without explicit dependencies, linear edges are created
		if len(dag.Edges) != 2 {
			t.Fatalf("expected 2 edges (linear), got %d", len(dag.Edges))
		}

		// Verify nodes are sorted
		if dag.Nodes[0].Version != 1 || dag.Nodes[1].Version != 2 || dag.Nodes[2].Version != 3 {
			t.Errorf("nodes not sorted: %v", dag.Nodes)
		}
	})

	t.Run("builds DAG with dependencies", func(t *testing.T) {
		dir := t.TempDir()
		createMigrationFile(t, dir, "001_create_users.go", "package migrations\n")
		createMigrationFile(t, dir, "002_create_posts.go",
			"package migrations\n\nvar _ = Migration{\n\tDependsOn: []int64{1},\n}\n")

		v := NewMigrationVisualizer(dir, nil)
		ctx := context.Background()

		dag, err := v.BuildDAG(ctx)
		if err != nil {
			t.Fatalf("BuildDAG failed: %v", err)
		}

		if len(dag.Nodes) != 2 {
			t.Fatalf("expected 2 nodes, got %d", len(dag.Nodes))
		}

		// Should have explicit dependency edge
		if len(dag.Edges) != 1 {
			t.Fatalf("expected 1 edge, got %d", len(dag.Edges))
		}
		if dag.Edges[0].From != 1 || dag.Edges[0].To != 2 {
			t.Errorf("expected edge 1->2, got %d->%d", dag.Edges[0].From, dag.Edges[0].To)
		}
	})

	t.Run("empty directory", func(t *testing.T) {
		dir := t.TempDir()

		v := NewMigrationVisualizer(dir, nil)
		ctx := context.Background()

		dag, err := v.BuildDAG(ctx)
		if err != nil {
			t.Fatalf("BuildDAG failed: %v", err)
		}

		if len(dag.Nodes) != 0 {
			t.Errorf("expected 0 nodes, got %d", len(dag.Nodes))
		}
	})

	t.Run("non-existing directory returns error", func(t *testing.T) {
		v := NewMigrationVisualizer("/nonexistent/path", nil)
		ctx := context.Background()

		_, err := v.BuildDAG(ctx)
		if err == nil {
			t.Fatal("expected error for non-existing directory")
		}
	})

	t.Run("skips non-go files and directories", func(t *testing.T) {
		dir := t.TempDir()
		createMigrationFile(t, dir, "001_create_users.go", "package migrations\n")
		createMigrationFile(t, dir, "README.md", "# Migrations\n")
		os.Mkdir(filepath.Join(dir, "subdir"), 0o755)

		v := NewMigrationVisualizer(dir, nil)
		ctx := context.Background()

		dag, err := v.BuildDAG(ctx)
		if err != nil {
			t.Fatalf("BuildDAG failed: %v", err)
		}

		if len(dag.Nodes) != 1 {
			t.Errorf("expected 1 node, got %d", len(dag.Nodes))
		}
	})

	t.Run("skips files that don't match pattern", func(t *testing.T) {
		dir := t.TempDir()
		createMigrationFile(t, dir, "001_create_users.go", "package migrations\n")
		createMigrationFile(t, dir, "helpers.go", "package migrations\n")
		createMigrationFile(t, dir, "no_version.go", "package migrations\n")

		v := NewMigrationVisualizer(dir, nil)
		ctx := context.Background()

		dag, err := v.BuildDAG(ctx)
		if err != nil {
			t.Fatalf("BuildDAG failed: %v", err)
		}

		if len(dag.Nodes) != 1 {
			t.Errorf("expected 1 node, got %d", len(dag.Nodes))
		}
	})

	t.Run("status defaults to pending", func(t *testing.T) {
		dir := t.TempDir()
		createMigrationFile(t, dir, "001_create_users.go", "package migrations\n")

		v := NewMigrationVisualizer(dir, nil)
		ctx := context.Background()

		dag, err := v.BuildDAG(ctx)
		if err != nil {
			t.Fatalf("BuildDAG failed: %v", err)
		}

		if dag.Nodes[0].Status != "pending" {
			t.Errorf("expected 'pending', got '%s'", dag.Nodes[0].Status)
		}
	})

	t.Run("marks applied migrations", func(t *testing.T) {
		dir := t.TempDir()
		createMigrationFile(t, dir, "001_create_users.go", "package migrations\n")

		v := NewMigrationVisualizer(dir, nil)
		v.appliedSet[1] = true
		ctx := context.Background()

		dag, err := v.BuildDAG(ctx)
		if err != nil {
			t.Fatalf("BuildDAG failed: %v", err)
		}

		if dag.Nodes[0].Status != "applied" {
			t.Errorf("expected 'applied', got '%s'", dag.Nodes[0].Status)
		}
	})

	t.Run("with executor that errors", func(t *testing.T) {
		dir := t.TempDir()
		createMigrationFile(t, dir, "001_create_users.go", "package migrations\n")

		// Use mock executor that errors
		executor := newMockExecutor(t)
		v := NewMigrationVisualizer(dir, executor)
		ctx := context.Background()

		// Should not return error even if loadAppliedMigrations fails
		dag, err := v.BuildDAG(ctx)
		if err != nil {
			t.Fatalf("BuildDAG should not fail on loadAppliedMigrations error: %v", err)
		}
		if len(dag.Nodes) != 1 {
			t.Errorf("expected 1 node, got %d", len(dag.Nodes))
		}
	})
}

// ========================================
// Tests for Visualize
// ========================================

func TestVisualize(t *testing.T) {
	dir := t.TempDir()
	createMigrationFile(t, dir, "001_create_users.go", "package migrations\n")
	createMigrationFile(t, dir, "002_create_posts.go", "package migrations\n")

	v := NewMigrationVisualizer(dir, nil)
	v.appliedSet[1] = true
	ctx := context.Background()

	t.Run("JSON format", func(t *testing.T) {
		var buf bytes.Buffer
		err := v.Visualize(ctx, OutputFormatJSON, &buf)
		if err != nil {
			t.Fatalf("Visualize JSON failed: %v", err)
		}

		var dag MigrationDAG
		if err := json.Unmarshal(buf.Bytes(), &dag); err != nil {
			t.Fatalf("invalid JSON: %v", err)
		}
		if len(dag.Nodes) != 2 {
			t.Errorf("expected 2 nodes, got %d", len(dag.Nodes))
		}
	})

	t.Run("DOT format", func(t *testing.T) {
		var buf bytes.Buffer
		err := v.Visualize(ctx, OutputFormatDOT, &buf)
		if err != nil {
			t.Fatalf("Visualize DOT failed: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "digraph migrations") {
			t.Error("expected digraph header")
		}
		if !strings.Contains(output, "lightgreen") {
			t.Error("expected lightgreen for applied migration")
		}
		if !strings.Contains(output, "lightblue") {
			t.Error("expected lightblue for pending migration")
		}
		if !strings.Contains(output, "->") {
			t.Error("expected edge arrow")
		}
	})

	t.Run("Mermaid format", func(t *testing.T) {
		var buf bytes.Buffer
		err := v.Visualize(ctx, OutputFormatMermaid, &buf)
		if err != nil {
			t.Fatalf("Visualize Mermaid failed: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "```mermaid") {
			t.Error("expected mermaid code fence")
		}
		if !strings.Contains(output, "graph TD") {
			t.Error("expected graph TD")
		}
		if !strings.Contains(output, "-->") {
			t.Error("expected edge arrow")
		}
		if !strings.Contains(output, "classDef applied") {
			t.Error("expected classDef applied")
		}
	})

	t.Run("ASCII format", func(t *testing.T) {
		var buf bytes.Buffer
		err := v.Visualize(ctx, OutputFormatASCII, &buf)
		if err != nil {
			t.Fatalf("Visualize ASCII failed: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "Migration DAG") {
			t.Error("expected Migration DAG header")
		}
		if !strings.Contains(output, "[APPLIED]") {
			t.Error("expected [APPLIED] status")
		}
		if !strings.Contains(output, "[PENDING]") {
			t.Error("expected [PENDING] status")
		}
		if !strings.Contains(output, "Legend") {
			t.Error("expected Legend")
		}
	})

	t.Run("HTML format", func(t *testing.T) {
		var buf bytes.Buffer
		err := v.Visualize(ctx, OutputFormatHTML, &buf)
		if err != nil {
			t.Fatalf("Visualize HTML failed: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "<!DOCTYPE html>") {
			t.Error("expected HTML doctype")
		}
		if !strings.Contains(output, "Migration DAG Visualizer") {
			t.Error("expected title")
		}
		if !strings.Contains(output, "dagre-d3") {
			t.Error("expected dagre-d3 reference")
		}
	})

	t.Run("unsupported format returns error", func(t *testing.T) {
		var buf bytes.Buffer
		err := v.Visualize(ctx, "unknown", &buf)
		if err == nil {
			t.Fatal("expected error for unsupported format")
		}
	})
}

// ========================================
// Tests for ValidateDAG
// ========================================

func TestValidateDAG(t *testing.T) {
	v := NewMigrationVisualizer("", nil)

	t.Run("valid DAG with no cycles", func(t *testing.T) {
		dag := &MigrationDAG{
			Nodes: []MigrationNode{
				{Version: 1}, {Version: 2}, {Version: 3},
			},
			Edges: []MigrationEdge{
				{From: 1, To: 2},
				{From: 2, To: 3},
			},
		}

		if err := v.ValidateDAG(dag); err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
	})

	t.Run("DAG with cycle", func(t *testing.T) {
		dag := &MigrationDAG{
			Nodes: []MigrationNode{
				{Version: 1}, {Version: 2}, {Version: 3},
			},
			Edges: []MigrationEdge{
				{From: 1, To: 2},
				{From: 2, To: 3},
				{From: 3, To: 1},
			},
		}

		err := v.ValidateDAG(dag)
		if err == nil {
			t.Fatal("expected cycle error")
		}
		if !strings.Contains(err.Error(), "cycle detected") {
			t.Errorf("expected 'cycle detected' error, got: %v", err)
		}
	})

	t.Run("empty DAG is valid", func(t *testing.T) {
		dag := &MigrationDAG{
			Nodes: []MigrationNode{},
			Edges: []MigrationEdge{},
		}

		if err := v.ValidateDAG(dag); err != nil {
			t.Fatalf("expected no error for empty DAG, got: %v", err)
		}
	})

	t.Run("single node is valid", func(t *testing.T) {
		dag := &MigrationDAG{
			Nodes: []MigrationNode{{Version: 1}},
			Edges: []MigrationEdge{},
		}

		if err := v.ValidateDAG(dag); err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
	})

	t.Run("self-loop is a cycle", func(t *testing.T) {
		dag := &MigrationDAG{
			Nodes: []MigrationNode{{Version: 1}},
			Edges: []MigrationEdge{{From: 1, To: 1}},
		}

		err := v.ValidateDAG(dag)
		if err == nil {
			t.Fatal("expected cycle error for self-loop")
		}
	})

	t.Run("diamond shape is valid", func(t *testing.T) {
		dag := &MigrationDAG{
			Nodes: []MigrationNode{
				{Version: 1}, {Version: 2}, {Version: 3}, {Version: 4},
			},
			Edges: []MigrationEdge{
				{From: 1, To: 2},
				{From: 1, To: 3},
				{From: 2, To: 4},
				{From: 3, To: 4},
			},
		}

		if err := v.ValidateDAG(dag); err != nil {
			t.Fatalf("expected no error for diamond, got: %v", err)
		}
	})
}

// ========================================
// Tests for GetExecutionOrder
// ========================================

func TestGetExecutionOrder(t *testing.T) {
	v := NewMigrationVisualizer("", nil)

	t.Run("linear order", func(t *testing.T) {
		dag := &MigrationDAG{
			Nodes: []MigrationNode{
				{Version: 1}, {Version: 2}, {Version: 3},
			},
			Edges: []MigrationEdge{
				{From: 1, To: 2},
				{From: 2, To: 3},
			},
		}

		order, err := v.GetExecutionOrder(dag)
		if err != nil {
			t.Fatalf("GetExecutionOrder failed: %v", err)
		}

		if len(order) != 3 {
			t.Fatalf("expected 3 items, got %d", len(order))
		}
		if order[0] != 1 || order[1] != 2 || order[2] != 3 {
			t.Errorf("expected [1, 2, 3], got %v", order)
		}
	})

	t.Run("diamond order", func(t *testing.T) {
		dag := &MigrationDAG{
			Nodes: []MigrationNode{
				{Version: 1}, {Version: 2}, {Version: 3}, {Version: 4},
			},
			Edges: []MigrationEdge{
				{From: 1, To: 2},
				{From: 1, To: 3},
				{From: 2, To: 4},
				{From: 3, To: 4},
			},
		}

		order, err := v.GetExecutionOrder(dag)
		if err != nil {
			t.Fatalf("GetExecutionOrder failed: %v", err)
		}

		if len(order) != 4 {
			t.Fatalf("expected 4 items, got %d", len(order))
		}

		// 1 must come before 2 and 3
		// 2 and 3 must come before 4
		pos := make(map[int64]int)
		for i, v := range order {
			pos[v] = i
		}

		if pos[1] > pos[2] || pos[1] > pos[3] {
			t.Error("1 should come before 2 and 3")
		}
		if pos[2] > pos[4] || pos[3] > pos[4] {
			t.Error("2 and 3 should come before 4")
		}
	})

	t.Run("cycle returns error", func(t *testing.T) {
		dag := &MigrationDAG{
			Nodes: []MigrationNode{
				{Version: 1}, {Version: 2},
			},
			Edges: []MigrationEdge{
				{From: 1, To: 2},
				{From: 2, To: 1},
			},
		}

		_, err := v.GetExecutionOrder(dag)
		if err == nil {
			t.Fatal("expected error for cycle")
		}
	})

	t.Run("empty DAG", func(t *testing.T) {
		dag := &MigrationDAG{
			Nodes: []MigrationNode{},
			Edges: []MigrationEdge{},
		}

		order, err := v.GetExecutionOrder(dag)
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if len(order) != 0 {
			t.Errorf("expected empty order, got %v", order)
		}
	})

	t.Run("deterministic order for independent nodes", func(t *testing.T) {
		dag := &MigrationDAG{
			Nodes: []MigrationNode{
				{Version: 3}, {Version: 1}, {Version: 2},
			},
			Edges: []MigrationEdge{},
		}

		order, err := v.GetExecutionOrder(dag)
		if err != nil {
			t.Fatalf("GetExecutionOrder failed: %v", err)
		}

		// Should be sorted by version (deterministic)
		if order[0] != 1 || order[1] != 2 || order[2] != 3 {
			t.Errorf("expected deterministic order [1, 2, 3], got %v", order)
		}
	})
}

// ========================================
// Tests for calculateDepths
// ========================================

func TestCalculateDepths(t *testing.T) {
	v := NewMigrationVisualizer("", nil)

	t.Run("linear depths", func(t *testing.T) {
		dag := &MigrationDAG{
			Nodes: []MigrationNode{
				{Version: 1}, {Version: 2}, {Version: 3},
			},
			Edges: []MigrationEdge{
				{From: 1, To: 2},
				{From: 2, To: 3},
			},
		}

		depths := make(map[int64]int)
		v.calculateDepths(dag, depths)

		if depths[1] != 0 {
			t.Errorf("expected depth 0 for version 1, got %d", depths[1])
		}
		if depths[2] != 1 {
			t.Errorf("expected depth 1 for version 2, got %d", depths[2])
		}
		if depths[3] != 2 {
			t.Errorf("expected depth 2 for version 3, got %d", depths[3])
		}
	})

	t.Run("diamond depths", func(t *testing.T) {
		dag := &MigrationDAG{
			Nodes: []MigrationNode{
				{Version: 1}, {Version: 2}, {Version: 3}, {Version: 4},
			},
			Edges: []MigrationEdge{
				{From: 1, To: 2},
				{From: 1, To: 3},
				{From: 2, To: 4},
				{From: 3, To: 4},
			},
		}

		depths := make(map[int64]int)
		v.calculateDepths(dag, depths)

		if depths[1] != 0 {
			t.Errorf("expected depth 0 for version 1, got %d", depths[1])
		}
		if depths[4] != 2 {
			t.Errorf("expected depth 2 for version 4, got %d", depths[4])
		}
	})
}

// ========================================
// Tests for parseInt64
// ========================================

func TestParseInt64(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"1", 1},
		{"001", 1},
		{"100", 100},
		{"0", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseInt64(tt.input)
			if result != tt.expected {
				t.Errorf("parseInt64(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

// ========================================
// Tests for OutputFormat constants
// ========================================

func TestOutputFormatConstants(t *testing.T) {
	tests := []struct {
		format   OutputFormat
		expected string
	}{
		{OutputFormatJSON, "json"},
		{OutputFormatHTML, "html"},
		{OutputFormatDOT, "dot"},
		{OutputFormatMermaid, "mermaid"},
		{OutputFormatASCII, "ascii"},
	}

	for _, tt := range tests {
		if string(tt.format) != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, tt.format)
		}
	}
}

// ========================================
// Tests for outputDOT with failed status
// ========================================

func TestOutputDOTFailedStatus(t *testing.T) {
	v := NewMigrationVisualizer("", nil)

	dag := &MigrationDAG{
		Nodes: []MigrationNode{
			{Version: 1, Description: "test", Status: "failed"},
		},
		Edges: []MigrationEdge{},
	}

	var buf bytes.Buffer
	err := v.outputDOT(dag, &buf)
	if err != nil {
		t.Fatalf("outputDOT failed: %v", err)
	}

	if !strings.Contains(buf.String(), "lightcoral") {
		t.Error("expected lightcoral for failed status")
	}
}

// ========================================
// Tests for outputASCII with failed status
// ========================================

func TestOutputASCIIFailedStatus(t *testing.T) {
	v := NewMigrationVisualizer("", nil)

	dag := &MigrationDAG{
		Nodes: []MigrationNode{
			{Version: 1, Description: "test", Status: "failed"},
		},
		Edges: []MigrationEdge{},
	}

	var buf bytes.Buffer
	err := v.outputASCII(dag, &buf)
	if err != nil {
		t.Fatalf("outputASCII failed: %v", err)
	}

	if !strings.Contains(buf.String(), "[FAILED]") {
		t.Error("expected [FAILED] status")
	}
}

// ========================================
// Tests for MigrationNode, MigrationDAG, MigrationEdge structs
// ========================================

func TestMigrationNodeStruct(t *testing.T) {
	node := MigrationNode{
		Version:     1,
		Description: "test migration",
		Status:      "pending",
		DependsOn:   []int64{},
		FileName:    "001_test.go",
	}

	if node.Version != 1 {
		t.Error("version mismatch")
	}
	if node.Description != "test migration" {
		t.Error("description mismatch")
	}
}

func TestMigrationEdgeStruct(t *testing.T) {
	edge := MigrationEdge{From: 1, To: 2}
	if edge.From != 1 || edge.To != 2 {
		t.Error("edge mismatch")
	}
}

func TestMigrationInfoStruct(t *testing.T) {
	info := MigrationInfo{
		Version:     1,
		Description: "test",
		FileName:    "001_test.go",
		DependsOn:   []int64{},
	}
	if info.Version != 1 {
		t.Error("version mismatch")
	}
}

// ========================================
// Tests for outputMermaid with applied nodes
// ========================================

func TestOutputMermaidAppliedNode(t *testing.T) {
	v := NewMigrationVisualizer("", nil)

	dag := &MigrationDAG{
		Nodes: []MigrationNode{
			{Version: 1, Description: "test", Status: "applied"},
		},
		Edges: []MigrationEdge{},
	}

	var buf bytes.Buffer
	err := v.outputMermaid(dag, &buf)
	if err != nil {
		t.Fatalf("outputMermaid failed: %v", err)
	}

	output := buf.String()
	// Applied nodes use ([%s]) shape
	if !strings.Contains(output, "([") {
		t.Error("expected applied node shape ([ ])")
	}
}

// ========================================
// Tests for loadAppliedMigrations with working executor
// ========================================

func TestLoadAppliedMigrations(t *testing.T) {
	t.Run("loads applied migrations from DB", func(t *testing.T) {
		executor := newMockExecutor(t)
		ctx := context.Background()

		// Create schema_migrations table and insert data
		_, err := executor.ExecContext(ctx, `CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, name TEXT, applied_at DATETIME)`)
		if err != nil {
			t.Fatalf("failed to create table: %v", err)
		}
		_, err = executor.ExecContext(ctx, `INSERT INTO schema_migrations (version, name, applied_at) VALUES (1, 'first', '2024-01-01')`)
		if err != nil {
			t.Fatalf("failed to insert: %v", err)
		}
		_, err = executor.ExecContext(ctx, `INSERT INTO schema_migrations (version, name, applied_at) VALUES (2, 'second', '2024-01-02')`)
		if err != nil {
			t.Fatalf("failed to insert: %v", err)
		}

		v := NewMigrationVisualizer("", executor)

		err = v.loadAppliedMigrations(ctx)
		if err != nil {
			t.Fatalf("loadAppliedMigrations failed: %v", err)
		}

		if !v.appliedSet[1] {
			t.Error("expected version 1 to be applied")
		}
		if !v.appliedSet[2] {
			t.Error("expected version 2 to be applied")
		}
		if v.appliedSet[3] {
			t.Error("expected version 3 to NOT be applied")
		}
	})
}

// ========================================
// Tests for BuildDAG with applied migrations from DB
// ========================================

func TestBuildDAGWithAppliedFromDB(t *testing.T) {
	dir := t.TempDir()
	createMigrationFile(t, dir, "001_create_users.go", "package migrations\n")
	createMigrationFile(t, dir, "002_create_posts.go", "package migrations\n")

	executor := newMockExecutor(t)
	ctx := context.Background()

	// Create schema_migrations table and mark version 1 as applied
	executor.ExecContext(ctx, `CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, name TEXT, applied_at DATETIME)`)
	executor.ExecContext(ctx, `INSERT INTO schema_migrations (version, name, applied_at) VALUES (1, 'create_users', '2024-01-01')`)

	v := NewMigrationVisualizer(dir, executor)
	dag, err := v.BuildDAG(ctx)
	if err != nil {
		t.Fatalf("BuildDAG failed: %v", err)
	}

	// Version 1 should be applied, version 2 pending
	for _, node := range dag.Nodes {
		if node.Version == 1 && node.Status != "applied" {
			t.Errorf("expected version 1 to be applied, got %s", node.Status)
		}
		if node.Version == 2 && node.Status != "pending" {
			t.Errorf("expected version 2 to be pending, got %s", node.Status)
		}
	}
}

// ========================================
// Tests for outputASCII with multiple depth layers
// ========================================

func TestOutputASCIIMultipleLayers(t *testing.T) {
	v := NewMigrationVisualizer("", nil)

	dag := &MigrationDAG{
		Nodes: []MigrationNode{
			{Version: 1, Description: "first", Status: "applied"},
			{Version: 2, Description: "second", Status: "pending"},
			{Version: 3, Description: "third", Status: "pending"},
		},
		Edges: []MigrationEdge{
			{From: 1, To: 2},
			{From: 2, To: 3},
		},
	}

	var buf bytes.Buffer
	err := v.outputASCII(dag, &buf)
	if err != nil {
		t.Fatalf("outputASCII failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "[APPLIED]") {
		t.Error("expected [APPLIED]")
	}
	if !strings.Contains(output, "[PENDING]") {
		t.Error("expected [PENDING]")
	}
}

// ========================================
// Helpers
// ========================================

func createMigrationFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to create migration file %s: %v", name, err)
	}
}
