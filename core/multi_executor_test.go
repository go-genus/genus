package core

import (
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
