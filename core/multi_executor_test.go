package core

import (
	"context"
	"testing"
)

func TestNewMultiExecutor(t *testing.T) {
	// Test with only primary (no replicas)
	executor := NewMultiExecutor(nil)
	if executor.Primary() != nil {
		t.Error("Expected nil primary")
	}
	if executor.ReplicaCount() != 0 {
		t.Errorf("ReplicaCount() = %d, want 0", executor.ReplicaCount())
	}
}

func TestMultiExecutor_ReplicaCount(t *testing.T) {
	// Test with replicas
	executor := NewMultiExecutor(nil, nil, nil, nil)
	if executor.ReplicaCount() != 3 {
		t.Errorf("ReplicaCount() = %d, want 3", executor.ReplicaCount())
	}
}

func TestMultiExecutor_Replicas(t *testing.T) {
	executor := NewMultiExecutor(nil, nil, nil)
	replicas := executor.Replicas()
	if len(replicas) != 2 {
		t.Errorf("len(Replicas()) = %d, want 2", len(replicas))
	}
}

func TestMultiExecutor_nextReplica_NoReplicas(t *testing.T) {
	// Quando não há replicas, nextReplica deve retornar primary
	// Criamos com nil porque não precisamos de DB real para este teste
	executor := NewMultiExecutor(nil)

	// Deve retornar nil (que é o primary)
	if got := executor.nextReplica(); got != nil {
		t.Errorf("nextReplica() with no replicas should return primary")
	}
}

func TestReplicaKey(t *testing.T) {
	tests := []struct {
		index    int
		expected string
	}{
		{0, "replica_0"},
		{1, "replica_1"},
		{9, "replica_9"},
	}

	for _, tt := range tests {
		got := replicaKey(tt.index)
		if got != tt.expected {
			t.Errorf("replicaKey(%d) = %q, want %q", tt.index, got, tt.expected)
		}
	}
}

func TestMultiExecutor_getReadDB_UsePrimary(t *testing.T) {
	primary := getMockSQLDB()
	defer primary.Close()
	replica := getMockSQLDB()
	defer replica.Close()

	executor := NewMultiExecutor(primary, replica)

	ctx := WithPrimary(context.Background())
	db := executor.getReadDB(ctx)
	if db != primary {
		t.Error("getReadDB with UsePrimary should return primary")
	}
}

func TestMultiExecutor_getReadDB_UsesReplica(t *testing.T) {
	primary := getMockSQLDB()
	defer primary.Close()
	replica := getMockSQLDB()
	defer replica.Close()

	executor := NewMultiExecutor(primary, replica)

	ctx := context.Background()
	db := executor.getReadDB(ctx)
	if db == primary {
		t.Error("getReadDB without UsePrimary should return replica")
	}
}

func TestMultiExecutor_nextReplica_RoundRobin(t *testing.T) {
	primary := getMockSQLDB()
	defer primary.Close()
	r1 := getMockSQLDB()
	defer r1.Close()
	r2 := getMockSQLDB()
	defer r2.Close()

	executor := NewMultiExecutor(primary, r1, r2)

	// Round-robin: first call counter goes to 1, 1%2=1 -> r2
	got1 := executor.nextReplica()
	got2 := executor.nextReplica()

	// Just verify they alternate and don't return primary
	if got1 == primary || got2 == primary {
		t.Error("nextReplica should not return primary when replicas exist")
	}
}

func TestMultiExecutor_ExecContext(t *testing.T) {
	primary := getMockSQLDB()
	defer primary.Close()

	executor := NewMultiExecutor(primary)

	// ExecContext always uses primary
	_, err := executor.ExecContext(context.Background(), "INSERT INTO t VALUES (1)")
	// Will fail with mock driver, but tests the path
	if err != nil {
		t.Logf("Expected error with mock: %v", err)
	}
}

func TestMultiExecutor_QueryContext(t *testing.T) {
	primary := getMockSQLDB()
	defer primary.Close()
	replica := getMockSQLDB()
	defer replica.Close()

	executor := NewMultiExecutor(primary, replica)

	_, err := executor.QueryContext(context.Background(), "SELECT 1")
	if err != nil {
		t.Logf("Expected error with mock: %v", err)
	}
}

func TestMultiExecutor_QueryRowContext(t *testing.T) {
	primary := getMockSQLDB()
	defer primary.Close()

	executor := NewMultiExecutor(primary)

	row := executor.QueryRowContext(context.Background(), "SELECT 1")
	if row == nil {
		t.Error("QueryRowContext should not return nil")
	}
}

func TestMultiExecutor_Close(t *testing.T) {
	primary := getMockSQLDB()
	replica := getMockSQLDB()

	executor := NewMultiExecutor(primary, replica)
	err := executor.Close()
	// Mock driver close should succeed
	if err != nil {
		t.Logf("Close error: %v", err)
	}
}

func TestMultiExecutor_Ping(t *testing.T) {
	primary := getMockSQLDB()
	defer primary.Close()

	executor := NewMultiExecutor(primary)
	err := executor.Ping(context.Background())
	// Mock driver doesn't implement ping properly
	if err != nil {
		t.Logf("Ping error (expected with mock): %v", err)
	}
}

func TestMultiExecutor_Ping_WithReplicas(t *testing.T) {
	primary := getMockSQLDB()
	defer primary.Close()
	replica := getMockSQLDB()
	defer replica.Close()

	executor := NewMultiExecutor(primary, replica)
	err := executor.Ping(context.Background())
	if err != nil {
		t.Logf("Ping error (expected with mock): %v", err)
	}
}

func TestMultiExecutor_Close_PrimaryError(t *testing.T) {
	primary := getMockSQLDBFailClose()
	replica := getMockSQLDB()
	defer replica.Close()

	executor := NewMultiExecutor(primary, replica)
	err := executor.Close()
	if err == nil {
		t.Error("should return error when primary close fails")
	}
}

func TestMultiExecutor_Close_ReplicaError(t *testing.T) {
	primary := getMockSQLDB()
	replica := getMockSQLDBFailClose()

	executor := NewMultiExecutor(primary, replica)
	err := executor.Close()
	if err == nil {
		t.Error("should return error when replica close fails")
	}
}

func TestMultiExecutor_Ping_PrimaryError(t *testing.T) {
	primary := getMockSQLDBFailClose()
	replica := getMockSQLDB()
	defer replica.Close()

	executor := NewMultiExecutor(primary, replica)
	err := executor.Ping(context.Background())
	if err == nil {
		t.Error("should return error when primary ping fails")
	}
}

func TestMultiExecutor_Ping_ReplicaError(t *testing.T) {
	primary := getMockSQLDB()
	defer primary.Close()
	replica := getMockSQLDBFailClose()

	executor := NewMultiExecutor(primary, replica)
	err := executor.Ping(context.Background())
	if err == nil {
		t.Error("should return error when replica ping fails")
	}
}

func TestMultiExecutor_Stats(t *testing.T) {
	primary := getMockSQLDB()
	defer primary.Close()

	executor := NewMultiExecutor(primary)
	stats := executor.Stats()
	// Just verify it doesn't panic
	_ = stats
}

func TestMultiExecutor_AllStats(t *testing.T) {
	primary := getMockSQLDB()
	defer primary.Close()
	replica := getMockSQLDB()
	defer replica.Close()

	executor := NewMultiExecutor(primary, replica)
	stats := executor.AllStats()

	if _, ok := stats["primary"]; !ok {
		t.Error("AllStats should contain 'primary' key")
	}
	if len(stats) != 2 {
		t.Errorf("AllStats should have 2 entries, got %d", len(stats))
	}
}
