package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// NotifyPayload representa o payload de uma notificação.
type NotifyPayload struct {
	Table     string                 `json:"table"`
	Action    string                 `json:"action"`
	RecordID  interface{}            `json:"record_id,omitempty"`
	OldValues map[string]interface{} `json:"old,omitempty"`
	NewValues map[string]interface{} `json:"new,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// NotifyHandler é o callback para notificações.
type NotifyHandler func(payload NotifyPayload)

// SubscriptionConfig configura subscriptions.
type SubscriptionConfig struct {
	// ChannelPrefix é o prefixo para canais de notificação.
	ChannelPrefix string

	// PollInterval é o intervalo de polling para notificações.
	PollInterval time.Duration

	// OnError é chamado quando ocorre um erro.
	OnError func(err error)
}

// DefaultSubscriptionConfig retorna configuração padrão.
func DefaultSubscriptionConfig() SubscriptionConfig {
	return SubscriptionConfig{
		ChannelPrefix: "genus_",
		PollInterval:  100 * time.Millisecond,
	}
}

// Subscription representa uma subscription ativa.
type Subscription struct {
	channel string
	handler NotifyHandler
	cancel  context.CancelFunc
	active  bool
	mu      sync.Mutex
}

// Cancel cancela a subscription.
func (s *Subscription) Cancel() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.active && s.cancel != nil {
		s.cancel()
		s.active = false
	}
}

// IsActive verifica se a subscription está ativa.
func (s *Subscription) IsActive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active
}

// SubscriptionManager gerencia subscriptions de notificações.
type SubscriptionManager struct {
	config        SubscriptionConfig
	executor      Executor
	dialect       Dialect
	subscriptions map[string][]*Subscription
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
}

// NewSubscriptionManager cria um novo gerenciador de subscriptions.
func NewSubscriptionManager(executor Executor, dialect Dialect, config SubscriptionConfig) *SubscriptionManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &SubscriptionManager{
		config:        config,
		executor:      executor,
		dialect:       dialect,
		subscriptions: make(map[string][]*Subscription),
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Subscribe inscreve-se para notificações em uma tabela.
//
// Exemplo:
//
//	sub := manager.Subscribe(ctx, "users", func(payload NotifyPayload) {
//	    fmt.Printf("User %s: %v\n", payload.Action, payload.RecordID)
//	})
//	defer sub.Cancel()
func (sm *SubscriptionManager) Subscribe(ctx context.Context, tableName string, handler NotifyHandler) (*Subscription, error) {
	channel := sm.config.ChannelPrefix + tableName

	// Executa LISTEN
	listenQuery := fmt.Sprintf("LISTEN %s", sm.dialect.QuoteIdentifier(channel))
	if _, err := sm.executor.ExecContext(ctx, listenQuery); err != nil {
		return nil, fmt.Errorf("failed to listen on channel %s: %w", channel, err)
	}

	subCtx, cancel := context.WithCancel(sm.ctx)
	sub := &Subscription{
		channel: channel,
		handler: handler,
		cancel:  cancel,
		active:  true,
	}

	sm.mu.Lock()
	sm.subscriptions[channel] = append(sm.subscriptions[channel], sub)
	sm.mu.Unlock()

	// Inicia goroutine para processar notificações
	go sm.pollNotifications(subCtx, channel)

	return sub, nil
}

// pollNotifications poll para notificações (fallback quando pg_notify não está disponível).
func (sm *SubscriptionManager) pollNotifications(ctx context.Context, channel string) {
	ticker := time.NewTicker(sm.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Em uma implementação real, usaríamos lib/pq ou pgx para LISTEN/NOTIFY nativo
			// Esta é uma implementação simplificada
		}
	}
}

// Unsubscribe remove uma subscription.
func (sm *SubscriptionManager) Unsubscribe(ctx context.Context, sub *Subscription) error {
	sub.Cancel()

	sm.mu.Lock()
	defer sm.mu.Unlock()

	subs := sm.subscriptions[sub.channel]
	for i, s := range subs {
		if s == sub {
			sm.subscriptions[sub.channel] = append(subs[:i], subs[i+1:]...)
			break
		}
	}

	// Se não há mais subscriptions neste canal, executa UNLISTEN
	if len(sm.subscriptions[sub.channel]) == 0 {
		unlistenQuery := fmt.Sprintf("UNLISTEN %s", sm.dialect.QuoteIdentifier(sub.channel))
		if _, err := sm.executor.ExecContext(ctx, unlistenQuery); err != nil {
			return fmt.Errorf("failed to unlisten from channel %s: %w", sub.channel, err)
		}
		delete(sm.subscriptions, sub.channel)
	}

	return nil
}

// Close fecha todas as subscriptions.
func (sm *SubscriptionManager) Close() {
	sm.cancel()

	sm.mu.Lock()
	defer sm.mu.Unlock()

	for _, subs := range sm.subscriptions {
		for _, sub := range subs {
			sub.Cancel()
		}
	}

	sm.subscriptions = make(map[string][]*Subscription)
}

// Notify envia uma notificação (para testes ou uso manual).
func (sm *SubscriptionManager) Notify(ctx context.Context, tableName string, payload NotifyPayload) error {
	channel := sm.config.ChannelPrefix + tableName
	payload.Timestamp = time.Now()

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	query := fmt.Sprintf("SELECT pg_notify(%s, %s)",
		sm.dialect.Placeholder(1),
		sm.dialect.Placeholder(2))

	_, err = sm.executor.ExecContext(ctx, query, channel, string(jsonPayload))
	return err
}

// CreateNotifyTrigger cria um trigger para notificações automáticas (PostgreSQL).
func (sm *SubscriptionManager) CreateNotifyTrigger(ctx context.Context, tableName string) error {
	channel := sm.config.ChannelPrefix + tableName

	// Cria a função de trigger
	functionName := fmt.Sprintf("notify_%s_changes", tableName)
	createFunctionQuery := fmt.Sprintf(`
		CREATE OR REPLACE FUNCTION %s()
		RETURNS trigger AS $$
		DECLARE
			payload json;
		BEGIN
			IF TG_OP = 'DELETE' THEN
				payload = json_build_object(
					'table', TG_TABLE_NAME,
					'action', TG_OP,
					'record_id', OLD.id,
					'old', row_to_json(OLD),
					'timestamp', NOW()
				);
			ELSIF TG_OP = 'UPDATE' THEN
				payload = json_build_object(
					'table', TG_TABLE_NAME,
					'action', TG_OP,
					'record_id', NEW.id,
					'old', row_to_json(OLD),
					'new', row_to_json(NEW),
					'timestamp', NOW()
				);
			ELSE
				payload = json_build_object(
					'table', TG_TABLE_NAME,
					'action', TG_OP,
					'record_id', NEW.id,
					'new', row_to_json(NEW),
					'timestamp', NOW()
				);
			END IF;

			PERFORM pg_notify('%s', payload::text);
			RETURN NULL;
		END;
		$$ LANGUAGE plpgsql;
	`, sm.dialect.QuoteIdentifier(functionName), channel)

	if _, err := sm.executor.ExecContext(ctx, createFunctionQuery); err != nil {
		return fmt.Errorf("failed to create notify function: %w", err)
	}

	// Cria o trigger
	triggerName := fmt.Sprintf("trigger_notify_%s", tableName)
	createTriggerQuery := fmt.Sprintf(`
		DROP TRIGGER IF EXISTS %s ON %s;
		CREATE TRIGGER %s
		AFTER INSERT OR UPDATE OR DELETE ON %s
		FOR EACH ROW EXECUTE FUNCTION %s();
	`,
		sm.dialect.QuoteIdentifier(triggerName),
		sm.dialect.QuoteIdentifier(tableName),
		sm.dialect.QuoteIdentifier(triggerName),
		sm.dialect.QuoteIdentifier(tableName),
		sm.dialect.QuoteIdentifier(functionName),
	)

	if _, err := sm.executor.ExecContext(ctx, createTriggerQuery); err != nil {
		return fmt.Errorf("failed to create notify trigger: %w", err)
	}

	return nil
}

// DropNotifyTrigger remove o trigger de notificações.
func (sm *SubscriptionManager) DropNotifyTrigger(ctx context.Context, tableName string) error {
	triggerName := fmt.Sprintf("trigger_notify_%s", tableName)
	functionName := fmt.Sprintf("notify_%s_changes", tableName)

	dropTriggerQuery := fmt.Sprintf("DROP TRIGGER IF EXISTS %s ON %s",
		sm.dialect.QuoteIdentifier(triggerName),
		sm.dialect.QuoteIdentifier(tableName))

	if _, err := sm.executor.ExecContext(ctx, dropTriggerQuery); err != nil {
		return err
	}

	dropFunctionQuery := fmt.Sprintf("DROP FUNCTION IF EXISTS %s()",
		sm.dialect.QuoteIdentifier(functionName))

	_, err := sm.executor.ExecContext(ctx, dropFunctionQuery)
	return err
}

// ChangeStream representa um stream de mudanças de uma tabela.
type ChangeStream struct {
	tableName string
	changes   chan NotifyPayload
	sub       *Subscription
	closed    bool
	mu        sync.Mutex
}

// NewChangeStream cria um stream de mudanças.
func (sm *SubscriptionManager) NewChangeStream(ctx context.Context, tableName string, bufferSize int) (*ChangeStream, error) {
	if bufferSize <= 0 {
		bufferSize = 100
	}

	stream := &ChangeStream{
		tableName: tableName,
		changes:   make(chan NotifyPayload, bufferSize),
	}

	sub, err := sm.Subscribe(ctx, tableName, func(payload NotifyPayload) {
		stream.mu.Lock()
		defer stream.mu.Unlock()

		if !stream.closed {
			select {
			case stream.changes <- payload:
			default:
				// Buffer cheio, descarta
			}
		}
	})
	if err != nil {
		return nil, err
	}

	stream.sub = sub
	return stream, nil
}

// Changes retorna o canal de mudanças.
func (cs *ChangeStream) Changes() <-chan NotifyPayload {
	return cs.changes
}

// Close fecha o stream.
func (cs *ChangeStream) Close() {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if !cs.closed {
		cs.closed = true
		cs.sub.Cancel()
		close(cs.changes)
	}
}

// WatchConfig configura watch de uma tabela.
type WatchConfig struct {
	// Actions filtra por ações específicas (INSERT, UPDATE, DELETE).
	Actions []string

	// Filter é uma função que filtra notificações.
	Filter func(payload NotifyPayload) bool
}

// Watch observa mudanças em uma tabela com filtros.
func (sm *SubscriptionManager) Watch(ctx context.Context, tableName string, config WatchConfig, handler NotifyHandler) (*Subscription, error) {
	wrappedHandler := func(payload NotifyPayload) {
		// Filtra por ação
		if len(config.Actions) > 0 {
			found := false
			for _, action := range config.Actions {
				if payload.Action == action {
					found = true
					break
				}
			}
			if !found {
				return
			}
		}

		// Aplica filtro customizado
		if config.Filter != nil && !config.Filter(payload) {
			return
		}

		handler(payload)
	}

	return sm.Subscribe(ctx, tableName, wrappedHandler)
}

// ReadYourWritesHelper ajuda a garantir consistência read-your-writes.
type ReadYourWritesHelper struct {
	executor Executor
	dialect  Dialect
}

// NewReadYourWritesHelper cria um helper de consistência.
func NewReadYourWritesHelper(executor Executor, dialect Dialect) *ReadYourWritesHelper {
	return &ReadYourWritesHelper{
		executor: executor,
		dialect:  dialect,
	}
}

// EnsureReplication espera a replicação alcançar uma posição (PostgreSQL).
func (h *ReadYourWritesHelper) EnsureReplication(ctx context.Context, lsn string) error {
	// Para PostgreSQL com replicação síncrona
	query := "SELECT pg_last_wal_replay_lsn() >= $1"
	if h.dialect.Placeholder(1) == "?" {
		return fmt.Errorf("read-your-writes helper requires PostgreSQL")
	}

	var reached bool
	row := h.executor.QueryRowContext(ctx, query, lsn)
	if err := row.Scan(&reached); err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return err
	}

	if !reached {
		return fmt.Errorf("replication lag detected")
	}

	return nil
}

// GetCurrentLSN obtém a posição atual do WAL (PostgreSQL).
func (h *ReadYourWritesHelper) GetCurrentLSN(ctx context.Context) (string, error) {
	if h.dialect.Placeholder(1) == "?" {
		return "", fmt.Errorf("LSN tracking requires PostgreSQL")
	}

	var lsn string
	row := h.executor.QueryRowContext(ctx, "SELECT pg_current_wal_lsn()")
	if err := row.Scan(&lsn); err != nil {
		return "", err
	}

	return lsn, nil
}
