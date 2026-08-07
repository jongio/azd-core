// Package auth provides Azure authentication token acquisition and caching.
package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

// namedCredential pairs a credential with its display name for diagnostics.
type namedCredential struct {
	name string
	cred tokenCredential
}

// resilientChainCredential tries every credential in order, continuing past
// hard errors (unlike DefaultAzureCredential which stops on
// AuthenticationFailedError). Only fails when ALL credentials fail.
type resilientChainCredential struct {
	creds []namedCredential
}

// GetToken iterates through all credentials, returning the first successful
// token. If all fail, returns an aggregate error preserving the full error chain.
func (c *resilientChainCredential) GetToken(
	ctx context.Context,
	options policy.TokenRequestOptions,
) (azcore.AccessToken, error) {
	errs := make([]error, 0, len(c.creds))
	for _, nc := range c.creds {
		if ctx.Err() != nil {
			return azcore.AccessToken{}, fmt.Errorf("credential chain canceled: %w", ctx.Err())
		}
		token, err := nc.cred.GetToken(ctx, options)
		if err == nil {
			return token, nil
		}
		errs = append(errs, fmt.Errorf("%s: %w", nc.name, err))
	}
	return azcore.AccessToken{}, fmt.Errorf(
		"all %d credentials failed: %w",
		len(c.creds),
		errors.Join(errs...),
	)
}

// newResilientCredentialChain builds a credential chain that tries every
// credential type regardless of error kind. Order is optimized for developer
// workstations: CLI credentials first (fast), then environment/workload
// (conditional on env-vars), then managed identity last (may be slow).
func newResilientCredentialChain() (tokenCredential, error) {
	var creds []namedCredential

	// Developer CLI credentials first - fast and most relevant for azd users.
	if cred, err := azidentity.NewAzureDeveloperCLICredential(nil); err == nil {
		creds = append(creds, namedCredential{"AzureDeveloperCLICredential", cred})
	} else {
		slog.Warn("credential init failed", "credential", "AzureDeveloperCLICredential", "error", err)
	}
	if cred, err := azidentity.NewAzureCLICredential(nil); err == nil {
		creds = append(creds, namedCredential{"AzureCLICredential", cred})
	} else {
		slog.Warn("credential init failed", "credential", "AzureCLICredential", "error", err)
	}

	// Environment and workload credentials - only available when env-vars are set.
	if cred, err := azidentity.NewEnvironmentCredential(nil); err == nil {
		creds = append(creds, namedCredential{"EnvironmentCredential", cred})
	} else {
		slog.Warn("credential init failed", "credential", "EnvironmentCredential", "error", err)
	}
	if cred, err := azidentity.NewWorkloadIdentityCredential(nil); err == nil {
		creds = append(creds, namedCredential{"WorkloadIdentityCredential", cred})
	} else {
		slog.Warn("credential init failed", "credential", "WorkloadIdentityCredential", "error", err)
	}

	// Managed identity last - may timeout on non-Azure hosts and causes
	// hard errors on Azure Arc machines (the root cause of issue #12).
	if cred, err := azidentity.NewManagedIdentityCredential(nil); err == nil {
		creds = append(creds, namedCredential{"ManagedIdentityCredential", cred})
	} else {
		slog.Warn("credential init failed", "credential", "ManagedIdentityCredential", "error", err)
	}

	if len(creds) == 0 {
		return nil, fmt.Errorf("no Azure credential types could be constructed")
	}

	return &resilientChainCredential{creds: creds}, nil
}

const (
	tokenExpirySkew    = 2 * time.Minute
	defaultAuthTimeout = 30 * time.Second
)

// TokenProvider supplies OAuth bearer tokens for a given scope.
type TokenProvider interface {
	GetToken(ctx context.Context, scope string) (string, error)
}

type tokenCredential interface {
	GetToken(ctx context.Context, options policy.TokenRequestOptions) (azcore.AccessToken, error)
}

// AzureTokenProvider implements TokenProvider using azd-core's credential chain
// (DefaultAzureCredential-equivalent) with in-memory token reuse.
type AzureTokenProvider struct {
	credential tokenCredential
	cache      map[string]azcore.AccessToken
	mu         sync.RWMutex
	now        func() time.Time
	timeout    time.Duration
}

var (
	defaultProvider   atomic.Pointer[TokenProvider]
	providerMu        sync.Mutex
	credentialFactory = newResilientCredentialChain
	timeNow           = time.Now
)

// NewAzureTokenProvider creates a provider backed by a resilient credential
// chain that tries all Azure credential types regardless of individual error
// types. The provider caches tokens per scope until close to expiration.
func NewAzureTokenProvider() (*AzureTokenProvider, error) {
	cred, err := credentialFactory()
	if err != nil {
		return nil, err
	}

	return &AzureTokenProvider{
		credential: cred,
		cache:      make(map[string]azcore.AccessToken),
		now:        timeNow,
		timeout:    defaultAuthTimeout,
	}, nil
}

// GetAzureToken acquires a bearer token for the supplied scope using the
// shared provider instance (cached credential and token reuse).
func GetAzureToken(ctx context.Context, scope string) (string, error) {
	provider, err := getDefaultProvider()
	if err != nil {
		return "", err
	}

	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, defaultAuthTimeout)
	defer cancel()

	return provider.GetToken(ctx, scope)
}

func getDefaultProvider() (TokenProvider, error) {
	// Fast path: load atomically without locking.
	if p := defaultProvider.Load(); p != nil {
		return *p, nil
	}

	// Slow path: acquire lock and double-check.
	providerMu.Lock()
	defer providerMu.Unlock()

	if p := defaultProvider.Load(); p != nil {
		return *p, nil
	}

	provider, err := NewAzureTokenProvider()
	if err != nil {
		// Don't cache the error - allow retry on next call.
		return nil, err
	}

	var tp TokenProvider = provider
	defaultProvider.Store(&tp)
	return tp, nil
}

// GetToken retrieves an access token for the specified scope with caching.
func (p *AzureTokenProvider) GetToken(ctx context.Context, scope string) (string, error) {
	scope = strings.TrimSpace(scope)
	if scope == "" {
		return "", fmt.Errorf("scope cannot be empty")
	}

	if ctx == nil {
		ctx = context.Background()
	}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, p.timeout)
		defer cancel()
	}

	if token, ok := p.getCached(scope); ok {
		return token, nil
	}

	accessToken, err := p.credential.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{scope},
	})
	if err != nil {
		return "", classifyAuthError(scope, err)
	}

	p.setCached(scope, accessToken)
	return accessToken.Token, nil
}

func (p *AzureTokenProvider) getCached(scope string) (string, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	token, ok := p.cache[scope]
	if !ok || token.Token == "" || token.ExpiresOn.IsZero() {
		return "", false
	}

	if token.ExpiresOn.After(p.now().Add(tokenExpirySkew)) {
		return token.Token, true
	}

	return "", false
}

func (p *AzureTokenProvider) setCached(scope string, token azcore.AccessToken) {
	if token.Token == "" || token.ExpiresOn.IsZero() {
		return
	}

	p.mu.Lock()
	p.cache[scope] = token
	p.mu.Unlock()
}

// AuthPermissionError indicates insufficient permissions for a scope.
type AuthPermissionError struct {
	Scope string
	Err   error
}

func (e *AuthPermissionError) Error() string {
	return fmt.Sprintf("authentication failed: insufficient permissions for scope %s: %s", e.Scope, e.Err)
}

func (e *AuthPermissionError) Unwrap() error { return e.Err }

// AuthCredentialUnavailableError indicates no valid credential is configured.
type AuthCredentialUnavailableError struct {
	Err error
}

func (e *AuthCredentialUnavailableError) Error() string {
	return fmt.Sprintf("authentication failed: not signed in or credential unavailable. Run 'azd auth login' or configure managed identity/environment credentials: %s", e.Err)
}

func (e *AuthCredentialUnavailableError) Unwrap() error { return e.Err }

// AuthError is a generic authentication failure for a scope.
type AuthError struct {
	Scope string
	Err   error
}

func (e *AuthError) Error() string {
	return fmt.Sprintf("authentication failed for scope %s: %s", e.Scope, e.Err)
}

func (e *AuthError) Unwrap() error { return e.Err }

func classifyAuthError(scope string, err error) error {
	// Use typed error checks from the Azure SDK where possible.
	var authFailedErr *azidentity.AuthenticationFailedError
	if errors.As(err, &authFailedErr) {
		resp := authFailedErr.RawResponse
		if resp != nil {
			switch resp.StatusCode {
			case http.StatusUnauthorized, http.StatusForbidden:
				return &AuthPermissionError{Scope: scope, Err: err}
			}
		}
	}

	// Fall back to string matching for errors that don't use SDK typed errors.
	lower := strings.ToLower(err.Error())

	switch {
	case strings.Contains(lower, "insufficient") ||
		strings.Contains(lower, "unauthorized") ||
		strings.Contains(lower, "forbidden") ||
		strings.Contains(lower, "permission"):
		return &AuthPermissionError{Scope: scope, Err: err}
	case strings.Contains(lower, "credential unavailable") ||
		strings.Contains(lower, "login") ||
		strings.Contains(lower, "no accounts") ||
		strings.Contains(lower, "authentication required") ||
		strings.Contains(lower, "configure"):
		return &AuthCredentialUnavailableError{Err: err}
	default:
		return &AuthError{Scope: scope, Err: err}
	}
}

// MockTokenProvider is a mock implementation for testing
type MockTokenProvider struct {
	Token string
	Error error
}

// GetToken returns the mock token or error
func (m *MockTokenProvider) GetToken(ctx context.Context, scope string) (string, error) {
	if m.Error != nil {
		return "", m.Error
	}
	return m.Token, nil
}
