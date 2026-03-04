package core

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"time"
)

// DB é a estrutura principal do ORM. Usa generics para type-safety.
type DB struct {
	executor Executor
	dialect  Dialect
	logger   Logger
}

// New cria uma nova instância do Genus DB com logging padrão.
func New(sqlDB *sql.DB, dialect Dialect) *DB {
	return &DB{
		executor: sqlDB,
		dialect:  dialect,
		logger:   NewDefaultLogger(false), // logging não-verbose por padrão
	}
}

// NewWithLogger cria uma nova instância do Genus DB com um logger customizado.
func NewWithLogger(sqlDB *sql.DB, dialect Dialect, logger Logger) *DB {
	return &DB{
		executor: sqlDB,
		dialect:  dialect,
		logger:   logger,
	}
}

// NewWithExecutor cria uma nova instância do Genus DB com um executor customizado.
// Útil para configurações avançadas como read replicas (MultiExecutor).
//
// Exemplo:
//
//	executor := core.NewMultiExecutor(primary, replica1, replica2)
//	db := core.NewWithExecutor(executor, dialect)
func NewWithExecutor(executor Executor, dialect Dialect) *DB {
	return &DB{
		executor: executor,
		dialect:  dialect,
		logger:   NewDefaultLogger(false),
	}
}

// NewWithExecutorAndLogger cria uma nova instância com executor e logger customizados.
func NewWithExecutorAndLogger(executor Executor, dialect Dialect, logger Logger) *DB {
	return &DB{
		executor: executor,
		dialect:  dialect,
		logger:   logger,
	}
}

// WithTx executa uma função dentro de uma transação.
func (db *DB) WithTx(ctx context.Context, fn func(*DB) error) error {
	sqlDB, ok := db.executor.(*sql.DB)
	if !ok {
		return fmt.Errorf("cannot start transaction: not a *sql.DB")
	}

	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	txDB := &DB{
		executor: tx,
		dialect:  db.dialect,
		logger:   db.logger, // propaga o logger para a transação
	}

	if err := fn(txDB); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("error rolling back transaction: %v (original error: %w)", rbErr, err)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Executor retorna o executor atual (útil para queries customizadas).
func (db *DB) Executor() Executor {
	return db.executor
}

// Dialect retorna o dialeto atual.
func (db *DB) Dialect() Dialect {
	return db.dialect
}

// Logger retorna o logger atual.
func (db *DB) Logger() Logger {
	return db.logger
}

// SetLogger define um novo logger.
func (db *DB) SetLogger(logger Logger) {
	db.logger = logger
}

// Create insere um novo registro no banco de dados.
// T deve ter embedded Model ou implementar TableNamer.
func (db *DB) Create(ctx context.Context, model interface{}) error {
	// Hook BeforeSave
	if bs, ok := model.(BeforeSaver); ok {
		if err := bs.BeforeSave(); err != nil {
			return fmt.Errorf("BeforeSave hook failed: %w", err)
		}
	}

	// Hook BeforeCreate
	if bc, ok := model.(BeforeCreater); ok {
		if err := bc.BeforeCreate(); err != nil {
			return fmt.Errorf("BeforeCreate hook failed: %w", err)
		}
	}

	tableName := getTableName(model)

	// Preenche timestamps se for Model
	setTimestamps(model)

	columns, values, err := getColumnsAndValues(model)
	if err != nil {
		return fmt.Errorf("failed to get columns and values: %w", err)
	}

	// Constrói a query INSERT
	placeholders := make([]string, len(columns))
	for i := range columns {
		placeholders[i] = db.dialect.Placeholder(i + 1)
	}

	query := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s) RETURNING id",
		db.dialect.QuoteIdentifier(tableName),
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
	)

	// Executa e pega o ID retornado
	var id int64
	start := time.Now()
	err = db.executor.QueryRowContext(ctx, query, values...).Scan(&id)
	duration := time.Since(start).Nanoseconds()

	if err != nil {
		db.logger.LogError(query, values, err)
		return fmt.Errorf("failed to insert: %w", err)
	}

	db.logger.LogQuery(query, values, duration)

	// Define o ID no modelo
	setID(model, id)

	// Hook AfterCreate
	if ac, ok := model.(AfterCreater); ok {
		if err := ac.AfterCreate(); err != nil {
			return fmt.Errorf("AfterCreate hook failed: %w", err)
		}
	}

	// Hook AfterSave
	if as, ok := model.(AfterSaver); ok {
		if err := as.AfterSave(); err != nil {
			return fmt.Errorf("AfterSave hook failed: %w", err)
		}
	}

	return nil
}

// Update atualiza um registro existente.
func (db *DB) Update(ctx context.Context, model interface{}) error {
	// Hook BeforeSave
	if bs, ok := model.(BeforeSaver); ok {
		if err := bs.BeforeSave(); err != nil {
			return fmt.Errorf("BeforeSave hook failed: %w", err)
		}
	}

	// Hook BeforeUpdate
	if bu, ok := model.(BeforeUpdater); ok {
		if err := bu.BeforeUpdate(); err != nil {
			return fmt.Errorf("BeforeUpdate hook failed: %w", err)
		}
	}

	tableName := getTableName(model)
	id := getID(model)

	if id == 0 {
		return fmt.Errorf("cannot update model with zero ID")
	}

	// Atualiza updated_at
	setUpdatedAt(model)

	columns, values, err := getColumnsAndValues(model)
	if err != nil {
		return fmt.Errorf("failed to get columns and values: %w", err)
	}

	// Remove 'id' das colunas a serem atualizadas
	filteredCols := []string{}
	filteredVals := []interface{}{}
	for i, col := range columns {
		if col != "id" {
			filteredCols = append(filteredCols, col)
			filteredVals = append(filteredVals, values[i])
		}
	}

	// Constrói SET clause
	setParts := make([]string, len(filteredCols))
	for i, col := range filteredCols {
		setParts[i] = fmt.Sprintf("%s = %s", col, db.dialect.Placeholder(i+1))
	}

	// Adiciona o ID como último parâmetro
	filteredVals = append(filteredVals, id)

	query := fmt.Sprintf(
		"UPDATE %s SET %s WHERE id = %s",
		db.dialect.QuoteIdentifier(tableName),
		strings.Join(setParts, ", "),
		db.dialect.Placeholder(len(filteredVals)),
	)

	start := time.Now()
	result, err := db.executor.ExecContext(ctx, query, filteredVals...)
	duration := time.Since(start).Nanoseconds()

	if err != nil {
		db.logger.LogError(query, filteredVals, err)
		return fmt.Errorf("failed to update: %w", err)
	}

	db.logger.LogQuery(query, filteredVals, duration)

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("no rows updated")
	}

	// Hook AfterUpdate
	if au, ok := model.(AfterUpdater); ok {
		if err := au.AfterUpdate(); err != nil {
			return fmt.Errorf("AfterUpdate hook failed: %w", err)
		}
	}

	// Hook AfterSave
	if as, ok := model.(AfterSaver); ok {
		if err := as.AfterSave(); err != nil {
			return fmt.Errorf("AfterSave hook failed: %w", err)
		}
	}

	return nil
}

// Delete remove um registro do banco de dados.
// Se o model implementa SoftDeletable, realiza soft delete (UPDATE deleted_at).
// Caso contrário, realiza hard delete (DELETE FROM).
func (db *DB) Delete(ctx context.Context, model interface{}) error {
	// Hook BeforeDelete
	if bd, ok := model.(BeforeDeleter); ok {
		if err := bd.BeforeDelete(); err != nil {
			return fmt.Errorf("BeforeDelete hook failed: %w", err)
		}
	}

	// Verifica se é soft deletable
	if sd, ok := model.(SoftDeletable); ok {
		// Soft delete: set deleted_at = NOW()
		now := time.Now()
		sd.SetDeletedAt(&now)

		// Usa Update ao invés de DELETE
		if err := db.Update(ctx, model); err != nil {
			return err
		}

		// Hook AfterDelete
		if ad, ok := model.(AfterDeleter); ok {
			if err := ad.AfterDelete(); err != nil {
				return fmt.Errorf("AfterDelete hook failed: %w", err)
			}
		}

		return nil
	}

	// Hard delete (comportamento original)
	tableName := getTableName(model)
	id := getID(model)

	if id == 0 {
		return fmt.Errorf("cannot delete model with zero ID")
	}

	query := fmt.Sprintf(
		"DELETE FROM %s WHERE id = %s",
		db.dialect.QuoteIdentifier(tableName),
		db.dialect.Placeholder(1),
	)

	start := time.Now()
	result, err := db.executor.ExecContext(ctx, query, id)
	duration := time.Since(start).Nanoseconds()

	if err != nil {
		db.logger.LogError(query, []interface{}{id}, err)
		return fmt.Errorf("failed to delete: %w", err)
	}

	db.logger.LogQuery(query, []interface{}{id}, duration)

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("no rows deleted")
	}

	// Hook AfterDelete
	if ad, ok := model.(AfterDeleter); ok {
		if err := ad.AfterDelete(); err != nil {
			return fmt.Errorf("AfterDelete hook failed: %w", err)
		}
	}

	return nil
}

// ForceDelete remove permanentemente um registro do banco de dados,
// ignorando soft delete mesmo se o model implementa SoftDeletable.
func (db *DB) ForceDelete(ctx context.Context, model interface{}) error {
	// Hook BeforeDelete
	if bd, ok := model.(BeforeDeleter); ok {
		if err := bd.BeforeDelete(); err != nil {
			return fmt.Errorf("BeforeDelete hook failed: %w", err)
		}
	}

	tableName := getTableName(model)
	id := getID(model)

	if id == 0 {
		return fmt.Errorf("cannot delete model with zero ID")
	}

	query := fmt.Sprintf(
		"DELETE FROM %s WHERE id = %s",
		db.dialect.QuoteIdentifier(tableName),
		db.dialect.Placeholder(1),
	)

	start := time.Now()
	result, err := db.executor.ExecContext(ctx, query, id)
	duration := time.Since(start).Nanoseconds()

	if err != nil {
		db.logger.LogError(query, []interface{}{id}, err)
		return fmt.Errorf("failed to delete: %w", err)
	}

	db.logger.LogQuery(query, []interface{}{id}, duration)

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("no rows deleted")
	}

	// Hook AfterDelete
	if ad, ok := model.(AfterDeleter); ok {
		if err := ad.AfterDelete(); err != nil {
			return fmt.Errorf("AfterDelete hook failed: %w", err)
		}
	}

	return nil
}

// Funções auxiliares usando reflection (uso mínimo)

func getTableName(model interface{}) string {
	if tn, ok := model.(TableNamer); ok {
		return tn.TableName()
	}

	// Usa o nome do tipo em snake_case
	t := reflect.TypeOf(model)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return toSnakeCase(t.Name())
}

func getColumnsAndValues(model interface{}) ([]string, []interface{}, error) {
	v := reflect.ValueOf(model)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	t := v.Type()
	var columns []string
	var values []interface{}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		// Pula campos não exportados
		if !fieldValue.CanInterface() {
			continue
		}

		// Se é embedded struct (como Model), processa recursivamente
		if field.Anonymous {
			cols, vals, err := getColumnsAndValues(fieldValue.Addr().Interface())
			if err != nil {
				return nil, nil, err
			}
			columns = append(columns, cols...)
			values = append(values, vals...)
			continue
		}

		// Pega o nome da coluna da tag db
		colName := field.Tag.Get("db")
		if colName == "" {
			colName = toSnakeCase(field.Name)
		}

		columns = append(columns, colName)
		values = append(values, fieldValue.Interface())
	}

	return columns, values, nil
}

func getID(model interface{}) int64 {
	v := reflect.ValueOf(model)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	// Procura por campo ID no modelo ou no embedded Model
	idField := v.FieldByName("ID")
	if !idField.IsValid() {
		return 0
	}

	return idField.Int()
}

func setID(model interface{}, id int64) {
	v := reflect.ValueOf(model)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	idField := v.FieldByName("ID")
	if idField.IsValid() && idField.CanSet() {
		idField.SetInt(id)
	}
}

func setTimestamps(model interface{}) {
	v := reflect.ValueOf(model)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	now := time.Now()

	// CreatedAt
	createdAtField := v.FieldByName("CreatedAt")
	if createdAtField.IsValid() && createdAtField.CanSet() {
		if createdAtField.IsZero() {
			createdAtField.Set(reflect.ValueOf(now))
		}
	}

	// UpdatedAt
	updatedAtField := v.FieldByName("UpdatedAt")
	if updatedAtField.IsValid() && updatedAtField.CanSet() {
		updatedAtField.Set(reflect.ValueOf(now))
	}
}

func setUpdatedAt(model interface{}) {
	v := reflect.ValueOf(model)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	updatedAtField := v.FieldByName("UpdatedAt")
	if updatedAtField.IsValid() && updatedAtField.CanSet() {
		updatedAtField.Set(reflect.ValueOf(time.Now()))
	}
}

func toSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteRune('_')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}
