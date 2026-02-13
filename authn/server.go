package authn

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

// ServerConfig holds the configuration for the mTLS token server.
type ServerConfig struct {
	// Port to listen on. Use 0 for auto-assign.
	Port int
	// Bind address. Empty string auto-detects: 0.0.0.0 on Linux, 127.0.0.1 elsewhere.
	Bind string
	// CertsDir is the directory to write client certs. Empty string uses a temp dir.
	CertsDir string
	// AllowedScopes is a comma-separated list of allowed scopes, or "*" for all.
	AllowedScopes string
	// ExtraSANs are additional DNS names or IP addresses for the server certificate.
	ExtraSANs []string
	// OnReady is called after the server starts listening, with the actual port.
	OnReady func(port int)
}

// Server is an mTLS token server that forwards Azure credential requests
// to the host's azd CLI.
type Server struct {
	Config ServerConfig

	bundle     *Bundle
	certsDir   string
	port       int
	httpServer *http.Server
	listener   net.Listener
	cleanupDir bool
	done       chan struct{} // closed when the serve goroutine exits
}

// Port returns the actual port the server is listening on.
func (s *Server) Port() int {
	return s.port
}

// CertsDir returns the directory where client certificates were written.
func (s *Server) CertsDir() string {
	return s.certsDir
}

// Bundle returns the certificate bundle used by the server.
func (s *Server) Bundle() *Bundle {
	return s.bundle
}

// Start generates certificates, writes client certs, starts the mTLS server
// in a background goroutine, and calls OnReady with the actual port.
func (s *Server) Start(ctx context.Context) error {
	// Auto-detect bind address
	bind := s.Config.Bind
	if bind == "" {
		if runtime.GOOS == "linux" {
			bind = "0.0.0.0"
		} else {
			bind = "127.0.0.1"
		}
	}

	// Determine certs directory
	certsDir := s.Config.CertsDir
	if certsDir == "" {
		tmpDir, err := os.MkdirTemp("", "azd-auth-certs-*")
		if err != nil {
			return fmt.Errorf("failed to create temp certs directory: %w", err)
		}
		certsDir = tmpDir
		s.cleanupDir = true
	}
	s.certsDir = certsDir

	// Generate cert bundle
	bundle, err := GenerateBundle(s.Config.ExtraSANs...)
	if err != nil {
		return fmt.Errorf("failed to generate cert bundle: %w", err)
	}
	s.bundle = bundle

	// Write client certs
	if err := WriteCertsToDir(bundle, certsDir); err != nil {
		return fmt.Errorf("failed to write certs to directory: %w", err)
	}

	// Create mTLS config
	tlsConfig, err := ServerTLSConfig(bundle)
	if err != nil {
		return fmt.Errorf("failed to create server TLS config: %w", err)
	}

	// Build allowed scopes
	allowedScopes := buildAllowedScopes(s.Config.AllowedScopes)

	// Create HTTP handler
	rl := newRateLimiter(30, 1*time.Minute)
	mux := http.NewServeMux()
	mux.HandleFunc("/token", rateLimitMiddleware(rl, makeHandleToken(allowedScopes)))

	// Create HTTPS server
	s.httpServer = &http.Server{
		Handler:      mux,
		TLSConfig:    tlsConfig,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Create TLS listener
	addr := fmt.Sprintf("%s:%d", bind, s.Config.Port)
	s.listener, err = tls.Listen("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}

	tcpAddr, ok := s.listener.Addr().(*net.TCPAddr)
	if !ok {
		_ = s.listener.Close()
		return fmt.Errorf("listener address is not a TCP address")
	}
	s.port = tcpAddr.Port

	// Start server in goroutine
	s.done = make(chan struct{})
	go func() {
		defer close(s.done)
		if err := s.httpServer.Serve(s.listener); err != nil && err != http.ErrServerClosed {
			log.Printf("authn server error: %v", err)
		}
	}()

	// Call OnReady callback
	if s.Config.OnReady != nil {
		s.Config.OnReady(s.port)
	}

	return nil
}

// Stop gracefully shuts down the server and cleans up resources.
func (s *Server) Stop() {
	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
		defer cancel()
		if err := s.httpServer.Shutdown(ctx); err != nil {
			log.Printf("authn server shutdown error: %v", err)
		}
	}
	// Ensure listener is closed even if Shutdown didn't do it
	if s.listener != nil {
		if err := s.listener.Close(); err != nil && !strings.Contains(err.Error(), "use of closed") {
			log.Printf("authn listener close error: %v", err)
		}
	}
	// Wait for serve goroutine to exit
	if s.done != nil {
		select {
		case <-s.done:
		case <-time.After(5 * time.Second):
			log.Printf("authn server goroutine did not exit within 5s")
		}
	}
	if s.cleanupDir && s.certsDir != "" {
		if err := os.RemoveAll(s.certsDir); err != nil {
			log.Printf("failed to cleanup certs directory: %v", err)
		}
	}
}

// rateLimiter implements a sliding-window rate limiter per client IP.
type rateLimiter struct {
	mu       sync.Mutex
	requests map[string][]time.Time
	limit    int
	window   time.Duration
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

func (rl *rateLimiter) allow(clientIP string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Normalize IP address to prevent rate limiter bypass via IPv6 representations
	if ip := net.ParseIP(clientIP); ip != nil {
		clientIP = ip.String()
	}

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Prune old entries
	entries := rl.requests[clientIP]
	valid := entries[:0]
	for _, t := range entries {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	// Clean up stale entries to prevent memory leak
	if len(valid) == 0 {
		delete(rl.requests, clientIP)
	}

	// Prevent unbounded memory growth from distributed attacks
	if len(rl.requests) > 1000 {
		// Evict entries with no recent activity
		for ip, entries := range rl.requests {
			if ip == clientIP {
				continue // Don't evict current client
			}
			allExpired := true
			for _, t := range entries {
				if t.After(cutoff) {
					allExpired = false
					break
				}
			}
			if allExpired {
				delete(rl.requests, ip)
			}
		}
	}

	if len(valid) >= rl.limit {
		rl.requests[clientIP] = valid
		return false
	}

	rl.requests[clientIP] = append(valid, now)
	return true
}

// normalizeScope fixes the Azure SDK double-slash bug where scopes like
// "https://management.azure.com//.default" should be "https://management.azure.com/.default".
func normalizeScope(scope string) string {
	return strings.ReplaceAll(scope, "//.", "/.")
}

// isScopeAllowed checks whether a scope is permitted by the allowlist.
// A nil allowlist means all scopes are allowed (wildcard mode).
func isScopeAllowed(scope string, allowed map[string]bool) bool {
	if allowed == nil {
		return true
	}
	normalized := normalizeScope(scope)
	return allowed[scope] || allowed[normalized]
}

// isValidScopeFormat validates that a scope is a well-formed HTTPS URL
// without control characters or shell metacharacters, and is at most 512 bytes.
func isValidScopeFormat(scope string) bool {
	if len(scope) > 512 {
		return false
	}
	if !strings.HasPrefix(scope, "https://") {
		return false
	}
	for _, c := range scope {
		// Allow only printable ASCII (0x20-0x7e), excluding shell metacharacters
		if c < 0x20 || c > 0x7e || c == '`' || c == ';' || c == '|' || c == '&' || c == '$' || c == '(' || c == ')' {
			return false
		}
	}
	return true
}

// buildAllowedScopes parses a comma-separated list of allowed scopes.
// The special value "*" returns nil, meaning all scopes are allowed.
func buildAllowedScopes(flag string) map[string]bool {
	if flag == "*" || flag == "" {
		return nil
	}
	scopes := make(map[string]bool)
	for _, s := range strings.Split(flag, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			scopes[s] = true
		}
	}
	return scopes
}

func rateLimitMiddleware(rl *rateLimiter, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clientIP, _, _ := net.SplitHostPort(r.RemoteAddr)
		if clientIP == "" {
			clientIP = r.RemoteAddr
		}
		// Normalize IP address to prevent rate limiter bypass via IPv6 representations
		if ip := net.ParseIP(clientIP); ip != nil {
			clientIP = ip.String()
		}
		if !rl.allow(clientIP) {
			log.Printf("[audit] client=%s result=rate-limited", clientIP)
			w.Header().Set("Retry-After", "60")
			writeErrorJSON(w, http.StatusTooManyRequests, "Rate limit exceeded. Try again later.")
			return
		}
		next(w, r)
	}
}

func makeHandleToken(allowedScopes map[string]bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		contentType := r.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "application/json") {
			writeErrorJSON(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
			return
		}

		var req TokenRequest
		if err := json.NewDecoder(io.LimitReader(r.Body, 4096)).Decode(&req); err != nil {
			writeErrorJSON(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		if len(req.Scopes) == 0 {
			writeErrorJSON(w, http.StatusBadRequest, "At least one scope is required")
			return
		}

		if len(req.Scopes) > 1 {
			writeErrorJSON(w, http.StatusBadRequest, "Only one scope is supported")
			return
		}

		scope := req.Scopes[0]
		scope = normalizeScope(scope)

		if !isValidScopeFormat(scope) {
			log.Printf("[audit] scope=%q result=denied reason=invalid-format", scope)
			writeErrorJSON(w, http.StatusBadRequest, "Invalid scope format. Expected https://<resource>/.default")
			return
		}

		if !isScopeAllowed(scope, allowedScopes) {
			log.Printf("[audit] scope=%q result=denied reason=scope-not-allowed", scope)
			// SECURITY: Error messages are intentionally generic to avoid leaking internal details
			writeErrorJSON(w, http.StatusForbidden, "Access denied")
			return
		}

		// SECURITY: Never log the token value
		tok, expiresOn, err := GetToken(scope)
		if err != nil {
			log.Printf("[audit] scope=%q result=error", scope)
			// SECURITY: Error messages are intentionally generic to avoid leaking internal details
			writeErrorJSON(w, http.StatusInternalServerError, "Authentication failed")
			return
		}

		log.Printf("[audit] scope=%q result=success", scope)

		resp := TokenResponse{
			Token:     tok,
			ExpiresOn: expiresOn.Format(time.RFC3339),
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("failed to encode response: %v", err)
		}
	}
}

func writeErrorJSON(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": message}); err != nil {
		log.Printf("failed to encode error response: %v", err)
	}
}
