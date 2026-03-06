package eventsourcing

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/go-genus/genus/core"
)

// Event representa um evento de domínio.
type Event struct {
	ID            string                 `json:"id" db:"id"`
	AggregateID   string                 `json:"aggregate_id" db:"aggregate_id"`
	AggregateType string                 `json:"aggregate_type" db:"aggregate_type"`
	EventType     string                 `json:"event_type" db:"event_type"`
	Version       int64                  `json:"version" db:"version"`
	Data          map[string]interface{} `json:"data" db:"data"`
	Metadata      map[string]interface{} `json:"metadata,omitempty" db:"metadata"`
	Timestamp     time.Time              `json:"timestamp" db:"timestamp"`
}

// EventStore armazena eventos.
type EventStore struct {
	executor core.Executor
	dialect  core.Dialect
	table    string
	mu       sync.RWMutex
}

// EventStoreConfig configura o event store.
type EventStoreConfig struct {
	TableName string
}

// DefaultEventStoreConfig retorna configuração padrão.
func DefaultEventStoreConfig() EventStoreConfig {
	return EventStoreConfig{
		TableName: "events",
	}
}

// NewEventStore cria um novo event store.
func NewEventStore(executor core.Executor, dialect core.Dialect, config EventStoreConfig) *EventStore {
	if config.TableName == "" {
		config.TableName = "events"
	}

	return &EventStore{
		executor: executor,
		dialect:  dialect,
		table:    config.TableName,
	}
}

// CreateTable cria a tabela de eventos.
func (es *EventStore) CreateTable(ctx context.Context) error {
	var query string

	if es.dialect.Placeholder(1) == "?" {
		// MySQL
		query = fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				id VARCHAR(36) PRIMARY KEY,
				aggregate_id VARCHAR(255) NOT NULL,
				aggregate_type VARCHAR(255) NOT NULL,
				event_type VARCHAR(255) NOT NULL,
				version BIGINT NOT NULL,
				data JSON NOT NULL,
				metadata JSON,
				timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				INDEX idx_aggregate (aggregate_id, aggregate_type),
				INDEX idx_timestamp (timestamp),
				UNIQUE KEY uk_aggregate_version (aggregate_id, aggregate_type, version)
			)
		`, es.dialect.QuoteIdentifier(es.table))
	} else {
		// PostgreSQL
		query = fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				id VARCHAR(36) PRIMARY KEY,
				aggregate_id VARCHAR(255) NOT NULL,
				aggregate_type VARCHAR(255) NOT NULL,
				event_type VARCHAR(255) NOT NULL,
				version BIGINT NOT NULL,
				data JSONB NOT NULL,
				metadata JSONB,
				timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				UNIQUE (aggregate_id, aggregate_type, version)
			);
			CREATE INDEX IF NOT EXISTS idx_events_aggregate ON %s (aggregate_id, aggregate_type);
			CREATE INDEX IF NOT EXISTS idx_events_timestamp ON %s (timestamp);
		`, es.dialect.QuoteIdentifier(es.table), es.dialect.QuoteIdentifier(es.table), es.dialect.QuoteIdentifier(es.table))
	}

	_, err := es.executor.ExecContext(ctx, query)
	return err
}

// Append adiciona eventos ao store.
func (es *EventStore) Append(ctx context.Context, events ...Event) error {
	es.mu.Lock()
	defer es.mu.Unlock()

	for _, event := range events {
		dataJSON, err := json.Marshal(event.Data)
		if err != nil {
			return err
		}

		metaJSON, err := json.Marshal(event.Metadata)
		if err != nil {
			return err
		}

		var query string
		if es.dialect.Placeholder(1) == "?" {
			query = fmt.Sprintf(`
				INSERT INTO %s (id, aggregate_id, aggregate_type, event_type, version, data, metadata, timestamp)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			`, es.dialect.QuoteIdentifier(es.table))
		} else {
			query = fmt.Sprintf(`
				INSERT INTO %s (id, aggregate_id, aggregate_type, event_type, version, data, metadata, timestamp)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			`, es.dialect.QuoteIdentifier(es.table))
		}

		if event.Timestamp.IsZero() {
			event.Timestamp = time.Now()
		}

		_, err = es.executor.ExecContext(ctx, query,
			event.ID,
			event.AggregateID,
			event.AggregateType,
			event.EventType,
			event.Version,
			string(dataJSON),
			string(metaJSON),
			event.Timestamp,
		)
		if err != nil {
			return err
		}
	}

	return nil
}

// Load carrega eventos de um aggregate.
func (es *EventStore) Load(ctx context.Context, aggregateID, aggregateType string) ([]Event, error) {
	return es.LoadFromVersion(ctx, aggregateID, aggregateType, 0)
}

// LoadFromVersion carrega eventos a partir de uma versão.
func (es *EventStore) LoadFromVersion(ctx context.Context, aggregateID, aggregateType string, fromVersion int64) ([]Event, error) {
	var query string
	if es.dialect.Placeholder(1) == "?" {
		query = fmt.Sprintf(`
			SELECT id, aggregate_id, aggregate_type, event_type, version, data, metadata, timestamp
			FROM %s
			WHERE aggregate_id = ? AND aggregate_type = ? AND version > ?
			ORDER BY version ASC
		`, es.dialect.QuoteIdentifier(es.table))
	} else {
		query = fmt.Sprintf(`
			SELECT id, aggregate_id, aggregate_type, event_type, version, data, metadata, timestamp
			FROM %s
			WHERE aggregate_id = $1 AND aggregate_type = $2 AND version > $3
			ORDER BY version ASC
		`, es.dialect.QuoteIdentifier(es.table))
	}

	rows, err := es.executor.QueryContext(ctx, query, aggregateID, aggregateType, fromVersion)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		var event Event
		var dataJSON, metaJSON []byte

		err := rows.Scan(
			&event.ID,
			&event.AggregateID,
			&event.AggregateType,
			&event.EventType,
			&event.Version,
			&dataJSON,
			&metaJSON,
			&event.Timestamp,
		)
		if err != nil {
			return nil, err
		}

		_ = json.Unmarshal(dataJSON, &event.Data)
		_ = json.Unmarshal(metaJSON, &event.Metadata)

		events = append(events, event)
	}

	return events, rows.Err()
}

// GetLatestVersion retorna a última versão de um aggregate.
func (es *EventStore) GetLatestVersion(ctx context.Context, aggregateID, aggregateType string) (int64, error) {
	var query string
	if es.dialect.Placeholder(1) == "?" {
		query = fmt.Sprintf(`
			SELECT COALESCE(MAX(version), 0)
			FROM %s
			WHERE aggregate_id = ? AND aggregate_type = ?
		`, es.dialect.QuoteIdentifier(es.table))
	} else {
		query = fmt.Sprintf(`
			SELECT COALESCE(MAX(version), 0)
			FROM %s
			WHERE aggregate_id = $1 AND aggregate_type = $2
		`, es.dialect.QuoteIdentifier(es.table))
	}

	var version int64
	row := es.executor.QueryRowContext(ctx, query, aggregateID, aggregateType)
	err := row.Scan(&version)
	return version, err
}

// Aggregate é a interface para aggregates.
type Aggregate interface {
	GetID() string
	GetType() string
	GetVersion() int64
	SetVersion(version int64)
	Apply(event Event) error
	GetUncommittedEvents() []Event
	ClearUncommittedEvents()
}

// BaseAggregate fornece implementação base para aggregates.
type BaseAggregate struct {
	ID                string
	Type              string
	Version           int64
	uncommittedEvents []Event
}

// GetID retorna o ID.
func (a *BaseAggregate) GetID() string {
	return a.ID
}

// GetType retorna o tipo.
func (a *BaseAggregate) GetType() string {
	return a.Type
}

// GetVersion retorna a versão.
func (a *BaseAggregate) GetVersion() int64 {
	return a.Version
}

// SetVersion define a versão.
func (a *BaseAggregate) SetVersion(version int64) {
	a.Version = version
}

// GetUncommittedEvents retorna eventos não commitados.
func (a *BaseAggregate) GetUncommittedEvents() []Event {
	return a.uncommittedEvents
}

// ClearUncommittedEvents limpa eventos não commitados.
func (a *BaseAggregate) ClearUncommittedEvents() {
	a.uncommittedEvents = nil
}

// RaiseEvent adiciona um evento ao aggregate.
func (a *BaseAggregate) RaiseEvent(eventType string, data map[string]interface{}) Event {
	a.Version++
	event := Event{
		ID:            generateUUID(),
		AggregateID:   a.ID,
		AggregateType: a.Type,
		EventType:     eventType,
		Version:       a.Version,
		Data:          data,
		Timestamp:     time.Now(),
	}
	a.uncommittedEvents = append(a.uncommittedEvents, event)
	return event
}

// AggregateRepository gerencia aggregates.
type AggregateRepository[T Aggregate] struct {
	store    *EventStore
	factory  func() T
	handlers map[string]func(T, Event) error
}

// NewAggregateRepository cria um novo repository.
func NewAggregateRepository[T Aggregate](store *EventStore, factory func() T) *AggregateRepository[T] {
	return &AggregateRepository[T]{
		store:    store,
		factory:  factory,
		handlers: make(map[string]func(T, Event) error),
	}
}

// RegisterHandler registra um handler para um tipo de evento.
func (r *AggregateRepository[T]) RegisterHandler(eventType string, handler func(T, Event) error) {
	r.handlers[eventType] = handler
}

// Load carrega um aggregate.
func (r *AggregateRepository[T]) Load(ctx context.Context, id string) (T, error) {
	aggregate := r.factory()
	aggregateType := aggregate.GetType()

	events, err := r.store.Load(ctx, id, aggregateType)
	if err != nil {
		return aggregate, err
	}

	for _, event := range events {
		if handler, exists := r.handlers[event.EventType]; exists {
			if err := handler(aggregate, event); err != nil {
				return aggregate, err
			}
		} else if err := aggregate.Apply(event); err != nil {
			return aggregate, err
		}
		aggregate.SetVersion(event.Version)
	}

	return aggregate, nil
}

// Save salva um aggregate.
func (r *AggregateRepository[T]) Save(ctx context.Context, aggregate T) error {
	events := aggregate.GetUncommittedEvents()
	if len(events) == 0 {
		return nil
	}

	err := r.store.Append(ctx, events...)
	if err != nil {
		return err
	}

	aggregate.ClearUncommittedEvents()
	return nil
}

// Snapshot representa um snapshot de um aggregate.
type Snapshot struct {
	AggregateID   string                 `json:"aggregate_id" db:"aggregate_id"`
	AggregateType string                 `json:"aggregate_type" db:"aggregate_type"`
	Version       int64                  `json:"version" db:"version"`
	State         map[string]interface{} `json:"state" db:"state"`
	Timestamp     time.Time              `json:"timestamp" db:"timestamp"`
}

// SnapshotStore armazena snapshots.
type SnapshotStore struct {
	executor core.Executor
	dialect  core.Dialect
	table    string
}

// NewSnapshotStore cria um novo snapshot store.
func NewSnapshotStore(executor core.Executor, dialect core.Dialect) *SnapshotStore {
	return &SnapshotStore{
		executor: executor,
		dialect:  dialect,
		table:    "snapshots",
	}
}

// CreateTable cria a tabela de snapshots.
func (ss *SnapshotStore) CreateTable(ctx context.Context) error {
	var query string

	if ss.dialect.Placeholder(1) == "?" {
		// MySQL
		query = fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				aggregate_id VARCHAR(255) NOT NULL,
				aggregate_type VARCHAR(255) NOT NULL,
				version BIGINT NOT NULL,
				state JSON NOT NULL,
				timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				PRIMARY KEY (aggregate_id, aggregate_type)
			)
		`, ss.dialect.QuoteIdentifier(ss.table))
	} else {
		// PostgreSQL
		query = fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				aggregate_id VARCHAR(255) NOT NULL,
				aggregate_type VARCHAR(255) NOT NULL,
				version BIGINT NOT NULL,
				state JSONB NOT NULL,
				timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				PRIMARY KEY (aggregate_id, aggregate_type)
			)
		`, ss.dialect.QuoteIdentifier(ss.table))
	}

	_, err := ss.executor.ExecContext(ctx, query)
	return err
}

// Save salva um snapshot.
func (ss *SnapshotStore) Save(ctx context.Context, snapshot Snapshot) error {
	stateJSON, err := json.Marshal(snapshot.State)
	if err != nil {
		return err
	}

	var query string
	if ss.dialect.Placeholder(1) == "?" {
		query = fmt.Sprintf(`
			INSERT INTO %s (aggregate_id, aggregate_type, version, state, timestamp)
			VALUES (?, ?, ?, ?, ?)
			ON DUPLICATE KEY UPDATE version = VALUES(version), state = VALUES(state), timestamp = VALUES(timestamp)
		`, ss.dialect.QuoteIdentifier(ss.table))
	} else {
		query = fmt.Sprintf(`
			INSERT INTO %s (aggregate_id, aggregate_type, version, state, timestamp)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (aggregate_id, aggregate_type)
			DO UPDATE SET version = EXCLUDED.version, state = EXCLUDED.state, timestamp = EXCLUDED.timestamp
		`, ss.dialect.QuoteIdentifier(ss.table))
	}

	_, err = ss.executor.ExecContext(ctx, query,
		snapshot.AggregateID,
		snapshot.AggregateType,
		snapshot.Version,
		string(stateJSON),
		time.Now(),
	)

	return err
}

// Load carrega um snapshot.
func (ss *SnapshotStore) Load(ctx context.Context, aggregateID, aggregateType string) (*Snapshot, error) {
	var query string
	if ss.dialect.Placeholder(1) == "?" {
		query = fmt.Sprintf(`
			SELECT aggregate_id, aggregate_type, version, state, timestamp
			FROM %s
			WHERE aggregate_id = ? AND aggregate_type = ?
		`, ss.dialect.QuoteIdentifier(ss.table))
	} else {
		query = fmt.Sprintf(`
			SELECT aggregate_id, aggregate_type, version, state, timestamp
			FROM %s
			WHERE aggregate_id = $1 AND aggregate_type = $2
		`, ss.dialect.QuoteIdentifier(ss.table))
	}

	var snapshot Snapshot
	var stateJSON []byte

	row := ss.executor.QueryRowContext(ctx, query, aggregateID, aggregateType)
	err := row.Scan(
		&snapshot.AggregateID,
		&snapshot.AggregateType,
		&snapshot.Version,
		&stateJSON,
		&snapshot.Timestamp,
	)

	if err != nil {
		return nil, err
	}

	_ = json.Unmarshal(stateJSON, &snapshot.State)
	return &snapshot, nil
}

// generateUUID gera um UUID simples.
func generateUUID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), time.Now().UnixNano()%1000000)
}

// Snapshotable interface para aggregates que suportam snapshots.
type Snapshotable interface {
	Aggregate
	ToSnapshot() map[string]interface{}
	FromSnapshot(state map[string]interface{}) error
}

// SnapshotRepository gerencia aggregates com snapshots.
type SnapshotRepository[T Snapshotable] struct {
	*AggregateRepository[T]
	snapshotStore    *SnapshotStore
	snapshotInterval int64
}

// NewSnapshotRepository cria um repository com suporte a snapshots.
func NewSnapshotRepository[T Snapshotable](
	store *EventStore,
	snapshotStore *SnapshotStore,
	factory func() T,
	snapshotInterval int64,
) *SnapshotRepository[T] {
	return &SnapshotRepository[T]{
		AggregateRepository: NewAggregateRepository(store, factory),
		snapshotStore:       snapshotStore,
		snapshotInterval:    snapshotInterval,
	}
}

// Load carrega um aggregate com snapshot.
func (r *SnapshotRepository[T]) Load(ctx context.Context, id string) (T, error) {
	aggregate := r.factory()
	aggregateType := aggregate.GetType()

	// Tenta carregar snapshot
	snapshot, err := r.snapshotStore.Load(ctx, id, aggregateType)
	if err == nil && snapshot != nil {
		if err := aggregate.FromSnapshot(snapshot.State); err != nil {
			return aggregate, err
		}
		aggregate.SetVersion(snapshot.Version)

		// Carrega eventos após o snapshot
		events, err := r.store.LoadFromVersion(ctx, id, aggregateType, snapshot.Version)
		if err != nil {
			return aggregate, err
		}

		for _, event := range events {
			if err := aggregate.Apply(event); err != nil {
				return aggregate, err
			}
			aggregate.SetVersion(event.Version)
		}

		return aggregate, nil
	}

	// Carrega todos os eventos
	return r.AggregateRepository.Load(ctx, id)
}

// Save salva um aggregate e cria snapshot se necessário.
func (r *SnapshotRepository[T]) Save(ctx context.Context, aggregate T) error {
	if err := r.AggregateRepository.Save(ctx, aggregate); err != nil {
		return err
	}

	// Cria snapshot se atingiu o intervalo
	if aggregate.GetVersion()%r.snapshotInterval == 0 {
		snapshot := Snapshot{
			AggregateID:   aggregate.GetID(),
			AggregateType: aggregate.GetType(),
			Version:       aggregate.GetVersion(),
			State:         aggregate.ToSnapshot(),
			Timestamp:     time.Now(),
		}
		return r.snapshotStore.Save(ctx, snapshot)
	}

	return nil
}

// EventHandler é um handler de eventos.
type EventHandler func(ctx context.Context, event Event) error

// EventBus distribui eventos para handlers.
type EventBus struct {
	handlers map[string][]EventHandler
	mu       sync.RWMutex
}

// NewEventBus cria um novo event bus.
func NewEventBus() *EventBus {
	return &EventBus{
		handlers: make(map[string][]EventHandler),
	}
}

// Subscribe inscreve um handler para um tipo de evento.
func (eb *EventBus) Subscribe(eventType string, handler EventHandler) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.handlers[eventType] = append(eb.handlers[eventType], handler)
}

// Publish publica um evento.
func (eb *EventBus) Publish(ctx context.Context, event Event) error {
	eb.mu.RLock()
	handlers := eb.handlers[event.EventType]
	allHandlers := eb.handlers["*"] // Wildcard handlers
	eb.mu.RUnlock()

	for _, handler := range handlers {
		if err := handler(ctx, event); err != nil {
			return err
		}
	}

	for _, handler := range allHandlers {
		if err := handler(ctx, event); err != nil {
			return err
		}
	}

	return nil
}

// PublishAll publica múltiplos eventos.
func (eb *EventBus) PublishAll(ctx context.Context, events []Event) error {
	for _, event := range events {
		if err := eb.Publish(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

// StructToMap converte um struct para map.
func StructToMap(v interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}
		name := field.Tag.Get("json")
		if name == "" {
			name = field.Name
		}
		if idx := len(name); idx > 0 {
			if comma := len(name); comma > 0 {
				for j, c := range name {
					if c == ',' {
						name = name[:j]
						break
					}
				}
			}
		}
		result[name] = val.Field(i).Interface()
	}

	return result
}
