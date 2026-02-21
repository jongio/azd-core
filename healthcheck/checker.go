package healthcheck

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jongio/azd-core/procutil"
	"github.com/sony/gobreaker"
	"golang.org/x/time/rate"
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
	breakers           map[string]*gobreaker.CircuitBreaker
	rateLimiters       map[string]*rate.Limiter
	endpointCache      map[string]string // Maps service:port to successful endpoint path
	mu                 sync.RWMutex
	enableBreaker      bool
	breakerFailures    int
	breakerTimeout     time.Duration
	rateLimit          int
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
		breakers:           make(map[string]*gobreaker.CircuitBreaker),
		rateLimiters:       make(map[string]*rate.Limiter),
		endpointCache:      make(map[string]string),
		enableBreaker:      config.EnableCircuitBreaker,
		breakerFailures:    config.CircuitBreakerFailures,
		breakerTimeout:     config.CircuitBreakerTimeout,
		rateLimit:          config.RateLimit,
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

// getOrCreateCircuitBreaker gets or creates a circuit breaker for a service.
func (c *HealthChecker) getOrCreateCircuitBreaker(serviceName string) *gobreaker.CircuitBreaker {
	if !c.enableBreaker {
		return nil
	}

	c.mu.RLock()
	breaker, exists := c.breakers[serviceName]
	c.mu.RUnlock()

	if exists {
		return breaker
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if breaker, exists := c.breakers[serviceName]; exists {
		return breaker
	}

	settings := gobreaker.Settings{
		Name:        serviceName,
		MaxRequests: 3,
		Interval:    c.breakerTimeout,
		Timeout:     c.breakerTimeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			if c.breakerFailures < 0 {
				return false
			}
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= uint32(c.breakerFailures) && failureRatio >= 0.6
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			if metricsEnabled.Load() {
				recordCircuitBreakerState(name, to)
			}
		},
	}

	breaker = gobreaker.NewCircuitBreaker(settings)
	c.breakers[serviceName] = breaker
	return breaker
}

// getOrCreateRateLimiter gets or creates a rate limiter for a service.
func (c *HealthChecker) getOrCreateRateLimiter(serviceName string) *rate.Limiter {
	if c.rateLimit <= 0 {
		return nil
	}

	c.mu.RLock()
	limiter, exists := c.rateLimiters[serviceName]
	c.mu.RUnlock()

	if exists {
		return limiter
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if limiter, exists := c.rateLimiters[serviceName]; exists {
		return limiter
	}

	limiter = rate.NewLimiter(rate.Limit(c.rateLimit), c.rateLimit*2)
	c.rateLimiters[serviceName] = limiter

	return limiter
}

// CheckService performs a health check on a single service using cascading strategy.
func (c *HealthChecker) CheckService(ctx context.Context, svc ServiceInfo) HealthCheckResult {
	startTime := time.Now()
	serviceName := svc.Name

	if svc.RegistryStatus == "stopped" {
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
	limiter := c.getOrCreateRateLimiter(serviceName)
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

	breaker := c.getOrCreateCircuitBreaker(serviceName)

	var result HealthCheckResult

	if breaker != nil {
		func() {
			defer func() {
				if r := recover(); r != nil {
					result = HealthCheckResult{
						ServiceName: serviceName,
						Timestamp:   time.Now(),
						Status:      HealthStatusUnknown,
						Error:       fmt.Sprintf("internal error: panic during health check: %v", r),
					}
				}
			}()

			output, err := breaker.Execute(func() (interface{}, error) {
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
		result.Details = make(map[string]interface{})

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
		result.Details = make(map[string]interface{})

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
	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
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
		Details:      make(map[string]interface{}),
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

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
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
		cmd = exec.CommandContext(ctx, "cmd", "/C", command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
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

	c.mu.Lock()
	if c.endpointCache == nil {
		c.endpointCache = make(map[string]string)
	}
	c.mu.Unlock()

	c.mu.RLock()
	cachedEndpoint, hasCached := c.endpointCache[cacheKey]
	c.mu.RUnlock()

	if hasCached {
		if cachedEndpoint == endpointCacheNone {
			return nil
		}

		result := c.checkSingleEndpoint(ctx, port, cachedEndpoint)
		if result != nil && result.Status == HealthStatusHealthy {
			return result
		}
		c.mu.Lock()
		delete(c.endpointCache, cacheKey)
		c.mu.Unlock()
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
				c.mu.Lock()
				c.endpointCache[cacheKey] = endpoint
				c.mu.Unlock()
				return result
			}
			lastResult = result
		}
	}

	if lastResult == nil {
		c.mu.Lock()
		c.endpointCache[cacheKey] = endpointCacheNone
		c.mu.Unlock()
	}

	return lastResult
}

// checkSingleEndpoint performs a single HTTP health check on a specific endpoint.
func (c *HealthChecker) checkSingleEndpoint(ctx context.Context, port int, endpoint string) *httpHealthCheckResult {
	url := fmt.Sprintf("http://localhost:%d%s", port, endpoint)

	startTime := time.Now()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
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
		Details:      make(map[string]interface{}),
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
	var details map[string]interface{}
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
			result.Details = make(map[string]interface{})
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
			result.Details = map[string]interface{}{"state": "building"}
		} else {
			result.Details = map[string]interface{}{"state": "running"}
		}
		return result
	}

	if svc.ExitCode != nil {
		if *svc.ExitCode == 0 {
			result.Status = HealthStatusHealthy
			if svc.Mode == ServiceModeBuild {
				result.Details = map[string]interface{}{"state": "built", "exitCode": 0}
			} else {
				result.Details = map[string]interface{}{"state": "completed", "exitCode": 0}
			}
		} else {
			result.Status = HealthStatusUnhealthy
			result.Error = fmt.Sprintf("process exited with code %d", *svc.ExitCode)
			result.Details = map[string]interface{}{"state": "failed", "exitCode": *svc.ExitCode}
		}
		return result
	}

	if svc.PID > 0 {
		result.Status = HealthStatusHealthy
		if svc.Mode == ServiceModeBuild {
			result.Details = map[string]interface{}{"state": "built", "note": "exit code not captured"}
		} else {
			result.Details = map[string]interface{}{"state": "completed", "note": "exit code not captured"}
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
		return "Server error. Check application logs for details."
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
			return "Server error. Check application logs for details."
		}
		return "HTTP request failed. Check service logs for details."
	}
}

// parseErrorDetailsFromBody attempts to extract error details from HTTP response body.
func parseErrorDetailsFromBody(body []byte) string {
	if len(body) == 0 {
		return ""
	}

	var jsonData map[string]interface{}
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
