package cloud

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

// CloudProvider representa um provedor de cloud.
type CloudProvider string

const (
	CloudProviderAWS   CloudProvider = "aws"
	CloudProviderGCP   CloudProvider = "gcp"
	CloudProviderAzure CloudProvider = "azure"
)

// AuroraConfig configuração para Amazon Aurora.
type AuroraConfig struct {
	// Cluster endpoint (writer)
	ClusterEndpoint string
	// Reader endpoint (read replicas)
	ReaderEndpoint string
	// Database name
	Database string
	// Username
	Username string
	// Password
	Password string
	// Region AWS
	Region string
	// Use IAM authentication
	UseIAM bool
	// SSL mode
	SSLMode string // disable, require, verify-ca, verify-full
	// CA Certificate path (for verify-ca/verify-full)
	CACertPath string
	// Connection pool settings
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// AuroraAdapter adapter para Amazon Aurora (PostgreSQL/MySQL).
type AuroraAdapter struct {
	config  AuroraConfig
	writer  *sql.DB
	reader  *sql.DB
	dialect string // postgres ou mysql
}

// NewAuroraPostgresAdapter cria um adapter Aurora PostgreSQL.
func NewAuroraPostgresAdapter(config AuroraConfig) (*AuroraAdapter, error) {
	adapter := &AuroraAdapter{
		config:  config,
		dialect: "postgres",
	}

	// Conecta ao writer (cluster endpoint)
	writerDSN := adapter.buildPostgresDSN(config.ClusterEndpoint)
	writer, err := sql.Open("postgres", writerDSN)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Aurora writer: %w", err)
	}
	adapter.configurePool(writer)
	adapter.writer = writer

	// Conecta ao reader se disponível
	if config.ReaderEndpoint != "" {
		readerDSN := adapter.buildPostgresDSN(config.ReaderEndpoint)
		reader, err := sql.Open("postgres", readerDSN)
		if err != nil {
			writer.Close()
			return nil, fmt.Errorf("failed to connect to Aurora reader: %w", err)
		}
		adapter.configurePool(reader)
		adapter.reader = reader
	}

	return adapter, nil
}

// NewAuroraMySQLAdapter cria um adapter Aurora MySQL.
func NewAuroraMySQLAdapter(config AuroraConfig) (*AuroraAdapter, error) {
	adapter := &AuroraAdapter{
		config:  config,
		dialect: "mysql",
	}

	// Conecta ao writer
	writerDSN := adapter.buildMySQLDSN(config.ClusterEndpoint)
	writer, err := sql.Open("mysql", writerDSN)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Aurora writer: %w", err)
	}
	adapter.configurePool(writer)
	adapter.writer = writer

	// Conecta ao reader se disponível
	if config.ReaderEndpoint != "" {
		readerDSN := adapter.buildMySQLDSN(config.ReaderEndpoint)
		reader, err := sql.Open("mysql", readerDSN)
		if err != nil {
			writer.Close()
			return nil, fmt.Errorf("failed to connect to Aurora reader: %w", err)
		}
		adapter.configurePool(reader)
		adapter.reader = reader
	}

	return adapter, nil
}

func (a *AuroraAdapter) buildPostgresDSN(host string) string {
	sslMode := a.config.SSLMode
	if sslMode == "" {
		sslMode = "require"
	}

	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s sslmode=%s",
		host, a.config.Username, a.config.Password, a.config.Database, sslMode,
	)

	if a.config.CACertPath != "" {
		dsn += fmt.Sprintf(" sslrootcert=%s", a.config.CACertPath)
	}

	return dsn
}

func (a *AuroraAdapter) buildMySQLDSN(host string) string {
	tlsConfig := ""
	if a.config.SSLMode != "disable" {
		tlsConfig = "?tls=true"
	}

	return fmt.Sprintf(
		"%s:%s@tcp(%s)/%s%s",
		a.config.Username, a.config.Password, host, a.config.Database, tlsConfig,
	)
}

func (a *AuroraAdapter) configurePool(db *sql.DB) {
	if a.config.MaxOpenConns > 0 {
		db.SetMaxOpenConns(a.config.MaxOpenConns)
	} else {
		db.SetMaxOpenConns(25)
	}
	if a.config.MaxIdleConns > 0 {
		db.SetMaxIdleConns(a.config.MaxIdleConns)
	} else {
		db.SetMaxIdleConns(5)
	}
	if a.config.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(a.config.ConnMaxLifetime)
	} else {
		db.SetConnMaxLifetime(5 * time.Minute)
	}
	if a.config.ConnMaxIdleTime > 0 {
		db.SetConnMaxIdleTime(a.config.ConnMaxIdleTime)
	} else {
		db.SetConnMaxIdleTime(1 * time.Minute)
	}
}

// Writer retorna a conexão de escrita.
func (a *AuroraAdapter) Writer() *sql.DB {
	return a.writer
}

// Reader retorna a conexão de leitura.
func (a *AuroraAdapter) Reader() *sql.DB {
	if a.reader != nil {
		return a.reader
	}
	return a.writer
}

// Close fecha todas as conexões.
func (a *AuroraAdapter) Close() error {
	var errs []error
	if a.writer != nil {
		if err := a.writer.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if a.reader != nil {
		if err := a.reader.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors closing connections: %v", errs)
	}
	return nil
}

// CloudSQLConfig configuração para Google Cloud SQL.
type CloudSQLConfig struct {
	// Instance connection name (project:region:instance)
	InstanceConnectionName string
	// Database name
	Database string
	// Username
	Username string
	// Password
	Password string
	// Use private IP
	UsePrivateIP bool
	// Use IAM authentication
	UseIAM bool
	// Connection pool settings
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// CloudSQLAdapter adapter para Google Cloud SQL.
type CloudSQLAdapter struct {
	config  CloudSQLConfig
	db      *sql.DB
	dialect string
}

// NewCloudSQLPostgresAdapter cria um adapter Cloud SQL PostgreSQL.
func NewCloudSQLPostgresAdapter(config CloudSQLConfig) (*CloudSQLAdapter, error) {
	adapter := &CloudSQLAdapter{
		config:  config,
		dialect: "postgres",
	}

	// Cloud SQL usa Unix socket ou Cloud SQL Proxy
	// Formato: host=/cloudsql/PROJECT:REGION:INSTANCE
	dsn := fmt.Sprintf(
		"host=/cloudsql/%s user=%s password=%s dbname=%s sslmode=disable",
		config.InstanceConnectionName, config.Username, config.Password, config.Database,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Cloud SQL: %w", err)
	}

	adapter.configurePool(db)
	adapter.db = db

	return adapter, nil
}

// NewCloudSQLMySQLAdapter cria um adapter Cloud SQL MySQL.
func NewCloudSQLMySQLAdapter(config CloudSQLConfig) (*CloudSQLAdapter, error) {
	adapter := &CloudSQLAdapter{
		config:  config,
		dialect: "mysql",
	}

	// Cloud SQL MySQL usa unix socket
	dsn := fmt.Sprintf(
		"%s:%s@unix(/cloudsql/%s)/%s?parseTime=true",
		config.Username, config.Password, config.InstanceConnectionName, config.Database,
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Cloud SQL: %w", err)
	}

	adapter.configurePool(db)
	adapter.db = db

	return adapter, nil
}

func (c *CloudSQLAdapter) configurePool(db *sql.DB) {
	if c.config.MaxOpenConns > 0 {
		db.SetMaxOpenConns(c.config.MaxOpenConns)
	} else {
		db.SetMaxOpenConns(25)
	}
	if c.config.MaxIdleConns > 0 {
		db.SetMaxIdleConns(c.config.MaxIdleConns)
	} else {
		db.SetMaxIdleConns(5)
	}
	if c.config.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(c.config.ConnMaxLifetime)
	} else {
		db.SetConnMaxLifetime(5 * time.Minute)
	}
}

// DB retorna a conexão.
func (c *CloudSQLAdapter) DB() *sql.DB {
	return c.db
}

// Close fecha a conexão.
func (c *CloudSQLAdapter) Close() error {
	return c.db.Close()
}

// CockroachDBConfig configuração para CockroachDB.
type CockroachDBConfig struct {
	// Hosts (comma-separated para cluster)
	Hosts []string
	// Port
	Port int
	// Database name
	Database string
	// Username
	Username string
	// Password
	Password string
	// SSL mode
	SSLMode string // disable, require, verify-ca, verify-full
	// CA Certificate path
	CACertPath string
	// Client certificate path
	ClientCertPath string
	// Client key path
	ClientKeyPath string
	// Cluster name (for routing)
	ClusterName string
	// Connection pool settings
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// CockroachDBAdapter adapter para CockroachDB.
type CockroachDBAdapter struct {
	config CockroachDBConfig
	db     *sql.DB
}

// NewCockroachDBAdapter cria um adapter CockroachDB.
func NewCockroachDBAdapter(config CockroachDBConfig) (*CockroachDBAdapter, error) {
	adapter := &CockroachDBAdapter{
		config: config,
	}

	if config.Port == 0 {
		config.Port = 26257
	}
	if config.SSLMode == "" {
		config.SSLMode = "require"
	}

	// Constrói DSN
	host := strings.Join(config.Hosts, ",")
	dsn := fmt.Sprintf(
		"postgresql://%s:%s@%s:%d/%s?sslmode=%s",
		config.Username, config.Password, host, config.Port, config.Database, config.SSLMode,
	)

	if config.CACertPath != "" {
		dsn += fmt.Sprintf("&sslrootcert=%s", config.CACertPath)
	}
	if config.ClientCertPath != "" {
		dsn += fmt.Sprintf("&sslcert=%s", config.ClientCertPath)
	}
	if config.ClientKeyPath != "" {
		dsn += fmt.Sprintf("&sslkey=%s", config.ClientKeyPath)
	}
	if config.ClusterName != "" {
		dsn += fmt.Sprintf("&options=--cluster=%s", config.ClusterName)
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to CockroachDB: %w", err)
	}

	adapter.configurePool(db)
	adapter.db = db

	return adapter, nil
}

func (c *CockroachDBAdapter) configurePool(db *sql.DB) {
	if c.config.MaxOpenConns > 0 {
		db.SetMaxOpenConns(c.config.MaxOpenConns)
	} else {
		db.SetMaxOpenConns(25)
	}
	if c.config.MaxIdleConns > 0 {
		db.SetMaxIdleConns(c.config.MaxIdleConns)
	} else {
		db.SetMaxIdleConns(5)
	}
	if c.config.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(c.config.ConnMaxLifetime)
	} else {
		db.SetConnMaxLifetime(30 * time.Minute)
	}
}

// DB retorna a conexão.
func (c *CockroachDBAdapter) DB() *sql.DB {
	return c.db
}

// Close fecha a conexão.
func (c *CockroachDBAdapter) Close() error {
	return c.db.Close()
}

// RunTransaction executa uma transação com retry automático para erros de serialização.
func (c *CockroachDBAdapter) RunTransaction(ctx context.Context, fn func(*sql.Tx) error) error {
	const maxRetries = 10

	for i := 0; i < maxRetries; i++ {
		tx, err := c.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}

		err = fn(tx)
		if err != nil {
			tx.Rollback()
			// CockroachDB retorna código 40001 para erros de serialização
			if isCockroachRetryError(err) {
				continue
			}
			return err
		}

		err = tx.Commit()
		if err != nil {
			if isCockroachRetryError(err) {
				continue
			}
			return err
		}

		return nil
	}

	return fmt.Errorf("transaction failed after %d retries", maxRetries)
}

func isCockroachRetryError(err error) bool {
	return strings.Contains(err.Error(), "40001") ||
		strings.Contains(err.Error(), "retry transaction") ||
		strings.Contains(err.Error(), "TransactionRetryError")
}

// PlanetScaleConfig configuração para PlanetScale (MySQL serverless).
type PlanetScaleConfig struct {
	// Host
	Host string
	// Database name
	Database string
	// Username
	Username string
	// Password
	Password string
	// Branch (main, development, etc)
	Branch string
	// Connection pool settings
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// PlanetScaleAdapter adapter para PlanetScale.
type PlanetScaleAdapter struct {
	config PlanetScaleConfig
	db     *sql.DB
}

// NewPlanetScaleAdapter cria um adapter PlanetScale.
func NewPlanetScaleAdapter(config PlanetScaleConfig) (*PlanetScaleAdapter, error) {
	adapter := &PlanetScaleAdapter{
		config: config,
	}

	// PlanetScale usa conexão TLS obrigatória
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s)/%s?tls=true&parseTime=true",
		config.Username, config.Password, config.Host, config.Database,
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to PlanetScale: %w", err)
	}

	// PlanetScale recomenda conexões de curta duração
	if config.MaxOpenConns > 0 {
		db.SetMaxOpenConns(config.MaxOpenConns)
	} else {
		db.SetMaxOpenConns(10)
	}
	if config.MaxIdleConns > 0 {
		db.SetMaxIdleConns(config.MaxIdleConns)
	} else {
		db.SetMaxIdleConns(2)
	}
	if config.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(config.ConnMaxLifetime)
	} else {
		db.SetConnMaxLifetime(3 * time.Minute)
	}

	adapter.db = db
	return adapter, nil
}

// DB retorna a conexão.
func (p *PlanetScaleAdapter) DB() *sql.DB {
	return p.db
}

// Close fecha a conexão.
func (p *PlanetScaleAdapter) Close() error {
	return p.db.Close()
}

// NeonConfig configuração para Neon (PostgreSQL serverless).
type NeonConfig struct {
	// Connection string completa
	ConnectionString string
	// Ou componentes individuais:
	Host     string
	Database string
	Username string
	Password string
	// SSL mode (sempre requer SSL)
	SSLMode string
	// Pooler mode: transaction ou session
	PoolerMode string
	// Connection pool settings
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// NeonAdapter adapter para Neon (PostgreSQL serverless).
type NeonAdapter struct {
	config NeonConfig
	db     *sql.DB
}

// NewNeonAdapter cria um adapter Neon.
func NewNeonAdapter(config NeonConfig) (*NeonAdapter, error) {
	adapter := &NeonAdapter{
		config: config,
	}

	var dsn string
	if config.ConnectionString != "" {
		dsn = config.ConnectionString
	} else {
		sslMode := config.SSLMode
		if sslMode == "" {
			sslMode = "require"
		}
		dsn = fmt.Sprintf(
			"postgres://%s:%s@%s/%s?sslmode=%s",
			config.Username, config.Password, config.Host, config.Database, sslMode,
		)
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Neon: %w", err)
	}

	// Neon serverless - conexões de curta duração
	if config.MaxOpenConns > 0 {
		db.SetMaxOpenConns(config.MaxOpenConns)
	} else {
		db.SetMaxOpenConns(10)
	}
	if config.MaxIdleConns > 0 {
		db.SetMaxIdleConns(config.MaxIdleConns)
	} else {
		db.SetMaxIdleConns(2)
	}
	if config.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(config.ConnMaxLifetime)
	} else {
		db.SetConnMaxLifetime(5 * time.Minute)
	}

	adapter.db = db
	return adapter, nil
}

// DB retorna a conexão.
func (n *NeonAdapter) DB() *sql.DB {
	return n.db
}

// Close fecha a conexão.
func (n *NeonAdapter) Close() error {
	return n.db.Close()
}

// TLSConfig cria configuração TLS para conexões seguras.
func TLSConfig(caCertPath, clientCertPath, clientKeyPath string, serverName string) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		ServerName: serverName,
	}

	// Carrega CA certificate
	if caCertPath != "" {
		caCert, err := os.ReadFile(caCertPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate: %w", err)
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}
		tlsConfig.RootCAs = caCertPool
	}

	// Carrega client certificate
	if clientCertPath != "" && clientKeyPath != "" {
		cert, err := tls.LoadX509KeyPair(clientCertPath, clientKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return tlsConfig, nil
}

// ResolveCloudEndpoint resolve o endpoint do banco de dados em nuvem.
func ResolveCloudEndpoint(endpoint string) ([]string, error) {
	// Resolve DNS para obter todos os IPs (útil para load balancing)
	ips, err := net.LookupHost(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve endpoint: %w", err)
	}
	return ips, nil
}
