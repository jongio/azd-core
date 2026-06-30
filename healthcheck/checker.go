package healthcheck

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/jongio/azd-core/procutil"
	"github.com/sony/gobreaker/v2"
)

var (
	// metricsEnabled controls whether Prometheus metrics are recorded.
	metricsEnabled atomic.Bool

	// sharedHTTPTransport is a shared HTTP transport for all health checkers
	sharedHTTPTransport = &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     HTTPIdleConnTimeout,
		DisableKeepAlives:   false,
		DialContext: (&net.Dialer{
			Timeout:   HTTPDialTimeout,
			KeepAlive: HTTPKeepAliveTimeout,
		}).DialContext,
		TLSHandshakeTimeout:   HTTPTLSHandshakeTimeout,
		ExpectContinueTimeout: HTTPExpectContinueTimeout,
	}
)

// HealthChecker performs individual health checks with circuit breaker and rate limiting.
type HealthChecker struct {
	timeout            time.Duration
	defaultEndpoint    string
	httpClient         *http.Client
	circuitBreakers    *CircuitBreakerManager
	rateLimiters       *RateLimiterManager
	endpointCache      *EndpointCache
	startupGracePeriod time.Duration
}

// NewHealthChecker creates a new HealthChecker from the given config.
func NewHealthChecker(config MonitorConfig) *HealthChecker {
	metricsEnabled.Store(config.EnableMetrics)

	gracePeriod := config.StartupGracePeriod
	if gracePeriod == 0 {
		gracePeriod = startupGracePeriod
	}

	return &HealthChecker{
		timeout:            config.Timeout,
		defaultEndpoint:    config.DefaultEndpoint,
		circuitBreakers:    NewCircuitBreakerManager(config.EnableCircuitBreaker, config.CircuitBreakerFailures, config.CircuitBreakerTimeout),
		rateLimiters:       NewRateLimiterManager(config.RateLimit),
		endpointCache:      NewEndpointCache(),
		startupGracePeriod: gracePeriod,
		httpClient: &http.Client{
			Timeout:   config.Timeout,
			Transport: sharedHTTPTransport,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

// CheckService performs a health check on a single service using cascading strategy.
func (c *HealthChecker) CheckService(ctx context.Context, svc ServiceInfo) HealthCheckResult {
	startTime := time.Now()
	serviceName := svc.Name

	if svc.RegistryStatus == registryStatusStopped {
		return HealthCheckResult{
			ServiceName:  serviceName,
			Timestamp:    time.Now(),
			Status:       HealthStatusUnknown,
			ResponseTime: time.Since(startTime),
			ServiceType:  svc.Type,
			ServiceMode:  svc.Mode,
		}
	}

	// Apply rate limiting if configured
	limiter := c.rateLimiters.GetOrCreate(serviceName)
	if limiter != nil {
		if err := limiter.Wait(ctx); err != nil {
			return HealthCheckResult{
				ServiceName: serviceName,
				Timestamp:   time.Now(),
				Status:      HealthStatusUnhealthy,
				Error:       "rate limit exceeded",
			}
		}
	}

	breaker := c.circuitBreakers.GetOrCreate(serviceName)

	var result HealthCheckResult

	if breaker != nil {
		func() {
			defer func() {
				if r := recover(); r != nil {
					stackTrace := captureStackTrace()
					panicValue := fmt.Sprint(r)

					slog.Error("panic during health check",
						"service", serviceName,
						detailKeyPanic, panicValue,
						"stack", stackTrace,
					)

					result = HealthCheckResult{
						ServiceName:  serviceName,
						Timestamp:    time.Now(),
						Status:       HealthStatusUnknown,
						Error:        fmt.Sprintf("panic recovered during health check: %v", r),
						ErrorDetails: stackTrace,
						Details: map[string]interface{}{
							detailKeyPanic: panicValue,
						},
					}
				}
			}()

			output, err := breaker.Execute(func() (any, error) {
				res := c.performServiceCheck(ctx, svc)
				if res.Status == HealthStatusUnhealthy {
					return res, fmt.Errorf("health check failed: %s", res.Error)
				}
				return res, nil
			})

			if err != nil {
				if errors.Is(err, gobreaker.ErrOpenState) {
					result = HealthCheckResult{
						ServiceName: serviceName,
						Timestamp:   time.Now(),
						Status:      HealthStatusUnhealthy,
						Error:       "circuit breaker open - service unavailable",
					}
				} else {
					result = HealthCheckResult{
						ServiceName: serviceName,
						Timestamp:   time.Now(),
						Status:      HealthStatusUnhealthy,
						Error:       err.Error(),
					}
				}
			} else {
				if typedResult, ok := output.(HealthCheckResult); ok {
					result = typedResult
				} else {
					result = HealthCheckResult{
						ServiceName: serviceName,
						Timestamp:   time.Now(),
						Status:      HealthStatusUnknown,
						Error:       "internal error: unexpected health check result type",
					}
				}
			}
		}()
	} else {
		result = c.performServiceCheck(ctx, svc)
	}

	duration := time.Since(startTime)
	result.ResponseTime = duration

	if metricsEnabled.Load() {
		recordHealthCheck(result)
	}

	result.ServiceType = svc.Type
	result.ServiceMode = svc.Mode

	return result
}

func captureStackTrace() string {
	buf := make([]byte, 64*1024)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}

// performServiceCheck executes the actual health check logic without circuit breaker.
func (c *HealthChecker) performServiceCheck(ctx context.Context, svc ServiceInfo) HealthCheckResult {
	result := HealthCheckResult{
		ServiceName: svc.Name,
		Timestamp:   time.Now(),
	}

	if !svc.StartTime.IsZero() {
		result.Uptime = time.Since(svc.StartTime)
	}

	gracePeriod := c.startupGracePeriod
	if gracePeriod == 0 {
		gracePeriod = startupGracePeriod
	}
	isInStartupGracePeriod := !svc.StartTime.IsZero() &&
		time.Since(svc.StartTime) < gracePeriod

	// For process-type services, use process-based health checks directly
	if svc.Type == ServiceTypeProcess {
		return c.performProcessHealthCheck(ctx, svc, isInStartupGracePeriod)
	}

	// Check for custom healthcheck config first
	if svc.HealthCheck != nil && len(svc.HealthCheck.Test) > 0 {
		if httpResult := c.tryCustomHealthCheck(ctx, svc.HealthCheck, svc); httpResult != nil {
			return c.buildResultFromHTTPCheck(result, httpResult, svc.Port, isInStartupGracePeriod)
		}
	}

	// Cascading strategy: HTTP -> Port -> Process

	// 1. Try HTTP health check
	if svc.Port > 0 {
		if httpResult := c.tryHTTPHealthCheck(ctx, svc.Port); httpResult != nil {
			result.Port = svc.Port
			return c.buildResultFromHTTPCheck(result, httpResult, svc.Port, isInStartupGracePeriod)
		}
	}

	// 2. Fall back to TCP port check
	if svc.Port > 0 {
		result.CheckType = HealthCheckTypeTCP
		result.Port = svc.Port
		result.Details = make(map[string]any)

		portCtx, cancel := context.WithTimeout(ctx, defaultPortCheckTimeout)
		defer cancel()

		address := fmt.Sprintf("localhost:%d", svc.Port)
		dialer := net.Dialer{Timeout: defaultPortCheckTimeout}
		conn, err := dialer.DialContext(portCtx, "tcp", address)

		if err == nil {
			_ = conn.Close()
			result.Status = HealthStatusHealthy
		} else {
			if isInStartupGracePeriod {
				result.Status = HealthStatusStarting
			} else {
				result.Status = HealthStatusUnhealthy
			}
			result.Error = fmt.Sprintf("port %d not listening", svc.Port)
			result.Details["suggestion"] = suggestTCPErrorAction(err, svc.Port)
			result.Details["port"] = svc.Port
		}
		return result
	}

	// 3. Fall back to process check
	if svc.PID > 0 {
		result.CheckType = HealthCheckTypeProcess
		result.PID = svc.PID
		result.Details = make(map[string]any)

		isRunning := isProcessRunning(svc.PID)
		if isRunning {
			result.Status = HealthStatusHealthy
			result.Details["pid"] = svc.PID
		} else {
			if isInStartupGracePeriod {
				result.Status = HealthStatusStarting
			} else {
				result.Status = HealthStatusUnhealthy
			}
			result.Error = fmt.Sprintf("process %d not running", svc.PID)
			result.Details["suggestion"] = suggestProcessErrorAction(svc.PID, isRunning, svc.Mode)
			result.Details["pid"] = svc.PID
		}
		return result
	}

	// No check available
	result.CheckType = HealthCheckTypeProcess
	if isInStartupGracePeriod {
		result.Status = HealthStatusStarting
	} else {
		result.Status = HealthStatusUnknown
	}
	result.Error = "no health check method available"

	return result
}

// buildResultFromHTTPCheck builds a HealthCheckResult from an HTTP check result.
func (c *HealthChecker) buildResultFromHTTPCheck(result HealthCheckResult, httpResult *httpHealthCheckResult, port int, isInStartupGracePeriod bool) HealthCheckResult {
	result.CheckType = HealthCheckTypeHTTP
	result.Endpoint = httpResult.Endpoint
	result.ResponseTime = httpResult.ResponseTime
	result.StatusCode = httpResult.StatusCode
	result.Status = httpResult.Status
	result.Details = httpResult.Details
	result.Error = httpResult.Error
	if httpResult.Error != "" && len(httpResult.Error) > 100 {
		result.ErrorDetails = httpResult.Error
		result.Error = httpResult.Error[:100] + "..."
	}
	if port > 0 {
		result.Port = port
	}
	if isInStartupGracePeriod && result.Status != HealthStatusHealthy {
		result.Status = HealthStatusStarting
	}
	return result
}

// tryCustomHealthCheck performs a health check using custom configuration.
func (c *HealthChecker) tryCustomHealthCheck(ctx context.Context, config *HealthCheckConfig, svc ServiceInfo) *httpHealthCheckResult {
	if len(config.Test) == 0 {
		return nil
	}

	test := config.Test[0]

	if strings.HasPrefix(test, "http://") || strings.HasPrefix(test, "https://") {
		return c.performHTTPCheck(ctx, test)
	}

	if len(config.Test) > 1 {
		switch config.Test[0] {
		case "CMD":
			return c.performCommandCheck(ctx, config.Test[1:], svc)
		case "CMD-SHELL":
			return c.performShellCheck(ctx, config.Test[1], svc)
		case "NONE":
			return &httpHealthCheckResult{
				Endpoint: "none",
				Status:   HealthStatusHealthy,
			}
		}
	}

	return c.performShellCheck(ctx, test, svc)
}

// performHTTPCheck performs a direct HTTP health check to a specific URL.
func (c *HealthChecker) performHTTPCheck(ctx context.Context, urlStr string) *httpHealthCheckResult {
	startTime := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return &httpHealthCheckResult{
			Endpoint: urlStr,
			Status:   HealthStatusUnhealthy,
			Error:    fmt.Sprintf("failed to create request: %v", err),
		}
	}

	resp, err := c.httpClient.Do(req)
	responseTime := time.Since(startTime)

	if err != nil {
		return &httpHealthCheckResult{
			Endpoint:     urlStr,
			ResponseTime: responseTime,
			Status:       HealthStatusUnhealthy,
			Error:        fmt.Sprintf("connection failed: %v", err),
		}
	}

	limitedReader := io.LimitReader(resp.Body, maxResponseBodySize)
	body, readErr := io.ReadAll(limitedReader)
	_ = resp.Body.Close()

	result := &httpHealthCheckResult{
		Endpoint:     urlStr,
		ResponseTime: responseTime,
		StatusCode:   resp.StatusCode,
		Details:      make(map[string]any),
	}

	result.Status = c.statusFromHTTPCode(resp.StatusCode)

	if resp.StatusCode >= 400 {
		result.Details["suggestion"] = suggestHTTPErrorAction(resp.StatusCode)
		if readErr == nil && len(body) > 0 {
			if errorDetails := parseErrorDetailsFromBody(body); errorDetails != "" {
				result.Error = errorDetails
			}
		}
	}

	if readErr == nil && len(body) > 0 && resp.StatusCode >= 200 && resp.StatusCode < 300 {
		c.parseHealthResponseBody(body, result)
	}

	return result
}

// performCommandCheck executes a command for health check (CMD format).
func (c *HealthChecker) performCommandCheck(ctx context.Context, args []string, svc ServiceInfo) *httpHealthCheckResult {
	if len(args) == 0 {
		return nil
	}

	startTime := time.Now()
	result := &httpHealthCheckResult{
		Endpoint:     strings.Join(args, " "),
		ResponseTime: 0,
	}

	cmd := exec.CommandContext(ctx, args[0], args[1:]...) // #nosec G204 -- Args from Docker HEALTHCHECK config, not user input
	err := cmd.Run()
	result.ResponseTime = time.Since(startTime)

	if err != nil {
		result.Status = HealthStatusUnhealthy
		result.Error = fmt.Sprintf("command failed: %v", err)
	} else {
		result.Status = HealthStatusHealthy
	}

	return result
}

// performShellCheck executes a shell command for health check (CMD-SHELL format).
func (c *HealthChecker) performShellCheck(ctx context.Context, command string, svc ServiceInfo) *httpHealthCheckResult {
	startTime := time.Now()
	result := &httpHealthCheckResult{
		Endpoint:     command,
		ResponseTime: 0,
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", command) // #nosec G204 -- Command from Docker HEALTHCHECK CMD-SHELL config
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", command) // #nosec G204 -- Command from Docker HEALTHCHECK CMD-SHELL config
	}

	err := cmd.Run()
	result.ResponseTime = time.Since(startTime)

	if err != nil {
		result.Status = HealthStatusUnhealthy
		result.Error = fmt.Sprintf("command failed: %v", err)
	} else {
		result.Status = HealthStatusHealthy
	}

	return result
}

// tryHTTPHealthCheck attempts HTTP health checks using smart endpoint discovery.
func (c *HealthChecker) tryHTTPHealthCheck(ctx context.Context, port int) *httpHealthCheckResult {
	cacheKey := fmt.Sprintf("port:%d", port)

	cachedEndpoint, hasCached := c.endpointCache.Get(cacheKey)

	if hasCached {
		if cachedEndpoint == endpointCacheNone {
			return nil
		}

		result := c.checkSingleEndpoint(ctx, port, cachedEndpoint)
		if result != nil && result.Status == HealthStatusHealthy {
			return result
		}
		// Cache miss on previously good endpoint - clear and rediscover
		c.endpointCache.Set(cacheKey, "")
	}

	endpoints := []string{c.defaultEndpoint}
	for _, path := range commonHealthPaths {
		if path != c.defaultEndpoint {
			endpoints = append(endpoints, path)
		}
	}

	var lastResult *httpHealthCheckResult

	for _, endpoint := range endpoints {
		if ctx.Err() != nil {
			return nil
		}

		result := c.checkSingleEndpoint(ctx, port, endpoint)
		if result != nil {
			if result.Status == HealthStatusHealthy {
				c.endpointCache.Set(cacheKey, endpoint)
				return result
			}
			lastResult = result
		}
	}

	if lastResult == nil {
		c.endpointCache.Set(cacheKey, endpointCacheNone)
	}

	return lastResult
}

// checkSingleEndpoint performs a single HTTP health check on a specific endpoint.
func (c *HealthChecker) checkSingleEndpoint(ctx context.Context, port int, endpoint string) *httpHealthCheckResult {
	url := fmt.Sprintf("http://localhost:%d%s", port, endpoint)

	startTime := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil
	}

	resp, err := c.httpClient.Do(req)
	responseTime := time.Since(startTime)

	if err != nil {
		return nil
	}

	limitedReader := io.LimitReader(resp.Body, maxResponseBodySize)
	body, readErr := io.ReadAll(limitedReader)
	_ = resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusBadRequest {
		return nil
	}

	result := &httpHealthCheckResult{
		Endpoint:     url,
		ResponseTime: responseTime,
		StatusCode:   resp.StatusCode,
		Status:       c.statusFromHTTPCode(resp.StatusCode),
		Details:      make(map[string]any),
	}

	if resp.StatusCode >= 400 {
		result.Details["suggestion"] = suggestHTTPErrorAction(resp.StatusCode)
		if readErr == nil && len(body) > 0 {
			if errorDetails := parseErrorDetailsFromBody(body); errorDetails != "" {
				result.Error = errorDetails
			}
		}
	}

	if readErr == nil && len(body) > 0 && resp.StatusCode >= 200 && resp.StatusCode < 300 {
		c.parseHealthResponseBody(body, result)
	}

	return result
}

// statusFromHTTPCode determines health status from HTTP status code.
func (c *HealthChecker) statusFromHTTPCode(statusCode int) HealthStatus {
	switch {
	case statusCode >= 200 && statusCode < 300:
		return HealthStatusHealthy
	case statusCode >= 300 && statusCode < 400:
		return HealthStatusHealthy // Redirects OK
	case statusCode >= 500:
		return HealthStatusUnhealthy
	default:
		return HealthStatusDegraded
	}
}

// parseHealthResponseBody parses JSON response body for health details.
func (c *HealthChecker) parseHealthResponseBody(body []byte, result *httpHealthCheckResult) {
	var details map[string]any
	if err := json.Unmarshal(body, &details); err == nil {
		result.Details = details

		if status, ok := details["status"].(string); ok {
			switch strings.ToLower(status) {
			case "healthy", "ok", "up":
				result.Status = HealthStatusHealthy
			case "degraded", "warning":
				result.Status = HealthStatusDegraded
			case "unhealthy", "down", "error":
				result.Status = HealthStatusUnhealthy
			}
		}
	}
}

// performProcessHealthCheck handles health checks for process-type services.
func (c *HealthChecker) performProcessHealthCheck(ctx context.Context, svc ServiceInfo, isInStartupGracePeriod bool) HealthCheckResult {
	result := HealthCheckResult{
		ServiceName: svc.Name,
		Timestamp:   time.Now(),
		CheckType:   HealthCheckTypeProcess,
		ServiceMode: svc.Mode,
	}

	if !svc.StartTime.IsZero() {
		if !svc.EndTime.IsZero() {
			result.Uptime = svc.EndTime.Sub(svc.StartTime)
		} else {
			result.Uptime = time.Since(svc.StartTime)
		}
	}

	if svc.Mode == ServiceModeBuild || svc.Mode == ServiceModeTask {
		return c.performBuildTaskHealthCheck(svc, isInStartupGracePeriod, result)
	}

	if svc.PID > 0 {
		result.PID = svc.PID
		if result.Details == nil {
			result.Details = make(map[string]any)
		}

		isRunning := isProcessRunning(svc.PID)
		if isRunning {
			result.Status = HealthStatusHealthy
			result.Details["pid"] = svc.PID
		} else {
			if isInStartupGracePeriod {
				result.Status = HealthStatusStarting
			} else {
				result.Status = HealthStatusUnhealthy
			}
			result.Error = fmt.Sprintf("process %d not running", svc.PID)
			result.Details["suggestion"] = suggestProcessErrorAction(svc.PID, isRunning, svc.Mode)
			result.Details["pid"] = svc.PID
		}
		return result
	}

	if isInStartupGracePeriod {
		result.Status = HealthStatusStarting
	} else {
		result.Status = HealthStatusUnknown
	}
	result.Error = "no process ID available for health check"

	return result
}

// performBuildTaskHealthCheck handles health checks for build and task mode services.
func (c *HealthChecker) performBuildTaskHealthCheck(svc ServiceInfo, isInStartupGracePeriod bool, result HealthCheckResult) HealthCheckResult {
	result.PID = svc.PID

	if svc.PID > 0 && isProcessRunning(svc.PID) {
		if isInStartupGracePeriod {
			result.Status = HealthStatusStarting
		} else {
			result.Status = HealthStatusHealthy
		}
		if svc.Mode == ServiceModeBuild {
			result.Details = map[string]any{detailKeyState: detailStateBuilding}
		} else {
			result.Details = map[string]any{detailKeyState: detailStateRunning}
		}
		return result
	}

	if svc.ExitCode != nil {
		if *svc.ExitCode == 0 {
			result.Status = HealthStatusHealthy
			if svc.Mode == ServiceModeBuild {
				result.Details = map[string]any{detailKeyState: detailStateBuilt, detailKeyExitCode: 0}
			} else {
				result.Details = map[string]any{detailKeyState: detailStateCompleted, detailKeyExitCode: 0}
			}
		} else {
			result.Status = HealthStatusUnhealthy
			result.Error = fmt.Sprintf("process exited with code %d", *svc.ExitCode)
			result.Details = map[string]any{detailKeyState: "failed", detailKeyExitCode: *svc.ExitCode}
		}
		return result
	}

	if svc.PID > 0 {
		result.Status = HealthStatusHealthy
		if svc.Mode == ServiceModeBuild {
			result.Details = map[string]any{detailKeyState: detailStateBuilt, "note": "exit code not captured"}
		} else {
			result.Details = map[string]any{detailKeyState: detailStateCompleted, "note": "exit code not captured"}
		}
		return result
	}

	if isInStartupGracePeriod {
		result.Status = HealthStatusStarting
		return result
	}

	result.Status = HealthStatusUnknown
	result.Error = "no process information available"
	return result
}

// checkPort checks if a TCP port is listening.
func (c *HealthChecker) checkPort(ctx context.Context, port int) bool {
	address := fmt.Sprintf("localhost:%d", port)
	dialer := net.Dialer{Timeout: defaultPortCheckTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// suggestTCPErrorAction provides actionable suggestions for TCP connection errors.
func suggestTCPErrorAction(err error, port int) string {
	if err == nil {
		return ""
	}

	errMsg := err.Error()
	if strings.Contains(errMsg, "connection refused") || strings.Contains(errMsg, "actively refused") {
		return fmt.Sprintf("Port %d connection refused. Verify service is running and port is correct.", port)
	}
	if strings.Contains(errMsg, "timeout") || strings.Contains(errMsg, "i/o timeout") {
		return fmt.Sprintf("Port %d connection timeout. Check network connectivity and firewall rules.", port)
	}
	if strings.Contains(errMsg, "no route to host") {
		return "Network unreachable. Check network configuration."
	}
	return fmt.Sprintf("Port %d connection failed. Verify service is running.", port)
}

// suggestProcessErrorAction provides actionable suggestions for process check errors.
func suggestProcessErrorAction(pid int, isRunning bool, mode string) string {
	if !isRunning {
		return fmt.Sprintf("Process %d not running. Check service logs and verify start command.", pid)
	}
	return ""
}

// isProcessRunning delegates to procutil.IsProcessRunning for cross-platform process detection.
func isProcessRunning(pid int) bool {
	return procutil.IsProcessRunning(pid)
}

// suggestHTTPErrorAction provides actionable suggestions based on HTTP status code.
func suggestHTTPErrorAction(statusCode int) string {
	switch statusCode {
	case 503:
		return "Service temporarily unavailable. Check if dependencies are running."
	case 500, 501, 502, 504, 505, 506, 507, 508, 509, 510, 511:
		return serverErrorAction
	case 404:
		return "Health endpoint not found. Verify endpoint configuration."
	case 401:
		return "Authentication failed. Check credentials."
	case 403:
		return "Authorization failed. Check permissions."
	case 429:
		return "Rate limited. Reduce request rate or check quotas."
	case 408:
		return "Request timeout. Check network connectivity and service performance."
	default:
		if statusCode >= 500 && statusCode < 600 {
			return serverErrorAction
		}
		return "HTTP request failed. Check service logs for details."
	}
}

// parseErrorDetailsFromBody attempts to extract error details from HTTP response body.
func parseErrorDetailsFromBody(body []byte) string {
	if len(body) == 0 {
		return ""
	}

	var jsonData map[string]any
	if err := json.Unmarshal(body, &jsonData); err == nil {
		for _, key := range []string{"error", "message", "detail", "details", "error_description"} {
			if val, ok := jsonData[key]; ok {
				if str, ok := val.(string); ok && str != "" {
					return str
				}
			}
		}
	}

	bodyStr := string(body)
	if len(bodyStr) > 200 {
		return bodyStr[:200] + "..."
	}
	return bodyStr
}
