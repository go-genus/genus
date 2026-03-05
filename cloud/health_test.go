package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ========================================
// NewHealthChecker Tests
// ========================================

func TestNewHealthChecker(t *testing.T) {
	db := getMockSQLDBWithPinger(nil)
	defer db.Close()

	hc := NewHealthChecker(HealthCheckerConfig{
		DB:      db,
		Version: "1.0.0",
	})

	if hc == nil {
		t.Fatal("NewHealthChecker() returned nil")
	}
	if hc.version != "1.0.0" {
		t.Errorf("version = %q, want %q", hc.version, "1.0.0")
	}
	if hc.liveTimeout != 5*time.Second {
		t.Errorf("liveTimeout = %v, want %v", hc.liveTimeout, 5*time.Second)
	}
	if hc.readyConfig.MaxLatency != 1*time.Second {
		t.Errorf("MaxLatency = %v, want %v", hc.readyConfig.MaxLatency, 1*time.Second)
	}
	if !hc.ready.Load() {
		t.Error("ready should be true by default")
	}
}

func TestNewHealthChecker_CustomConfig(t *testing.T) {
	db := getMockSQLDBWithPinger(nil)
	defer db.Close()

	hc := NewHealthChecker(HealthCheckerConfig{
		DB:          db,
		Version:     "2.0.0",
		LiveTimeout: 10 * time.Second,
		ReadinessConfig: ReadinessConfig{
			MinConnections: 5,
			MaxLatency:     500 * time.Millisecond,
		},
	})

	if hc.liveTimeout != 10*time.Second {
		t.Errorf("liveTimeout = %v, want %v", hc.liveTimeout, 10*time.Second)
	}
	if hc.readyConfig.MinConnections != 5 {
		t.Errorf("MinConnections = %d, want 5", hc.readyConfig.MinConnections)
	}
	if hc.readyConfig.MaxLatency != 500*time.Millisecond {
		t.Errorf("MaxLatency = %v, want %v", hc.readyConfig.MaxLatency, 500*time.Millisecond)
	}
}

// ========================================
// AddCheck Tests
// ========================================

func TestHealthChecker_AddCheck(t *testing.T) {
	db := getMockSQLDBWithPinger(nil)
	defer db.Close()

	hc := NewHealthChecker(HealthCheckerConfig{DB: db})

	check := func(ctx context.Context) HealthCheck {
		return HealthCheck{Name: "custom", Status: HealthStatusHealthy}
	}

	hc.AddCheck(check)

	hc.mu.RLock()
	defer hc.mu.RUnlock()
	if len(hc.checks) != 1 {
		t.Errorf("checks count = %d, want 1", len(hc.checks))
	}
}

// ========================================
// SetReady Tests
// ========================================

func TestHealthChecker_SetReady(t *testing.T) {
	db := getMockSQLDBWithPinger(nil)
	defer db.Close()

	hc := NewHealthChecker(HealthCheckerConfig{DB: db})

	if !hc.ready.Load() {
		t.Error("should be ready initially")
	}

	hc.SetReady(false)
	if hc.ready.Load() {
		t.Error("should be not ready after SetReady(false)")
	}

	hc.SetReady(true)
	if !hc.ready.Load() {
		t.Error("should be ready after SetReady(true)")
	}
}

// ========================================
// checkDatabase Tests
// ========================================

func TestHealthChecker_checkDatabase_Healthy(t *testing.T) {
	db := getMockSQLDBWithPinger(nil)
	defer db.Close()

	hc := NewHealthChecker(HealthCheckerConfig{
		DB: db,
		ReadinessConfig: ReadinessConfig{
			MaxLatency: 10 * time.Second,
		},
	})

	check := hc.checkDatabase(t.Context())

	if check.Name != "database" {
		t.Errorf("Name = %q, want %q", check.Name, "database")
	}
	if check.Status != HealthStatusHealthy {
		t.Errorf("Status = %q, want %q", check.Status, HealthStatusHealthy)
	}
	if check.Message != "connected" {
		t.Errorf("Message = %q, want %q", check.Message, "connected")
	}
}

func TestHealthChecker_checkDatabase_Unhealthy(t *testing.T) {
	db := getMockSQLDBWithPinger(errors.New("connection refused"))
	defer db.Close()
	// Force connection establishment
	db.Ping()

	hc := NewHealthChecker(HealthCheckerConfig{DB: db})

	check := hc.checkDatabase(t.Context())

	if check.Status != HealthStatusUnhealthy {
		t.Errorf("Status = %q, want %q", check.Status, HealthStatusUnhealthy)
	}
}

func TestHealthChecker_checkDatabase_MinConnections(t *testing.T) {
	db := getMockSQLDBWithPinger(nil)
	defer db.Close()

	hc := NewHealthChecker(HealthCheckerConfig{
		DB: db,
		ReadinessConfig: ReadinessConfig{
			MinConnections: 100, // impossible to reach
			MaxLatency:     10 * time.Second,
		},
	})

	check := hc.checkDatabase(t.Context())

	if check.Status != HealthStatusDegraded {
		t.Errorf("Status = %q, want %q", check.Status, HealthStatusDegraded)
	}
	if check.Message != "below minimum connections" {
		t.Errorf("Message = %q, want %q", check.Message, "below minimum connections")
	}
}

// ========================================
// LivenessHandler Tests
// ========================================

func TestHealthChecker_LivenessHandler_Healthy(t *testing.T) {
	db := getMockSQLDBWithPinger(nil)
	defer db.Close()

	hc := NewHealthChecker(HealthCheckerConfig{
		DB:      db,
		Version: "1.0.0",
		ReadinessConfig: ReadinessConfig{
			MaxLatency: 10 * time.Second,
		},
	})

	handler := hc.LivenessHandler()
	req := httptest.NewRequest(http.MethodGet, "/live", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", w.Code, http.StatusOK)
	}

	var resp HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Status != HealthStatusHealthy {
		t.Errorf("Status = %q, want %q", resp.Status, HealthStatusHealthy)
	}
	if len(resp.Checks) != 1 {
		t.Errorf("Checks count = %d, want 1", len(resp.Checks))
	}
}

func TestHealthChecker_LivenessHandler_Unhealthy(t *testing.T) {
	db := getMockSQLDBWithPinger(errors.New("dead"))
	defer db.Close()
	db.Ping()

	hc := NewHealthChecker(HealthCheckerConfig{DB: db})

	handler := hc.LivenessHandler()
	req := httptest.NewRequest(http.MethodGet, "/live", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status code = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}

	var resp HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Status != HealthStatusUnhealthy {
		t.Errorf("Status = %q, want %q", resp.Status, HealthStatusUnhealthy)
	}
}

// ========================================
// ReadinessHandler Tests
// ========================================

func TestHealthChecker_ReadinessHandler_Healthy(t *testing.T) {
	db := getMockSQLDBWithPinger(nil)
	defer db.Close()

	hc := NewHealthChecker(HealthCheckerConfig{
		DB:      db,
		Version: "1.0.0",
		ReadinessConfig: ReadinessConfig{
			MaxLatency: 10 * time.Second,
		},
	})

	handler := hc.ReadinessHandler()
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", w.Code, http.StatusOK)
	}

	var resp HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Status != HealthStatusHealthy {
		t.Errorf("Status = %q, want %q", resp.Status, HealthStatusHealthy)
	}
}

func TestHealthChecker_ReadinessHandler_NotReady(t *testing.T) {
	db := getMockSQLDBWithPinger(nil)
	defer db.Close()

	hc := NewHealthChecker(HealthCheckerConfig{DB: db})
	hc.SetReady(false)

	handler := hc.ReadinessHandler()
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status code = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}

	var resp HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Status != HealthStatusUnhealthy {
		t.Errorf("Status = %q, want %q", resp.Status, HealthStatusUnhealthy)
	}
	if len(resp.Checks) == 0 || resp.Checks[0].Name != "manual_ready_flag" {
		t.Error("should have manual_ready_flag check")
	}
}

func TestHealthChecker_ReadinessHandler_WithCustomChecks(t *testing.T) {
	db := getMockSQLDBWithPinger(nil)
	defer db.Close()

	hc := NewHealthChecker(HealthCheckerConfig{
		DB: db,
		ReadinessConfig: ReadinessConfig{
			MaxLatency: 10 * time.Second,
		},
	})

	hc.AddCheck(func(ctx context.Context) HealthCheck {
		return HealthCheck{
			Name:    "custom_check",
			Status:  HealthStatusHealthy,
			Message: "all good",
		}
	})

	handler := hc.ReadinessHandler()
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", w.Code, http.StatusOK)
	}

	var resp HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Checks) != 2 {
		t.Errorf("Checks count = %d, want 2", len(resp.Checks))
	}
}

func TestHealthChecker_ReadinessHandler_WithUnhealthyCustomCheck(t *testing.T) {
	db := getMockSQLDBWithPinger(nil)
	defer db.Close()

	hc := NewHealthChecker(HealthCheckerConfig{
		DB: db,
		ReadinessConfig: ReadinessConfig{
			MaxLatency: 10 * time.Second,
		},
	})

	hc.AddCheck(func(ctx context.Context) HealthCheck {
		return HealthCheck{
			Name:    "failing_check",
			Status:  HealthStatusUnhealthy,
			Message: "service down",
		}
	})

	handler := hc.ReadinessHandler()
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status code = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}

	var resp HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Status != HealthStatusUnhealthy {
		t.Errorf("Status = %q, want %q", resp.Status, HealthStatusUnhealthy)
	}
}

func TestHealthChecker_ReadinessHandler_WithDegradedCustomCheck(t *testing.T) {
	db := getMockSQLDBWithPinger(nil)
	defer db.Close()

	hc := NewHealthChecker(HealthCheckerConfig{
		DB: db,
		ReadinessConfig: ReadinessConfig{
			MaxLatency: 10 * time.Second,
		},
	})

	hc.AddCheck(func(ctx context.Context) HealthCheck {
		return HealthCheck{
			Name:    "degraded_check",
			Status:  HealthStatusDegraded,
			Message: "slow",
		}
	})

	handler := hc.ReadinessHandler()
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	// Degraded still returns 200
	if w.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", w.Code, http.StatusOK)
	}

	var resp HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Status != HealthStatusDegraded {
		t.Errorf("Status = %q, want %q", resp.Status, HealthStatusDegraded)
	}
}

// ========================================
// StartupHandler Tests
// ========================================

func TestHealthChecker_StartupHandler_Healthy(t *testing.T) {
	db := getMockSQLDBWithPinger(nil)
	defer db.Close()

	hc := NewHealthChecker(HealthCheckerConfig{
		DB: db,
		ReadinessConfig: ReadinessConfig{
			MaxLatency: 10 * time.Second,
		},
	})

	handler := hc.StartupHandler()
	req := httptest.NewRequest(http.MethodGet, "/startup", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status code = %d, want %d", w.Code, http.StatusOK)
	}

	var resp HealthResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Status != HealthStatusHealthy {
		t.Errorf("Status = %q, want %q", resp.Status, HealthStatusHealthy)
	}
}

func TestHealthChecker_StartupHandler_Unhealthy(t *testing.T) {
	db := getMockSQLDBWithPinger(errors.New("not started"))
	defer db.Close()
	db.Ping()

	hc := NewHealthChecker(HealthCheckerConfig{DB: db})

	handler := hc.StartupHandler()
	req := httptest.NewRequest(http.MethodGet, "/startup", nil)
	w := httptest.NewRecorder()

	handler(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status code = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
}

// ========================================
// RegisterHandlers Tests
// ========================================

func TestHealthChecker_RegisterHandlers(t *testing.T) {
	db := getMockSQLDBWithPinger(nil)
	defer db.Close()

	hc := NewHealthChecker(HealthCheckerConfig{
		DB: db,
		ReadinessConfig: ReadinessConfig{
			MaxLatency: 10 * time.Second,
		},
	})

	mux := http.NewServeMux()
	hc.RegisterHandlers(mux)

	// Test that all registered paths respond
	paths := []string{"/live", "/livez", "/ready", "/readyz", "/startup", "/health"}
	for _, path := range paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("path %s: status code = %d, want %d", path, w.Code, http.StatusOK)
		}
	}
}

// ========================================
// GracefulShutdown Tests
// ========================================

func TestHealthChecker_GracefulShutdown(t *testing.T) {
	db := getMockSQLDBWithPinger(nil)
	defer db.Close()

	hc := NewHealthChecker(HealthCheckerConfig{DB: db})

	if !hc.ready.Load() {
		t.Error("should be ready before shutdown")
	}

	// Use very short drain duration for test
	hc.GracefulShutdown(1 * time.Millisecond)

	if hc.ready.Load() {
		t.Error("should not be ready after shutdown")
	}
}

// ========================================
// RedisHealthCheck Tests
// ========================================

func TestRedisHealthCheck_Healthy(t *testing.T) {
	check := RedisHealthCheck(func(ctx context.Context) error {
		return nil
	})

	result := check(t.Context())

	if result.Name != "redis" {
		t.Errorf("Name = %q, want %q", result.Name, "redis")
	}
	if result.Status != HealthStatusHealthy {
		t.Errorf("Status = %q, want %q", result.Status, HealthStatusHealthy)
	}
	if result.Message != "connected" {
		t.Errorf("Message = %q, want %q", result.Message, "connected")
	}
}

func TestRedisHealthCheck_Unhealthy(t *testing.T) {
	check := RedisHealthCheck(func(ctx context.Context) error {
		return errors.New("connection refused")
	})

	result := check(t.Context())

	if result.Status != HealthStatusUnhealthy {
		t.Errorf("Status = %q, want %q", result.Status, HealthStatusUnhealthy)
	}
	if result.Message != "connection refused" {
		t.Errorf("Message = %q, want %q", result.Message, "connection refused")
	}
}

// ========================================
// KafkaHealthCheck Tests
// ========================================

func TestKafkaHealthCheck_Healthy(t *testing.T) {
	check := KafkaHealthCheck(func(ctx context.Context) error {
		return nil
	})

	result := check(t.Context())

	if result.Name != "kafka" {
		t.Errorf("Name = %q, want %q", result.Name, "kafka")
	}
	if result.Status != HealthStatusHealthy {
		t.Errorf("Status = %q, want %q", result.Status, HealthStatusHealthy)
	}
}

func TestKafkaHealthCheck_Unhealthy(t *testing.T) {
	check := KafkaHealthCheck(func(ctx context.Context) error {
		return errors.New("broker unavailable")
	})

	result := check(t.Context())

	if result.Status != HealthStatusUnhealthy {
		t.Errorf("Status = %q, want %q", result.Status, HealthStatusUnhealthy)
	}
}

// ========================================
// ExternalServiceHealthCheck Tests
// ========================================

func TestExternalServiceHealthCheck_Healthy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	check := ExternalServiceHealthCheck("test-service", server.URL, 5*time.Second)
	result := check(t.Context())

	if result.Name != "test-service" {
		t.Errorf("Name = %q, want %q", result.Name, "test-service")
	}
	if result.Status != HealthStatusHealthy {
		t.Errorf("Status = %q, want %q", result.Status, HealthStatusHealthy)
	}
	if result.Message != "reachable" {
		t.Errorf("Message = %q, want %q", result.Message, "reachable")
	}
}

func TestExternalServiceHealthCheck_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	check := ExternalServiceHealthCheck("test-service", server.URL, 5*time.Second)
	result := check(t.Context())

	if result.Status != HealthStatusUnhealthy {
		t.Errorf("Status = %q, want %q", result.Status, HealthStatusUnhealthy)
	}
	if result.Message != "service unavailable" {
		t.Errorf("Message = %q, want %q", result.Message, "service unavailable")
	}
}

func TestExternalServiceHealthCheck_ClientError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	check := ExternalServiceHealthCheck("test-service", server.URL, 5*time.Second)
	result := check(t.Context())

	if result.Status != HealthStatusDegraded {
		t.Errorf("Status = %q, want %q", result.Status, HealthStatusDegraded)
	}
	if result.Message != "unexpected status code" {
		t.Errorf("Message = %q, want %q", result.Message, "unexpected status code")
	}
}

func TestExternalServiceHealthCheck_ConnectionError(t *testing.T) {
	check := ExternalServiceHealthCheck("test-service", "http://localhost:1", 100*time.Millisecond)
	result := check(t.Context())

	if result.Status != HealthStatusUnhealthy {
		t.Errorf("Status = %q, want %q", result.Status, HealthStatusUnhealthy)
	}
}

func TestExternalServiceHealthCheck_InvalidURL(t *testing.T) {
	check := ExternalServiceHealthCheck("test-service", "://invalid", 5*time.Second)
	result := check(t.Context())

	if result.Status != HealthStatusUnhealthy {
		t.Errorf("Status = %q, want %q", result.Status, HealthStatusUnhealthy)
	}
}

// ========================================
// DiskSpaceHealthCheck Tests
// ========================================

func TestDiskSpaceHealthCheck(t *testing.T) {
	check := DiskSpaceHealthCheck("/", 1024)
	result := check(t.Context())

	if result.Name != "disk_space" {
		t.Errorf("Name = %q, want %q", result.Name, "disk_space")
	}
	if result.Status != HealthStatusHealthy {
		t.Errorf("Status = %q, want %q", result.Status, HealthStatusHealthy)
	}
	if result.Details["path"] != "/" {
		t.Errorf("Details[path] = %v, want %q", result.Details["path"], "/")
	}
}

// ========================================
// MemoryHealthCheck Tests
// ========================================

func TestMemoryHealthCheck(t *testing.T) {
	check := MemoryHealthCheck(90.0)
	result := check(t.Context())

	if result.Name != "memory" {
		t.Errorf("Name = %q, want %q", result.Name, "memory")
	}
	if result.Status != HealthStatusHealthy {
		t.Errorf("Status = %q, want %q", result.Status, HealthStatusHealthy)
	}
}

// ========================================
// HealthStatus Constants Tests
// ========================================

func TestHealthStatusConstants(t *testing.T) {
	if HealthStatusHealthy != "healthy" {
		t.Errorf("HealthStatusHealthy = %q, want %q", HealthStatusHealthy, "healthy")
	}
	if HealthStatusUnhealthy != "unhealthy" {
		t.Errorf("HealthStatusUnhealthy = %q, want %q", HealthStatusUnhealthy, "unhealthy")
	}
	if HealthStatusDegraded != "degraded" {
		t.Errorf("HealthStatusDegraded = %q, want %q", HealthStatusDegraded, "degraded")
	}
}
