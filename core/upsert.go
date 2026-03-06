package core

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"
)

// UpsertConfig configura operações de upsert.
type UpsertConfig struct {
	// ConflictColumns são as colunas que definem o conflito.
	// Se vazio, usa a primary key (id).
	ConflictColumns []string

	// UpdateColumns são as colunas a atualizar em caso de conflito.
	// Se vazio, atualiza todas as colunas exceto as de conflito.
	UpdateColumns []string

	// DoNothing indica se deve ignorar conflitos (INSERT ... ON CONFLICT DO NOTHING).
	DoNothing bool

	// UpdateWhere adiciona uma condição WHERE ao UPDATE.
	// Útil para updates condicionais (ex: só atualiza se versão for maior).
	UpdateWhere string

	// UpdateWhereArgs são os argumentos para UpdateWhere.
	UpdateWhereArgs []interface{}
}

// Upsert insere um registro ou atualiza se já existir.
// Usa INSERT ... ON CONFLICT para PostgreSQL e INSERT ... ON DUPLICATE KEY para MySQL.
//
// Exemplo:
//
//	user := &User{Email: "test@example.com", Name: "Test"}
//	err := db.Upsert(ctx, user, core.UpsertConfig{
//	    ConflictColumns: []string{"email"},
//	    UpdateColumns:   []string{"name", "updated_at"},
//	})
func (db *DB) Upsert(ctx context.Context, model interface{}) error {
	return db.UpsertWithConfig(ctx, model, UpsertConfig{})
}

// UpsertWithConfig insere ou atualiza com configuração personalizada.
func (db *DB) UpsertWithConfig(ctx context.Context, model interface{}, config UpsertConfig) error {
	start := time.Now()

	v := reflect.ValueOf(model)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	_ = v

	tableName := getTableName(model)
	columns, values, err := getColumnsAndValues(model)
	if err != nil {
		return err
	}

	query, args := db.buildUpsertQuery(tableName, columns, values, config)

	result, err := db.executor.ExecContext(ctx, query, args...)
	if err != nil {
		if db.logger != nil {
			db.logger.LogError(query, args, err)
		}
		return err
	}

	if db.logger != nil {
		db.logger.LogQuery(query, args, time.Since(start).Milliseconds())
	}

	// Tenta obter o ID inserido
	if lastID, err := result.LastInsertId(); err == nil && lastID > 0 {
		setID(model, lastID)
	}

	return nil
}

// buildUpsertQuery constrói a query de upsert específica para cada dialeto.
func (db *DB) buildUpsertQuery(tableName string, columns []string, values []interface{}, config UpsertConfig) (string, []interface{}) {
	// Detecta o dialeto
	placeholder := db.dialect.Placeholder(1)

	if placeholder == "?" {
		// MySQL: INSERT ... ON DUPLICATE KEY UPDATE
		return db.buildMySQLUpsert(tableName, columns, values, config)
	}

	// PostgreSQL/SQLite: INSERT ... ON CONFLICT
	return db.buildPostgresUpsert(tableName, columns, values, config)
}

// buildPostgresUpsert constrói upsert para PostgreSQL.
func (db *DB) buildPostgresUpsert(tableName string, columns []string, values []interface{}, config UpsertConfig) (string, []interface{}) {
	var sql strings.Builder

	// INSERT INTO table (columns) VALUES (...)
	sql.WriteString("INSERT INTO ")
	sql.WriteString(db.dialect.QuoteIdentifier(tableName))
	sql.WriteString(" (")

	quotedColumns := make([]string, len(columns))
	placeholders := make([]string, len(columns))
	for i, col := range columns {
		quotedColumns[i] = db.dialect.QuoteIdentifier(col)
		placeholders[i] = db.dialect.Placeholder(i + 1)
	}

	sql.WriteString(strings.Join(quotedColumns, ", "))
	sql.WriteString(") VALUES (")
	sql.WriteString(strings.Join(placeholders, ", "))
	sql.WriteString(")")

	// ON CONFLICT
	sql.WriteString(" ON CONFLICT ")

	conflictCols := config.ConflictColumns
	if len(conflictCols) == 0 {
		conflictCols = []string{"id"}
	}

	sql.WriteString("(")
	quotedConflict := make([]string, len(conflictCols))
	for i, col := range conflictCols {
		quotedConflict[i] = db.dialect.QuoteIdentifier(col)
	}
	sql.WriteString(strings.Join(quotedConflict, ", "))
	sql.WriteString(")")

	if config.DoNothing {
		sql.WriteString(" DO NOTHING")
	} else {
		sql.WriteString(" DO UPDATE SET ")

		// Determina colunas para update
		updateCols := config.UpdateColumns
		if len(updateCols) == 0 {
			// Atualiza todas exceto as de conflito
			conflictMap := make(map[string]bool)
			for _, c := range conflictCols {
				conflictMap[c] = true
			}
			for _, col := range columns {
				if !conflictMap[col] && col != "id" && col != "created_at" {
					updateCols = append(updateCols, col)
				}
			}
		}

		updateParts := make([]string, len(updateCols))
		for i, col := range updateCols {
			updateParts[i] = fmt.Sprintf("%s = EXCLUDED.%s",
				db.dialect.QuoteIdentifier(col),
				db.dialect.QuoteIdentifier(col))
		}
		sql.WriteString(strings.Join(updateParts, ", "))

		// WHERE condition for update
		if config.UpdateWhere != "" {
			sql.WriteString(" WHERE ")
			sql.WriteString(config.UpdateWhere)
			values = append(values, config.UpdateWhereArgs...)
		}
	}

	return sql.String(), values
}

// buildMySQLUpsert constrói upsert para MySQL.
func (db *DB) buildMySQLUpsert(tableName string, columns []string, values []interface{}, config UpsertConfig) (string, []interface{}) {
	var sql strings.Builder

	// INSERT INTO table (columns) VALUES (...)
	sql.WriteString("INSERT INTO ")
	sql.WriteString(db.dialect.QuoteIdentifier(tableName))
	sql.WriteString(" (")

	quotedColumns := make([]string, len(columns))
	placeholders := make([]string, len(columns))
	for i, col := range columns {
		quotedColumns[i] = db.dialect.QuoteIdentifier(col)
		placeholders[i] = "?"
	}

	sql.WriteString(strings.Join(quotedColumns, ", "))
	sql.WriteString(") VALUES (")
	sql.WriteString(strings.Join(placeholders, ", "))
	sql.WriteString(")")

	if config.DoNothing {
		// MySQL: usar INSERT IGNORE ou ON DUPLICATE KEY UPDATE com valores iguais
		sql.Reset()
		sql.WriteString("INSERT IGNORE INTO ")
		sql.WriteString(db.dialect.QuoteIdentifier(tableName))
		sql.WriteString(" (")
		sql.WriteString(strings.Join(quotedColumns, ", "))
		sql.WriteString(") VALUES (")
		sql.WriteString(strings.Join(placeholders, ", "))
		sql.WriteString(")")
	} else {
		sql.WriteString(" ON DUPLICATE KEY UPDATE ")

		// Determina colunas para update
		conflictCols := config.ConflictColumns
		if len(conflictCols) == 0 {
			conflictCols = []string{"id"}
		}

		updateCols := config.UpdateColumns
		if len(updateCols) == 0 {
			conflictMap := make(map[string]bool)
			for _, c := range conflictCols {
				conflictMap[c] = true
			}
			for _, col := range columns {
				if !conflictMap[col] && col != "id" && col != "created_at" {
					updateCols = append(updateCols, col)
				}
			}
		}

		updateParts := make([]string, len(updateCols))
		for i, col := range updateCols {
			updateParts[i] = fmt.Sprintf("%s = VALUES(%s)",
				db.dialect.QuoteIdentifier(col),
				db.dialect.QuoteIdentifier(col))
		}
		sql.WriteString(strings.Join(updateParts, ", "))
	}

	return sql.String(), values
}

// BatchUpsert executa upsert em lote.
func (db *DB) BatchUpsert(ctx context.Context, models interface{}) error {
	return db.BatchUpsertWithConfig(ctx, models, UpsertConfig{}, BatchConfig{})
}

// BatchUpsertWithConfig executa upsert em lote com configuração.
func (db *DB) BatchUpsertWithConfig(ctx context.Context, models interface{}, upsertConfig UpsertConfig, batchConfig BatchConfig) error {
	val := reflect.ValueOf(models)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Slice {
		return fmt.Errorf("models must be a slice")
	}

	if val.Len() == 0 {
		return nil
	}

	batchSize := batchConfig.BatchSize
	if batchSize <= 0 {
		batchSize = 100
	}

	// Processa em batches
	for i := 0; i < val.Len(); i += batchSize {
		end := i + batchSize
		if end > val.Len() {
			end = val.Len()
		}

		batch := val.Slice(i, end)
		if err := db.executeBatchUpsert(ctx, batch, upsertConfig); err != nil {
			return err
		}
	}

	return nil
}

// executeBatchUpsert executa um batch de upserts.
func (db *DB) executeBatchUpsert(ctx context.Context, batch reflect.Value, config UpsertConfig) error {
	if batch.Len() == 0 {
		return nil
	}

	// Obtém metadata do primeiro item
	first := batch.Index(0)
	if first.Kind() == reflect.Ptr {
		first = first.Elem()
	}

	tableName := db.getTableNameFromValue(first)
	columns := db.getColumnNames(first)

	// Detecta dialeto
	placeholder := db.dialect.Placeholder(1)
	isMySQL := placeholder == "?"

	var sql strings.Builder
	var args []interface{}

	// Constrói INSERT
	sql.WriteString("INSERT INTO ")
	sql.WriteString(db.dialect.QuoteIdentifier(tableName))
	sql.WriteString(" (")

	quotedColumns := make([]string, len(columns))
	for i, col := range columns {
		quotedColumns[i] = db.dialect.QuoteIdentifier(col)
	}
	sql.WriteString(strings.Join(quotedColumns, ", "))
	sql.WriteString(") VALUES ")

	// Valores para cada item
	valueSets := make([]string, batch.Len())
	argIndex := 1

	for i := 0; i < batch.Len(); i++ {
		item := batch.Index(i)
		if item.Kind() == reflect.Ptr {
			item = item.Elem()
		}

		_, values, _ := getColumnsAndValuesFromValue(item)
		placeholders := make([]string, len(values))

		for j, v := range values {
			if isMySQL {
				placeholders[j] = "?"
			} else {
				placeholders[j] = db.dialect.Placeholder(argIndex)
				argIndex++
			}
			args = append(args, v)
		}

		valueSets[i] = "(" + strings.Join(placeholders, ", ") + ")"
	}

	sql.WriteString(strings.Join(valueSets, ", "))

	// ON CONFLICT / ON DUPLICATE KEY
	if isMySQL {
		if config.DoNothing {
			// Reescreve como INSERT IGNORE
			sql.Reset()
			sql.WriteString("INSERT IGNORE INTO ")
			sql.WriteString(db.dialect.QuoteIdentifier(tableName))
			sql.WriteString(" (")
			sql.WriteString(strings.Join(quotedColumns, ", "))
			sql.WriteString(") VALUES ")
			sql.WriteString(strings.Join(valueSets, ", "))
		} else {
			sql.WriteString(" ON DUPLICATE KEY UPDATE ")
			updateCols := db.getUpdateColumns(columns, config)
			updateParts := make([]string, len(updateCols))
			for i, col := range updateCols {
				updateParts[i] = fmt.Sprintf("%s = VALUES(%s)",
					db.dialect.QuoteIdentifier(col),
					db.dialect.QuoteIdentifier(col))
			}
			sql.WriteString(strings.Join(updateParts, ", "))
		}
	} else {
		// PostgreSQL
		conflictCols := config.ConflictColumns
		if len(conflictCols) == 0 {
			conflictCols = []string{"id"}
		}

		sql.WriteString(" ON CONFLICT (")
		quotedConflict := make([]string, len(conflictCols))
		for i, col := range conflictCols {
			quotedConflict[i] = db.dialect.QuoteIdentifier(col)
		}
		sql.WriteString(strings.Join(quotedConflict, ", "))
		sql.WriteString(")")

		if config.DoNothing {
			sql.WriteString(" DO NOTHING")
		} else {
			sql.WriteString(" DO UPDATE SET ")
			updateCols := db.getUpdateColumns(columns, config)
			updateParts := make([]string, len(updateCols))
			for i, col := range updateCols {
				updateParts[i] = fmt.Sprintf("%s = EXCLUDED.%s",
					db.dialect.QuoteIdentifier(col),
					db.dialect.QuoteIdentifier(col))
			}
			sql.WriteString(strings.Join(updateParts, ", "))
		}
	}

	start := time.Now()
	_, err := db.executor.ExecContext(ctx, sql.String(), args...)
	if err != nil {
		if db.logger != nil {
			db.logger.LogError(sql.String(), args, err)
		}
		return err
	}

	if db.logger != nil {
		db.logger.LogQuery(sql.String(), args, time.Since(start).Milliseconds())
	}

	return nil
}

// getUpdateColumns retorna as colunas a serem atualizadas.
func (db *DB) getUpdateColumns(columns []string, config UpsertConfig) []string {
	if len(config.UpdateColumns) > 0 {
		return config.UpdateColumns
	}

	conflictMap := make(map[string]bool)
	for _, c := range config.ConflictColumns {
		conflictMap[c] = true
	}
	conflictMap["id"] = true
	conflictMap["created_at"] = true

	var updateCols []string
	for _, col := range columns {
		if !conflictMap[col] {
			updateCols = append(updateCols, col)
		}
	}

	return updateCols
}

// getTableNameFromValue obtém o nome da tabela de um reflect.Value.
func (db *DB) getTableNameFromValue(val reflect.Value) string {
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	// Verifica se implementa TableNamer
	if val.CanAddr() {
		if tn, ok := val.Addr().Interface().(TableNamer); ok {
			return tn.TableName()
		}
	}

	// Fallback: converte nome do tipo para snake_case
	return toSnakeCase(val.Type().Name())
}

// getColumnsAndValuesFromValue extrai colunas e valores de um reflect.Value.
func getColumnsAndValuesFromValue(val reflect.Value) ([]string, []interface{}, error) {
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	var columns []string
	var values []interface{}
	typ := val.Type()

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		fieldVal := val.Field(i)

		// Pula campos não exportados
		if !field.IsExported() {
			continue
		}

		// Verifica tag db
		dbTag := field.Tag.Get("db")
		if dbTag == "-" || dbTag == "" {
			// Embedded structs
			if field.Anonymous && fieldVal.Kind() == reflect.Struct {
				embeddedCols, embeddedVals, _ := getColumnsAndValuesFromValue(fieldVal)
				columns = append(columns, embeddedCols...)
				values = append(values, embeddedVals...)
			}
			continue
		}

		// Pula ID se for zero (será gerado pelo banco)
		if dbTag == "id" && fieldVal.IsZero() {
			continue
		}

		columns = append(columns, dbTag)
		values = append(values, fieldVal.Interface())
	}

	return columns, values, nil
}

// getColumnNames retorna os nomes das colunas de um struct.
func (db *DB) getColumnNames(val reflect.Value) []string {
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	var columns []string
	typ := val.Type()

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		// Pula campos não exportados
		if !field.IsExported() {
			continue
		}

		// Verifica tag db
		dbTag := field.Tag.Get("db")
		if dbTag == "-" {
			continue
		}

		// Embedded structs
		if field.Anonymous {
			embeddedVal := val.Field(i)
			if embeddedVal.Kind() == reflect.Struct {
				columns = append(columns, db.getColumnNames(embeddedVal)...)
			}
			continue
		}

		if dbTag != "" {
			columns = append(columns, dbTag)
		}
	}

	return columns
}
