package cloud

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"database/sql"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ========================================
// AuroraAdapter Tests
// ========================================

func TestNewAuroraPostgresAdapter(t *testing.T) {
	config := AuroraConfig{
		ClusterEndpoint: "aurora-cluster.amazonaws.com",
		Database:        "testdb",
		Username:        "user",
		Password:        "pass",
		Region:          "us-east-1",
		SSLMode:         "require",
		MaxOpenConns:    10,
		MaxIdleConns:    3,
		ConnMaxLifetime: 10 * time.Minute,
		ConnMaxIdleTime: 2 * time.Minute,
	}

	adapter, err := NewAuroraPostgresAdapter(config)
	if err != nil {
		t.Fatalf("NewAuroraPostgresAdapter() error = %v", err)
	}
	defer adapter.Close()

	if adapter.writer == nil {
		t.Error("writer should not be nil")
	}
	if adapter.dialect != "postgres" {
		t.Errorf("dialect = %q, want %q", adapter.dialect, "postgres")
	}
	if adapter.reader != nil {
		t.Error("reader should be nil when ReaderEndpoint is empty")
	}
}

func TestNewAuroraPostgresAdapterWithReader(t *testing.T) {
	config := AuroraConfig{
		ClusterEndpoint: "aurora-cluster.amazonaws.com",
		ReaderEndpoint:  "aurora-reader.amazonaws.com",
		Database:        "testdb",
		Username:        "user",
		Password:        "pass",
	}

	adapter, err := NewAuroraPostgresAdapter(config)
	if err != nil {
		t.Fatalf("NewAuroraPostgresAdapter() error = %v", err)
	}
	defer adapter.Close()

	if adapter.reader == nil {
		t.Error("reader should not be nil when ReaderEndpoint is set")
	}
}

func TestNewAuroraMySQLAdapter(t *testing.T) {
	config := AuroraConfig{
		ClusterEndpoint: "aurora-cluster.amazonaws.com",
		Database:        "testdb",
		Username:        "user",
		Password:        "pass",
		SSLMode:         "disable",
	}

	adapter, err := NewAuroraMySQLAdapter(config)
	if err != nil {
		t.Fatalf("NewAuroraMySQLAdapter() error = %v", err)
	}
	defer adapter.Close()

	if adapter.writer == nil {
		t.Error("writer should not be nil")
	}
	if adapter.dialect != "mysql" {
		t.Errorf("dialect = %q, want %q", adapter.dialect, "mysql")
	}
}

func TestNewAuroraMySQLAdapterWithReader(t *testing.T) {
	config := AuroraConfig{
		ClusterEndpoint: "aurora-cluster.amazonaws.com",
		ReaderEndpoint:  "aurora-reader.amazonaws.com",
		Database:        "testdb",
		Username:        "user",
		Password:        "pass",
	}

	adapter, err := NewAuroraMySQLAdapter(config)
	if err != nil {
		t.Fatalf("NewAuroraMySQLAdapter() error = %v", err)
	}
	defer adapter.Close()

	if adapter.reader == nil {
		t.Error("reader should not be nil when ReaderEndpoint is set")
	}
}

func TestAuroraAdapter_Writer(t *testing.T) {
	db := getMockSQLDB()
	defer db.Close()

	adapter := &AuroraAdapter{writer: db}
	if adapter.Writer() != db {
		t.Error("Writer() should return the writer db")
	}
}

func TestAuroraAdapter_Reader_WithReader(t *testing.T) {
	writerDB := getMockSQLDB()
	defer writerDB.Close()
	readerDB := getMockSQLDB()
	defer readerDB.Close()

	adapter := &AuroraAdapter{writer: writerDB, reader: readerDB}
	if adapter.Reader() != readerDB {
		t.Error("Reader() should return the reader db when set")
	}
}

func TestAuroraAdapter_Reader_WithoutReader(t *testing.T) {
	writerDB := getMockSQLDB()
	defer writerDB.Close()

	adapter := &AuroraAdapter{writer: writerDB}
	if adapter.Reader() != writerDB {
		t.Error("Reader() should return the writer db when reader is nil")
	}
}

func TestAuroraAdapter_Close(t *testing.T) {
	writerDB := getMockSQLDB()
	readerDB := getMockSQLDB()

	adapter := &AuroraAdapter{writer: writerDB, reader: readerDB}
	err := adapter.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestAuroraAdapter_Close_NilConnections(t *testing.T) {
	adapter := &AuroraAdapter{}
	err := adapter.Close()
	if err != nil {
		t.Errorf("Close() with nil connections error = %v", err)
	}
}

func TestAuroraAdapter_buildPostgresDSN(t *testing.T) {
	adapter := &AuroraAdapter{
		config: AuroraConfig{
			Username: "user",
			Password: "pass",
			Database: "testdb",
			SSLMode:  "verify-full",
		},
	}

	dsn := adapter.buildPostgresDSN("myhost.com")
	if dsn != "host=myhost.com user=user password=pass dbname=testdb sslmode=verify-full" {
		t.Errorf("unexpected DSN: %s", dsn)
	}
}

func TestAuroraAdapter_buildPostgresDSN_DefaultSSL(t *testing.T) {
	adapter := &AuroraAdapter{
		config: AuroraConfig{
			Username: "user",
			Password: "pass",
			Database: "testdb",
		},
	}

	dsn := adapter.buildPostgresDSN("myhost.com")
	if dsn != "host=myhost.com user=user password=pass dbname=testdb sslmode=require" {
		t.Errorf("unexpected DSN: %s", dsn)
	}
}

func TestAuroraAdapter_buildPostgresDSN_WithCACert(t *testing.T) {
	adapter := &AuroraAdapter{
		config: AuroraConfig{
			Username:   "user",
			Password:   "pass",
			Database:   "testdb",
			SSLMode:    "verify-ca",
			CACertPath: "/path/to/ca.pem",
		},
	}

	dsn := adapter.buildPostgresDSN("myhost.com")
	expected := "host=myhost.com user=user password=pass dbname=testdb sslmode=verify-ca sslrootcert=/path/to/ca.pem"
	if dsn != expected {
		t.Errorf("unexpected DSN:\ngot:  %s\nwant: %s", dsn, expected)
	}
}

func TestAuroraAdapter_buildMySQLDSN(t *testing.T) {
	adapter := &AuroraAdapter{
		config: AuroraConfig{
			Username: "user",
			Password: "pass",
			Database: "testdb",
			SSLMode:  "require",
		},
	}

	dsn := adapter.buildMySQLDSN("myhost.com")
	expected := "user:pass@tcp(myhost.com)/testdb?tls=true"
	if dsn != expected {
		t.Errorf("unexpected DSN:\ngot:  %s\nwant: %s", dsn, expected)
	}
}

func TestAuroraAdapter_buildMySQLDSN_Disabled(t *testing.T) {
	adapter := &AuroraAdapter{
		config: AuroraConfig{
			Username: "user",
			Password: "pass",
			Database: "testdb",
			SSLMode:  "disable",
		},
	}

	dsn := adapter.buildMySQLDSN("myhost.com")
	expected := "user:pass@tcp(myhost.com)/testdb"
	if dsn != expected {
		t.Errorf("unexpected DSN:\ngot:  %s\nwant: %s", dsn, expected)
	}
}

func TestAuroraAdapter_configurePool_Defaults(t *testing.T) {
	db := getMockSQLDB()
	defer db.Close()

	adapter := &AuroraAdapter{config: AuroraConfig{}}
	adapter.configurePool(db)

	stats := db.Stats()
	if stats.MaxOpenConnections != 25 {
		t.Errorf("MaxOpenConnections = %d, want 25", stats.MaxOpenConnections)
	}
}

func TestAuroraAdapter_configurePool_Custom(t *testing.T) {
	db := getMockSQLDB()
	defer db.Close()

	adapter := &AuroraAdapter{
		config: AuroraConfig{
			MaxOpenConns:    50,
			MaxIdleConns:    10,
			ConnMaxLifetime: 10 * time.Minute,
			ConnMaxIdleTime: 2 * time.Minute,
		},
	}
	adapter.configurePool(db)

	stats := db.Stats()
	if stats.MaxOpenConnections != 50 {
		t.Errorf("MaxOpenConnections = %d, want 50", stats.MaxOpenConnections)
	}
}

// ========================================
// CloudSQLAdapter Tests
// ========================================

func TestNewCloudSQLPostgresAdapter(t *testing.T) {
	config := CloudSQLConfig{
		InstanceConnectionName: "project:region:instance",
		Database:               "testdb",
		Username:               "user",
		Password:               "pass",
		MaxOpenConns:           10,
		MaxIdleConns:           3,
		ConnMaxLifetime:        10 * time.Minute,
	}

	adapter, err := NewCloudSQLPostgresAdapter(config)
	if err != nil {
		t.Fatalf("NewCloudSQLPostgresAdapter() error = %v", err)
	}
	defer adapter.Close()

	if adapter.db == nil {
		t.Error("db should not be nil")
	}
	if adapter.dialect != "postgres" {
		t.Errorf("dialect = %q, want %q", adapter.dialect, "postgres")
	}
}

func TestNewCloudSQLMySQLAdapter(t *testing.T) {
	config := CloudSQLConfig{
		InstanceConnectionName: "project:region:instance",
		Database:               "testdb",
		Username:               "user",
		Password:               "pass",
	}

	adapter, err := NewCloudSQLMySQLAdapter(config)
	if err != nil {
		t.Fatalf("NewCloudSQLMySQLAdapter() error = %v", err)
	}
	defer adapter.Close()

	if adapter.db == nil {
		t.Error("db should not be nil")
	}
	if adapter.dialect != "mysql" {
		t.Errorf("dialect = %q, want %q", adapter.dialect, "mysql")
	}
}

func TestCloudSQLAdapter_DB(t *testing.T) {
	db := getMockSQLDB()
	defer db.Close()

	adapter := &CloudSQLAdapter{db: db}
	if adapter.DB() != db {
		t.Error("DB() should return the db")
	}
}

func TestCloudSQLAdapter_Close(t *testing.T) {
	db := getMockSQLDB()
	adapter := &CloudSQLAdapter{db: db}
	err := adapter.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestCloudSQLAdapter_configurePool_Defaults(t *testing.T) {
	db := getMockSQLDB()
	defer db.Close()

	adapter := &CloudSQLAdapter{config: CloudSQLConfig{}}
	adapter.configurePool(db)

	stats := db.Stats()
	if stats.MaxOpenConnections != 25 {
		t.Errorf("MaxOpenConnections = %d, want 25", stats.MaxOpenConnections)
	}
}

func TestCloudSQLAdapter_configurePool_Custom(t *testing.T) {
	db := getMockSQLDB()
	defer db.Close()

	adapter := &CloudSQLAdapter{
		config: CloudSQLConfig{
			MaxOpenConns:    30,
			MaxIdleConns:    8,
			ConnMaxLifetime: 15 * time.Minute,
		},
	}
	adapter.configurePool(db)

	stats := db.Stats()
	if stats.MaxOpenConnections != 30 {
		t.Errorf("MaxOpenConnections = %d, want 30", stats.MaxOpenConnections)
	}
}

// ========================================
// CockroachDBAdapter Tests
// ========================================

func TestNewCockroachDBAdapter(t *testing.T) {
	config := CockroachDBConfig{
		Hosts:    []string{"host1", "host2"},
		Port:     26257,
		Database: "testdb",
		Username: "user",
		Password: "pass",
		SSLMode:  "require",
	}

	adapter, err := NewCockroachDBAdapter(config)
	if err != nil {
		t.Fatalf("NewCockroachDBAdapter() error = %v", err)
	}
	defer adapter.Close()

	if adapter.db == nil {
		t.Error("db should not be nil")
	}
}

func TestNewCockroachDBAdapter_Defaults(t *testing.T) {
	config := CockroachDBConfig{
		Hosts:    []string{"localhost"},
		Database: "testdb",
		Username: "user",
		Password: "pass",
	}

	adapter, err := NewCockroachDBAdapter(config)
	if err != nil {
		t.Fatalf("NewCockroachDBAdapter() error = %v", err)
	}
	defer adapter.Close()

	if adapter.db == nil {
		t.Error("db should not be nil")
	}
}

func TestNewCockroachDBAdapter_WithCerts(t *testing.T) {
	config := CockroachDBConfig{
		Hosts:          []string{"host1"},
		Port:           26257,
		Database:       "testdb",
		Username:       "user",
		Password:       "pass",
		SSLMode:        "verify-full",
		CACertPath:     "/path/to/ca.crt",
		ClientCertPath: "/path/to/client.crt",
		ClientKeyPath:  "/path/to/client.key",
		ClusterName:    "my-cluster",
		MaxOpenConns:   50,
		MaxIdleConns:   10,
		ConnMaxLifetime: 30 * time.Minute,
	}

	adapter, err := NewCockroachDBAdapter(config)
	if err != nil {
		t.Fatalf("NewCockroachDBAdapter() error = %v", err)
	}
	defer adapter.Close()

	if adapter.db == nil {
		t.Error("db should not be nil")
	}
}

func TestCockroachDBAdapter_DB(t *testing.T) {
	db := getMockSQLDB()
	defer db.Close()

	adapter := &CockroachDBAdapter{db: db}
	if adapter.DB() != db {
		t.Error("DB() should return the db")
	}
}

func TestCockroachDBAdapter_Close(t *testing.T) {
	db := getMockSQLDB()
	adapter := &CockroachDBAdapter{db: db}
	err := adapter.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestCockroachDBAdapter_configurePool_Defaults(t *testing.T) {
	db := getMockSQLDB()
	defer db.Close()

	adapter := &CockroachDBAdapter{config: CockroachDBConfig{}}
	adapter.configurePool(db)

	stats := db.Stats()
	if stats.MaxOpenConnections != 25 {
		t.Errorf("MaxOpenConnections = %d, want 25", stats.MaxOpenConnections)
	}
}

func TestCockroachDBAdapter_configurePool_Custom(t *testing.T) {
	db := getMockSQLDB()
	defer db.Close()

	adapter := &CockroachDBAdapter{
		config: CockroachDBConfig{
			MaxOpenConns:    40,
			MaxIdleConns:    15,
			ConnMaxLifetime: 20 * time.Minute,
		},
	}
	adapter.configurePool(db)

	stats := db.Stats()
	if stats.MaxOpenConnections != 40 {
		t.Errorf("MaxOpenConnections = %d, want 40", stats.MaxOpenConnections)
	}
}

func TestCockroachDBAdapter_RunTransaction_Success(t *testing.T) {
	db := getMockSQLDB()
	defer db.Close()

	adapter := &CockroachDBAdapter{db: db}
	called := false
	err := adapter.RunTransaction(t.Context(), func(tx *sql.Tx) error {
		called = true
		return nil
	})

	if err != nil {
		t.Errorf("RunTransaction() error = %v", err)
	}
	if !called {
		t.Error("transaction function was not called")
	}
}

func TestCockroachDBAdapter_RunTransaction_FnError(t *testing.T) {
	db := getMockSQLDB()
	defer db.Close()

	adapter := &CockroachDBAdapter{db: db}
	err := adapter.RunTransaction(t.Context(), func(tx *sql.Tx) error {
		return errTest
	})

	if err != errTest {
		t.Errorf("RunTransaction() error = %v, want %v", err, errTest)
	}
}

func TestCockroachDBAdapter_RunTransaction_RetryOnSerializationError(t *testing.T) {
	db := getMockSQLDBRetryError(2)
	defer db.Close()

	adapter := &CockroachDBAdapter{db: db}
	err := adapter.RunTransaction(t.Context(), func(tx *sql.Tx) error {
		return nil
	})

	if err != nil {
		t.Errorf("RunTransaction() should succeed after retries, error = %v", err)
	}
}

func TestCockroachDBAdapter_RunTransaction_BeginError(t *testing.T) {
	db := getMockSQLDBFailBegin()
	defer db.Close()

	adapter := &CockroachDBAdapter{db: db}
	err := adapter.RunTransaction(t.Context(), func(tx *sql.Tx) error {
		return nil
	})

	if err == nil {
		t.Error("RunTransaction() should fail when Begin fails")
	}
}

func TestIsCockroachRetryError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"40001 error", errors.New("ERROR: 40001 serialization failure"), true},
		{"retry transaction", errors.New("restart transaction: retry transaction"), true},
		{"TransactionRetryError", errors.New("TransactionRetryError: something"), true},
		{"normal error", errors.New("some other error"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isCockroachRetryError(tt.err)
			if got != tt.want {
				t.Errorf("isCockroachRetryError() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ========================================
// PlanetScaleAdapter Tests
// ========================================

func TestNewPlanetScaleAdapter(t *testing.T) {
	config := PlanetScaleConfig{
		Host:            "aws.connect.psdb.cloud",
		Database:        "testdb",
		Username:        "user",
		Password:        "pass",
		MaxOpenConns:    5,
		MaxIdleConns:    1,
		ConnMaxLifetime: 2 * time.Minute,
	}

	adapter, err := NewPlanetScaleAdapter(config)
	if err != nil {
		t.Fatalf("NewPlanetScaleAdapter() error = %v", err)
	}
	defer adapter.Close()

	if adapter.db == nil {
		t.Error("db should not be nil")
	}
}

func TestNewPlanetScaleAdapter_Defaults(t *testing.T) {
	config := PlanetScaleConfig{
		Host:     "aws.connect.psdb.cloud",
		Database: "testdb",
		Username: "user",
		Password: "pass",
	}

	adapter, err := NewPlanetScaleAdapter(config)
	if err != nil {
		t.Fatalf("NewPlanetScaleAdapter() error = %v", err)
	}
	defer adapter.Close()

	stats := adapter.db.Stats()
	if stats.MaxOpenConnections != 10 {
		t.Errorf("MaxOpenConnections = %d, want 10", stats.MaxOpenConnections)
	}
}

func TestPlanetScaleAdapter_DB(t *testing.T) {
	db := getMockSQLDB()
	defer db.Close()

	adapter := &PlanetScaleAdapter{db: db}
	if adapter.DB() != db {
		t.Error("DB() should return the db")
	}
}

func TestPlanetScaleAdapter_Close(t *testing.T) {
	db := getMockSQLDB()
	adapter := &PlanetScaleAdapter{db: db}
	err := adapter.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

// ========================================
// NeonAdapter Tests
// ========================================

func TestNewNeonAdapter_WithConnectionString(t *testing.T) {
	config := NeonConfig{
		ConnectionString: "postgres://user:pass@host/db?sslmode=require",
		MaxOpenConns:     5,
		MaxIdleConns:     1,
		ConnMaxLifetime:  3 * time.Minute,
	}

	adapter, err := NewNeonAdapter(config)
	if err != nil {
		t.Fatalf("NewNeonAdapter() error = %v", err)
	}
	defer adapter.Close()

	if adapter.db == nil {
		t.Error("db should not be nil")
	}
}

func TestNewNeonAdapter_WithComponents(t *testing.T) {
	config := NeonConfig{
		Host:     "ep-cool-name-123456.us-east-2.aws.neon.tech",
		Database: "testdb",
		Username: "user",
		Password: "pass",
		SSLMode:  "verify-full",
	}

	adapter, err := NewNeonAdapter(config)
	if err != nil {
		t.Fatalf("NewNeonAdapter() error = %v", err)
	}
	defer adapter.Close()

	if adapter.db == nil {
		t.Error("db should not be nil")
	}
}

func TestNewNeonAdapter_DefaultSSL(t *testing.T) {
	config := NeonConfig{
		Host:     "host.neon.tech",
		Database: "testdb",
		Username: "user",
		Password: "pass",
	}

	adapter, err := NewNeonAdapter(config)
	if err != nil {
		t.Fatalf("NewNeonAdapter() error = %v", err)
	}
	defer adapter.Close()
}

func TestNewNeonAdapter_Defaults(t *testing.T) {
	config := NeonConfig{
		Host:     "host.neon.tech",
		Database: "testdb",
		Username: "user",
		Password: "pass",
	}

	adapter, err := NewNeonAdapter(config)
	if err != nil {
		t.Fatalf("NewNeonAdapter() error = %v", err)
	}
	defer adapter.Close()

	stats := adapter.db.Stats()
	if stats.MaxOpenConnections != 10 {
		t.Errorf("MaxOpenConnections = %d, want 10", stats.MaxOpenConnections)
	}
}

func TestNeonAdapter_DB(t *testing.T) {
	db := getMockSQLDB()
	defer db.Close()

	adapter := &NeonAdapter{db: db}
	if adapter.DB() != db {
		t.Error("DB() should return the db")
	}
}

func TestNeonAdapter_Close(t *testing.T) {
	db := getMockSQLDB()
	adapter := &NeonAdapter{db: db}
	err := adapter.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

// ========================================
// TLSConfig Tests
// ========================================

func TestTLSConfig_EmptyPaths(t *testing.T) {
	tlsCfg, err := TLSConfig("", "", "", "example.com")
	if err != nil {
		t.Fatalf("TLSConfig() error = %v", err)
	}
	if tlsCfg.ServerName != "example.com" {
		t.Errorf("ServerName = %q, want %q", tlsCfg.ServerName, "example.com")
	}
	if tlsCfg.RootCAs != nil {
		t.Error("RootCAs should be nil when no CA cert path")
	}
	if len(tlsCfg.Certificates) != 0 {
		t.Error("Certificates should be empty when no client cert paths")
	}
}

func TestTLSConfig_InvalidCACertPath(t *testing.T) {
	_, err := TLSConfig("/nonexistent/ca.pem", "", "", "example.com")
	if err == nil {
		t.Error("TLSConfig() should fail with invalid CA cert path")
	}
}

func TestTLSConfig_InvalidCACertContent(t *testing.T) {
	// Create a temp file with invalid PEM content
	tmpDir := t.TempDir()
	caPath := filepath.Join(tmpDir, "ca.pem")
	if err := os.WriteFile(caPath, []byte("not a valid pem"), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := TLSConfig(caPath, "", "", "example.com")
	if err == nil {
		t.Error("TLSConfig() should fail with invalid CA cert content")
	}
}

func TestTLSConfig_WithValidCACert(t *testing.T) {
	tmpDir := t.TempDir()
	caPath, _, _ := generateTestCerts(t, tmpDir)

	tlsCfg, err := TLSConfig(caPath, "", "", "example.com")
	if err != nil {
		t.Fatalf("TLSConfig() error = %v", err)
	}
	if tlsCfg.RootCAs == nil {
		t.Error("RootCAs should not be nil")
	}
}

func TestTLSConfig_WithClientCerts(t *testing.T) {
	tmpDir := t.TempDir()
	caPath, certPath, keyPath := generateTestCerts(t, tmpDir)

	tlsCfg, err := TLSConfig(caPath, certPath, keyPath, "example.com")
	if err != nil {
		t.Fatalf("TLSConfig() error = %v", err)
	}
	if len(tlsCfg.Certificates) != 1 {
		t.Errorf("Certificates count = %d, want 1", len(tlsCfg.Certificates))
	}
}

func TestTLSConfig_InvalidClientCert(t *testing.T) {
	_, err := TLSConfig("", "/nonexistent/cert.pem", "/nonexistent/key.pem", "example.com")
	if err == nil {
		t.Error("TLSConfig() should fail with invalid client cert paths")
	}
}

// ========================================
// ResolveCloudEndpoint Tests
// ========================================

func TestResolveCloudEndpoint_Localhost(t *testing.T) {
	ips, err := ResolveCloudEndpoint("localhost")
	if err != nil {
		t.Fatalf("ResolveCloudEndpoint() error = %v", err)
	}
	if len(ips) == 0 {
		t.Error("should resolve at least one IP for localhost")
	}
}

func TestResolveCloudEndpoint_InvalidHost(t *testing.T) {
	_, err := ResolveCloudEndpoint("this-host-definitely-does-not-exist-12345.invalid")
	if err == nil {
		t.Error("ResolveCloudEndpoint() should fail for invalid host")
	}
}

// ========================================
// CloudProvider Constants Tests
// ========================================

func TestCloudProviderConstants(t *testing.T) {
	if CloudProviderAWS != "aws" {
		t.Errorf("CloudProviderAWS = %q, want %q", CloudProviderAWS, "aws")
	}
	if CloudProviderGCP != "gcp" {
		t.Errorf("CloudProviderGCP = %q, want %q", CloudProviderGCP, "gcp")
	}
	if CloudProviderAzure != "azure" {
		t.Errorf("CloudProviderAzure = %q, want %q", CloudProviderAzure, "azure")
	}
}

// ========================================
// AuroraAdapter Close with errors Tests
// ========================================

func TestAuroraAdapter_Close_WriterError(t *testing.T) {
	writerDB := getMockSQLDBFailClose()
	adapter := &AuroraAdapter{writer: writerDB}
	err := adapter.Close()
	if err == nil {
		t.Error("Close() should return error when writer close fails")
	}
}

func TestAuroraAdapter_Close_ReaderError(t *testing.T) {
	writerDB := getMockSQLDB()
	readerDB := getMockSQLDBFailClose()

	adapter := &AuroraAdapter{writer: writerDB, reader: readerDB}
	err := adapter.Close()
	if err == nil {
		t.Error("Close() should return error when reader close fails")
	}
}

func TestAuroraAdapter_Close_BothErrors(t *testing.T) {
	writerDB := getMockSQLDBFailClose()
	readerDB := getMockSQLDBFailClose()

	adapter := &AuroraAdapter{writer: writerDB, reader: readerDB}
	err := adapter.Close()
	if err == nil {
		t.Error("Close() should return error when both fail")
	}
}

// ========================================
// RunTransaction max retries Tests
// ========================================

func TestCockroachDBAdapter_RunTransaction_MaxRetries(t *testing.T) {
	// Use a retry error driver that always fails
	db := getMockSQLDBRetryError(100) // always fail
	defer db.Close()

	adapter := &CockroachDBAdapter{db: db}
	err := adapter.RunTransaction(t.Context(), func(tx *sql.Tx) error {
		return nil
	})

	if err == nil {
		t.Error("RunTransaction() should fail after max retries")
	}
}

func TestCockroachDBAdapter_RunTransaction_FnRetryError(t *testing.T) {
	db := getMockSQLDB()
	defer db.Close()

	adapter := &CockroachDBAdapter{db: db}
	callCount := 0
	err := adapter.RunTransaction(t.Context(), func(tx *sql.Tx) error {
		callCount++
		if callCount <= 2 {
			return errors.New("TransactionRetryError: please retry")
		}
		return nil
	})

	if err != nil {
		t.Errorf("RunTransaction() error = %v, should succeed after retries", err)
	}
	if callCount != 3 {
		t.Errorf("callCount = %d, want 3", callCount)
	}
}

func TestCockroachDBAdapter_RunTransaction_CommitRetryError(t *testing.T) {
	db := getMockSQLDBRetryError(1) // fail once on commit then succeed
	defer db.Close()

	adapter := &CockroachDBAdapter{db: db}
	err := adapter.RunTransaction(t.Context(), func(tx *sql.Tx) error {
		return nil
	})

	if err != nil {
		t.Errorf("RunTransaction() error = %v, should succeed after retry", err)
	}
}

// ========================================
// Helpers
// ========================================

// generateTestCerts creates self-signed CA and client certificates for testing.
func generateTestCerts(t *testing.T, dir string) (caPath, certPath, keyPath string) {
	t.Helper()

	// Generate CA key
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	// CA certificate template
	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{Organization: []string{"Test CA"}},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		IsCA:         true,
		KeyUsage:     x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}

	caCertDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}

	// Write CA cert
	caPath = filepath.Join(dir, "ca.pem")
	caFile, err := os.Create(caPath)
	if err != nil {
		t.Fatal(err)
	}
	pem.Encode(caFile, &pem.Block{Type: "CERTIFICATE", Bytes: caCertDER})
	caFile.Close()

	// Generate client key
	clientKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	// Client certificate template
	clientTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{Organization: []string{"Test Client"}},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}

	caCert, err := x509.ParseCertificate(caCertDER)
	if err != nil {
		t.Fatal(err)
	}

	clientCertDER, err := x509.CreateCertificate(rand.Reader, clientTemplate, caCert, &clientKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}

	// Write client cert
	certPath = filepath.Join(dir, "client.pem")
	certFile, err := os.Create(certPath)
	if err != nil {
		t.Fatal(err)
	}
	pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: clientCertDER})
	certFile.Close()

	// Write client key
	keyPath = filepath.Join(dir, "client-key.pem")
	keyFile, err := os.Create(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(clientKey)
	if err != nil {
		t.Fatal(err)
	}
	pem.Encode(keyFile, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	keyFile.Close()

	return caPath, certPath, keyPath
}
