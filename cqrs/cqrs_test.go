package cqrs

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

// ========================================
// Test Commands and Queries
// ========================================

type CreateUserCommand struct {
	Name  string
	Email string
}

func (c CreateUserCommand) CommandName() string {
	return "CreateUser"
}

type ValidatableCommand struct {
	valid bool
}

func (c ValidatableCommand) CommandName() string {
	return "Validatable"
}

func (c ValidatableCommand) Validate() error {
	if !c.valid {
		return errors.New("invalid command")
	}
	return nil
}

type GetUserQuery struct {
	ID string
}

func (q GetUserQuery) QueryName() string {
	return "GetUser"
}

type UserResult struct {
	ID   string
	Name string
}

// ========================================
// Mock Repositories
// ========================================

type mockWriteRepo struct {
	saveCalled   bool
	deleteCalled bool
	saveErr      error
	deleteErr    error
}

func (r *mockWriteRepo) Save(ctx context.Context, entity UserResult) error {
	r.saveCalled = true
	return r.saveErr
}

func (r *mockWriteRepo) Delete(ctx context.Context, id string) error {
	r.deleteCalled = true
	return r.deleteErr
}

type mockReadRepo struct {
	findByIDResult UserResult
	findByIDErr    error
	findAllResult  []UserResult
	findAllErr     error
	findByResult   []UserResult
	findByErr      error
}

func (r *mockReadRepo) FindByID(ctx context.Context, id string) (UserResult, error) {
	return r.findByIDResult, r.findByIDErr
}

func (r *mockReadRepo) FindAll(ctx context.Context) ([]UserResult, error) {
	return r.findAllResult, r.findAllErr
}

func (r *mockReadRepo) FindBy(ctx context.Context, criteria map[string]interface{}) ([]UserResult, error) {
	return r.findByResult, r.findByErr
}

// Mock cache
type mockCache struct {
	data map[string]interface{}
}

func newMockCache() *mockCache {
	return &mockCache{data: make(map[string]interface{})}
}

func (c *mockCache) Get(key string) (interface{}, bool) {
	v, ok := c.data[key]
	return v, ok
}

func (c *mockCache) Set(key string, value interface{}) {
	c.data[key] = value
}

// Mock ReadModel
type TestReadModel struct {
	ID   string
	Name string
}

func (m TestReadModel) TableName() string {
	return "test_read_models"
}

// ========================================
// Tests: BaseCommand and BaseQuery
// ========================================

func TestNewBaseCommand(t *testing.T) {
	cmd := NewBaseCommand("CreateUser")
	if cmd.CommandName() != "CreateUser" {
		t.Errorf("CommandName() = %q, want %q", cmd.CommandName(), "CreateUser")
	}
}

func TestNewBaseQuery(t *testing.T) {
	q := NewBaseQuery("GetUser")
	if q.QueryName() != "GetUser" {
		t.Errorf("QueryName() = %q, want %q", q.QueryName(), "GetUser")
	}
}

// ========================================
// Tests: CommandBus
// ========================================

func TestNewCommandBus(t *testing.T) {
	bus := NewCommandBus()
	if bus == nil {
		t.Fatal("NewCommandBus returned nil")
	}
	if bus.handlers == nil {
		t.Error("handlers map should be initialized")
	}
}

func TestRegister_AndDispatch(t *testing.T) {
	bus := NewCommandBus()
	var handled bool

	Register(bus, CommandHandlerFunc[CreateUserCommand](func(ctx context.Context, cmd CreateUserCommand) error {
		handled = true
		if cmd.Name != "Alice" {
			t.Errorf("Name = %q, want %q", cmd.Name, "Alice")
		}
		return nil
	}))

	cmd := CreateUserCommand{
		Name:  "Alice",
		Email: "alice@example.com",
	}

	err := Dispatch(context.Background(), bus, cmd)
	if err != nil {
		t.Errorf("Dispatch error = %v", err)
	}
	if !handled {
		t.Error("handler was not called")
	}
}

func TestRegisterFunc_AndDispatch(t *testing.T) {
	bus := NewCommandBus()
	var handled bool

	RegisterFunc(bus, func(ctx context.Context, cmd CreateUserCommand) error {
		handled = true
		return nil
	})

	cmd := CreateUserCommand{}
	err := Dispatch(context.Background(), bus, cmd)
	if err != nil {
		t.Errorf("Dispatch error = %v", err)
	}
	if !handled {
		t.Error("handler was not called")
	}
}

func TestDispatch_NoHandler(t *testing.T) {
	bus := NewCommandBus()
	cmd := CreateUserCommand{}
	err := Dispatch(context.Background(), bus, cmd)
	if err == nil {
		t.Error("expected error for missing handler")
	}
	if expected := "no handler registered for command: CreateUser"; err.Error() != expected {
		t.Errorf("error = %q, want %q", err.Error(), expected)
	}
}

func TestDispatch_InvalidHandlerType(t *testing.T) {
	bus := NewCommandBus()
	// Register with wrong type manually
	bus.handlers["CreateUser"] = "not a handler"

	cmd := CreateUserCommand{}
	err := Dispatch(context.Background(), bus, cmd)
	if err == nil {
		t.Error("expected error for invalid handler type")
	}
	if expected := "invalid handler type for command: CreateUser"; err.Error() != expected {
		t.Errorf("error = %q, want %q", err.Error(), expected)
	}
}

func TestDispatch_HandlerReturnsError(t *testing.T) {
	bus := NewCommandBus()
	expectedErr := errors.New("handler error")

	RegisterFunc(bus, func(ctx context.Context, cmd CreateUserCommand) error {
		return expectedErr
	})

	cmd := CreateUserCommand{}
	err := Dispatch(context.Background(), bus, cmd)
	if !errors.Is(err, expectedErr) {
		t.Errorf("error = %v, want %v", err, expectedErr)
	}
}

func TestCommandBus_Use_Middleware(t *testing.T) {
	bus := NewCommandBus()
	var order []string

	bus.Use(func(ctx context.Context, cmd Command, next func(ctx context.Context, cmd Command) error) error {
		order = append(order, "mw1-before")
		err := next(ctx, cmd)
		order = append(order, "mw1-after")
		return err
	})

	bus.Use(func(ctx context.Context, cmd Command, next func(ctx context.Context, cmd Command) error) error {
		order = append(order, "mw2-before")
		err := next(ctx, cmd)
		order = append(order, "mw2-after")
		return err
	})

	RegisterFunc(bus, func(ctx context.Context, cmd CreateUserCommand) error {
		order = append(order, "handler")
		return nil
	})

	cmd := CreateUserCommand{}
	err := Dispatch(context.Background(), bus, cmd)
	if err != nil {
		t.Errorf("Dispatch error = %v", err)
	}

	expected := []string{"mw1-before", "mw2-before", "handler", "mw2-after", "mw1-after"}
	if len(order) != len(expected) {
		t.Fatalf("order = %v, want %v", order, expected)
	}
	for i := range expected {
		if order[i] != expected[i] {
			t.Errorf("order[%d] = %q, want %q", i, order[i], expected[i])
		}
	}
}

// ========================================
// Tests: QueryBus
// ========================================

func TestNewQueryBus(t *testing.T) {
	bus := NewQueryBus()
	if bus == nil {
		t.Fatal("NewQueryBus returned nil")
	}
	if bus.handlers == nil {
		t.Error("handlers map should be initialized")
	}
}

func TestRegisterQuery_AndAsk(t *testing.T) {
	bus := NewQueryBus()

	RegisterQuery(bus, QueryHandlerFunc[GetUserQuery, UserResult](func(ctx context.Context, q GetUserQuery) (UserResult, error) {
		return UserResult{ID: q.ID, Name: "Alice"}, nil
	}))

	query := GetUserQuery{ID: "123"}
	result, err := Ask[GetUserQuery, UserResult](context.Background(), bus, query)
	if err != nil {
		t.Errorf("Ask error = %v", err)
	}
	if result.ID != "123" {
		t.Errorf("ID = %q, want %q", result.ID, "123")
	}
	if result.Name != "Alice" {
		t.Errorf("Name = %q, want %q", result.Name, "Alice")
	}
}

func TestRegisterQueryFunc_AndAsk(t *testing.T) {
	bus := NewQueryBus()

	RegisterQueryFunc(bus, func(ctx context.Context, q GetUserQuery) (UserResult, error) {
		return UserResult{ID: q.ID, Name: "Bob"}, nil
	})

	query := GetUserQuery{ID: "456"}
	result, err := Ask[GetUserQuery, UserResult](context.Background(), bus, query)
	if err != nil {
		t.Errorf("Ask error = %v", err)
	}
	if result.Name != "Bob" {
		t.Errorf("Name = %q, want %q", result.Name, "Bob")
	}
}

func TestAsk_NoHandler(t *testing.T) {
	bus := NewQueryBus()
	query := GetUserQuery{ID: "123"}
	_, err := Ask[GetUserQuery, UserResult](context.Background(), bus, query)
	if err == nil {
		t.Error("expected error for missing handler")
	}
	if expected := "no handler registered for query: GetUser"; err.Error() != expected {
		t.Errorf("error = %q, want %q", err.Error(), expected)
	}
}

func TestAsk_InvalidHandlerType(t *testing.T) {
	bus := NewQueryBus()
	bus.handlers["GetUser"] = "not a handler"

	query := GetUserQuery{ID: "123"}
	_, err := Ask[GetUserQuery, UserResult](context.Background(), bus, query)
	if err == nil {
		t.Error("expected error for invalid handler type")
	}
	if expected := "invalid handler type for query: GetUser"; err.Error() != expected {
		t.Errorf("error = %q, want %q", err.Error(), expected)
	}
}

func TestAsk_HandlerReturnsError(t *testing.T) {
	bus := NewQueryBus()
	expectedErr := errors.New("query error")

	RegisterQueryFunc(bus, func(ctx context.Context, q GetUserQuery) (UserResult, error) {
		return UserResult{}, expectedErr
	})

	query := GetUserQuery{ID: "123"}
	_, err := Ask[GetUserQuery, UserResult](context.Background(), bus, query)
	if !errors.Is(err, expectedErr) {
		t.Errorf("error = %v, want %v", err, expectedErr)
	}
}

func TestQueryBus_Use_Middleware(t *testing.T) {
	bus := NewQueryBus()
	var order []string

	bus.Use(func(ctx context.Context, q Query, next func(ctx context.Context, q Query) (interface{}, error)) (interface{}, error) {
		order = append(order, "mw-before")
		result, err := next(ctx, q)
		order = append(order, "mw-after")
		return result, err
	})

	RegisterQueryFunc(bus, func(ctx context.Context, q GetUserQuery) (UserResult, error) {
		order = append(order, "handler")
		return UserResult{Name: "Alice"}, nil
	})

	query := GetUserQuery{ID: "123"}
	result, err := Ask[GetUserQuery, UserResult](context.Background(), bus, query)
	if err != nil {
		t.Errorf("Ask error = %v", err)
	}
	if result.Name != "Alice" {
		t.Errorf("Name = %q, want %q", result.Name, "Alice")
	}

	expected := []string{"mw-before", "handler", "mw-after"}
	if len(order) != len(expected) {
		t.Fatalf("order = %v, want %v", order, expected)
	}
	for i := range expected {
		if order[i] != expected[i] {
			t.Errorf("order[%d] = %q, want %q", i, order[i], expected[i])
		}
	}
}

func TestQueryBus_Middleware_Error(t *testing.T) {
	bus := NewQueryBus()
	expectedErr := errors.New("middleware error")

	bus.Use(func(ctx context.Context, q Query, next func(ctx context.Context, q Query) (interface{}, error)) (interface{}, error) {
		return nil, expectedErr
	})

	RegisterQueryFunc(bus, func(ctx context.Context, q GetUserQuery) (UserResult, error) {
		return UserResult{}, nil
	})

	query := GetUserQuery{ID: "123"}
	_, err := Ask[GetUserQuery, UserResult](context.Background(), bus, query)
	if !errors.Is(err, expectedErr) {
		t.Errorf("error = %v, want %v", err, expectedErr)
	}
}

// ========================================
// Tests: ReadModelProjection
// ========================================

func TestNewReadModelProjection(t *testing.T) {
	proj := NewReadModelProjection[TestReadModel]()
	if proj == nil {
		t.Fatal("NewReadModelProjection returned nil")
	}
}

func TestReadModelProjection_On_And_Apply(t *testing.T) {
	proj := NewReadModelProjection[TestReadModel]()

	proj.On("UserCreated", func(event interface{}, model *TestReadModel) error {
		data := event.(map[string]string)
		model.Name = data["name"]
		return nil
	})

	model := TestReadModel{}
	err := proj.Apply("UserCreated", map[string]string{"name": "Alice"}, &model)
	if err != nil {
		t.Errorf("Apply error = %v", err)
	}
	if model.Name != "Alice" {
		t.Errorf("Name = %q, want %q", model.Name, "Alice")
	}
}

func TestReadModelProjection_Apply_NoHandler(t *testing.T) {
	proj := NewReadModelProjection[TestReadModel]()
	model := TestReadModel{}
	err := proj.Apply("UnknownEvent", nil, &model)
	if err != nil {
		t.Errorf("Apply should return nil for unknown events, got %v", err)
	}
}

func TestReadModelProjection_Apply_HandlerError(t *testing.T) {
	proj := NewReadModelProjection[TestReadModel]()
	expectedErr := errors.New("projection error")

	proj.On("UserCreated", func(event interface{}, model *TestReadModel) error {
		return expectedErr
	})

	model := TestReadModel{}
	err := proj.Apply("UserCreated", nil, &model)
	if !errors.Is(err, expectedErr) {
		t.Errorf("error = %v, want %v", err, expectedErr)
	}
}

// ========================================
// Tests: CQRSRepository
// ========================================

func TestNewCQRSRepository(t *testing.T) {
	write := &mockWriteRepo{}
	read := &mockReadRepo{}

	repo := NewCQRSRepository[UserResult](write, read)
	if repo == nil {
		t.Fatal("NewCQRSRepository returned nil")
	}
}

func TestCQRSRepository_Write(t *testing.T) {
	write := &mockWriteRepo{}
	read := &mockReadRepo{}
	repo := NewCQRSRepository[UserResult](write, read)

	w := repo.Write()
	if w != write {
		t.Error("Write() should return the write repository")
	}
}

func TestCQRSRepository_Read(t *testing.T) {
	write := &mockWriteRepo{}
	read := &mockReadRepo{}
	repo := NewCQRSRepository[UserResult](write, read)

	r := repo.Read()
	if r != read {
		t.Error("Read() should return the read repository")
	}
}

// ========================================
// Tests: Middleware Functions
// ========================================

func TestLoggingMiddleware_Success(t *testing.T) {
	var logs []string
	logger := func(format string, args ...interface{}) {
		logs = append(logs, fmt.Sprintf(format, args...))
	}

	bus := NewCommandBus()
	bus.Use(LoggingMiddleware(logger))

	RegisterFunc(bus, func(ctx context.Context, cmd CreateUserCommand) error {
		return nil
	})

	cmd := CreateUserCommand{}
	err := Dispatch(context.Background(), bus, cmd)
	if err != nil {
		t.Errorf("Dispatch error = %v", err)
	}

	if len(logs) != 2 {
		t.Fatalf("expected 2 log entries, got %d: %v", len(logs), logs)
	}
	if logs[0] != "Executing command: CreateUser" {
		t.Errorf("log[0] = %q", logs[0])
	}
	if logs[1] != "Command CreateUser completed successfully" {
		t.Errorf("log[1] = %q", logs[1])
	}
}

func TestLoggingMiddleware_Error(t *testing.T) {
	var logs []string
	logger := func(format string, args ...interface{}) {
		logs = append(logs, fmt.Sprintf(format, args...))
	}

	bus := NewCommandBus()
	bus.Use(LoggingMiddleware(logger))

	RegisterFunc(bus, func(ctx context.Context, cmd CreateUserCommand) error {
		return errors.New("fail")
	})

	cmd := CreateUserCommand{}
	_ = Dispatch(context.Background(), bus, cmd)

	if len(logs) != 2 {
		t.Fatalf("expected 2 log entries, got %d: %v", len(logs), logs)
	}
	if logs[1] != "Command CreateUser failed: fail" {
		t.Errorf("log[1] = %q", logs[1])
	}
}

func TestValidationMiddleware_Pass(t *testing.T) {
	bus := NewCommandBus()
	bus.Use(ValidationMiddleware(func(cmd Command) error {
		return nil
	}))

	var handled bool
	RegisterFunc(bus, func(ctx context.Context, cmd CreateUserCommand) error {
		handled = true
		return nil
	})

	cmd := CreateUserCommand{}
	err := Dispatch(context.Background(), bus, cmd)
	if err != nil {
		t.Errorf("Dispatch error = %v", err)
	}
	if !handled {
		t.Error("handler should have been called")
	}
}

func TestValidationMiddleware_Fail(t *testing.T) {
	bus := NewCommandBus()
	bus.Use(ValidationMiddleware(func(cmd Command) error {
		return errors.New("bad command")
	}))

	RegisterFunc(bus, func(ctx context.Context, cmd CreateUserCommand) error {
		return nil
	})

	cmd := CreateUserCommand{}
	err := Dispatch(context.Background(), bus, cmd)
	if err == nil {
		t.Error("expected validation error")
	}
	if expected := "validation failed: bad command"; err.Error() != expected {
		t.Errorf("error = %q, want %q", err.Error(), expected)
	}
}

func TestAutoValidationMiddleware_ValidCommand(t *testing.T) {
	bus := NewCommandBus()
	bus.Use(AutoValidationMiddleware())

	var handled bool
	RegisterFunc(bus, func(ctx context.Context, cmd ValidatableCommand) error {
		handled = true
		return nil
	})

	cmd := ValidatableCommand{valid: true}
	err := Dispatch(context.Background(), bus, cmd)
	if err != nil {
		t.Errorf("Dispatch error = %v", err)
	}
	if !handled {
		t.Error("handler should have been called")
	}
}

func TestAutoValidationMiddleware_InvalidCommand(t *testing.T) {
	bus := NewCommandBus()
	bus.Use(AutoValidationMiddleware())

	RegisterFunc(bus, func(ctx context.Context, cmd ValidatableCommand) error {
		return nil
	})

	cmd := ValidatableCommand{valid: false}
	err := Dispatch(context.Background(), bus, cmd)
	if err == nil {
		t.Error("expected validation error")
	}
	if expected := "validation failed: invalid command"; err.Error() != expected {
		t.Errorf("error = %q, want %q", err.Error(), expected)
	}
}

func TestAutoValidationMiddleware_NonValidatableCommand(t *testing.T) {
	bus := NewCommandBus()
	bus.Use(AutoValidationMiddleware())

	var handled bool
	RegisterFunc(bus, func(ctx context.Context, cmd CreateUserCommand) error {
		handled = true
		return nil
	})

	cmd := CreateUserCommand{}
	err := Dispatch(context.Background(), bus, cmd)
	if err != nil {
		t.Errorf("Dispatch error = %v", err)
	}
	if !handled {
		t.Error("handler should have been called for non-validatable command")
	}
}

func TestCachingQueryMiddleware_CacheHit(t *testing.T) {
	bus := NewQueryBus()
	cache := newMockCache()

	bus.Use(CachingQueryMiddleware(cache))

	var callCount int
	RegisterQueryFunc(bus, func(ctx context.Context, q GetUserQuery) (UserResult, error) {
		callCount++
		return UserResult{Name: "Alice"}, nil
	})

	query := GetUserQuery{ID: "123"}

	// First call
	result1, err := Ask[GetUserQuery, UserResult](context.Background(), bus, query)
	if err != nil {
		t.Errorf("Ask error = %v", err)
	}
	if result1.Name != "Alice" {
		t.Errorf("Name = %q, want %q", result1.Name, "Alice")
	}

	// Second call - should be cached (returns interface{} from cache middleware)
	result2, err := Ask[GetUserQuery, UserResult](context.Background(), bus, query)
	if err != nil {
		t.Errorf("Ask error = %v", err)
	}
	_ = result2

	if callCount != 1 {
		t.Errorf("handler called %d times, want 1 (cached)", callCount)
	}
}

func TestCachingQueryMiddleware_CacheMiss_Error(t *testing.T) {
	bus := NewQueryBus()
	cache := newMockCache()

	bus.Use(CachingQueryMiddleware(cache))

	expectedErr := errors.New("query error")
	RegisterQueryFunc(bus, func(ctx context.Context, q GetUserQuery) (UserResult, error) {
		return UserResult{}, expectedErr
	})

	query := GetUserQuery{ID: "123"}
	_, err := Ask[GetUserQuery, UserResult](context.Background(), bus, query)
	if !errors.Is(err, expectedErr) {
		t.Errorf("error = %v, want %v", err, expectedErr)
	}
}

// ========================================
// Tests: Mediator
// ========================================

func TestNewMediator(t *testing.T) {
	m := NewMediator()
	if m == nil {
		t.Fatal("NewMediator returned nil")
	}
	if m.CommandBus() == nil {
		t.Error("CommandBus() should not be nil")
	}
	if m.QueryBus() == nil {
		t.Error("QueryBus() should not be nil")
	}
}

func TestMediator_Send(t *testing.T) {
	m := NewMediator()
	var handled bool

	Register(m.CommandBus(), CommandHandlerFunc[CreateUserCommand](func(ctx context.Context, cmd CreateUserCommand) error {
		handled = true
		return nil
	}))

	cmd := CreateUserCommand{}
	err := Send(context.Background(), m, cmd)
	if err != nil {
		t.Errorf("Send error = %v", err)
	}
	if !handled {
		t.Error("handler was not called")
	}
}

func TestMediator_Request(t *testing.T) {
	m := NewMediator()

	RegisterQueryFunc(m.QueryBus(), func(ctx context.Context, q GetUserQuery) (UserResult, error) {
		return UserResult{ID: q.ID, Name: "Alice"}, nil
	})

	query := GetUserQuery{ID: "123"}
	result, err := Request[GetUserQuery, UserResult](context.Background(), m, query)
	if err != nil {
		t.Errorf("Request error = %v", err)
	}
	if result.Name != "Alice" {
		t.Errorf("Name = %q, want %q", result.Name, "Alice")
	}
}

// ========================================
// Tests: StructName
// ========================================

func TestStructName_Value(t *testing.T) {
	name := StructName(UserResult{})
	if name != "UserResult" {
		t.Errorf("StructName = %q, want %q", name, "UserResult")
	}
}

func TestStructName_Pointer(t *testing.T) {
	name := StructName(&UserResult{})
	if name != "UserResult" {
		t.Errorf("StructName = %q, want %q", name, "UserResult")
	}
}

// ========================================
// Tests: CommandHandlerFunc
// ========================================

func TestCommandHandlerFunc_Handle(t *testing.T) {
	var called bool
	handler := CommandHandlerFunc[CreateUserCommand](func(ctx context.Context, cmd CreateUserCommand) error {
		called = true
		return nil
	})

	err := handler.Handle(context.Background(), CreateUserCommand{})
	if err != nil {
		t.Errorf("Handle error = %v", err)
	}
	if !called {
		t.Error("handler function was not called")
	}
}

// ========================================
// Tests: QueryHandlerFunc
// ========================================

func TestQueryHandlerFunc_Handle(t *testing.T) {
	handler := QueryHandlerFunc[GetUserQuery, UserResult](func(ctx context.Context, q GetUserQuery) (UserResult, error) {
		return UserResult{Name: "test"}, nil
	})

	result, err := handler.Handle(context.Background(), GetUserQuery{})
	if err != nil {
		t.Errorf("Handle error = %v", err)
	}
	if result.Name != "test" {
		t.Errorf("Name = %q, want %q", result.Name, "test")
	}
}
