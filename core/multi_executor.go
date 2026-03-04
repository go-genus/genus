package core

import (
	"context"
	"database/sql"
	"sync/atomic"
)

// MultiExecutor implementa Executor com suporte a read replicas.
// Writes sempre vão para o primary, reads vão para replicas em round-robin.
// Se não houver replicas configuradas, todas as operações vão para o primary.
type MultiExecutor struct {
	primary  *sql.DB
	replicas []*sql.DB
	counter  uint64 // Contador atômico para round-robin
}

// NewMultiExecutor cria um novo MultiExecutor.
// O primary é obrigatório, replicas são opcionais.
func NewMultiExecutor(primary *sql.DB, replicas ...*sql.DB) *MultiExecutor {
	return &MultiExecutor{
		primary:  primary,
		replicas: replicas,
		counter:  0,
	}
}

// Primary retorna o database primary.
func (m *MultiExecutor) Primary() *sql.DB {
	return m.primary
}

// Replicas retorna a lista de replicas.
func (m *MultiExecutor) Replicas() []*sql.DB {
	return m.replicas
}

// ReplicaCount retorna o número de replicas configuradas.
func (m *MultiExecutor) ReplicaCount() int {
	return len(m.replicas)
}

// nextReplica retorna a próxima replica usando round-robin.
// Se não houver replicas, retorna o primary.
func (m *MultiExecutor) nextReplica() *sql.DB {
	if len(m.replicas) == 0 {
		return m.primary
	}

	// Incrementa atomicamente e usa módulo para round-robin
	idx := atomic.AddUint64(&m.counter, 1) % uint64(len(m.replicas))
	return m.replicas[idx]
}

// getReadDB retorna o database apropriado para leitura.
// Usa primary se o contexto indicar WithPrimary, caso contrário usa replica.
func (m *MultiExecutor) getReadDB(ctx context.Context) *sql.DB {
	if UsePrimary(ctx) {
		return m.primary
	}
	return m.nextReplica()
}

// ExecContext executa uma query de escrita (sempre no primary).
// Implementa a interface Executor.
func (m *MultiExecutor) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return m.primary.ExecContext(ctx, query, args...)
}

// QueryContext executa uma query de leitura (replica ou primary baseado no contexto).
// Implementa a interface Executor.
func (m *MultiExecutor) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return m.getReadDB(ctx).QueryContext(ctx, query, args...)
}

// QueryRowContext executa uma query que retorna uma única linha (replica ou primary).
// Implementa a interface Executor.
func (m *MultiExecutor) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return m.getReadDB(ctx).QueryRowContext(ctx, query, args...)
}

// Close fecha todas as conexões (primary e replicas).
func (m *MultiExecutor) Close() error {
	var lastErr error

	if err := m.primary.Close(); err != nil {
		lastErr = err
	}

	for _, replica := range m.replicas {
		if err := replica.Close(); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// Ping verifica a conectividade de todas as conexões.
func (m *MultiExecutor) Ping(ctx context.Context) error {
	if err := m.primary.PingContext(ctx); err != nil {
		return err
	}

	for _, replica := range m.replicas {
		if err := replica.PingContext(ctx); err != nil {
			return err
		}
	}

	return nil
}

// Stats retorna estatísticas do primary database.
func (m *MultiExecutor) Stats() sql.DBStats {
	return m.primary.Stats()
}

// AllStats retorna estatísticas de todas as conexões.
func (m *MultiExecutor) AllStats() map[string]sql.DBStats {
	stats := make(map[string]sql.DBStats)
	stats["primary"] = m.primary.Stats()

	for i, replica := range m.replicas {
		key := replicaKey(i)
		stats[key] = replica.Stats()
	}

	return stats
}

func replicaKey(index int) string {
	return "replica_" + string(rune('0'+index))
}
