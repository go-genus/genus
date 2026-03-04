package core

import (
	"context"
	"encoding/json"
	"reflect"
	"time"
)

// AuditAction representa o tipo de ação auditada.
type AuditAction string

const (
	AuditCreate AuditAction = "CREATE"
	AuditUpdate AuditAction = "UPDATE"
	AuditDelete AuditAction = "DELETE"
	AuditRead   AuditAction = "READ"
)

// AuditEntry representa uma entrada de auditoria.
type AuditEntry struct {
	ID        int64                  `json:"id" db:"id"`
	TableName string                 `json:"table_name" db:"table_name"`
	RecordID  interface{}            `json:"record_id" db:"record_id"`
	Action    AuditAction            `json:"action" db:"action"`
	OldValues map[string]interface{} `json:"old_values,omitempty" db:"old_values"`
	NewValues map[string]interface{} `json:"new_values,omitempty" db:"new_values"`
	ChangedBy string                 `json:"changed_by,omitempty" db:"changed_by"`
	ChangedAt time.Time              `json:"changed_at" db:"changed_at"`
	IPAddress string                 `json:"ip_address,omitempty" db:"ip_address"`
	UserAgent string                 `json:"user_agent,omitempty" db:"user_agent"`
	Metadata  map[string]interface{} `json:"metadata,omitempty" db:"metadata"`
}

// AuditConfig configura o sistema de auditoria.
type AuditConfig struct {
	// Enabled ativa a auditoria.
	Enabled bool

	// AuditTable é o nome da tabela de auditoria.
	AuditTable string

	// ExcludeTables são tabelas que não serão auditadas.
	ExcludeTables []string

	// ExcludeColumns são colunas que não serão incluídas no audit log.
	// Útil para excluir senhas, tokens, etc.
	ExcludeColumns []string

	// AuditReads indica se operações de leitura devem ser auditadas.
	AuditReads bool

	// OnAudit é chamado para cada entrada de auditoria.
	// Pode ser usado para logging customizado ou envio para serviços externos.
	OnAudit func(entry AuditEntry)

	// GetCurrentUser retorna o ID/nome do usuário atual do contexto.
	GetCurrentUser func(ctx context.Context) string

	// GetIPAddress retorna o endereço IP do contexto.
	GetIPAddress func(ctx context.Context) string

	// GetUserAgent retorna o user agent do contexto.
	GetUserAgent func(ctx context.Context) string
}

// DefaultAuditConfig retorna configuração padrão.
func DefaultAuditConfig() AuditConfig {
	return AuditConfig{
		Enabled:        false,
		AuditTable:     "audit_logs",
		ExcludeColumns: []string{"password", "password_hash", "token", "secret", "api_key"},
		AuditReads:     false,
	}
}

// Auditor gerencia a auditoria de operações.
type Auditor struct {
	config   AuditConfig
	executor Executor
	dialect  Dialect
}

// NewAuditor cria um novo auditor.
func NewAuditor(executor Executor, dialect Dialect, config AuditConfig) *Auditor {
	return &Auditor{
		config:   config,
		executor: executor,
		dialect:  dialect,
	}
}

// LogCreate registra uma operação de criação.
func (a *Auditor) LogCreate(ctx context.Context, tableName string, recordID interface{}, newValues interface{}) error {
	if !a.config.Enabled {
		return nil
	}

	if a.isExcludedTable(tableName) {
		return nil
	}

	values := a.extractValues(newValues)
	entry := AuditEntry{
		TableName: tableName,
		RecordID:  recordID,
		Action:    AuditCreate,
		NewValues: values,
		ChangedAt: time.Now(),
	}

	a.enrichEntry(ctx, &entry)
	return a.saveEntry(ctx, entry)
}

// LogUpdate registra uma operação de atualização.
func (a *Auditor) LogUpdate(ctx context.Context, tableName string, recordID interface{}, oldValues, newValues interface{}) error {
	if !a.config.Enabled {
		return nil
	}

	if a.isExcludedTable(tableName) {
		return nil
	}

	oldVals := a.extractValues(oldValues)
	newVals := a.extractValues(newValues)

	// Filtra apenas valores que mudaram
	changes := make(map[string]interface{})
	for k, v := range newVals {
		if oldV, exists := oldVals[k]; !exists || !reflect.DeepEqual(v, oldV) {
			changes[k] = v
		}
	}

	if len(changes) == 0 {
		return nil // Nada mudou
	}

	entry := AuditEntry{
		TableName: tableName,
		RecordID:  recordID,
		Action:    AuditUpdate,
		OldValues: oldVals,
		NewValues: changes,
		ChangedAt: time.Now(),
	}

	a.enrichEntry(ctx, &entry)
	return a.saveEntry(ctx, entry)
}

// LogDelete registra uma operação de exclusão.
func (a *Auditor) LogDelete(ctx context.Context, tableName string, recordID interface{}, oldValues interface{}) error {
	if !a.config.Enabled {
		return nil
	}

	if a.isExcludedTable(tableName) {
		return nil
	}

	values := a.extractValues(oldValues)
	entry := AuditEntry{
		TableName: tableName,
		RecordID:  recordID,
		Action:    AuditDelete,
		OldValues: values,
		ChangedAt: time.Now(),
	}

	a.enrichEntry(ctx, &entry)
	return a.saveEntry(ctx, entry)
}

// LogRead registra uma operação de leitura (opcional).
func (a *Auditor) LogRead(ctx context.Context, tableName string, recordID interface{}) error {
	if !a.config.Enabled || !a.config.AuditReads {
		return nil
	}

	if a.isExcludedTable(tableName) {
		return nil
	}

	entry := AuditEntry{
		TableName: tableName,
		RecordID:  recordID,
		Action:    AuditRead,
		ChangedAt: time.Now(),
	}

	a.enrichEntry(ctx, &entry)
	return a.saveEntry(ctx, entry)
}

// enrichEntry adiciona informações de contexto à entrada.
func (a *Auditor) enrichEntry(ctx context.Context, entry *AuditEntry) {
	if a.config.GetCurrentUser != nil {
		entry.ChangedBy = a.config.GetCurrentUser(ctx)
	}
	if a.config.GetIPAddress != nil {
		entry.IPAddress = a.config.GetIPAddress(ctx)
	}
	if a.config.GetUserAgent != nil {
		entry.UserAgent = a.config.GetUserAgent(ctx)
	}
}

// saveEntry salva a entrada de auditoria.
func (a *Auditor) saveEntry(ctx context.Context, entry AuditEntry) error {
	// Callback customizado
	if a.config.OnAudit != nil {
		a.config.OnAudit(entry)
	}

	// Salva no banco de dados
	if a.executor == nil {
		return nil
	}

	oldJSON, _ := json.Marshal(entry.OldValues)
	newJSON, _ := json.Marshal(entry.NewValues)
	metaJSON, _ := json.Marshal(entry.Metadata)

	query := `
		INSERT INTO ` + a.dialect.QuoteIdentifier(a.config.AuditTable) + `
		(table_name, record_id, action, old_values, new_values, changed_by, changed_at, ip_address, user_agent, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	if a.dialect.Placeholder(1) == "?" {
		query = `
			INSERT INTO ` + a.dialect.QuoteIdentifier(a.config.AuditTable) + `
			(table_name, record_id, action, old_values, new_values, changed_by, changed_at, ip_address, user_agent, metadata)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`
	}

	_, err := a.executor.ExecContext(ctx, query,
		entry.TableName,
		entry.RecordID,
		string(entry.Action),
		string(oldJSON),
		string(newJSON),
		entry.ChangedBy,
		entry.ChangedAt,
		entry.IPAddress,
		entry.UserAgent,
		string(metaJSON),
	)

	return err
}

// extractValues extrai valores de um struct como map.
func (a *Auditor) extractValues(model interface{}) map[string]interface{} {
	if model == nil {
		return nil
	}

	values := make(map[string]interface{})
	val := reflect.ValueOf(model)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return values
	}

	typ := val.Type()
	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}

		// Usa tag db se existir
		name := field.Tag.Get("db")
		if name == "" || name == "-" {
			name = toSnakeCase(field.Name)
		}

		// Verifica se é coluna excluída
		if a.isExcludedColumn(name) {
			continue
		}

		values[name] = val.Field(i).Interface()
	}

	return values
}

// isExcludedTable verifica se a tabela está excluída.
func (a *Auditor) isExcludedTable(tableName string) bool {
	for _, t := range a.config.ExcludeTables {
		if t == tableName {
			return true
		}
	}
	return false
}

// isExcludedColumn verifica se a coluna está excluída.
func (a *Auditor) isExcludedColumn(column string) bool {
	for _, c := range a.config.ExcludeColumns {
		if c == column {
			return true
		}
	}
	return false
}

// CreateAuditTable cria a tabela de auditoria.
func (a *Auditor) CreateAuditTable(ctx context.Context) error {
	var query string

	if a.dialect.Placeholder(1) == "?" {
		// MySQL
		query = `
			CREATE TABLE IF NOT EXISTS ` + a.dialect.QuoteIdentifier(a.config.AuditTable) + ` (
				id BIGINT AUTO_INCREMENT PRIMARY KEY,
				table_name VARCHAR(255) NOT NULL,
				record_id VARCHAR(255),
				action VARCHAR(50) NOT NULL,
				old_values JSON,
				new_values JSON,
				changed_by VARCHAR(255),
				changed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				ip_address VARCHAR(45),
				user_agent TEXT,
				metadata JSON,
				INDEX idx_audit_table (table_name),
				INDEX idx_audit_action (action),
				INDEX idx_audit_changed_at (changed_at)
			)
		`
	} else {
		// PostgreSQL
		query = `
			CREATE TABLE IF NOT EXISTS ` + a.dialect.QuoteIdentifier(a.config.AuditTable) + ` (
				id BIGSERIAL PRIMARY KEY,
				table_name VARCHAR(255) NOT NULL,
				record_id VARCHAR(255),
				action VARCHAR(50) NOT NULL,
				old_values JSONB,
				new_values JSONB,
				changed_by VARCHAR(255),
				changed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				ip_address VARCHAR(45),
				user_agent TEXT,
				metadata JSONB
			);
			CREATE INDEX IF NOT EXISTS idx_audit_table ON ` + a.dialect.QuoteIdentifier(a.config.AuditTable) + ` (table_name);
			CREATE INDEX IF NOT EXISTS idx_audit_action ON ` + a.dialect.QuoteIdentifier(a.config.AuditTable) + ` (action);
			CREATE INDEX IF NOT EXISTS idx_audit_changed_at ON ` + a.dialect.QuoteIdentifier(a.config.AuditTable) + ` (changed_at);
		`
	}

	_, err := a.executor.ExecContext(ctx, query)
	return err
}

// GetAuditHistory retorna o histórico de auditoria para um registro.
func (a *Auditor) GetAuditHistory(ctx context.Context, tableName string, recordID interface{}) ([]AuditEntry, error) {
	query := `
		SELECT id, table_name, record_id, action, old_values, new_values,
		       changed_by, changed_at, ip_address, user_agent, metadata
		FROM ` + a.dialect.QuoteIdentifier(a.config.AuditTable) + `
		WHERE table_name = $1 AND record_id = $2
		ORDER BY changed_at DESC
	`

	if a.dialect.Placeholder(1) == "?" {
		query = `
			SELECT id, table_name, record_id, action, old_values, new_values,
			       changed_by, changed_at, ip_address, user_agent, metadata
			FROM ` + a.dialect.QuoteIdentifier(a.config.AuditTable) + `
			WHERE table_name = ? AND record_id = ?
			ORDER BY changed_at DESC
		`
	}

	rows, err := a.executor.QueryContext(ctx, query, tableName, recordID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []AuditEntry
	for rows.Next() {
		var entry AuditEntry
		var oldJSON, newJSON, metaJSON []byte

		err := rows.Scan(
			&entry.ID,
			&entry.TableName,
			&entry.RecordID,
			&entry.Action,
			&oldJSON,
			&newJSON,
			&entry.ChangedBy,
			&entry.ChangedAt,
			&entry.IPAddress,
			&entry.UserAgent,
			&metaJSON,
		)
		if err != nil {
			return nil, err
		}

		json.Unmarshal(oldJSON, &entry.OldValues)
		json.Unmarshal(newJSON, &entry.NewValues)
		json.Unmarshal(metaJSON, &entry.Metadata)

		entries = append(entries, entry)
	}

	return entries, rows.Err()
}
