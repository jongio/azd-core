// Package metrics provides Prometheus metrics collection for health checks.
// Extracted from the healthcheck package so consumers can use health check
// types without pulling in Prometheus dependencies.
package metrics

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	healthCheckDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "azd_health_check_duration_seconds",
		Help:    "Duration of health checks in seconds",
		Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
	}, []string{"service", "status", "check_type"})

	healthCheckTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "azd_health_check_total",
		Help: "Total number of health checks performed",
	}, []string{"service", "status", "check_type"})

	healthCheckErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "azd_health_check_errors_total",
		Help: "Total number of health check errors",
	}, []string{"service", "error_type"})

	serviceUptime = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "azd_service_uptime_seconds",
		Help: "Service uptime in seconds since last health check detected it running",
	}, []string{"service"})

	circuitBreakerState = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "azd_circuit_breaker_state",
		Help: "Circuit breaker state (0=closed, 1=half-open, 2=open)",
	}, []string{"service"})

	healthCheckResponseCode = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "azd_health_check_http_status_total",
		Help: "HTTP status codes from health checks",
	}, []string{"service", "status_code"})
)

// CircuitBreakerState represents circuit breaker state as an integer.
type CircuitBreakerState int

const (
	StateClosed   CircuitBreakerState = 0
	StateHalfOpen CircuitBreakerState = 1
	StateOpen     CircuitBreakerState = 2
)

// CheckResult holds the outcome of a single health check for metrics recording.
type CheckResult struct {
	ServiceName string
	Status      string
	CheckType   string
	Duration    time.Duration
	Error       string
	StatusCode  int
	Uptime      time.Duration
	IsHealthy   bool
}

// RecordHealthCheck records metrics for a health check result.
func RecordHealthCheck(result CheckResult) {
	labels := prometheus.Labels{
		"service":    result.ServiceName,
		"status":     result.Status,
		"check_type": result.CheckType,
	}
	healthCheckDuration.With(labels).Observe(result.Duration.Seconds())
	healthCheckTotal.With(labels).Inc()
	if result.Error != "" {
		errorType := GetErrorType(result.Error)
		healthCheckErrors.With(prometheus.Labels{
			"service":    result.ServiceName,
			"error_type": errorType,
		}).Inc()
	}
	if result.StatusCode > 0 {
		healthCheckResponseCode.With(prometheus.Labels{
			"service":     result.ServiceName,
			"status_code": http.StatusText(result.StatusCode),
		}).Inc()
	}
	if result.IsHealthy && result.Uptime > 0 {
		serviceUptime.With(prometheus.Labels{
			"service": result.ServiceName,
		}).Set(result.Uptime.Seconds())
	}
}

// RecordCircuitBreakerState records the circuit breaker state.
func RecordCircuitBreakerState(serviceName string, state CircuitBreakerState) {
	circuitBreakerState.With(prometheus.Labels{
		"service": serviceName,
	}).Set(float64(state))
}

// GetErrorType categorizes error messages for metrics labeling.
func GetErrorType(errMsg string) string {
	switch {
	case containsAny(errMsg, "timeout", "deadline", "timed out"):
		return "timeout"
	case containsAny(errMsg, "connection refused", "no connection", "unreachable"):
		return "connection_refused"
	case containsAny(errMsg, "circuit breaker", "circuit open", "too many failures"):
		return "circuit_breaker"
	case containsAny(errMsg, "panic"):
		return "panic"
	case containsAny(errMsg, "context canceled", "canceled"):
		return "canceled"
	case containsAny(errMsg, "500", "503", "502", "504"):
		return "server_error"
	case containsAny(errMsg, "401", "403"):
		return "auth_error"
	case containsAny(errMsg, "404"):
		return "not_found"
	case containsAny(errMsg, "process", "PID"):
		return "process_error"
	case containsAny(errMsg, "port"):
		return "port_error"
	default:
		return "unknown"
	}
}

// ContainsAny checks if a string contains any of the given substrings.
func ContainsAny(s string, substrs ...string) bool {
	return containsAny(s, substrs...)
}

func containsAny(s string, substrs ...string) bool {
	for _, substr := range substrs {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}

// ServeMetrics starts a Prometheus metrics HTTP server.
func ServeMetrics(port int) error {
	server := CreateMetricsServer(port)
	return server.ListenAndServe()
}

// CreateMetricsServer creates a configured HTTP server for Prometheus metrics.
func CreateMetricsServer(port int) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})
	addr := fmt.Sprintf(":%d", port)
	return &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}
