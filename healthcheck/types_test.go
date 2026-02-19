package healthcheck

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestHealthStatus(t *testing.T) {
	statuses := []HealthStatus{
		HealthStatusHealthy,
		HealthStatusDegraded,
		HealthStatusUnhealthy,
		HealthStatusStarting,
		HealthStatusUnknown,
	}

	for _, status := range statuses {
		if string(status) == "" {
			t.Errorf("HealthStatus should not be empty")
		}
	}
}

func TestHealthCheckType(t *testing.T) {
	types := []HealthCheckType{
		HealthCheckTypeHTTP,
		HealthCheckTypeTCP,
		HealthCheckTypeProcess,
	}

	for _, checkType := range types {
		if string(checkType) == "" {
			t.Errorf("HealthCheckType should not be empty")
		}
	}
}

func TestCalculateSummary(t *testing.T) {
	tests := []struct {
		name     string
		results  []HealthCheckResult
		expected HealthSummary
	}{
		{
			name: "all healthy",
			results: []HealthCheckResult{
				{Status: HealthStatusHealthy},
				{Status: HealthStatusHealthy},
			},
			expected: HealthSummary{
				Total:   2,
				Healthy: 2,
				Overall: HealthStatusHealthy,
			},
		},
		{
			name: "mixed status",
			results: []HealthCheckResult{
				{Status: HealthStatusHealthy},
				{Status: HealthStatusDegraded},
			},
			expected: HealthSummary{
				Total:    2,
				Healthy:  1,
				Degraded: 1,
				Overall:  HealthStatusDegraded,
			},
		},
		{
			name: "has unhealthy",
			results: []HealthCheckResult{
				{Status: HealthStatusHealthy},
				{Status: HealthStatusUnhealthy},
			},
			expected: HealthSummary{
				Total:     2,
				Healthy:   1,
				Unhealthy: 1,
				Overall:   HealthStatusUnhealthy,
			},
		},
		{
			name:    "empty",
			results: []HealthCheckResult{},
			expected: HealthSummary{
				Total:   0,
				Overall: HealthStatusUnknown,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := calculateSummary(tt.results)

			if summary.Total != tt.expected.Total {
				t.Errorf("Expected total %d, got %d", tt.expected.Total, summary.Total)
			}
			if summary.Healthy != tt.expected.Healthy {
				t.Errorf("Expected healthy %d, got %d", tt.expected.Healthy, summary.Healthy)
			}
			if summary.Degraded != tt.expected.Degraded {
				t.Errorf("Expected degraded %d, got %d", tt.expected.Degraded, summary.Degraded)
			}
			if summary.Unhealthy != tt.expected.Unhealthy {
				t.Errorf("Expected unhealthy %d, got %d", tt.expected.Unhealthy, summary.Unhealthy)
			}
			if summary.Overall != tt.expected.Overall {
				t.Errorf("Expected overall %s, got %s", tt.expected.Overall, summary.Overall)
			}
		})
	}
}

func TestHTTPHealthCheck(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		responseBody   string
		expectedStatus HealthStatus
	}{
		{
			name:           "200 OK",
			statusCode:     200,
			responseBody:   `{"status":"healthy"}`,
			expectedStatus: HealthStatusHealthy,
		},
		{
			name:           "200 degraded",
			statusCode:     200,
			responseBody:   `{"status":"degraded"}`,
			expectedStatus: HealthStatusDegraded,
		},
		{
			name:           "500 error",
			statusCode:     500,
			responseBody:   `{"error":"internal server error"}`,
			expectedStatus: HealthStatusUnhealthy,
		},
		{
			name:           "302 redirect",
			statusCode:     302,
			responseBody:   "",
			expectedStatus: HealthStatusHealthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			tcpAddr, _ := server.Listener.Addr().(*net.TCPAddr)
			port := tcpAddr.Port

			checker := &HealthChecker{
				timeout:         5 * time.Second,
				defaultEndpoint: "/health",
				httpClient: &http.Client{
					Timeout: 5 * time.Second,
					CheckRedirect: func(req *http.Request, via []*http.Request) error {
						return http.ErrUseLastResponse
					},
				},
			}

			result := checker.tryHTTPHealthCheck(context.Background(), port)

			if result == nil {
				t.Fatal("Expected result, got nil")
			}

			if result.Status != tt.expectedStatus {
				t.Errorf("Expected status %s, got %s", tt.expectedStatus, result.Status)
			}

			if result.StatusCode != tt.statusCode {
				t.Errorf("Expected status code %d, got %d", tt.statusCode, result.StatusCode)
			}
		})
	}
}

func TestPortCheck(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer server.Close()

	tcpAddr, _ := server.Listener.Addr().(*net.TCPAddr)
	port := tcpAddr.Port

	checker := &HealthChecker{
		timeout: 5 * time.Second,
	}

	if !checker.checkPort(context.Background(), port) {
		t.Error("Expected port to be listening")
	}

	if checker.checkPort(context.Background(), 64999) {
		t.Error("Expected port to not be listening")
	}
}

func TestFilterServices(t *testing.T) {
	services := []ServiceInfo{
		{Name: "web"},
		{Name: "api"},
		{Name: "db"},
	}

	filter := []string{"web", "api"}
	filtered := FilterServices(services, filter)

	if len(filtered) != 2 {
		t.Errorf("Expected 2 services, got %d", len(filtered))
	}

	for _, svc := range filtered {
		if svc.Name != "web" && svc.Name != "api" {
			t.Errorf("Unexpected service in filtered list: %s", svc.Name)
		}
	}
}

func TestCheckService(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"status":"healthy"}`))
	}))
	defer server.Close()

	tcpAddr, _ := server.Listener.Addr().(*net.TCPAddr)
	port := tcpAddr.Port

	checker := &HealthChecker{
		timeout:         5 * time.Second,
		defaultEndpoint: "/health",
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}

	svc := ServiceInfo{
		Name: "test-service",
		Port: port,
	}

	result := checker.CheckService(context.Background(), svc)

	if result.ServiceName != "test-service" {
		t.Errorf("Expected service name 'test-service', got '%s'", result.ServiceName)
	}

	if result.Status != HealthStatusHealthy {
		t.Errorf("Expected status healthy, got %s", result.Status)
	}

	if result.CheckType != HealthCheckTypeHTTP {
		t.Errorf("Expected check type HTTP, got %s", result.CheckType)
	}
}

func TestCheckServiceFallback(t *testing.T) {
	checker := &HealthChecker{
		timeout:         5 * time.Second,
		defaultEndpoint: "/health",
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}

	svc := ServiceInfo{
		Name: "test-service",
		Port: 64998,
	}

	result := checker.CheckService(context.Background(), svc)

	if result.CheckType != HealthCheckTypeTCP {
		t.Errorf("Expected check type tcp, got %s", result.CheckType)
	}

	if result.Status != HealthStatusUnhealthy {
		t.Errorf("Expected status unhealthy, got %s", result.Status)
	}
}

func TestCustomHealthCheck_HTTPUrl(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ready" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"healthy","connections":4}`))
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	checker := &HealthChecker{
		timeout:         5 * time.Second,
		defaultEndpoint: "/health",
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}

	config := &HealthCheckConfig{
		Test: []string{server.URL + "/ready"},
	}

	result := checker.tryCustomHealthCheck(context.Background(), config, ServiceInfo{})

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if result.Status != HealthStatusHealthy {
		t.Errorf("Expected healthy status, got %s", result.Status)
	}

	if result.StatusCode != http.StatusOK {
		t.Errorf("Expected status code 200, got %d", result.StatusCode)
	}
}

func TestCustomHealthCheck_HTTPUrl_Unhealthy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	checker := &HealthChecker{
		timeout:         5 * time.Second,
		defaultEndpoint: "/health",
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}

	config := &HealthCheckConfig{
		Test: []string{server.URL + "/health"},
	}

	result := checker.tryCustomHealthCheck(context.Background(), config, ServiceInfo{})

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if result.Status != HealthStatusUnhealthy {
		t.Errorf("Expected unhealthy status, got %s", result.Status)
	}
}

func TestTryHTTPHealthCheck_Skips404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ready":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"healthy"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	_, portStr, _ := net.SplitHostPort(server.Listener.Addr().String())
	port := 0
	_, _ = fmt.Sscanf(portStr, "%d", &port)

	checker := &HealthChecker{
		timeout:         5 * time.Second,
		defaultEndpoint: "/health",
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}

	result := checker.tryHTTPHealthCheck(context.Background(), port)

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if result.Status != HealthStatusHealthy {
		t.Errorf("Expected healthy status, got %s (endpoint: %s)", result.Status, result.Endpoint)
	}
}

func TestTryHTTPHealthCheck_Skips400BadRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("Bad Request"))
	}))
	defer server.Close()

	_, portStr, _ := net.SplitHostPort(server.Listener.Addr().String())
	port := 0
	_, _ = fmt.Sscanf(portStr, "%d", &port)

	checker := &HealthChecker{
		timeout:         5 * time.Second,
		defaultEndpoint: "/health",
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}

	result := checker.tryHTTPHealthCheck(context.Background(), port)

	if result != nil {
		t.Errorf("Expected nil result for 400 responses (cascade to port check), got status: %s", result.Status)
	}
}

func TestCheckService_StoppedService(t *testing.T) {
	checker := &HealthChecker{
		timeout:         5 * time.Second,
		defaultEndpoint: "/health",
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}

	svc := ServiceInfo{
		Name:           "stopped-service",
		Port:           64998,
		RegistryStatus: "stopped",
	}

	result := checker.CheckService(context.Background(), svc)

	if result.Status != HealthStatusUnknown {
		t.Errorf("Expected status unknown for stopped service, got %s", result.Status)
	}

	if result.ServiceName != "stopped-service" {
		t.Errorf("Expected service name 'stopped-service', got '%s'", result.ServiceName)
	}
}

func TestCheckService_RunningService(t *testing.T) {
	checker := &HealthChecker{
		timeout:         5 * time.Second,
		defaultEndpoint: "/health",
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}

	svc := ServiceInfo{
		Name:           "running-service",
		Port:           64998,
		RegistryStatus: "running",
	}

	result := checker.CheckService(context.Background(), svc)

	if result.Status != HealthStatusUnhealthy {
		t.Errorf("Expected status unhealthy for running service with dead port, got %s", result.Status)
	}
}

// TrackFailure helper for tests (simulates monitor failure tracking).
type failureTracker struct {
	failureCount    map[string]int
	lastSuccessTime map[string]time.Time
	mu              sync.RWMutex
}

func newFailureTracker() *failureTracker {
	return &failureTracker{
		failureCount:    make(map[string]int),
		lastSuccessTime: make(map[string]time.Time),
	}
}

func (ft *failureTracker) trackFailure(result *HealthCheckResult) {
	ft.mu.Lock()
	defer ft.mu.Unlock()

	serviceName := result.ServiceName

	switch result.Status {
	case HealthStatusUnhealthy:
		ft.failureCount[serviceName]++
		result.ConsecutiveFailures = ft.failureCount[serviceName]
		if lastSuccess, exists := ft.lastSuccessTime[serviceName]; exists {
			result.LastSuccessTime = &lastSuccess
		}
	case HealthStatusHealthy:
		ft.failureCount[serviceName] = 0
		result.ConsecutiveFailures = 0
		now := time.Now()
		ft.lastSuccessTime[serviceName] = now
		result.LastSuccessTime = &now
	default:
		if count, exists := ft.failureCount[serviceName]; exists {
			result.ConsecutiveFailures = count
		}
		if lastSuccess, exists := ft.lastSuccessTime[serviceName]; exists {
			result.LastSuccessTime = &lastSuccess
		}
	}
}

func TestTrackFailure(t *testing.T) {
	tracker := newFailureTracker()
	serviceName := "test-service"

	result := HealthCheckResult{ServiceName: serviceName, Status: HealthStatusUnhealthy}
	tracker.trackFailure(&result)
	if result.ConsecutiveFailures != 1 {
		t.Errorf("Expected 1, got %d", result.ConsecutiveFailures)
	}

	result2 := HealthCheckResult{ServiceName: serviceName, Status: HealthStatusUnhealthy}
	tracker.trackFailure(&result2)
	if result2.ConsecutiveFailures != 2 {
		t.Errorf("Expected 2, got %d", result2.ConsecutiveFailures)
	}

	result3 := HealthCheckResult{ServiceName: serviceName, Status: HealthStatusHealthy}
	tracker.trackFailure(&result3)
	if result3.ConsecutiveFailures != 0 {
		t.Errorf("Expected 0, got %d", result3.ConsecutiveFailures)
	}
	if result3.LastSuccessTime == nil {
		t.Error("Expected last success time to be set")
	}

	result4 := HealthCheckResult{ServiceName: serviceName, Status: HealthStatusUnhealthy}
	tracker.trackFailure(&result4)
	if result4.ConsecutiveFailures != 1 {
		t.Errorf("Expected 1, got %d", result4.ConsecutiveFailures)
	}
}

func TestTrackFailure_DegradedStatus(t *testing.T) {
	tracker := newFailureTracker()
	serviceName := "test-service"
	tracker.failureCount[serviceName] = 3

	result := HealthCheckResult{ServiceName: serviceName, Status: HealthStatusDegraded}
	tracker.trackFailure(&result)

	if result.ConsecutiveFailures != 3 {
		t.Errorf("Expected 3, got %d", result.ConsecutiveFailures)
	}
}

func TestTrackFailure_ConcurrentAccess(t *testing.T) {
	tracker := newFailureTracker()
	serviceName := "test-service"
	concurrency := 100

	done := make(chan bool, concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			result := HealthCheckResult{ServiceName: serviceName, Status: HealthStatusUnhealthy}
			tracker.trackFailure(&result)
			done <- true
		}()
	}

	for i := 0; i < concurrency; i++ {
		<-done
	}

	if tracker.failureCount[serviceName] != concurrency {
		t.Errorf("Expected %d, got %d", concurrency, tracker.failureCount[serviceName])
	}
}

func TestTrackFailure_MultipleServices(t *testing.T) {
	tracker := newFailureTracker()

	for i := 0; i < 3; i++ {
		result := HealthCheckResult{ServiceName: "service1", Status: HealthStatusUnhealthy}
		tracker.trackFailure(&result)
	}

	for i := 0; i < 5; i++ {
		result := HealthCheckResult{ServiceName: "service2", Status: HealthStatusUnhealthy}
		tracker.trackFailure(&result)
	}

	if tracker.failureCount["service1"] != 3 {
		t.Errorf("Expected 3, got %d", tracker.failureCount["service1"])
	}
	if tracker.failureCount["service2"] != 5 {
		t.Errorf("Expected 5, got %d", tracker.failureCount["service2"])
	}

	result := HealthCheckResult{ServiceName: "service1", Status: HealthStatusHealthy}
	tracker.trackFailure(&result)

	if tracker.failureCount["service1"] != 0 {
		t.Errorf("Expected 0, got %d", tracker.failureCount["service1"])
	}
	if tracker.failureCount["service2"] != 5 {
		t.Errorf("Expected 5, got %d", tracker.failureCount["service2"])
	}
}

func TestTrackFailure_LastSuccessTime(t *testing.T) {
	tracker := newFailureTracker()
	serviceName := "test-service"

	result1 := HealthCheckResult{ServiceName: serviceName, Status: HealthStatusHealthy}
	tracker.trackFailure(&result1)

	if result1.LastSuccessTime == nil {
		t.Fatal("Expected last success time to be set")
	}
	firstSuccess := *result1.LastSuccessTime

	time.Sleep(10 * time.Millisecond)

	result2 := HealthCheckResult{ServiceName: serviceName, Status: HealthStatusUnhealthy}
	tracker.trackFailure(&result2)

	if result2.LastSuccessTime == nil {
		t.Fatal("Expected last success time to be preserved")
	}
	if !result2.LastSuccessTime.Equal(firstSuccess) {
		t.Error("Expected last success time to match first success")
	}

	result3 := HealthCheckResult{ServiceName: serviceName, Status: HealthStatusHealthy}
	tracker.trackFailure(&result3)

	if result3.LastSuccessTime == nil {
		t.Fatal("Expected last success time to be updated")
	}
	if result3.LastSuccessTime.Before(firstSuccess) {
		t.Error("Expected last success time to be more recent")
	}
}
