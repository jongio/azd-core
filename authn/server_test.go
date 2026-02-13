package authn

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestIsValidScopeFormat(t *testing.T) {
	tests := []struct {
		name  string
		scope string
		want  bool
	}{
		{"valid HTTPS scope", "https://management.azure.com/.default", true},
		{"valid HTTPS with path", "https://graph.microsoft.com/.default", true},
		{"reject HTTP", "http://management.azure.com/.default", false},
		{"reject empty", "", false},
		{"reject control char newline", "https://evil.com/.default\n", false},
		{"reject control char tab", "https://evil.com/.default\t", false},
		{"reject backtick", "https://evil.com/`whoami`", false},
		{"reject semicolon", "https://evil.com/;rm -rf /", false},
		{"reject pipe", "https://evil.com/|cat /etc/passwd", false},
		{"reject ampersand", "https://evil.com/&background", false},
		{"reject dollar", "https://evil.com/$HOME", false},
		{"reject parens", "https://evil.com/$(whoami)", false},
		{"reject >512 bytes", "https://example.com/" + strings.Repeat("a", 500), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidScopeFormat(tt.scope)
			if got != tt.want {
				t.Errorf("isValidScopeFormat(%q) = %v, want %v", tt.scope, got, tt.want)
			}
		})
	}
}

func TestNormalizeScope(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no change needed", "https://management.azure.com/.default", "https://management.azure.com/.default"},
		{"fix double slash", "https://management.azure.com//.default", "https://management.azure.com/.default"},
		{"multiple double slashes", "https://example.com//.default//.other", "https://example.com/.default/.other"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeScope(tt.input)
			if got != tt.want {
				t.Errorf("normalizeScope(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsScopeAllowed(t *testing.T) {
	allowed := map[string]bool{
		"https://management.azure.com/.default": true,
		"https://graph.microsoft.com/.default":  true,
	}

	tests := []struct {
		name    string
		scope   string
		allowed map[string]bool
		want    bool
	}{
		{"allowed scope", "https://management.azure.com/.default", allowed, true},
		{"denied scope", "https://evil.com/.default", allowed, false},
		{"wildcard nil map", "https://anything.com/.default", nil, true},
		{"allowed via normalization", "https://management.azure.com//.default", allowed, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isScopeAllowed(tt.scope, tt.allowed)
			if got != tt.want {
				t.Errorf("isScopeAllowed(%q) = %v, want %v", tt.scope, got, tt.want)
			}
		})
	}
}

func TestBuildAllowedScopes(t *testing.T) {
	tests := []struct {
		name  string
		input string
		isNil bool
		keys  []string
	}{
		{"wildcard star", "*", true, nil},
		{"empty string", "", true, nil},
		{"single scope", "https://management.azure.com/.default", false, []string{"https://management.azure.com/.default"}},
		{"comma-separated", "https://management.azure.com/.default, https://graph.microsoft.com/.default", false, []string{
			"https://management.azure.com/.default",
			"https://graph.microsoft.com/.default",
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildAllowedScopes(tt.input)
			if tt.isNil {
				if got != nil {
					t.Errorf("buildAllowedScopes(%q) = %v, want nil", tt.input, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("buildAllowedScopes(%q) = nil, want non-nil", tt.input)
			}
			for _, key := range tt.keys {
				if !got[key] {
					t.Errorf("buildAllowedScopes(%q) missing key %q", tt.input, key)
				}
			}
		})
	}
}

// --- Integration tests ---

func startTestServer(t *testing.T, allowedScopes string) (*Server, *http.Client) {
	t.Helper()
	s := &Server{
		Config: ServerConfig{
			Port:          0,
			Bind:          "127.0.0.1",
			AllowedScopes: allowedScopes,
		},
	}
	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	t.Cleanup(s.Stop)

	clientTLS, err := ClientTLSConfig(s.Bundle())
	if err != nil {
		t.Fatalf("ClientTLSConfig() error: %v", err)
	}

	client := &http.Client{
		Transport: &http.Transport{TLSClientConfig: clientTLS},
	}
	return s, client
}

func serverURL(s *Server, path string) string {
	return fmt.Sprintf("https://localhost:%d%s", s.Port(), path)
}

func postToken(client *http.Client, url string, body interface{}) (*http.Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return client.Post(url, "application/json", bytes.NewReader(data))
}

func TestServerMTLSEnforcement(t *testing.T) {
	s, mtlsClient := startTestServer(t, "*")

	// With client cert: TLS handshake succeeds (handler may return 500 from GetToken)
	resp, err := postToken(mtlsClient, serverURL(s, "/token"), TokenRequest{Scopes: []string{"https://management.azure.com/.default"}})
	if err != nil {
		t.Fatalf("mTLS request failed: %v", err)
	}
	resp.Body.Close()

	// Without client cert: TLS handshake should fail
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(s.Bundle().CACertPEM)
	noClientCert := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:    caCertPool,
				MinVersion: tls.VersionTLS13,
			},
		},
	}
	_, err = noClientCert.Post(
		serverURL(s, "/token"), "application/json",
		strings.NewReader(`{"scopes":["https://test.com/.default"]}`),
	)
	if err == nil {
		t.Fatal("expected TLS handshake error without client cert, got nil")
	}
}

func TestServerRejectsWrongMethod(t *testing.T) {
	s, client := startTestServer(t, "*")
	resp, err := client.Get(serverURL(s, "/token"))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", resp.StatusCode)
	}
}

func TestServerRejectsInvalidContentType(t *testing.T) {
	s, client := startTestServer(t, "*")
	resp, err := client.Post(
		serverURL(s, "/token"), "text/plain",
		strings.NewReader(`{"scopes":["https://test.com/.default"]}`),
	)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Errorf("expected 415, got %d", resp.StatusCode)
	}
}

func TestServerRejectsEmptyScope(t *testing.T) {
	s, client := startTestServer(t, "*")
	resp, err := postToken(client, serverURL(s, "/token"), TokenRequest{Scopes: []string{}})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestServerRejectsMultipleScopes(t *testing.T) {
	s, client := startTestServer(t, "*")
	resp, err := postToken(client, serverURL(s, "/token"), TokenRequest{
		Scopes: []string{"https://a.com/.default", "https://b.com/.default"},
	})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestServerRejectsInvalidScopeFormat(t *testing.T) {
	s, client := startTestServer(t, "*")
	resp, err := postToken(client, serverURL(s, "/token"), TokenRequest{
		Scopes: []string{"https://evil.com;rm -rf /"},
	})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestServerScopeAllowlist(t *testing.T) {
	allowedScope := "https://management.azure.com/.default"
	s, client := startTestServer(t, allowedScope)
	url := serverURL(s, "/token")

	// Allowed scope: should pass allowlist (may get 500 from GetToken, but not 403)
	resp, err := postToken(client, url, TokenRequest{Scopes: []string{allowedScope}})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusForbidden {
		t.Error("allowed scope got 403")
	}

	// Disallowed scope: should get 403
	resp, err = postToken(client, url, TokenRequest{Scopes: []string{"https://evil.example.com/.default"}})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("disallowed scope: expected 403, got %d", resp.StatusCode)
	}
}

func TestServerRateLimiting(t *testing.T) {
	s, client := startTestServer(t, "*")
	url := serverURL(s, "/token")

	// Send 30 requests (all should pass rate limit, return 405 for GET)
	for i := 0; i < 30; i++ {
		resp, err := client.Get(url)
		if err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests {
			t.Fatalf("request %d got 429 unexpectedly", i)
		}
	}

	// 31st should be rate limited
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("request 31 failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", resp.StatusCode)
	}
}

func TestServerRequestSizeLimit(t *testing.T) {
	s, client := startTestServer(t, "*")
	// Create a body larger than 4096 bytes (the server's LimitReader cap)
	largeScope := "https://example.com/" + strings.Repeat("a", 4080)
	resp, err := postToken(client, serverURL(s, "/token"), TokenRequest{Scopes: []string{largeScope}})
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestIPv6RateLimiterNormalization(t *testing.T) {
	rl := newRateLimiter(5, 1*time.Minute)

	// These are all the same IPv6 address in different representations
	ipv6Variants := []string{
		"::1",
		"0:0:0:0:0:0:0:1",
		"0000:0000:0000:0000:0000:0000:0000:0001",
	}

	// Test if we can exceed rate limit using different representations
	totalAllowed := 0
	for i := 0; i < 10; i++ {
		ip := ipv6Variants[i%len(ipv6Variants)]
		if rl.allow(ip) {
			totalAllowed++
			t.Logf("Request %d with IP '%s': allowed (total=%d)", i+1, ip, totalAllowed)
		} else {
			t.Logf("Request %d with IP '%s': rate limited", i+1, ip)
		}
	}

	// If different representations are treated as different IPs, 
	// we'd get more than 5 requests allowed
	if totalAllowed > 5 {
		t.Errorf("Rate limiter can be bypassed! Allowed %d requests using IPv6 variants (limit was 5)", totalAllowed)
	}
}

func TestRateLimiterMemoryLeak(t *testing.T) {
	rl := newRateLimiter(5, 100*time.Millisecond)

	// Simulate 1000 different IPs making requests just under the limit
	for ip := 0; ip < 1000; ip++ {
		clientIP := fmt.Sprintf("192.168.%d.%d", ip/256, ip%256)
		// Make 4 requests (under the limit of 5)
		for req := 0; req < 4; req++ {
			rl.allow(clientIP)
		}
	}

	// Check map size - should have 1000 entries
	rl.mu.Lock()
	mapSize := len(rl.requests)
	rl.mu.Unlock()

	if mapSize != 1000 {
		t.Logf("Map has %d entries (expected 1000)", mapSize)
	}

	// Wait for window to expire
	time.Sleep(150 * time.Millisecond)

	// Make requests from 1000 NEW IPs
	for ip := 1000; ip < 2000; ip++ {
		clientIP := fmt.Sprintf("192.168.%d.%d", ip/256, ip%256)
		rl.allow(clientIP)
	}

	// Check map size again
	rl.mu.Lock()
	newMapSize := len(rl.requests)
	rl.mu.Unlock()

	// Should have ~1000 entries (old ones cleaned up), but will have 2000 if no cleanup
	if newMapSize > 1500 {
		t.Errorf("Rate limiter memory leak detected! Map has %d entries, should be ~1000 (old entries not cleaned up)", newMapSize)
	}
}

func TestServerStartCustomCertsDir(t *testing.T) {
	customDir := t.TempDir()
	srv := &Server{
		Config: ServerConfig{
			Port:          0,
			CertsDir:      customDir,
			AllowedScopes: "*",
		},
	}
	err := srv.Start(context.Background())
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer srv.Stop()

	// Verify certs written to custom dir
	for _, f := range []string{"ca.pem", "client.pem", "client-key.pem"} {
		path := filepath.Join(customDir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected cert file %s in custom dir", f)
		}
	}

	// Verify CertsDir() returns custom dir
	if srv.CertsDir() != customDir {
		t.Errorf("CertsDir() = %q, want %q", srv.CertsDir(), customDir)
	}
}

func TestServerOnReadyCallback(t *testing.T) {
	var readyPort int
	srv := &Server{
		Config: ServerConfig{
			Port:          0,
			AllowedScopes: "*",
			OnReady: func(port int) {
				readyPort = port
			},
		},
	}
	err := srv.Start(context.Background())
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer srv.Stop()

	if readyPort == 0 {
		t.Error("OnReady callback was not called")
	}
	if readyPort != srv.Port() {
		t.Errorf("OnReady port = %d, want %d", readyPort, srv.Port())
	}
}

func TestServerStopCleansUpTempDir(t *testing.T) {
	srv := &Server{
		Config: ServerConfig{
			Port:          0,
			AllowedScopes: "*",
		},
	}
	err := srv.Start(context.Background())
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	certsDir := srv.CertsDir()
	// Verify dir exists
	if _, err := os.Stat(certsDir); os.IsNotExist(err) {
		t.Fatal("certs dir does not exist after Start")
	}

	srv.Stop()

	// Verify temp dir was cleaned up
	if _, err := os.Stat(certsDir); !os.IsNotExist(err) {
		t.Error("certs dir still exists after Stop")
	}
}

func TestServerStopPreservesCustomDir(t *testing.T) {
	customDir := t.TempDir()
	srv := &Server{
		Config: ServerConfig{
			Port:          0,
			CertsDir:      customDir,
			AllowedScopes: "*",
		},
	}
	err := srv.Start(context.Background())
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	srv.Stop()

	// Verify custom dir still exists
	if _, err := os.Stat(customDir); os.IsNotExist(err) {
		t.Error("custom certs dir should not be deleted on Stop")
	}
}

func TestServerStopIdempotent(t *testing.T) {
	srv := &Server{
		Config: ServerConfig{
			Port:          0,
			AllowedScopes: "*",
		},
	}
	err := srv.Start(context.Background())
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Should not panic
	srv.Stop()
	srv.Stop()
}

func TestServerBundleAccessors(t *testing.T) {
	srv := &Server{
		Config: ServerConfig{
			Port:          0,
			AllowedScopes: "*",
		},
	}
	err := srv.Start(context.Background())
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer srv.Stop()

	if srv.Port() == 0 {
		t.Error("Port() should be non-zero after Start")
	}
	if srv.CertsDir() == "" {
		t.Error("CertsDir() should be non-empty after Start")
	}
	if srv.Bundle() == nil {
		t.Error("Bundle() should be non-nil after Start")
	}
	if err := srv.Bundle().Validate(); err != nil {
		t.Errorf("Bundle().Validate() failed: %v", err)
	}
}

func TestWriteCertsToDirSymlinkTOCTOU(t *testing.T) {
	bundle, err := GenerateBundle()
	if err != nil {
		t.Fatalf("GenerateBundle() error: %v", err)
	}

	dir := t.TempDir()

	// Create a target directory that we'll symlink to
	targetDir := filepath.Join(dir, "target")
	if err := os.MkdirAll(targetDir, 0700); err != nil {
		t.Fatalf("failed to create target dir: %v", err)
	}

	// Create the certs directory normally first
	certsDir := filepath.Join(dir, "certs")
	if err := os.MkdirAll(certsDir, 0700); err != nil {
		t.Fatalf("failed to create certs dir: %v", err)
	}

	// Now remove it and create a symlink in its place
	if err := os.RemoveAll(certsDir); err != nil {
		t.Fatalf("failed to remove certs dir: %v", err)
	}

	// Skip on Windows - symlinks require admin privileges
	if runtime.GOOS == "windows" {
		t.Skip("Skipping symlink test on Windows")
	}

	if err := os.Symlink(targetDir, certsDir); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	// This should be detected and rejected by WriteCertsToDir
	err = WriteCertsToDir(bundle, certsDir)
	if err == nil {
		t.Error("WriteCertsToDir should reject symlink directory, but it succeeded")
	}
	if err != nil && !strings.Contains(err.Error(), "symlink") {
		t.Errorf("Expected symlink error, got: %v", err)
	}
}
