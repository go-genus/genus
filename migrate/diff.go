package migrate

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-genus/genus/core"
)

// SchemaChange representa uma mudança de schema.
type SchemaChange struct {
	Type        ChangeType
	Table       string
	Column      string
	OldType     string
	NewType     string
	Description string
	SQL         string
	Reversible  bool
	ReverseSQL  string
}

// ChangeType representa o tipo de mudança.
type ChangeType string

const (
	ChangeAddTable       ChangeType = "ADD_TABLE"
	ChangeDropTable      ChangeType = "DROP_TABLE"
	ChangeAddColumn      ChangeType = "ADD_COLUMN"
	ChangeDropColumn     ChangeType = "DROP_COLUMN"
	ChangeModifyColumn   ChangeType = "MODIFY_COLUMN"
	ChangeAddIndex       ChangeType = "ADD_INDEX"
	ChangeDropIndex      ChangeType = "DROP_INDEX"
	ChangeAddForeignKey  ChangeType = "ADD_FOREIGN_KEY"
	ChangeDropForeignKey ChangeType = "DROP_FOREIGN_KEY"
	ChangeAddConstraint  ChangeType = "ADD_CONSTRAINT"
	ChangeDropConstraint ChangeType = "DROP_CONSTRAINT"
)

// TableSchema representa o schema de uma tabela.
type TableSchema struct {
	Name        string
	Columns     []ColumnSchema
	Indexes     []IndexSchema
	ForeignKeys []ForeignKeySchema
	Constraints []ConstraintSchema
}

// ColumnSchema representa o schema de uma coluna.
type ColumnSchema struct {
	Name         string
	Type         string
	Nullable     bool
	Default      string
	PrimaryKey   bool
	AutoIncrement bool
}

// IndexSchema representa o schema de um índice.
type IndexSchema struct {
	Name    string
	Columns []string
	Unique  bool
	Type    string // btree, hash, gin, etc.
}

// ForeignKeySchema representa o schema de uma foreign key.
type ForeignKeySchema struct {
	Name            string
	Columns         []string
	RefTable        string
	RefColumns      []string
	OnDelete        string
	OnUpdate        string
}

// ConstraintSchema representa o schema de uma constraint.
type ConstraintSchema struct {
	Name       string
	Type       string // CHECK, UNIQUE, etc.
	Definition string
}

// SchemaDiffer compara schemas e gera migrações.
type SchemaDiffer struct {
	executor core.Executor
	dialect  core.Dialect
}

// NewSchemaDiffer cria um novo differ de schema.
func NewSchemaDiffer(executor core.Executor, dialect core.Dialect) *SchemaDiffer {
	return &SchemaDiffer{
		executor: executor,
		dialect:  dialect,
	}
}

// GetCurrentSchema obtém o schema atual do banco de dados.
func (d *SchemaDiffer) GetCurrentSchema(ctx context.Context) (map[string]*TableSchema, error) {
	schemas := make(map[string]*TableSchema)

	tables, err := d.getTables(ctx)
	if err != nil {
		return nil, err
	}

	for _, tableName := range tables {
		schema, err := d.getTableSchema(ctx, tableName)
		if err != nil {
			return nil, err
		}
		schemas[tableName] = schema
	}

	return schemas, nil
}

// getTables lista todas as tabelas do banco.
func (d *SchemaDiffer) getTables(ctx context.Context) ([]string, error) {
	var query string

	if d.dialect.Placeholder(1) == "?" {
		// MySQL
		query = `
			SELECT table_name
			FROM information_schema.tables
			WHERE table_schema = DATABASE()
			AND table_type = 'BASE TABLE'
		`
	} else {
		// PostgreSQL
		query = `
			SELECT tablename
			FROM pg_tables
			WHERE schemaname = 'public'
		`
	}

	rows, err := d.executor.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}

	return tables, rows.Err()
}

// getTableSchema obtém o schema de uma tabela.
func (d *SchemaDiffer) getTableSchema(ctx context.Context, tableName string) (*TableSchema, error) {
	schema := &TableSchema{Name: tableName}

	// Obtém colunas
	columns, err := d.getColumns(ctx, tableName)
	if err != nil {
		return nil, err
	}
	schema.Columns = columns

	// Obtém índices
	indexes, err := d.getIndexes(ctx, tableName)
	if err != nil {
		return nil, err
	}
	schema.Indexes = indexes

	// Obtém foreign keys
	fks, err := d.getForeignKeys(ctx, tableName)
	if err != nil {
		return nil, err
	}
	schema.ForeignKeys = fks

	return schema, nil
}

// getColumns obtém as colunas de uma tabela.
func (d *SchemaDiffer) getColumns(ctx context.Context, tableName string) ([]ColumnSchema, error) {
	var query string

	if d.dialect.Placeholder(1) == "?" {
		// MySQL
		query = `
			SELECT
				column_name,
				column_type,
				is_nullable,
				column_default,
				column_key = 'PRI',
				extra LIKE '%auto_increment%'
			FROM information_schema.columns
			WHERE table_schema = DATABASE()
			AND table_name = ?
			ORDER BY ordinal_position
		`
	} else {
		// PostgreSQL
		query = `
			SELECT
				a.attname,
				pg_catalog.format_type(a.atttypid, a.atttypmod),
				NOT a.attnotnull,
				COALESCE(pg_get_expr(d.adbin, d.adrelid), ''),
				COALESCE(pk.contype = 'p', false),
				COALESCE(pg_get_serial_sequence(c.relname::text, a.attname::text) IS NOT NULL, false)
			FROM pg_attribute a
			JOIN pg_class c ON a.attrelid = c.oid
			LEFT JOIN pg_attrdef d ON a.attrelid = d.adrelid AND a.attnum = d.adnum
			LEFT JOIN pg_constraint pk ON pk.conrelid = c.oid AND a.attnum = ANY(pk.conkey) AND pk.contype = 'p'
			WHERE c.relname = $1
			AND a.attnum > 0
			AND NOT a.attisdropped
			ORDER BY a.attnum
		`
	}

	rows, err := d.executor.QueryContext(ctx, query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []ColumnSchema
	for rows.Next() {
		var col ColumnSchema
		var defaultVal *string
		if err := rows.Scan(&col.Name, &col.Type, &col.Nullable, &defaultVal, &col.PrimaryKey, &col.AutoIncrement); err != nil {
			return nil, err
		}
		if defaultVal != nil {
			col.Default = *defaultVal
		}
		columns = append(columns, col)
	}

	return columns, rows.Err()
}

// getIndexes obtém os índices de uma tabela.
func (d *SchemaDiffer) getIndexes(ctx context.Context, tableName string) ([]IndexSchema, error) {
	var query string

	if d.dialect.Placeholder(1) == "?" {
		// MySQL
		query = `
			SELECT
				index_name,
				GROUP_CONCAT(column_name ORDER BY seq_in_index),
				NOT non_unique,
				index_type
			FROM information_schema.statistics
			WHERE table_schema = DATABASE()
			AND table_name = ?
			AND index_name != 'PRIMARY'
			GROUP BY index_name, non_unique, index_type
		`
	} else {
		// PostgreSQL
		query = `
			SELECT
				i.relname,
				array_to_string(array_agg(a.attname ORDER BY k.ord), ','),
				ix.indisunique,
				am.amname
			FROM pg_index ix
			JOIN pg_class t ON t.oid = ix.indrelid
			JOIN pg_class i ON i.oid = ix.indexrelid
			JOIN pg_am am ON am.oid = i.relam
			JOIN LATERAL unnest(ix.indkey) WITH ORDINALITY AS k(attnum, ord) ON TRUE
			JOIN pg_attribute a ON a.attrelid = t.oid AND a.attnum = k.attnum
			WHERE t.relname = $1
			AND NOT ix.indisprimary
			GROUP BY i.relname, ix.indisunique, am.amname
		`
	}

	rows, err := d.executor.QueryContext(ctx, query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var indexes []IndexSchema
	for rows.Next() {
		var idx IndexSchema
		var columnsStr string
		if err := rows.Scan(&idx.Name, &columnsStr, &idx.Unique, &idx.Type); err != nil {
			return nil, err
		}
		idx.Columns = strings.Split(columnsStr, ",")
		indexes = append(indexes, idx)
	}

	return indexes, rows.Err()
}

// getForeignKeys obtém as foreign keys de uma tabela.
func (d *SchemaDiffer) getForeignKeys(ctx context.Context, tableName string) ([]ForeignKeySchema, error) {
	var query string

	if d.dialect.Placeholder(1) == "?" {
		// MySQL
		query = `
			SELECT
				constraint_name,
				column_name,
				referenced_table_name,
				referenced_column_name
			FROM information_schema.key_column_usage
			WHERE table_schema = DATABASE()
			AND table_name = ?
			AND referenced_table_name IS NOT NULL
		`
	} else {
		// PostgreSQL
		query = `
			SELECT
				tc.constraint_name,
				kcu.column_name,
				ccu.table_name AS referenced_table,
				ccu.column_name AS referenced_column
			FROM information_schema.table_constraints tc
			JOIN information_schema.key_column_usage kcu
				ON tc.constraint_name = kcu.constraint_name
			JOIN information_schema.constraint_column_usage ccu
				ON ccu.constraint_name = tc.constraint_name
			WHERE tc.constraint_type = 'FOREIGN KEY'
			AND tc.table_name = $1
		`
	}

	rows, err := d.executor.QueryContext(ctx, query, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	fkMap := make(map[string]*ForeignKeySchema)
	for rows.Next() {
		var name, column, refTable, refColumn string
		if err := rows.Scan(&name, &column, &refTable, &refColumn); err != nil {
			return nil, err
		}

		if fk, exists := fkMap[name]; exists {
			fk.Columns = append(fk.Columns, column)
			fk.RefColumns = append(fk.RefColumns, refColumn)
		} else {
			fkMap[name] = &ForeignKeySchema{
				Name:       name,
				Columns:    []string{column},
				RefTable:   refTable,
				RefColumns: []string{refColumn},
			}
		}
	}

	var fks []ForeignKeySchema
	for _, fk := range fkMap {
		fks = append(fks, *fk)
	}

	return fks, rows.Err()
}

// GetSchemaFromModels gera schema a partir de structs Go.
func (d *SchemaDiffer) GetSchemaFromModels(models ...interface{}) map[string]*TableSchema {
	schemas := make(map[string]*TableSchema)

	for _, model := range models {
		schema := d.modelToSchema(model)
		schemas[schema.Name] = schema
	}

	return schemas
}

// modelToSchema converte um model Go para schema.
func (d *SchemaDiffer) modelToSchema(model interface{}) *TableSchema {
	typ := reflect.TypeOf(model)
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	tableName := toSnakeCaseMigrate(typ.Name()) + "s"

	// Verifica se implementa TableNamer
	if tn, ok := model.(interface{ TableName() string }); ok {
		tableName = tn.TableName()
	}

	schema := &TableSchema{
		Name:    tableName,
		Columns: make([]ColumnSchema, 0),
	}

	d.extractColumns(typ, schema)

	return schema
}

// extractColumns extrai colunas de um tipo.
func (d *SchemaDiffer) extractColumns(typ reflect.Type, schema *TableSchema) {
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		// Processa embedded structs
		if field.Anonymous && field.Type.Kind() == reflect.Struct {
			d.extractColumns(field.Type, schema)
			continue
		}

		if !field.IsExported() {
			continue
		}

		dbTag := field.Tag.Get("db")
		if dbTag == "-" || dbTag == "" {
			continue
		}

		col := ColumnSchema{
			Name:     dbTag,
			Type:     d.goTypeToSQLType(field.Type),
			Nullable: field.Type.Kind() == reflect.Ptr,
		}

		if dbTag == "id" {
			col.PrimaryKey = true
			col.AutoIncrement = true
		}

		schema.Columns = append(schema.Columns, col)
	}
}

// goTypeToSQLType converte tipo Go para SQL.
func (d *SchemaDiffer) goTypeToSQLType(typ reflect.Type) string {
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	return d.dialect.GetType(typ.String())
}

// Diff compara dois schemas e retorna as mudanças.
func (d *SchemaDiffer) Diff(current, target map[string]*TableSchema) []SchemaChange {
	var changes []SchemaChange

	// Tabelas para adicionar
	for name, targetTable := range target {
		if _, exists := current[name]; !exists {
			changes = append(changes, d.createTableChange(targetTable))
		}
	}

	// Tabelas para remover
	for name, currentTable := range current {
		if _, exists := target[name]; !exists {
			changes = append(changes, d.dropTableChange(currentTable))
		}
	}

	// Comparar colunas de tabelas existentes
	for name, targetTable := range target {
		if currentTable, exists := current[name]; exists {
			changes = append(changes, d.diffColumns(currentTable, targetTable)...)
			changes = append(changes, d.diffIndexes(currentTable, targetTable)...)
			changes = append(changes, d.diffForeignKeys(currentTable, targetTable)...)
		}
	}

	return changes
}

// createTableChange cria mudança para adicionar tabela.
func (d *SchemaDiffer) createTableChange(table *TableSchema) SchemaChange {
	var sb strings.Builder

	sb.WriteString("CREATE TABLE ")
	sb.WriteString(d.dialect.QuoteIdentifier(table.Name))
	sb.WriteString(" (\n")

	for i, col := range table.Columns {
		sb.WriteString("  ")
		sb.WriteString(d.dialect.QuoteIdentifier(col.Name))
		sb.WriteString(" ")
		sb.WriteString(col.Type)

		if !col.Nullable {
			sb.WriteString(" NOT NULL")
		}
		if col.PrimaryKey {
			sb.WriteString(" PRIMARY KEY")
		}
		if col.AutoIncrement {
			if d.dialect.Placeholder(1) == "?" {
				sb.WriteString(" AUTO_INCREMENT")
			}
		}
		if col.Default != "" {
			sb.WriteString(" DEFAULT ")
			sb.WriteString(col.Default)
		}

		if i < len(table.Columns)-1 {
			sb.WriteString(",")
		}
		sb.WriteString("\n")
	}

	sb.WriteString(")")

	return SchemaChange{
		Type:        ChangeAddTable,
		Table:       table.Name,
		Description: fmt.Sprintf("Create table %s", table.Name),
		SQL:         sb.String(),
		Reversible:  true,
		ReverseSQL:  fmt.Sprintf("DROP TABLE %s", d.dialect.QuoteIdentifier(table.Name)),
	}
}

// dropTableChange cria mudança para remover tabela.
func (d *SchemaDiffer) dropTableChange(table *TableSchema) SchemaChange {
	return SchemaChange{
		Type:        ChangeDropTable,
		Table:       table.Name,
		Description: fmt.Sprintf("Drop table %s", table.Name),
		SQL:         fmt.Sprintf("DROP TABLE %s", d.dialect.QuoteIdentifier(table.Name)),
		Reversible:  false,
	}
}

// diffColumns compara colunas entre duas tabelas.
func (d *SchemaDiffer) diffColumns(current, target *TableSchema) []SchemaChange {
	var changes []SchemaChange

	currentCols := make(map[string]ColumnSchema)
	for _, col := range current.Columns {
		currentCols[col.Name] = col
	}

	targetCols := make(map[string]ColumnSchema)
	for _, col := range target.Columns {
		targetCols[col.Name] = col
	}

	// Colunas para adicionar
	for name, col := range targetCols {
		if _, exists := currentCols[name]; !exists {
			changes = append(changes, d.addColumnChange(target.Name, col))
		}
	}

	// Colunas para remover
	for name, col := range currentCols {
		if _, exists := targetCols[name]; !exists {
			changes = append(changes, d.dropColumnChange(target.Name, col))
		}
	}

	// Colunas para modificar
	for name, targetCol := range targetCols {
		if currentCol, exists := currentCols[name]; exists {
			if d.columnChanged(currentCol, targetCol) {
				changes = append(changes, d.modifyColumnChange(target.Name, currentCol, targetCol))
			}
		}
	}

	return changes
}

// addColumnChange cria mudança para adicionar coluna.
func (d *SchemaDiffer) addColumnChange(table string, col ColumnSchema) SchemaChange {
	var sb strings.Builder
	sb.WriteString("ALTER TABLE ")
	sb.WriteString(d.dialect.QuoteIdentifier(table))
	sb.WriteString(" ADD COLUMN ")
	sb.WriteString(d.dialect.QuoteIdentifier(col.Name))
	sb.WriteString(" ")
	sb.WriteString(col.Type)

	if !col.Nullable {
		sb.WriteString(" NOT NULL")
	}
	if col.Default != "" {
		sb.WriteString(" DEFAULT ")
		sb.WriteString(col.Default)
	}

	return SchemaChange{
		Type:        ChangeAddColumn,
		Table:       table,
		Column:      col.Name,
		NewType:     col.Type,
		Description: fmt.Sprintf("Add column %s.%s", table, col.Name),
		SQL:         sb.String(),
		Reversible:  true,
		ReverseSQL:  fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", d.dialect.QuoteIdentifier(table), d.dialect.QuoteIdentifier(col.Name)),
	}
}

// dropColumnChange cria mudança para remover coluna.
func (d *SchemaDiffer) dropColumnChange(table string, col ColumnSchema) SchemaChange {
	return SchemaChange{
		Type:        ChangeDropColumn,
		Table:       table,
		Column:      col.Name,
		OldType:     col.Type,
		Description: fmt.Sprintf("Drop column %s.%s", table, col.Name),
		SQL:         fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", d.dialect.QuoteIdentifier(table), d.dialect.QuoteIdentifier(col.Name)),
		Reversible:  false,
	}
}

// modifyColumnChange cria mudança para modificar coluna.
func (d *SchemaDiffer) modifyColumnChange(table string, old, new ColumnSchema) SchemaChange {
	var sql string
	if d.dialect.Placeholder(1) == "?" {
		// MySQL
		sql = fmt.Sprintf("ALTER TABLE %s MODIFY COLUMN %s %s",
			d.dialect.QuoteIdentifier(table),
			d.dialect.QuoteIdentifier(new.Name),
			new.Type)
	} else {
		// PostgreSQL
		sql = fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s TYPE %s",
			d.dialect.QuoteIdentifier(table),
			d.dialect.QuoteIdentifier(new.Name),
			new.Type)
	}

	return SchemaChange{
		Type:        ChangeModifyColumn,
		Table:       table,
		Column:      new.Name,
		OldType:     old.Type,
		NewType:     new.Type,
		Description: fmt.Sprintf("Modify column %s.%s from %s to %s", table, new.Name, old.Type, new.Type),
		SQL:         sql,
		Reversible:  true,
	}
}

// columnChanged verifica se uma coluna mudou.
func (d *SchemaDiffer) columnChanged(old, new ColumnSchema) bool {
	return old.Type != new.Type ||
		old.Nullable != new.Nullable ||
		old.Default != new.Default
}

// diffIndexes compara índices.
func (d *SchemaDiffer) diffIndexes(current, target *TableSchema) []SchemaChange {
	// Implementação simplificada
	return nil
}

// diffForeignKeys compara foreign keys.
func (d *SchemaDiffer) diffForeignKeys(current, target *TableSchema) []SchemaChange {
	// Implementação simplificada
	return nil
}

// GenerateMigration gera uma migração a partir das mudanças.
func (d *SchemaDiffer) GenerateMigration(changes []SchemaChange) string {
	var sb strings.Builder

	sb.WriteString("-- Migration generated by Genus\n")
	sb.WriteString("-- Up\n\n")

	for _, change := range changes {
		sb.WriteString(fmt.Sprintf("-- %s\n", change.Description))
		sb.WriteString(change.SQL)
		sb.WriteString(";\n\n")
	}

	sb.WriteString("-- Down\n\n")

	// Gera rollback em ordem reversa
	for i := len(changes) - 1; i >= 0; i-- {
		change := changes[i]
		if change.Reversible && change.ReverseSQL != "" {
			sb.WriteString(fmt.Sprintf("-- Reverse: %s\n", change.Description))
			sb.WriteString(change.ReverseSQL)
			sb.WriteString(";\n\n")
		} else {
			sb.WriteString(fmt.Sprintf("-- WARNING: Cannot reverse: %s\n\n", change.Description))
		}
	}

	return sb.String()
}

// toSnakeCaseMigrate converte para snake_case.
func toSnakeCaseMigrate(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteRune('_')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}
