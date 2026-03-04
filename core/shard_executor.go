package core

import (
	"context"
	"database/sql"
	"fmt"
	"sync"

	"github.com/go-genus/genus/sharding"
)

// ShardExecutor implementa Executor para queries sharded.
// Roteia automaticamente queries para o shard correto baseado no contexto.
type ShardExecutor struct {
	manager *sharding.ShardManager
	dialect Dialect
}

// NewShardExecutor cria um novo executor sharded.
func NewShardExecutor(manager *sharding.ShardManager, dialect Dialect) *ShardExecutor {
	return &ShardExecutor{
		manager: manager,
		dialect: dialect,
	}
}

// GetManager retorna o shard manager.
func (se *ShardExecutor) GetManager() *sharding.ShardManager {
	return se.manager
}

// ExecContext executa uma query no shard apropriado.
func (se *ShardExecutor) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	shard := se.manager.GetShardFromContext(ctx)
	return shard.DB.ExecContext(ctx, query, args...)
}

// QueryContext executa uma query SELECT no shard apropriado.
func (se *ShardExecutor) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	shard := se.manager.GetShardFromContext(ctx)
	return shard.DB.QueryContext(ctx, query, args...)
}

// QueryRowContext executa uma query que retorna uma única linha no shard apropriado.
func (se *ShardExecutor) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	shard := se.manager.GetShardFromContext(ctx)
	return shard.DB.QueryRowContext(ctx, query, args...)
}

// ExecOnShard executa uma query em um shard específico.
func (se *ShardExecutor) ExecOnShard(ctx context.Context, shardIndex int, query string, args ...interface{}) (sql.Result, error) {
	shard := se.manager.GetShardByIndex(shardIndex)
	if shard == nil {
		return nil, fmt.Errorf("shard %d not found", shardIndex)
	}
	return shard.DB.ExecContext(ctx, query, args...)
}

// QueryOnShard executa uma query SELECT em um shard específico.
func (se *ShardExecutor) QueryOnShard(ctx context.Context, shardIndex int, query string, args ...interface{}) (*sql.Rows, error) {
	shard := se.manager.GetShardByIndex(shardIndex)
	if shard == nil {
		return nil, fmt.Errorf("shard %d not found", shardIndex)
	}
	return shard.DB.QueryContext(ctx, query, args...)
}

// ExecOnAllShards executa uma query em todos os shards.
// Retorna um slice de resultados, um por shard.
func (se *ShardExecutor) ExecOnAllShards(ctx context.Context, query string, args ...interface{}) ([]sql.Result, []error) {
	shards := se.manager.AllShards()
	results := make([]sql.Result, len(shards))
	errors := make([]error, len(shards))

	var wg sync.WaitGroup
	for i, shard := range shards {
		wg.Add(1)
		go func(idx int, s *sharding.Shard) {
			defer wg.Done()
			result, err := s.DB.ExecContext(ctx, query, args...)
			results[idx] = result
			errors[idx] = err
		}(i, shard)
	}
	wg.Wait()

	return results, errors
}

// QueryAllShards executa uma query SELECT em todos os shards em paralelo.
// O callback é chamado para cada conjunto de resultados.
func (se *ShardExecutor) QueryAllShards(ctx context.Context, query string, args []interface{}, callback func(shardIndex int, rows *sql.Rows) error) []error {
	shards := se.manager.AllShards()
	errors := make([]error, len(shards))

	var wg sync.WaitGroup
	for i, shard := range shards {
		wg.Add(1)
		go func(idx int, s *sharding.Shard) {
			defer wg.Done()

			rows, err := s.DB.QueryContext(ctx, query, args...)
			if err != nil {
				errors[idx] = err
				return
			}
			defer rows.Close()

			if err := callback(idx, rows); err != nil {
				errors[idx] = err
			}
		}(i, shard)
	}
	wg.Wait()

	return errors
}

// BeginTxOnShard inicia uma transação em um shard específico.
func (se *ShardExecutor) BeginTxOnShard(ctx context.Context, shardIndex int, opts *sql.TxOptions) (*sql.Tx, error) {
	shard := se.manager.GetShardByIndex(shardIndex)
	if shard == nil {
		return nil, fmt.Errorf("shard %d not found", shardIndex)
	}
	return shard.DB.BeginTx(ctx, opts)
}

// BeginTx inicia uma transação no shard determinado pelo contexto.
func (se *ShardExecutor) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	shard := se.manager.GetShardFromContext(ctx)
	return shard.DB.BeginTx(ctx, opts)
}

// Close fecha todas as conexões de shard.
func (se *ShardExecutor) Close() error {
	return se.manager.Close()
}

// NumShards retorna o número de shards.
func (se *ShardExecutor) NumShards() int {
	return se.manager.NumShards()
}

// ShardedDB é um wrapper que fornece operações CRUD sharded.
type ShardedDB struct {
	executor *ShardExecutor
	dialect  Dialect
	logger   Logger
}

// NewShardedDB cria um novo ShardedDB.
func NewShardedDB(executor *ShardExecutor, dialect Dialect) *ShardedDB {
	return &ShardedDB{
		executor: executor,
		dialect:  dialect,
	}
}

// NewShardedDBWithLogger cria um novo ShardedDB com logger.
func NewShardedDBWithLogger(executor *ShardExecutor, dialect Dialect, logger Logger) *ShardedDB {
	return &ShardedDB{
		executor: executor,
		dialect:  dialect,
		logger:   logger,
	}
}

// Executor retorna o executor sharded.
func (sdb *ShardedDB) Executor() *ShardExecutor {
	return sdb.executor
}

// Dialect retorna o dialeto SQL.
func (sdb *ShardedDB) Dialect() Dialect {
	return sdb.dialect
}

// Logger retorna o logger.
func (sdb *ShardedDB) Logger() Logger {
	return sdb.logger
}

// SetLogger define o logger.
func (sdb *ShardedDB) SetLogger(logger Logger) {
	sdb.logger = logger
}

// Close fecha todas as conexões.
func (sdb *ShardedDB) Close() error {
	return sdb.executor.Close()
}

// WithShardKey é um helper para criar um contexto com shard key.
func WithShardKey(ctx context.Context, key sharding.ShardKey) context.Context {
	return sharding.WithShardKey(ctx, key)
}

// Int64ShardKey é um alias para facilitar o uso.
type Int64ShardKey = sharding.Int64ShardKey

// StringShardKey é um alias para facilitar o uso.
type StringShardKey = sharding.StringShardKey
