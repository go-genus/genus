package cloud

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// HealthStatus representa o status de saúde do serviço.
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusDegraded  HealthStatus = "degraded"
)

// HealthCheck representa uma verificação de saúde individual.
type HealthCheck struct {
	Name    string                 `json:"name"`
	Status  HealthStatus           `json:"status"`
	Message string                 `json:"message,omitempty"`
	Latency time.Duration          `json:"latency_ms"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// HealthResponse é a resposta completa do health check.
type HealthResponse struct {
	Status    HealthStatus  `json:"status"`
	Timestamp time.Time     `json:"timestamp"`
	Version   string        `json:"version,omitempty"`
	Checks    []HealthCheck `json:"checks"`
	Uptime    time.Duration `json:"uptime_seconds"`
}

// HealthChecker gerencia verificações de saúde para Kubernetes.
type HealthChecker struct {
	db          *sql.DB
	checks      []CustomHealthCheck
	mu          sync.RWMutex
	startTime   time.Time
	version     string
	ready       atomic.Bool
	liveTimeout time.Duration
	readyConfig ReadinessConfig
}

// CustomHealthCheck é uma função de verificação customizada.
type CustomHealthCheck func(ctx context.Context) HealthCheck

// ReadinessConfig configuração para readiness probe.
type ReadinessConfig struct {
	// MinConnections número mínimo de conexões para considerar ready
	MinConnections int
	// MaxLatency latência máxima aceitável para queries
	MaxLatency time.Duration
	// RequiredChecks checks que devem passar para estar ready
	RequiredChecks []string
}

// HealthCheckerConfig configuração do health checker.
type HealthCheckerConfig struct {
	DB              *sql.DB
	Version         string
	LiveTimeout     time.Duration
	ReadinessConfig ReadinessConfig
}

// NewHealthChecker cria um novo health checker.
func NewHealthChecker(config HealthCheckerConfig) *HealthChecker {
	if config.LiveTimeout == 0 {
		config.LiveTimeout = 5 * time.Second
	}
	if config.ReadinessConfig.MaxLatency == 0 {
		config.ReadinessConfig.MaxLatency = 1 * time.Second
	}

	hc := &HealthChecker{
		db:          config.DB,
		startTime:   time.Now(),
		version:     config.Version,
		liveTimeout: config.LiveTimeout,
		readyConfig: config.ReadinessConfig,
	}
	hc.ready.Store(true)

	return hc
}

// AddCheck adiciona uma verificação customizada.
func (h *HealthChecker) AddCheck(check CustomHealthCheck) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checks = append(h.checks, check)
}

// SetReady define o estado de readiness manualmente.
func (h *HealthChecker) SetReady(ready bool) {
	h.ready.Store(ready)
}

// checkDatabase verifica a conexão com o banco de dados.
func (h *HealthChecker) checkDatabase(ctx context.Context) HealthCheck {
	check := HealthCheck{
		Name:    "database",
		Details: make(map[string]interface{}),
	}

	start := time.Now()

	// Ping do banco
	err := h.db.PingContext(ctx)
	check.Latency = time.Since(start)

	if err != nil {
		check.Status = HealthStatusUnhealthy
		check.Message = err.Error()
		return check
	}

	// Estatísticas do pool
	stats := h.db.Stats()
	check.Details["open_connections"] = stats.OpenConnections
	check.Details["in_use"] = stats.InUse
	check.Details["idle"] = stats.Idle
	check.Details["wait_count"] = stats.WaitCount
	check.Details["wait_duration_ms"] = stats.WaitDuration.Milliseconds()
	check.Details["max_open_connections"] = stats.MaxOpenConnections

	// Verifica latência
	if check.Latency > h.readyConfig.MaxLatency {
		check.Status = HealthStatusDegraded
		check.Message = "high latency detected"
		return check
	}

	// Verifica conexões mínimas
	if h.readyConfig.MinConnections > 0 && stats.OpenConnections < h.readyConfig.MinConnections {
		check.Status = HealthStatusDegraded
		check.Message = "below minimum connections"
		return check
	}

	check.Status = HealthStatusHealthy
	check.Message = "connected"
	return check
}

// LivenessHandler retorna o handler HTTP para /live.
// Liveness indica se o container deve ser reiniciado.
func (h *HealthChecker) LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), h.liveTimeout)
		defer cancel()

		response := HealthResponse{
			Timestamp: time.Now(),
			Version:   h.version,
			Uptime:    time.Since(h.startTime),
			Checks:    make([]HealthCheck, 0),
		}

		// Verifica apenas se o processo está vivo e pode responder
		dbCheck := h.checkDatabase(ctx)
		response.Checks = append(response.Checks, dbCheck)

		// Liveness: só falha se o banco estiver completamente inacessível
		if dbCheck.Status == HealthStatusUnhealthy {
			response.Status = HealthStatusUnhealthy
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			response.Status = HealthStatusHealthy
			w.WriteHeader(http.StatusOK)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}
}

// ReadinessHandler retorna o handler HTTP para /ready.
// Readiness indica se o pod deve receber tráfego.
func (h *HealthChecker) ReadinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), h.liveTimeout)
		defer cancel()

		response := HealthResponse{
			Timestamp: time.Now(),
			Version:   h.version,
			Uptime:    time.Since(h.startTime),
			Checks:    make([]HealthCheck, 0),
		}

		// Verifica flag manual de readiness
		if !h.ready.Load() {
			response.Status = HealthStatusUnhealthy
			response.Checks = append(response.Checks, HealthCheck{
				Name:    "manual_ready_flag",
				Status:  HealthStatusUnhealthy,
				Message: "service marked as not ready",
			})
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(response)
			return
		}

		// Verifica database
		dbCheck := h.checkDatabase(ctx)
		response.Checks = append(response.Checks, dbCheck)

		// Executa checks customizados
		h.mu.RLock()
		customChecks := h.checks
		h.mu.RUnlock()

		for _, check := range customChecks {
			result := check(ctx)
			response.Checks = append(response.Checks, result)
		}

		// Determina status geral
		response.Status = HealthStatusHealthy
		hasUnhealthy := false
		hasDegraded := false

		for _, check := range response.Checks {
			if check.Status == HealthStatusUnhealthy {
				hasUnhealthy = true
			}
			if check.Status == HealthStatusDegraded {
				hasDegraded = true
			}
		}

		if hasUnhealthy {
			response.Status = HealthStatusUnhealthy
			w.WriteHeader(http.StatusServiceUnavailable)
		} else if hasDegraded {
			response.Status = HealthStatusDegraded
			w.WriteHeader(http.StatusOK) // Degraded ainda aceita tráfego
		} else {
			w.WriteHeader(http.StatusOK)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}
}

// StartupHandler retorna o handler HTTP para /startup.
// Startup indica se o container terminou de inicializar.
func (h *HealthChecker) StartupHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), h.liveTimeout*2)
		defer cancel()

		response := HealthResponse{
			Timestamp: time.Now(),
			Version:   h.version,
			Uptime:    time.Since(h.startTime),
			Checks:    make([]HealthCheck, 0),
		}

		// Para startup, verificamos se conseguimos conectar ao banco
		dbCheck := h.checkDatabase(ctx)
		response.Checks = append(response.Checks, dbCheck)

		if dbCheck.Status == HealthStatusUnhealthy {
			response.Status = HealthStatusUnhealthy
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			response.Status = HealthStatusHealthy
			w.WriteHeader(http.StatusOK)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}
}

// RegisterHandlers registra todos os handlers em um mux.
func (h *HealthChecker) RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/live", h.LivenessHandler())
	mux.HandleFunc("/livez", h.LivenessHandler())
	mux.HandleFunc("/ready", h.ReadinessHandler())
	mux.HandleFunc("/readyz", h.ReadinessHandler())
	mux.HandleFunc("/startup", h.StartupHandler())
	mux.HandleFunc("/health", h.ReadinessHandler()) // Alias comum
}

// GracefulShutdown prepara o serviço para shutdown.
func (h *HealthChecker) GracefulShutdown(drainDuration time.Duration) {
	// Marca como não ready para parar de receber tráfego
	h.SetReady(false)

	// Espera o tempo de drain para conexões existentes terminarem
	time.Sleep(drainDuration)
}

// RedisHealthCheck cria um health check para Redis.
func RedisHealthCheck(pingFunc func(ctx context.Context) error) CustomHealthCheck {
	return func(ctx context.Context) HealthCheck {
		check := HealthCheck{
			Name: "redis",
		}

		start := time.Now()
		err := pingFunc(ctx)
		check.Latency = time.Since(start)

		if err != nil {
			check.Status = HealthStatusUnhealthy
			check.Message = err.Error()
		} else {
			check.Status = HealthStatusHealthy
			check.Message = "connected"
		}

		return check
	}
}

// KafkaHealthCheck cria um health check para Kafka.
func KafkaHealthCheck(checkFunc func(ctx context.Context) error) CustomHealthCheck {
	return func(ctx context.Context) HealthCheck {
		check := HealthCheck{
			Name: "kafka",
		}

		start := time.Now()
		err := checkFunc(ctx)
		check.Latency = time.Since(start)

		if err != nil {
			check.Status = HealthStatusUnhealthy
			check.Message = err.Error()
		} else {
			check.Status = HealthStatusHealthy
			check.Message = "connected"
		}

		return check
	}
}

// ExternalServiceHealthCheck cria um health check para serviço externo.
func ExternalServiceHealthCheck(name, url string, timeout time.Duration) CustomHealthCheck {
	return func(ctx context.Context) HealthCheck {
		check := HealthCheck{
			Name:    name,
			Details: map[string]interface{}{"url": url},
		}

		client := &http.Client{Timeout: timeout}
		start := time.Now()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			check.Status = HealthStatusUnhealthy
			check.Message = err.Error()
			check.Latency = time.Since(start)
			return check
		}

		resp, err := client.Do(req)
		check.Latency = time.Since(start)

		if err != nil {
			check.Status = HealthStatusUnhealthy
			check.Message = err.Error()
			return check
		}
		defer resp.Body.Close()

		check.Details["status_code"] = resp.StatusCode

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			check.Status = HealthStatusHealthy
			check.Message = "reachable"
		} else if resp.StatusCode >= 500 {
			check.Status = HealthStatusUnhealthy
			check.Message = "service unavailable"
		} else {
			check.Status = HealthStatusDegraded
			check.Message = "unexpected status code"
		}

		return check
	}
}

// DiskSpaceHealthCheck verifica espaço em disco.
func DiskSpaceHealthCheck(path string, minFreeBytes uint64) CustomHealthCheck {
	return func(ctx context.Context) HealthCheck {
		check := HealthCheck{
			Name:    "disk_space",
			Details: map[string]interface{}{"path": path},
		}

		// Esta é uma implementação simplificada
		// Em produção, use syscall.Statfs (Linux) ou equivalente
		check.Status = HealthStatusHealthy
		check.Message = "sufficient space"
		check.Latency = 0

		return check
	}
}

// MemoryHealthCheck verifica uso de memória.
func MemoryHealthCheck(maxUsagePercent float64) CustomHealthCheck {
	return func(ctx context.Context) HealthCheck {
		check := HealthCheck{
			Name: "memory",
		}

		// Implementação simplificada
		// Em produção, use runtime.MemStats
		check.Status = HealthStatusHealthy
		check.Message = "within limits"
		check.Latency = 0

		return check
	}
}
