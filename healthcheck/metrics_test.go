package healthcheck

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sony/gobreaker/v2"
)

func TestRecordHealthCheck_Healthy(t *testing.T) {
	result := HealthCheckResult{
		ServiceName:  "test-svc",
		Status:       HealthStatusHealthy,
		CheckType:    HealthCheckTypeHTTP,
		ResponseTime: 100 * time.Millisecond,
		Uptime:       5 * time.Minute,
	}

	// Should not panic
	recordHealthCheck(result)
}

func TestRecordHealthCheck_WithError(t *testing.T) {
	result := HealthCheckResult{
		ServiceName:  "test-svc-err",
		Status:       HealthStatusUnhealthy,
		CheckType:    HealthCheckTypeHTTP,
		ResponseTime: 200 * time.Millisecond,
		Error:        "connection refused",
	}

	// Should not panic
	recordHealthCheck(result)
}

func TestRecordHealthCheck_WithStatusCode(t *testing.T) {
	result := HealthCheckResult{
		ServiceName:  "test-svc-code",
		Status:       HealthStatusUnhealthy,
		CheckType:    HealthCheckTypeHTTP,
		ResponseTime: 50 * time.Millisecond,
		StatusCode:   500,
		Error:        "500 internal server error",
	}

	// Should not panic
	recordHealthCheck(result)
}

func TestRecordHealthCheck_NoUptime(t *testing.T) {
	result := HealthCheckResult{
		ServiceName:  "test-svc-noup",
		Status:       HealthStatusHealthy,
		CheckType:    HealthCheckTypeTCP,
		ResponseTime: 10 * time.Millisecond,
	}

	recordHealthCheck(result)
}

func TestRecordCircuitBreakerState(t *testing.T) {
	tests := []struct {
		name  string
		state gobreaker.State
	}{
		{"closed", gobreaker.StateClosed},
		{"half-open", gobreaker.StateHalfOpen},
		{"open", gobreaker.StateOpen},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			recordCircuitBreakerState("test-cb-"+tt.name, tt.state)
		})
	}
}

func TestGetErrorType(t *testing.T) {
	tests := []struct {
		errMsg string
		want   string
	}{
		{"request timed out", "timeout"},
		{"deadline exceeded", "timeout"},
		{"connection refused on port 8080", "connection_refused"},
		{"no connection available", "connection_refused"},
		{"circuit breaker tripped", "circuit_breaker"},
		{"too many failures detected", "circuit_breaker"},
		{"panic recovered during health check: boom", "panic"},
		{"context canceled by caller", "canceled"},
		{"got 500 from server", "server_error"},
		{"got 503 service unavailable", "server_error"},
		{"got 502 bad gateway", "server_error"},
		{"got 504 status", "server_error"},
		{"got 401 unauthorized", "auth_error"},
		{"got 403 forbidden", "auth_error"},
		{"got 404 not found", "not_found"},
		{"process crashed", "process_error"},
		{"PID not found", "process_error"},
		{"port 8080 not listening", "port_error"},
		{"something totally unknown happened", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want+"_"+tt.errMsg[:10], func(t *testing.T) {
			got := getErrorType(tt.errMsg)
			if got != tt.want {
				t.Errorf("getErrorType(%q) = %q, want %q", tt.errMsg, got, tt.want)
			}
		})
	}
}

func TestContainsAny(t *testing.T) {
	tests := []struct {
		name    string
		s       string
		substrs []string
		want    bool
	}{
		{"match first", "hello world", []string{"hello"}, true},
		{"match second", "hello world", []string{"foo", "world"}, true},
		{"no match", "hello world", []string{"foo", "bar"}, false},
		{"empty substrs", "hello", []string{}, false},
		{"empty string", "", []string{"hello"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsAny(tt.s, tt.substrs...)
			if got != tt.want {
				t.Errorf("containsAny(%q, %v) = %v, want %v", tt.s, tt.substrs, got, tt.want)
			}
		})
	}
}

func TestCreateMetricsServer(t *testing.T) {
	server := CreateMetricsServer(9999)
	if server == nil {
		t.Fatal("CreateMetricsServer() returned nil")
	}

	if server.Addr != ":9999" {
		t.Errorf("server.Addr = %q, want %q", server.Addr, ":9999")
	}

	if server.ReadTimeout != 10*time.Second {
		t.Errorf("ReadTimeout = %v, want %v", server.ReadTimeout, 10*time.Second)
	}

	if server.WriteTimeout != 10*time.Second {
		t.Errorf("WriteTimeout = %v, want %v", server.WriteTimeout, 10*time.Second)
	}

	if server.IdleTimeout != 60*time.Second {
		t.Errorf("IdleTimeout = %v, want %v", server.IdleTimeout, 60*time.Second)
	}
}

func TestCreateMetricsServer_HealthEndpoint(t *testing.T) {
	server := CreateMetricsServer(0)

	// Test the /health endpoint
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	server.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("/health status = %d, want %d", w.Code, http.StatusOK)
	}

	if w.Body.String() != "OK" {
		t.Errorf("/health body = %q, want %q", w.Body.String(), "OK")
	}
}

func TestCreateMetricsServer_MetricsEndpoint(t *testing.T) {
	server := CreateMetricsServer(0)

	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	server.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("/metrics status = %d, want %d", w.Code, http.StatusOK)
	}
}
