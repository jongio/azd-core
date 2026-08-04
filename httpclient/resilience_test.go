package httpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestIsRetryableStatus(t *testing.T) {
	t.Parallel()

	retryable := []int{
		http.StatusRequestTimeout,
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout,
	}

	for _, code := range retryable {
		if !isRetryableStatus(code) {
			t.Errorf("status %d should be retryable", code)
		}
	}

	// 501 and 505 describe a permanent property of the server, and the 2xx/4xx
	// codes below are not transient either.
	permanent := []int{
		http.StatusOK,
		http.StatusCreated,
		http.StatusBadRequest,
		http.StatusUnauthorized,
		http.StatusForbidden,
		http.StatusNotFound,
		http.StatusConflict,
		http.StatusNotImplemented,
		http.StatusHTTPVersionNotSupported,
	}

	for _, code := range permanent {
		if isRetryableStatus(code) {
			t.Errorf("status %d should not be retryable", code)
		}
	}
}

func TestRetryBackoff_GrowsAndStaysInJitterWindow(t *testing.T) {
	t.Parallel()

	// Attempt n has a base delay of 2^(n-1) seconds, jittered into [80%, 120%).
	for _, tc := range []struct {
		attempt int
		base    time.Duration
	}{
		{1, time.Second},
		{2, 2 * time.Second},
		{3, 4 * time.Second},
	} {
		low := time.Duration(float64(tc.base) * 0.8)
		high := time.Duration(float64(tc.base) * 1.2)

		for range 50 {
			got := retryBackoff(tc.attempt)
			if got < low || got >= high {
				t.Fatalf("attempt %d: backoff %v outside [%v, %v)", tc.attempt, got, low, high)
			}
		}
	}
}

func TestRetryBackoff_CapsAtMaxRetryBackoff(t *testing.T) {
	t.Parallel()

	// A large attempt would overflow or produce an absurd delay without the cap.
	high := time.Duration(float64(MaxRetryBackoff) * 1.2)

	for _, attempt := range []int{10, 30, 60, 1000} {
		got := retryBackoff(attempt)
		if got >= high {
			t.Errorf("attempt %d: backoff %v exceeds capped window %v", attempt, got, high)
		}
	}
}

func TestRetryBackoff_Jitters(t *testing.T) {
	t.Parallel()

	// Without jitter every client throttled by the same server retries in
	// lockstep, so distinct values across draws are the point of the function.
	seen := make(map[time.Duration]struct{})
	for range 100 {
		seen[retryBackoff(2)] = struct{}{}
	}

	if len(seen) < 2 {
		t.Fatalf("expected jittered delays, got %d distinct value(s)", len(seen))
	}
}

func TestRetryAfterFromResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		headers map[string]string
		want    time.Duration
	}{
		{"nil headers", nil, 0},
		{"retry-after-ms", map[string]string{"retry-after-ms": "250"}, 250 * time.Millisecond},
		{"x-ms-retry-after-ms", map[string]string{"x-ms-retry-after-ms": "1500"}, 1500 * time.Millisecond},
		{"retry-after seconds", map[string]string{"retry-after": "7"}, 7 * time.Second},
		{"zero is ignored", map[string]string{"retry-after": "0"}, 0},
		{"negative is ignored", map[string]string{"retry-after": "-5"}, 0},
		{"garbage is ignored", map[string]string{"retry-after": "soon"}, 0},
		{
			"ms header wins over seconds header",
			map[string]string{"retry-after-ms": "100", "retry-after": "60"},
			100 * time.Millisecond,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			resp := &http.Response{Header: http.Header{}}
			for k, v := range tc.headers {
				resp.Header.Set(k, v)
			}

			if got := retryAfterFromResponse(resp); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRetryAfterFromResponse_NilResponse(t *testing.T) {
	t.Parallel()

	if got := retryAfterFromResponse(nil); got != 0 {
		t.Errorf("got %v, want 0", got)
	}
}

func TestRetryAfterFromResponse_HTTPDate(t *testing.T) {
	t.Parallel()

	resp := &http.Response{Header: http.Header{}}
	resp.Header.Set("retry-after", time.Now().Add(30*time.Second).UTC().Format(http.TimeFormat))

	got := retryAfterFromResponse(resp)
	if got <= 25*time.Second || got > 30*time.Second {
		t.Errorf("got %v, want roughly 30s", got)
	}
}

func TestRetryAfterFromResponse_PastHTTPDateIsIgnored(t *testing.T) {
	t.Parallel()

	resp := &http.Response{Header: http.Header{}}
	resp.Header.Set("retry-after", time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat))

	if got := retryAfterFromResponse(resp); got != 0 {
		t.Errorf("got %v, want 0 for a date already in the past", got)
	}
}

func TestExecute_RetriesTooManyRequests(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			// A tiny Retry-After keeps the test fast and exercises the header
			// path at the same time.
			w.Header().Set("retry-after-ms", "10")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	client := NewClient(nil, false, 5*time.Second)

	resp, err := client.Execute(context.Background(), RequestOptions{
		Method: http.MethodGet,
		URL:    srv.URL,
		Retry:  2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want 200", resp.StatusCode)
	}

	if got := calls.Load(); got != 2 {
		t.Errorf("got %d calls, want 2", got)
	}
}

func TestExecute_ReturnsLastRetryableResponseWhenRetriesExhausted(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("retry-after-ms", "10")
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	client := NewClient(nil, false, 5*time.Second)

	resp, err := client.Execute(context.Background(), RequestOptions{
		Method: http.MethodGet,
		URL:    srv.URL,
		Retry:  2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The caller still gets the response so it can report the real status,
	// rather than a synthetic retry error.
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("got status %d, want 503", resp.StatusCode)
	}

	if got := calls.Load(); got != 3 {
		t.Errorf("got %d calls, want 3 (initial plus 2 retries)", got)
	}
}

func TestExecute_DoesNotRetryNotImplemented(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNotImplemented)
	}))
	defer srv.Close()

	client := NewClient(nil, false, 5*time.Second)

	resp, err := client.Execute(context.Background(), RequestOptions{
		Method: http.MethodGet,
		URL:    srv.URL,
		Retry:  3,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != http.StatusNotImplemented {
		t.Errorf("got status %d, want 501", resp.StatusCode)
	}

	if got := calls.Load(); got != 1 {
		t.Errorf("got %d calls, want 1: 501 is permanent and must not be retried", got)
	}
}

func TestExecute_RetryHonorsContextCancellation(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	client := NewClient(nil, false, 5*time.Second)

	// The first backoff is roughly a second, so the context expires while the
	// client is waiting to retry.
	_, err := client.Execute(ctx, RequestOptions{
		Method: http.MethodGet,
		URL:    srv.URL,
		Retry:  3,
	})
	if err == nil {
		t.Fatal("expected an error once the context is canceled")
	}

	if !strings.Contains(err.Error(), "canceled") {
		t.Errorf("got %q, want a cancellation error", err)
	}
}

func TestIsLoopbackHost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		host string
		want bool
	}{
		{"localhost", true},
		{"LOCALHOST", true},
		{"LocalHost", true},
		{"127.0.0.1", true},
		{"127.1.2.3", true},
		{"::1", true},
		{"example.com", false},
		{"10.0.0.1", false},
		{"169.254.169.254", false},
		{"", false},
		{"not-an-ip", false},
	}

	for _, tc := range tests {
		if got := isLoopbackHost(tc.host); got != tc.want {
			t.Errorf("isLoopbackHost(%q) = %v, want %v", tc.host, got, tc.want)
		}
	}
}

// redirectRequest builds a request carrying the redirect policy that
// checkRedirect reads out of the context.
func redirectRequest(t *testing.T, rawURL string, follow bool, maxRedirects int) *http.Request {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		t.Fatalf("building request for %q: %v", rawURL, err)
	}

	return req.WithContext(context.WithValue(req.Context(), redirectContextKey, redirectConfig{
		followRedirects: follow,
		maxRedirects:    maxRedirects,
	}))
}

func TestCheckRedirect_NotFollowingStopsWithLastResponse(t *testing.T) {
	t.Parallel()

	req := redirectRequest(t, "https://example.com/next", false, 10)
	via := []*http.Request{redirectRequest(t, "https://example.com/first", false, 10)}

	if err := checkRedirect(req, via); err != http.ErrUseLastResponse {
		t.Errorf("got %v, want http.ErrUseLastResponse", err)
	}
}

func TestCheckRedirect_EnforcesMaxRedirects(t *testing.T) {
	t.Parallel()

	req := redirectRequest(t, "https://example.com/3", true, 2)
	via := []*http.Request{
		redirectRequest(t, "https://example.com/1", true, 2),
		redirectRequest(t, "https://example.com/2", true, 2),
	}

	err := checkRedirect(req, via)
	if err == nil || !strings.Contains(err.Error(), "stopped after 2 redirects") {
		t.Errorf("got %v, want a max redirect error", err)
	}
}

func TestCheckRedirect_BlocksMetadataEndpoint(t *testing.T) {
	t.Parallel()

	req := redirectRequest(t, "http://169.254.169.254/metadata/identity/oauth2/token", true, 10)
	via := []*http.Request{redirectRequest(t, "http://example.com/start", true, 10)}

	if err := checkRedirect(req, via); err == nil {
		t.Fatal("a redirect to the cloud metadata endpoint must be blocked")
	}
}

func TestCheckRedirect_BlocksHTTPSDowngrade(t *testing.T) {
	t.Parallel()

	req := redirectRequest(t, "http://example.com/insecure", true, 10)
	via := []*http.Request{redirectRequest(t, "https://example.com/secure", true, 10)}

	if err := checkRedirect(req, via); err == nil {
		t.Fatal("a redirect from HTTPS to HTTP must be blocked so credentials stay encrypted")
	}
}

func TestCheckRedirect_BlocksLoopbackFromRemoteOrigin(t *testing.T) {
	t.Parallel()

	req := redirectRequest(t, "http://127.0.0.1:8080/admin", true, 10)
	via := []*http.Request{redirectRequest(t, "http://example.com/start", true, 10)}

	if err := checkRedirect(req, via); err == nil {
		t.Fatal("a remote server must not be able to redirect the client onto the local machine")
	}
}

func TestCheckRedirect_AllowsLoopbackWhenCallerTargetedLocalhost(t *testing.T) {
	t.Parallel()

	// Running an extension against a local API server is a first-class azd
	// workflow, so a caller who asked for localhost keeps working.
	req := redirectRequest(t, "http://localhost:8080/v2", true, 10)
	via := []*http.Request{redirectRequest(t, "http://localhost:8080/v1", true, 10)}

	if err := checkRedirect(req, via); err != nil {
		t.Errorf("got %v, want localhost to localhost redirects to be allowed", err)
	}
}

func TestCheckRedirect_BlocksUnresolvableHost(t *testing.T) {
	t.Parallel()

	// azdext.SSRFSafeRedirect resolves the redirect host so it can check the
	// address against the private ranges, and it fails closed when resolution
	// does not succeed. A redirect to a host that cannot be resolved is
	// therefore refused rather than attempted.
	req := redirectRequest(t, "https://redirect-target.invalid/asset", true, 10)
	via := []*http.Request{redirectRequest(t, "https://example.com/asset", true, 10)}

	err := checkRedirect(req, via)
	if err == nil {
		t.Fatal("expected an unresolvable redirect host to be blocked")
	}

	if !strings.Contains(err.Error(), "DNS resolution failed") {
		t.Errorf("got %v, want a DNS resolution failure", err)
	}
}

func TestExecute_FollowsRedirectOnLocalhost(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	mux.HandleFunc("/v2", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	mux.HandleFunc("/v1", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, srv.URL+"/v2", http.StatusFound)
	})

	client := NewClient(nil, false, 5*time.Second)

	resp, err := client.Execute(context.Background(), RequestOptions{
		Method:          http.MethodGet,
		URL:             srv.URL + "/v1",
		FollowRedirects: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got status %d, want 200", resp.StatusCode)
	}

	if string(resp.Body) != `{"ok":true}` {
		t.Errorf("got body %q, want the redirect target body", resp.Body)
	}
}
