package cqrs

import (
	"context"
	"fmt"
	"reflect"
	"sync"
)

// Command representa um comando no padrão CQRS.
type Command interface {
	CommandName() string
}

// CommandHandler processa um comando.
type CommandHandler[C Command] interface {
	Handle(ctx context.Context, command C) error
}

// CommandHandlerFunc é uma função que implementa CommandHandler.
type CommandHandlerFunc[C Command] func(ctx context.Context, command C) error

// Handle implementa CommandHandler.
func (f CommandHandlerFunc[C]) Handle(ctx context.Context, command C) error {
	return f(ctx, command)
}

// Query representa uma query no padrão CQRS.
type Query interface {
	QueryName() string
}

// QueryHandler processa uma query e retorna um resultado.
type QueryHandler[Q Query, R any] interface {
	Handle(ctx context.Context, query Q) (R, error)
}

// QueryHandlerFunc é uma função que implementa QueryHandler.
type QueryHandlerFunc[Q Query, R any] func(ctx context.Context, query Q) (R, error)

// Handle implementa QueryHandler.
func (f QueryHandlerFunc[Q, R]) Handle(ctx context.Context, query Q) (R, error) {
	return f(ctx, query)
}

// CommandBus despacha comandos para handlers.
type CommandBus struct {
	handlers   map[string]interface{}
	middleware []CommandMiddleware
	mu         sync.RWMutex
}

// CommandMiddleware é um middleware para comandos.
type CommandMiddleware func(ctx context.Context, command Command, next func(ctx context.Context, command Command) error) error

// NewCommandBus cria um novo command bus.
func NewCommandBus() *CommandBus {
	return &CommandBus{
		handlers: make(map[string]interface{}),
	}
}

// Register registra um handler para um tipo de comando.
func Register[C Command](bus *CommandBus, handler CommandHandler[C]) {
	var zero C
	bus.mu.Lock()
	defer bus.mu.Unlock()
	bus.handlers[zero.CommandName()] = handler
}

// RegisterFunc registra uma função como handler.
func RegisterFunc[C Command](bus *CommandBus, handler func(ctx context.Context, command C) error) {
	Register(bus, CommandHandlerFunc[C](handler))
}

// Use adiciona um middleware.
func (bus *CommandBus) Use(middleware CommandMiddleware) {
	bus.mu.Lock()
	defer bus.mu.Unlock()
	bus.middleware = append(bus.middleware, middleware)
}

// Dispatch despacha um comando para seu handler.
func Dispatch[C Command](ctx context.Context, bus *CommandBus, command C) error {
	bus.mu.RLock()
	handler, exists := bus.handlers[command.CommandName()]
	middleware := bus.middleware
	bus.mu.RUnlock()

	if !exists {
		return fmt.Errorf("no handler registered for command: %s", command.CommandName())
	}

	h, ok := handler.(CommandHandler[C])
	if !ok {
		return fmt.Errorf("invalid handler type for command: %s", command.CommandName())
	}

	// Constrói a cadeia de middleware
	final := func(ctx context.Context, cmd Command) error {
		return h.Handle(ctx, cmd.(C))
	}

	chain := final
	for i := len(middleware) - 1; i >= 0; i-- {
		mw := middleware[i]
		next := chain
		chain = func(ctx context.Context, cmd Command) error {
			return mw(ctx, cmd, next)
		}
	}

	return chain(ctx, command)
}

// QueryBus despacha queries para handlers.
type QueryBus struct {
	handlers   map[string]interface{}
	middleware []QueryMiddleware
	mu         sync.RWMutex
}

// QueryMiddleware é um middleware para queries.
type QueryMiddleware func(ctx context.Context, query Query, next func(ctx context.Context, query Query) (interface{}, error)) (interface{}, error)

// NewQueryBus cria um novo query bus.
func NewQueryBus() *QueryBus {
	return &QueryBus{
		handlers: make(map[string]interface{}),
	}
}

// RegisterQuery registra um handler para um tipo de query.
func RegisterQuery[Q Query, R any](bus *QueryBus, handler QueryHandler[Q, R]) {
	var zero Q
	bus.mu.Lock()
	defer bus.mu.Unlock()
	bus.handlers[zero.QueryName()] = handler
}

// RegisterQueryFunc registra uma função como handler de query.
func RegisterQueryFunc[Q Query, R any](bus *QueryBus, handler func(ctx context.Context, query Q) (R, error)) {
	RegisterQuery(bus, QueryHandlerFunc[Q, R](handler))
}

// Use adiciona um middleware para queries.
func (bus *QueryBus) Use(middleware QueryMiddleware) {
	bus.mu.Lock()
	defer bus.mu.Unlock()
	bus.middleware = append(bus.middleware, middleware)
}

// Ask executa uma query e retorna o resultado.
func Ask[Q Query, R any](ctx context.Context, bus *QueryBus, query Q) (R, error) {
	var zero R

	bus.mu.RLock()
	handler, exists := bus.handlers[query.QueryName()]
	middleware := bus.middleware
	bus.mu.RUnlock()

	if !exists {
		return zero, fmt.Errorf("no handler registered for query: %s", query.QueryName())
	}

	h, ok := handler.(QueryHandler[Q, R])
	if !ok {
		return zero, fmt.Errorf("invalid handler type for query: %s", query.QueryName())
	}

	// Constrói a cadeia de middleware
	final := func(ctx context.Context, q Query) (interface{}, error) {
		return h.Handle(ctx, q.(Q))
	}

	chain := final
	for i := len(middleware) - 1; i >= 0; i-- {
		mw := middleware[i]
		next := chain
		chain = func(ctx context.Context, q Query) (interface{}, error) {
			return mw(ctx, q, next)
		}
	}

	result, err := chain(ctx, query)
	if err != nil {
		return zero, err
	}

	return result.(R), nil
}

// BaseCommand fornece implementação base para comandos.
type BaseCommand struct {
	name string
}

// NewBaseCommand cria um comando base.
func NewBaseCommand(name string) BaseCommand {
	return BaseCommand{name: name}
}

// CommandName retorna o nome do comando.
func (c BaseCommand) CommandName() string {
	return c.name
}

// BaseQuery fornece implementação base para queries.
type BaseQuery struct {
	name string
}

// NewBaseQuery cria uma query base.
func NewBaseQuery(name string) BaseQuery {
	return BaseQuery{name: name}
}

// QueryName retorna o nome da query.
func (q BaseQuery) QueryName() string {
	return q.name
}

// ReadModel representa um read model para CQRS.
type ReadModel interface {
	TableName() string
}

// ReadModelProjection projeta eventos em read models.
type ReadModelProjection[T ReadModel] struct {
	handlers map[string]func(event interface{}, model *T) error
}

// NewReadModelProjection cria uma nova projeção.
func NewReadModelProjection[T ReadModel]() *ReadModelProjection[T] {
	return &ReadModelProjection[T]{
		handlers: make(map[string]func(event interface{}, model *T) error),
	}
}

// On registra um handler para um tipo de evento.
func (p *ReadModelProjection[T]) On(eventType string, handler func(event interface{}, model *T) error) {
	p.handlers[eventType] = handler
}

// Apply aplica um evento ao read model.
func (p *ReadModelProjection[T]) Apply(eventType string, event interface{}, model *T) error {
	handler, exists := p.handlers[eventType]
	if !exists {
		return nil // Ignora eventos sem handler
	}
	return handler(event, model)
}

// WriteRepository interface para escrita.
type WriteRepository[T any] interface {
	Save(ctx context.Context, entity T) error
	Delete(ctx context.Context, id string) error
}

// ReadRepository interface para leitura.
type ReadRepository[T any] interface {
	FindByID(ctx context.Context, id string) (T, error)
	FindAll(ctx context.Context) ([]T, error)
	FindBy(ctx context.Context, criteria map[string]interface{}) ([]T, error)
}

// CQRSRepository combina write e read repositories.
type CQRSRepository[T any] struct {
	write WriteRepository[T]
	read  ReadRepository[T]
}

// NewCQRSRepository cria um novo repository CQRS.
func NewCQRSRepository[T any](write WriteRepository[T], read ReadRepository[T]) *CQRSRepository[T] {
	return &CQRSRepository[T]{
		write: write,
		read:  read,
	}
}

// Write retorna o write repository.
func (r *CQRSRepository[T]) Write() WriteRepository[T] {
	return r.write
}

// Read retorna o read repository.
func (r *CQRSRepository[T]) Read() ReadRepository[T] {
	return r.read
}

// LoggingMiddleware é um middleware que loga comandos.
func LoggingMiddleware(logger func(string, ...interface{})) CommandMiddleware {
	return func(ctx context.Context, command Command, next func(ctx context.Context, command Command) error) error {
		logger("Executing command: %s", command.CommandName())
		err := next(ctx, command)
		if err != nil {
			logger("Command %s failed: %v", command.CommandName(), err)
		} else {
			logger("Command %s completed successfully", command.CommandName())
		}
		return err
	}
}

// ValidationMiddleware valida comandos antes de executar.
func ValidationMiddleware(validator func(Command) error) CommandMiddleware {
	return func(ctx context.Context, command Command, next func(ctx context.Context, command Command) error) error {
		if err := validator(command); err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}
		return next(ctx, command)
	}
}

// Validatable interface para comandos validáveis.
type Validatable interface {
	Validate() error
}

// AutoValidationMiddleware valida automaticamente comandos que implementam Validatable.
func AutoValidationMiddleware() CommandMiddleware {
	return func(ctx context.Context, command Command, next func(ctx context.Context, command Command) error) error {
		if v, ok := command.(Validatable); ok {
			if err := v.Validate(); err != nil {
				return fmt.Errorf("validation failed: %w", err)
			}
		}
		return next(ctx, command)
	}
}

// CachingQueryMiddleware adiciona cache a queries.
func CachingQueryMiddleware(cache interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{})
}) QueryMiddleware {
	return func(ctx context.Context, query Query, next func(ctx context.Context, query Query) (interface{}, error)) (interface{}, error) {
		key := fmt.Sprintf("query:%s:%v", query.QueryName(), query)

		if cached, ok := cache.Get(key); ok {
			return cached, nil
		}

		result, err := next(ctx, query)
		if err != nil {
			return nil, err
		}

		cache.Set(key, result)
		return result, nil
	}
}

// EventSourcedAggregate interface para aggregates com event sourcing.
type EventSourcedAggregate interface {
	GetID() string
	GetVersion() int64
	GetUncommittedEvents() []interface{}
	LoadFromHistory(events []interface{}) error
}

// Mediator centraliza command e query bus.
type Mediator struct {
	commandBus *CommandBus
	queryBus   *QueryBus
}

// NewMediator cria um novo mediator.
func NewMediator() *Mediator {
	return &Mediator{
		commandBus: NewCommandBus(),
		queryBus:   NewQueryBus(),
	}
}

// CommandBus retorna o command bus.
func (m *Mediator) CommandBus() *CommandBus {
	return m.commandBus
}

// QueryBus retorna o query bus.
func (m *Mediator) QueryBus() *QueryBus {
	return m.queryBus
}

// Send envia um comando.
func Send[C Command](ctx context.Context, m *Mediator, command C) error {
	return Dispatch(ctx, m.commandBus, command)
}

// Request faz uma query.
func Request[Q Query, R any](ctx context.Context, m *Mediator, query Q) (R, error) {
	return Ask[Q, R](ctx, m.queryBus, query)
}

// StructName retorna o nome de um struct.
func StructName(v interface{}) string {
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name()
}
