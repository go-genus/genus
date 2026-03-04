package core

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"time"
)

// BatchConfig contém configurações para operações em lote.
type BatchConfig struct {
	// BatchSize define o tamanho máximo de cada lote.
	// Default: 100
	BatchSize int

	// SkipHooks ignora os hooks de lifecycle (BeforeSave, AfterSave, etc).
	// Útil para operações de alta performance onde hooks não são necessários.
	// Default: false
	SkipHooks bool
}

// DefaultBatchConfig retorna a configuração padrão para operações em lote.
func DefaultBatchConfig() BatchConfig {
	return BatchConfig{
		BatchSize: 100,
		SkipHooks: false,
	}
}

// BatchInsert insere múltiplos registros em uma única query.
// Os modelos devem ser um slice de ponteiros para structs (ex: []*User).
// IDs são definidos nos modelos após a inserção (se o banco suportar RETURNING).
//
// Exemplo:
//
//	users := []*User{{Name: "Alice"}, {Name: "Bob"}}
//	err := db.BatchInsert(ctx, users)
func (db *DB) BatchInsert(ctx context.Context, models interface{}) error {
	return db.BatchInsertWithConfig(ctx, models, DefaultBatchConfig())
}

// BatchInsertWithConfig insere múltiplos registros com configuração personalizada.
func (db *DB) BatchInsertWithConfig(ctx context.Context, models interface{}, config BatchConfig) error {
	slice := reflect.ValueOf(models)
	if slice.Kind() != reflect.Slice {
		return fmt.Errorf("models must be a slice")
	}

	if slice.Len() == 0 {
		return nil
	}

	// Valida que é um slice de ponteiros
	elemType := slice.Type().Elem()
	if elemType.Kind() != reflect.Ptr {
		return fmt.Errorf("models must be a slice of pointers (e.g., []*User)")
	}

	// Processa em lotes
	batchSize := config.BatchSize
	if batchSize <= 0 {
		batchSize = 100
	}

	for i := 0; i < slice.Len(); i += batchSize {
		end := i + batchSize
		if end > slice.Len() {
			end = slice.Len()
		}

		batch := make([]interface{}, end-i)
		for j := i; j < end; j++ {
			batch[j-i] = slice.Index(j).Interface()
		}

		if err := db.insertBatch(ctx, batch, config); err != nil {
			return fmt.Errorf("batch insert failed at index %d: %w", i, err)
		}
	}

	return nil
}

// insertBatch insere um lote de modelos.
func (db *DB) insertBatch(ctx context.Context, models []interface{}, config BatchConfig) error {
	if len(models) == 0 {
		return nil
	}

	// Executa hooks BeforeSave e BeforeCreate
	if !config.SkipHooks {
		for _, model := range models {
			if bs, ok := model.(BeforeSaver); ok {
				if err := bs.BeforeSave(); err != nil {
					return fmt.Errorf("BeforeSave hook failed: %w", err)
				}
			}
			if bc, ok := model.(BeforeCreater); ok {
				if err := bc.BeforeCreate(); err != nil {
					return fmt.Errorf("BeforeCreate hook failed: %w", err)
				}
			}
		}
	}

	// Preenche timestamps
	for _, model := range models {
		setTimestamps(model)
	}

	tableName := getTableName(models[0])

	// Pega colunas do primeiro modelo
	columns, _, err := getColumnsAndValues(models[0])
	if err != nil {
		return fmt.Errorf("failed to get columns: %w", err)
	}

	// Constrói a query INSERT com múltiplos VALUES
	var sb strings.Builder
	sb.WriteString("INSERT INTO ")
	sb.WriteString(db.dialect.QuoteIdentifier(tableName))
	sb.WriteString(" (")
	sb.WriteString(strings.Join(columns, ", "))
	sb.WriteString(") VALUES ")

	var allArgs []interface{}
	argIndex := 1

	for i, model := range models {
		if i > 0 {
			sb.WriteString(", ")
		}

		_, values, err := getColumnsAndValues(model)
		if err != nil {
			return fmt.Errorf("failed to get values for model %d: %w", i, err)
		}

		sb.WriteString("(")
		placeholders := make([]string, len(columns))
		for j := range columns {
			placeholders[j] = db.dialect.Placeholder(argIndex)
			argIndex++
		}
		sb.WriteString(strings.Join(placeholders, ", "))
		sb.WriteString(")")

		allArgs = append(allArgs, values...)
	}

	// Adiciona RETURNING id para PostgreSQL
	placeholder := db.dialect.Placeholder(1)
	if placeholder == "$1" { // PostgreSQL
		sb.WriteString(" RETURNING id")
	}

	query := sb.String()

	start := time.Now()

	// Para PostgreSQL, usa QueryContext para pegar IDs retornados
	if placeholder == "$1" {
		rows, err := db.executor.QueryContext(ctx, query, allArgs...)
		if err != nil {
			db.logger.LogError(query, allArgs, err)
			return fmt.Errorf("failed to batch insert: %w", err)
		}
		defer rows.Close()

		// Define IDs nos modelos
		idx := 0
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				return fmt.Errorf("failed to scan returned id: %w", err)
			}
			if idx < len(models) {
				setID(models[idx], id)
			}
			idx++
		}

		if err := rows.Err(); err != nil {
			return fmt.Errorf("rows error: %w", err)
		}
	} else {
		// Para MySQL/SQLite, usa ExecContext
		result, err := db.executor.ExecContext(ctx, query, allArgs...)
		if err != nil {
			db.logger.LogError(query, allArgs, err)
			return fmt.Errorf("failed to batch insert: %w", err)
		}

		// Tenta definir IDs se o driver suportar LastInsertId
		lastID, err := result.LastInsertId()
		if err == nil && lastID > 0 {
			// MySQL retorna o primeiro ID inserido
			for i := range models {
				setID(models[i], lastID+int64(i))
			}
		}
	}

	duration := time.Since(start).Nanoseconds()
	db.logger.LogQuery(query, allArgs, duration)

	// Executa hooks AfterCreate e AfterSave
	if !config.SkipHooks {
		for _, model := range models {
			if ac, ok := model.(AfterCreater); ok {
				if err := ac.AfterCreate(); err != nil {
					return fmt.Errorf("AfterCreate hook failed: %w", err)
				}
			}
			if as, ok := model.(AfterSaver); ok {
				if err := as.AfterSave(); err != nil {
					return fmt.Errorf("AfterSave hook failed: %w", err)
				}
			}
		}
	}

	return nil
}

// BatchUpdate atualiza múltiplos registros.
// Usa uma transação para garantir atomicidade.
// Os modelos devem ter IDs não-zero.
//
// Exemplo:
//
//	users := []*User{{ID: 1, Name: "Alice"}, {ID: 2, Name: "Bob"}}
//	err := db.BatchUpdate(ctx, users)
func (db *DB) BatchUpdate(ctx context.Context, models interface{}) error {
	return db.BatchUpdateWithConfig(ctx, models, DefaultBatchConfig())
}

// BatchUpdateWithConfig atualiza múltiplos registros com configuração personalizada.
func (db *DB) BatchUpdateWithConfig(ctx context.Context, models interface{}, config BatchConfig) error {
	slice := reflect.ValueOf(models)
	if slice.Kind() != reflect.Slice {
		return fmt.Errorf("models must be a slice")
	}

	if slice.Len() == 0 {
		return nil
	}

	// Usa transação se o executor for *sql.DB
	sqlDB, ok := db.executor.(*sql.DB)
	if ok {
		return db.batchUpdateWithTx(ctx, sqlDB, slice, config)
	}

	// Se já está em transação, executa diretamente
	return db.batchUpdateDirect(ctx, slice, config)
}

func (db *DB) batchUpdateWithTx(ctx context.Context, sqlDB *sql.DB, slice reflect.Value, config BatchConfig) error {
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	txDB := &DB{
		executor: tx,
		dialect:  db.dialect,
		logger:   db.logger,
	}

	if err := txDB.batchUpdateDirect(ctx, slice, config); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("rollback failed: %v (original error: %w)", rbErr, err)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit failed: %w", err)
	}

	return nil
}

func (db *DB) batchUpdateDirect(ctx context.Context, slice reflect.Value, config BatchConfig) error {
	for i := 0; i < slice.Len(); i++ {
		model := slice.Index(i).Interface()

		// Executa hooks se não estiverem desabilitados
		if !config.SkipHooks {
			if bs, ok := model.(BeforeSaver); ok {
				if err := bs.BeforeSave(); err != nil {
					return fmt.Errorf("BeforeSave hook failed at index %d: %w", i, err)
				}
			}
			if bu, ok := model.(BeforeUpdater); ok {
				if err := bu.BeforeUpdate(); err != nil {
					return fmt.Errorf("BeforeUpdate hook failed at index %d: %w", i, err)
				}
			}
		}

		// Atualiza updated_at
		setUpdatedAt(model)

		tableName := getTableName(model)
		id := getID(model)

		if id == 0 {
			return fmt.Errorf("model at index %d has zero ID", i)
		}

		columns, values, err := getColumnsAndValues(model)
		if err != nil {
			return fmt.Errorf("failed to get columns at index %d: %w", i, err)
		}

		// Remove 'id' das colunas
		filteredCols := []string{}
		filteredVals := []interface{}{}
		for j, col := range columns {
			if col != "id" {
				filteredCols = append(filteredCols, col)
				filteredVals = append(filteredVals, values[j])
			}
		}

		// Constrói SET clause
		setParts := make([]string, len(filteredCols))
		for j, col := range filteredCols {
			setParts[j] = fmt.Sprintf("%s = %s", col, db.dialect.Placeholder(j+1))
		}

		filteredVals = append(filteredVals, id)

		query := fmt.Sprintf(
			"UPDATE %s SET %s WHERE id = %s",
			db.dialect.QuoteIdentifier(tableName),
			strings.Join(setParts, ", "),
			db.dialect.Placeholder(len(filteredVals)),
		)

		start := time.Now()
		_, err = db.executor.ExecContext(ctx, query, filteredVals...)
		duration := time.Since(start).Nanoseconds()

		if err != nil {
			db.logger.LogError(query, filteredVals, err)
			return fmt.Errorf("failed to update model at index %d: %w", i, err)
		}

		db.logger.LogQuery(query, filteredVals, duration)

		// Executa hooks após update
		if !config.SkipHooks {
			if au, ok := model.(AfterUpdater); ok {
				if err := au.AfterUpdate(); err != nil {
					return fmt.Errorf("AfterUpdate hook failed at index %d: %w", i, err)
				}
			}
			if as, ok := model.(AfterSaver); ok {
				if err := as.AfterSave(); err != nil {
					return fmt.Errorf("AfterSave hook failed at index %d: %w", i, err)
				}
			}
		}
	}

	return nil
}

// BatchDelete remove múltiplos registros em uma única query.
// Usa DELETE FROM ... WHERE id IN (...).
// Os modelos devem ter IDs não-zero.
//
// Exemplo:
//
//	users := []*User{{ID: 1}, {ID: 2}, {ID: 3}}
//	err := db.BatchDelete(ctx, users)
func (db *DB) BatchDelete(ctx context.Context, models interface{}) error {
	return db.BatchDeleteWithConfig(ctx, models, DefaultBatchConfig())
}

// BatchDeleteWithConfig remove múltiplos registros com configuração personalizada.
func (db *DB) BatchDeleteWithConfig(ctx context.Context, models interface{}, config BatchConfig) error {
	slice := reflect.ValueOf(models)
	if slice.Kind() != reflect.Slice {
		return fmt.Errorf("models must be a slice")
	}

	if slice.Len() == 0 {
		return nil
	}

	// Executa hooks BeforeDelete
	if !config.SkipHooks {
		for i := 0; i < slice.Len(); i++ {
			model := slice.Index(i).Interface()
			if bd, ok := model.(BeforeDeleter); ok {
				if err := bd.BeforeDelete(); err != nil {
					return fmt.Errorf("BeforeDelete hook failed at index %d: %w", i, err)
				}
			}
		}
	}

	// Coleta IDs e verifica soft delete
	firstModel := slice.Index(0).Interface()
	tableName := getTableName(firstModel)

	ids := make([]interface{}, slice.Len())
	isSoftDeletable := false

	for i := 0; i < slice.Len(); i++ {
		model := slice.Index(i).Interface()
		id := getID(model)
		if id == 0 {
			return fmt.Errorf("model at index %d has zero ID", i)
		}
		ids[i] = id

		// Verifica se é soft deletable
		if _, ok := model.(SoftDeletable); ok {
			isSoftDeletable = true
		}
	}

	var query string
	var args []interface{}

	if isSoftDeletable {
		// Soft delete: UPDATE SET deleted_at = NOW() WHERE id IN (...)
		now := time.Now()
		placeholders := make([]string, len(ids))
		for i := range ids {
			placeholders[i] = db.dialect.Placeholder(i + 2) // +2 porque $1 é para deleted_at
		}

		query = fmt.Sprintf(
			"UPDATE %s SET deleted_at = %s WHERE id IN (%s)",
			db.dialect.QuoteIdentifier(tableName),
			db.dialect.Placeholder(1),
			strings.Join(placeholders, ", "),
		)

		args = append([]interface{}{now}, ids...)

		// Atualiza os modelos com deleted_at
		for i := 0; i < slice.Len(); i++ {
			model := slice.Index(i).Interface()
			if sd, ok := model.(SoftDeletable); ok {
				sd.SetDeletedAt(&now)
			}
		}
	} else {
		// Hard delete: DELETE FROM ... WHERE id IN (...)
		placeholders := make([]string, len(ids))
		for i := range ids {
			placeholders[i] = db.dialect.Placeholder(i + 1)
		}

		query = fmt.Sprintf(
			"DELETE FROM %s WHERE id IN (%s)",
			db.dialect.QuoteIdentifier(tableName),
			strings.Join(placeholders, ", "),
		)

		args = ids
	}

	start := time.Now()
	result, err := db.executor.ExecContext(ctx, query, args...)
	duration := time.Since(start).Nanoseconds()

	if err != nil {
		db.logger.LogError(query, args, err)
		return fmt.Errorf("failed to batch delete: %w", err)
	}

	db.logger.LogQuery(query, args, duration)

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("no rows deleted")
	}

	// Executa hooks AfterDelete
	if !config.SkipHooks {
		for i := 0; i < slice.Len(); i++ {
			model := slice.Index(i).Interface()
			if ad, ok := model.(AfterDeleter); ok {
				if err := ad.AfterDelete(); err != nil {
					return fmt.Errorf("AfterDelete hook failed at index %d: %w", i, err)
				}
			}
		}
	}

	return nil
}

// BatchDeleteByIDs remove múltiplos registros pelos IDs.
// Mais eficiente que BatchDelete quando você só tem os IDs.
//
// Exemplo:
//
//	err := db.BatchDeleteByIDs(ctx, "users", []int64{1, 2, 3})
func (db *DB) BatchDeleteByIDs(ctx context.Context, tableName string, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}

	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = db.dialect.Placeholder(i + 1)
		args[i] = id
	}

	query := fmt.Sprintf(
		"DELETE FROM %s WHERE id IN (%s)",
		db.dialect.QuoteIdentifier(tableName),
		strings.Join(placeholders, ", "),
	)

	start := time.Now()
	result, err := db.executor.ExecContext(ctx, query, args...)
	duration := time.Since(start).Nanoseconds()

	if err != nil {
		db.logger.LogError(query, args, err)
		return fmt.Errorf("failed to batch delete by IDs: %w", err)
	}

	db.logger.LogQuery(query, args, duration)

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("no rows deleted")
	}

	return nil
}
