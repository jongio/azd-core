// metrics.go delegates to the metrics sub-package for backward compatibility.
package healthcheck

import (
	"net/http"

	"github.com/jongio/azd-core/healthcheck/metrics"
	"github.com/sony/gobreaker"
)

func recordHealthCheck(result HealthCheckResult) {
	metrics.RecordHealthCheck(metrics.CheckResult{
		ServiceName: result.ServiceName,
		Status:      string(result.Status),
		CheckType:   string(result.CheckType),
		Duration:    result.ResponseTime,
		Error:       result.Error,
		StatusCode:  result.StatusCode,
		Uptime:      result.Uptime,
		IsHealthy:   result.Status == HealthStatusHealthy,
	})
}

func recordCircuitBreakerState(serviceName string, state gobreaker.State) {
	metrics.RecordCircuitBreakerState(serviceName, metrics.CircuitBreakerState(state))
}

func getErrorType(errMsg string) string {
	return metrics.GetErrorType(errMsg)
}

func containsAny(s string, substrs ...string) bool {
	return metrics.ContainsAny(s, substrs...)
}

// ServeMetrics starts a Prometheus metrics HTTP server on the given port.
func ServeMetrics(port int) error {
	return metrics.ServeMetrics(port)
}

// CreateMetricsServer creates a configured HTTP server for Prometheus metrics.
func CreateMetricsServer(port int) *http.Server {
	return metrics.CreateMetricsServer(port)
}
