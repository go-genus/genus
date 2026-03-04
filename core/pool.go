package core

import (
	"database/sql"
	"time"
)

// PoolConfig contém as configurações de pool de conexões.
// Use DefaultPoolConfig() para obter valores sensatos padrão.
type PoolConfig struct {
	// MaxOpenConns é o número máximo de conexões abertas com o banco.
	// Se <= 0, não há limite.
	// Default: 25
	MaxOpenConns int

	// MaxIdleConns é o número máximo de conexões no pool idle.
	// Se <= 0, nenhuma conexão idle é mantida.
	// Default: 10
	MaxIdleConns int

	// ConnMaxLifetime é o tempo máximo que uma conexão pode ser reutilizada.
	// Conexões expiradas são fechadas preguiçosamente antes de serem reutilizadas.
	// Se <= 0, conexões não são fechadas por idade.
	// Default: 30 minutos
	ConnMaxLifetime time.Duration

	// ConnMaxIdleTime é o tempo máximo que uma conexão pode ficar ociosa.
	// Conexões ociosas são fechadas preguiçosamente.
	// Se <= 0, conexões não são fechadas por tempo de inatividade.
	// Default: 5 minutos
	ConnMaxIdleTime time.Duration
}

// DefaultPoolConfig retorna uma configuração de pool com valores sensatos.
// Estes valores são adequados para a maioria das aplicações web.
//
// Valores padrão:
//   - MaxOpenConns: 25
//   - MaxIdleConns: 10
//   - ConnMaxLifetime: 30 minutos
//   - ConnMaxIdleTime: 5 minutos
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MaxOpenConns:    25,
		MaxIdleConns:    10,
		ConnMaxLifetime: 30 * time.Minute,
		ConnMaxIdleTime: 5 * time.Minute,
	}
}

// HighPerformancePoolConfig retorna uma configuração otimizada para alta carga.
// Use com cuidado, pois consome mais recursos do banco de dados.
//
// Valores:
//   - MaxOpenConns: 100
//   - MaxIdleConns: 50
//   - ConnMaxLifetime: 1 hora
//   - ConnMaxIdleTime: 10 minutos
func HighPerformancePoolConfig() PoolConfig {
	return PoolConfig{
		MaxOpenConns:    100,
		MaxIdleConns:    50,
		ConnMaxLifetime: time.Hour,
		ConnMaxIdleTime: 10 * time.Minute,
	}
}

// MinimalPoolConfig retorna uma configuração mínima para ambientes limitados.
// Útil para desenvolvimento local ou testes.
//
// Valores:
//   - MaxOpenConns: 5
//   - MaxIdleConns: 2
//   - ConnMaxLifetime: 15 minutos
//   - ConnMaxIdleTime: 2 minutos
func MinimalPoolConfig() PoolConfig {
	return PoolConfig{
		MaxOpenConns:    5,
		MaxIdleConns:    2,
		ConnMaxLifetime: 15 * time.Minute,
		ConnMaxIdleTime: 2 * time.Minute,
	}
}

// Apply aplica as configurações de pool a uma conexão *sql.DB.
func (c PoolConfig) Apply(db *sql.DB) {
	if c.MaxOpenConns > 0 {
		db.SetMaxOpenConns(c.MaxOpenConns)
	}
	if c.MaxIdleConns > 0 {
		db.SetMaxIdleConns(c.MaxIdleConns)
	}
	if c.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(c.ConnMaxLifetime)
	}
	if c.ConnMaxIdleTime > 0 {
		db.SetConnMaxIdleTime(c.ConnMaxIdleTime)
	}
}

// WithMaxOpenConns retorna uma cópia da configuração com MaxOpenConns alterado.
func (c PoolConfig) WithMaxOpenConns(n int) PoolConfig {
	c.MaxOpenConns = n
	return c
}

// WithMaxIdleConns retorna uma cópia da configuração com MaxIdleConns alterado.
func (c PoolConfig) WithMaxIdleConns(n int) PoolConfig {
	c.MaxIdleConns = n
	return c
}

// WithConnMaxLifetime retorna uma cópia da configuração com ConnMaxLifetime alterado.
func (c PoolConfig) WithConnMaxLifetime(d time.Duration) PoolConfig {
	c.ConnMaxLifetime = d
	return c
}

// WithConnMaxIdleTime retorna uma cópia da configuração com ConnMaxIdleTime alterado.
func (c PoolConfig) WithConnMaxIdleTime(d time.Duration) PoolConfig {
	c.ConnMaxIdleTime = d
	return c
}
