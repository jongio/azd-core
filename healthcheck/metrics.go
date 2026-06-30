package healthcheck

import (
	"net/http"
	"strings"

	"github.com/jongio/azd-core/healthcheck/metrics"
	"github.com/sony/gobreaker/v2"
)

// recordHealthCheck records metrics for a health check result.
// It delegates to the metrics sub-package for actual Prometheus instrumentation.
func recordHealthCheck(result HealthCheckResult) {
	metrics.RecordHealthCheck(metrics.HealthCheckResult{
		ServiceName:  result.ServiceName,
		Status:       string(result.Status),
		CheckType:    string(result.CheckType),
		ResponseTime: result.ResponseTime,
		Error:        result.Error,
		StatusCode:   result.StatusCode,
		Uptime:       result.Uptime,
		IsHealthy:    result.Status == HealthStatusHealthy,
	})
}

// recordCircuitBreakerState records the circuit breaker state.
func recordCircuitBreakerState(serviceName string, state gobreaker.State) {
	metrics.RecordCircuitBreakerState(serviceName, state)
}

// ServeMetrics starts a Prometheus metrics HTTP server.
func ServeMetrics(port int) error {
	return metrics.ServeMetrics(port)
}

// CreateMetricsServer creates a configured HTTP server for Prometheus metrics.
func CreateMetricsServer(port int) *http.Server {
	return metrics.CreateMetricsServer(port)
}

// getErrorType categorizes error messages for better metrics.
// Kept for backward compatibility with tests.
func getErrorType(errMsg string) string {
	switch {
	case containsAny(errMsg, errorTypeTimeout, "deadline", "timed out"):
		return errorTypeTimeout
	case containsAny(errMsg, "connection refused", "no connection", "unreachable"):
		return errorTypeConnectionRefused
	case containsAny(errMsg, "circuit breaker", "circuit open", "too many failures"):
		return errorTypeCircuitBreaker
	case containsAny(errMsg, errorTypePanic):
		return errorTypePanic
	case containsAny(errMsg, "context canceled", "canceled"):
		return "canceled"
	case containsAny(errMsg, "500", "503", "502", "504"):
		return errorTypeServerError
	case containsAny(errMsg, "401", "403"):
		return errorTypeAuthError
	case containsAny(errMsg, "404"):
		return "not_found"
	case containsAny(errMsg, "process", "PID"):
		return errorTypeProcessError
	case containsAny(errMsg, "port"):
		return "port_error"
	default:
		return "unknown"
	}
}

// containsAny checks if a string contains any of the given substrings.
func containsAny(s string, substrs ...string) bool {
	for _, substr := range substrs {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}

// Ensure backward compatibility - kept as re-exports only for callers
// that used CreateMetricsServer or ServeMetrics directly.
