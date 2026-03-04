package cloud

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// ServerlessPoolConfig configuração para connection pooling serverless.
// Compatível com PgBouncer, pgcat, e outros poolers.
type ServerlessPoolConfig struct {
	// Pool mode: transaction, session, statement
	PoolMode PoolMode

	// Connection limits
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration

	// Serverless specific
	// WarmConnections mantém N conexões quentes
	WarmConnections int
	// ColdStartTimeout timeout para primeira conexão
	ColdStartTimeout time.Duration
	// ScaleDownDelay tempo antes de escalar para baixo
	ScaleDownDelay time.Duration

	// Health check
	HealthCheckInterval time.Duration
	HealthCheckTimeout  time.Duration

	// Retry
	MaxRetries     int
	RetryDelay     time.Duration
	RetryMaxDelay  time.Duration
	RetryMultiplier float64
}

// PoolMode modo de pooling.
type PoolMode string

const (
	// PoolModeTransaction libera conexão após cada transação
	PoolModeTransaction PoolMode = "transaction"
	// PoolModeSession mantém conexão durante a sessão
	PoolModeSession PoolMode = "session"
	// PoolModeStatement libera conexão após cada statement
	PoolModeStatement PoolMode = "statement"
)

// DefaultServerlessPoolConfig retorna configuração padrão para serverless.
func DefaultServerlessPoolConfig() ServerlessPoolConfig {
	return ServerlessPoolConfig{
		PoolMode:            PoolModeTransaction,
		MaxOpenConns:        10,
		MaxIdleConns:        2,
		ConnMaxLifetime:     5 * time.Minute,
		ConnMaxIdleTime:     30 * time.Second,
		WarmConnections:     1,
		ColdStartTimeout:    10 * time.Second,
		ScaleDownDelay:      30 * time.Second,
		HealthCheckInterval: 30 * time.Second,
		HealthCheckTimeout:  5 * time.Second,
		MaxRetries:          3,
		RetryDelay:          100 * time.Millisecond,
		RetryMaxDelay:       2 * time.Second,
		RetryMultiplier:     2.0,
	}
}

// ServerlessPool gerencia conexões para ambiente serverless.
type ServerlessPool struct {
	db        *sql.DB
	config    ServerlessPoolConfig
	mu        sync.RWMutex
	stats     ServerlessPoolStats
	warmConns []*sql.Conn
	stopChan  chan struct{}
	running   atomic.Bool
}

// ServerlessPoolStats estatísticas do pool.
type ServerlessPoolStats struct {
	TotalConnections   int64
	ActiveConnections  int64
	IdleConnections    int64
	WarmConnections    int64
	ColdStarts         int64
	FailedConnections  int64
	RetriedConnections int64
	TotalQueries       int64
	FailedQueries      int64
	AverageLatency     time.Duration
}

// NewServerlessPool cria um novo pool serverless.
func NewServerlessPool(db *sql.DB, config ServerlessPoolConfig) *ServerlessPool {
	// Aplica configurações ao sql.DB
	db.SetMaxOpenConns(config.MaxOpenConns)
	db.SetMaxIdleConns(config.MaxIdleConns)
	db.SetConnMaxLifetime(config.ConnMaxLifetime)
	db.SetConnMaxIdleTime(config.ConnMaxIdleTime)

	pool := &ServerlessPool{
		db:       db,
		config:   config,
		stopChan: make(chan struct{}),
	}

	return pool
}

// Start inicia o pool manager.
func (p *ServerlessPool) Start(ctx context.Context) error {
	if p.running.Load() {
		return nil
	}
	p.running.Store(true)

	// Aquece conexões iniciais
	if err := p.warmUp(ctx); err != nil {
		return fmt.Errorf("failed to warm up connections: %w", err)
	}

	// Inicia health check em background
	go p.healthCheckLoop()

	// Inicia scale down monitor
	go p.scaleDownLoop()

	return nil
}

// Stop para o pool manager.
func (p *ServerlessPool) Stop() error {
	if !p.running.Load() {
		return nil
	}
	p.running.Store(false)
	close(p.stopChan)

	// Fecha conexões quentes
	p.mu.Lock()
	for _, conn := range p.warmConns {
		conn.Close()
	}
	p.warmConns = nil
	p.mu.Unlock()

	return nil
}

// warmUp aquece conexões iniciais.
func (p *ServerlessPool) warmUp(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, p.config.ColdStartTimeout)
	defer cancel()

	for i := 0; i < p.config.WarmConnections; i++ {
		conn, err := p.db.Conn(ctx)
		if err != nil {
			atomic.AddInt64(&p.stats.ColdStarts, 1)
			return err
		}

		// Verifica se a conexão está saudável
		if err := conn.PingContext(ctx); err != nil {
			conn.Close()
			atomic.AddInt64(&p.stats.FailedConnections, 1)
			continue
		}

		p.warmConns = append(p.warmConns, conn)
		atomic.AddInt64(&p.stats.WarmConnections, 1)
	}

	return nil
}

// healthCheckLoop executa health checks periódicos.
func (p *ServerlessPool) healthCheckLoop() {
	ticker := time.NewTicker(p.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopChan:
			return
		case <-ticker.C:
			p.checkHealth()
		}
	}
}

// checkHealth verifica saúde das conexões.
func (p *ServerlessPool) checkHealth() {
	ctx, cancel := context.WithTimeout(context.Background(), p.config.HealthCheckTimeout)
	defer cancel()

	// Verifica conexões quentes
	p.mu.Lock()
	var healthyConns []*sql.Conn
	for _, conn := range p.warmConns {
		if err := conn.PingContext(ctx); err == nil {
			healthyConns = append(healthyConns, conn)
		} else {
			conn.Close()
			atomic.AddInt64(&p.stats.WarmConnections, -1)
		}
	}
	p.warmConns = healthyConns
	p.mu.Unlock()

	// Repõe conexões quentes se necessário
	p.mu.RLock()
	warmCount := len(p.warmConns)
	p.mu.RUnlock()

	if warmCount < p.config.WarmConnections {
		for i := warmCount; i < p.config.WarmConnections; i++ {
			conn, err := p.db.Conn(ctx)
			if err != nil {
				continue
			}
			p.mu.Lock()
			p.warmConns = append(p.warmConns, conn)
			atomic.AddInt64(&p.stats.WarmConnections, 1)
			p.mu.Unlock()
		}
	}
}

// scaleDownLoop monitora e escala para baixo quando apropriado.
func (p *ServerlessPool) scaleDownLoop() {
	ticker := time.NewTicker(p.config.ScaleDownDelay)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopChan:
			return
		case <-ticker.C:
			p.maybeScaleDown()
		}
	}
}

// maybeScaleDown escala para baixo conexões ociosas.
func (p *ServerlessPool) maybeScaleDown() {
	stats := p.db.Stats()

	// Se muitas conexões ociosas, fecha algumas
	if stats.Idle > p.config.MaxIdleConns {
		// sql.DB gerencia isso automaticamente via SetMaxIdleConns
		// mas podemos forçar fechamento de conexões quentes extras
		p.mu.Lock()
		for len(p.warmConns) > p.config.WarmConnections {
			conn := p.warmConns[len(p.warmConns)-1]
			p.warmConns = p.warmConns[:len(p.warmConns)-1]
			conn.Close()
			atomic.AddInt64(&p.stats.WarmConnections, -1)
		}
		p.mu.Unlock()
	}
}

// Conn obtém uma conexão do pool.
func (p *ServerlessPool) Conn(ctx context.Context) (*sql.Conn, error) {
	// Tenta pegar uma conexão quente primeiro
	p.mu.Lock()
	if len(p.warmConns) > 0 {
		conn := p.warmConns[len(p.warmConns)-1]
		p.warmConns = p.warmConns[:len(p.warmConns)-1]
		atomic.AddInt64(&p.stats.WarmConnections, -1)
		p.mu.Unlock()

		// Verifica se ainda está saudável
		if err := conn.PingContext(ctx); err == nil {
			atomic.AddInt64(&p.stats.ActiveConnections, 1)
			return conn, nil
		}
		conn.Close()
	} else {
		p.mu.Unlock()
	}

	// Cold start - nova conexão
	atomic.AddInt64(&p.stats.ColdStarts, 1)

	var conn *sql.Conn
	var err error

	// Retry com backoff
	delay := p.config.RetryDelay
	for i := 0; i <= p.config.MaxRetries; i++ {
		conn, err = p.db.Conn(ctx)
		if err == nil {
			atomic.AddInt64(&p.stats.ActiveConnections, 1)
			atomic.AddInt64(&p.stats.TotalConnections, 1)
			return conn, nil
		}

		if i < p.config.MaxRetries {
			atomic.AddInt64(&p.stats.RetriedConnections, 1)
			time.Sleep(delay)
			delay = time.Duration(float64(delay) * p.config.RetryMultiplier)
			if delay > p.config.RetryMaxDelay {
				delay = p.config.RetryMaxDelay
			}
		}
	}

	atomic.AddInt64(&p.stats.FailedConnections, 1)
	return nil, fmt.Errorf("failed to get connection after %d retries: %w", p.config.MaxRetries, err)
}

// ReleaseConn devolve uma conexão ao pool.
func (p *ServerlessPool) ReleaseConn(conn *sql.Conn) {
	atomic.AddInt64(&p.stats.ActiveConnections, -1)

	// Em modo transaction, a conexão é devolvida automaticamente
	// Em modo session, mantemos a conexão quente se possível
	p.mu.Lock()
	if len(p.warmConns) < p.config.WarmConnections {
		p.warmConns = append(p.warmConns, conn)
		atomic.AddInt64(&p.stats.WarmConnections, 1)
		p.mu.Unlock()
		return
	}
	p.mu.Unlock()

	// Fecha se não precisamos mais
	conn.Close()
}

// DB retorna o sql.DB subjacente.
func (p *ServerlessPool) DB() *sql.DB {
	return p.db
}

// Stats retorna estatísticas do pool.
func (p *ServerlessPool) Stats() ServerlessPoolStats {
	dbStats := p.db.Stats()

	return ServerlessPoolStats{
		TotalConnections:   atomic.LoadInt64(&p.stats.TotalConnections),
		ActiveConnections:  atomic.LoadInt64(&p.stats.ActiveConnections),
		IdleConnections:    int64(dbStats.Idle),
		WarmConnections:    atomic.LoadInt64(&p.stats.WarmConnections),
		ColdStarts:         atomic.LoadInt64(&p.stats.ColdStarts),
		FailedConnections:  atomic.LoadInt64(&p.stats.FailedConnections),
		RetriedConnections: atomic.LoadInt64(&p.stats.RetriedConnections),
		TotalQueries:       atomic.LoadInt64(&p.stats.TotalQueries),
		FailedQueries:      atomic.LoadInt64(&p.stats.FailedQueries),
	}
}

// ExecContext executa uma query com gerenciamento automático de conexão.
func (p *ServerlessPool) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	atomic.AddInt64(&p.stats.TotalQueries, 1)

	result, err := p.db.ExecContext(ctx, query, args...)
	if err != nil {
		atomic.AddInt64(&p.stats.FailedQueries, 1)
	}

	return result, err
}

// QueryContext executa uma query que retorna rows.
func (p *ServerlessPool) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	atomic.AddInt64(&p.stats.TotalQueries, 1)

	rows, err := p.db.QueryContext(ctx, query, args...)
	if err != nil {
		atomic.AddInt64(&p.stats.FailedQueries, 1)
	}

	return rows, err
}

// QueryRowContext executa uma query que retorna uma única row.
func (p *ServerlessPool) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	atomic.AddInt64(&p.stats.TotalQueries, 1)
	return p.db.QueryRowContext(ctx, query, args...)
}

// BeginTx inicia uma transação.
func (p *ServerlessPool) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return p.db.BeginTx(ctx, opts)
}

// PgBouncerConfig configuração específica para PgBouncer.
type PgBouncerConfig struct {
	// Host do PgBouncer
	Host string
	// Port do PgBouncer (default 6432)
	Port int
	// Database
	Database string
	// Username
	Username string
	// Password
	Password string
	// Pool mode
	PoolMode PoolMode
	// Application name (para identificação)
	ApplicationName string
}

// NewPgBouncerPool cria um pool configurado para PgBouncer.
func NewPgBouncerPool(config PgBouncerConfig) (*ServerlessPool, error) {
	if config.Port == 0 {
		config.Port = 6432
	}

	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		config.Host, config.Port, config.Username, config.Password, config.Database,
	)

	if config.ApplicationName != "" {
		dsn += fmt.Sprintf(" application_name=%s", config.ApplicationName)
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PgBouncer: %w", err)
	}

	// Configurações otimizadas para PgBouncer
	poolConfig := DefaultServerlessPoolConfig()
	poolConfig.PoolMode = config.PoolMode

	// Em modo transaction, não use prepared statements
	if config.PoolMode == PoolModeTransaction {
		poolConfig.MaxIdleConns = 1
	}

	return NewServerlessPool(db, poolConfig), nil
}

// PgCatConfig configuração para pgcat (Postgres connection pooler).
type PgCatConfig struct {
	Host     string
	Port     int
	Database string
	Username string
	Password string
	// Sharding key para multi-tenant
	ShardingKey string
}

// NewPgCatPool cria um pool configurado para pgcat.
func NewPgCatPool(config PgCatConfig) (*ServerlessPool, error) {
	if config.Port == 0 {
		config.Port = 6432
	}

	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		config.Host, config.Port, config.Username, config.Password, config.Database,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to pgcat: %w", err)
	}

	poolConfig := DefaultServerlessPoolConfig()
	return NewServerlessPool(db, poolConfig), nil
}

// ProxySQLConfig configuração para ProxySQL (MySQL).
type ProxySQLConfig struct {
	Host            string
	Port            int
	Database        string
	Username        string
	Password        string
	ApplicationName string
}

// NewProxySQLPool cria um pool configurado para ProxySQL.
func NewProxySQLPool(config ProxySQLConfig) (*ServerlessPool, error) {
	if config.Port == 0 {
		config.Port = 6033
	}

	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?parseTime=true",
		config.Username, config.Password, config.Host, config.Port, config.Database,
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to ProxySQL: %w", err)
	}

	poolConfig := DefaultServerlessPoolConfig()
	return NewServerlessPool(db, poolConfig), nil
}
