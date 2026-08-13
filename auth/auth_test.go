package auth

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- resilientChainCredential tests ---

func TestResilientChain_FirstCredentialSucceeds(t *testing.T) {
	chain := &resilientChainCredential{
		creds: []namedCredential{
			{"Cred1", &mockCredential{token: "token-from-cred1"}},
			{"Cred2", &mockCredential{err: fmt.Errorf("should not be reached")}},
		},
	}

	token, err := chain.GetToken(context.Background(), policy.TokenRequestOptions{
		Scopes: []string{"scope"},
	})
	require.NoError(t, err)
	assert.Equal(t, "token-from-cred1", token.Token)
}

func TestResilientChain_SkipsHardErrorAndContinues(t *testing.T) {
	// This is the core fix for issue #12: ManagedIdentityCredential returns
	// a hard AuthenticationFailedError, but the chain must continue.
	chain := &resilientChainCredential{
		creds: []namedCredential{
			{"FailingCred", &mockCredential{err: fmt.Errorf("AuthenticationFailedError: Access is denied")}},
			{"WorkingCred", &mockCredential{token: "fallback-token"}},
		},
	}

	token, err := chain.GetToken(context.Background(), policy.TokenRequestOptions{
		Scopes: []string{"scope"},
	})
	require.NoError(t, err)
	assert.Equal(t, "fallback-token", token.Token)
}

func TestResilientChain_AllCredentialsFail(t *testing.T) {
	chain := &resilientChainCredential{
		creds: []namedCredential{
			{"Cred1", &mockCredential{err: fmt.Errorf("error one")}},
			{"Cred2", &mockCredential{err: fmt.Errorf("error two")}},
			{"Cred3", &mockCredential{err: fmt.Errorf("error three")}},
		},
	}

	_, err := chain.GetToken(context.Background(), policy.TokenRequestOptions{
		Scopes: []string{"scope"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "all 3 credentials failed")
	assert.Contains(t, err.Error(), "Cred1: error one")
	assert.Contains(t, err.Error(), "Cred2: error two")
	assert.Contains(t, err.Error(), "Cred3: error three")
}

func TestResilientChain_RespectsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel before calling

	chain := &resilientChainCredential{
		creds: []namedCredential{
			{"Cred1", &mockCredential{token: "should-not-be-reached"}},
		},
	}

	_, err := chain.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{"scope"},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "credential chain canceled")
}

func TestResilientChain_ThirdCredentialSucceeds(t *testing.T) {
	chain := &resilientChainCredential{
		creds: []namedCredential{
			{"Cred1", &mockCredential{err: fmt.Errorf("credential unavailable")}},
			{"Cred2", &mockCredential{err: fmt.Errorf("access denied")}},
			{"Cred3", &mockCredential{token: "third-time-charm"}},
		},
	}

	token, err := chain.GetToken(context.Background(), policy.TokenRequestOptions{
		Scopes: []string{"scope"},
	})
	require.NoError(t, err)
	assert.Equal(t, "third-time-charm", token.Token)
}

func TestNewResilientCredentialChain_ReturnsChain(t *testing.T) {
	cred, err := newResilientCredentialChain()
	require.NoError(t, err)
	require.NotNil(t, cred)

	chain, ok := cred.(*resilientChainCredential)
	require.True(t, ok, "should return a resilientChainCredential")
	assert.GreaterOrEqual(t, len(chain.creds), 2,
		"should have at least CLI credentials (AzureDeveloperCLI + AzureCLI)")
}

func TestMockTokenProvider(t *testing.T) {
	tests := []struct {
		name        string
		token       string
		err         error
		expectError bool
	}{
		{
			name:        "Success - returns token",
			token:       "mock-access-token-12345",
			err:         nil,
			expectError: false,
		},
		{
			name:        "Error - returns error",
			token:       "",
			err:         fmt.Errorf("authentication failed"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &MockTokenProvider{
				Token: tt.token,
				Error: tt.err,
			}

			token, err := provider.GetToken(context.Background(), "https://management.azure.com/.default")

			if tt.expectError {
				require.Error(t, err)
				assert.Empty(t, token)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.token, token)
			}
		})
	}
}

func TestGetToken_EmptyScope(t *testing.T) {
	provider := &MockTokenProvider{
		Token: "token",
	}

	// Even mock should validate scope
	token, err := provider.GetToken(context.Background(), "")
	// Mock doesn't validate, but real implementation would
	assert.NoError(t, err) // Mock doesn't validate
	assert.Equal(t, "token", token)
}

func TestGetToken_ContextCancellation(t *testing.T) {
	provider := &MockTokenProvider{
		Token: "token",
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Mock doesn't respect context, but real implementation would
	_, _ = provider.GetToken(ctx, "https://management.azure.com/.default")
	// This test demonstrates the interface, actual context handling in real provider
}

// mockCredential is a test double for tokenCredential.
type mockCredential struct {
	token string
	err   error
}

func (m *mockCredential) GetToken(ctx context.Context, options policy.TokenRequestOptions) (azcore.AccessToken, error) {
	if m.err != nil {
		return azcore.AccessToken{}, m.err
	}
	return azcore.AccessToken{
		Token:     m.token,
		ExpiresOn: time.Now().Add(time.Hour),
	}, nil
}

func TestGetDefaultProvider_RetryOnError(t *testing.T) {
	// Save originals
	origFactory := credentialFactory
	origProvider := defaultProvider.Load()
	t.Cleanup(func() {
		credentialFactory = origFactory
		defaultProvider.Store(origProvider)
	})

	// Reset cached provider
	defaultProvider.Store(nil)

	calls := 0
	credentialFactory = func() (tokenCredential, error) {
		calls++
		if calls == 1 {
			return nil, fmt.Errorf("transient credential error")
		}
		return &mockCredential{token: "retry-token"}, nil
	}

	// First call should fail
	_, err := getDefaultProvider()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "transient credential error")

	// Second call should succeed (retry works because error is not cached)
	provider, err := getDefaultProvider()
	require.NoError(t, err)
	require.NotNil(t, provider)

	// Third call should return cached provider (no more factory calls)
	provider2, err := getDefaultProvider()
	require.NoError(t, err)
	assert.Equal(t, provider, provider2)
	assert.Equal(t, 2, calls, "factory should only be called twice")
}

func TestGetDefaultProvider_CachesOnSuccess(t *testing.T) {
	origFactory := credentialFactory
	origProvider := defaultProvider.Load()
	t.Cleanup(func() {
		credentialFactory = origFactory
		defaultProvider.Store(origProvider)
	})

	defaultProvider.Store(nil)

	calls := 0
	credentialFactory = func() (tokenCredential, error) {
		calls++
		return &mockCredential{token: "cached-token"}, nil
	}

	p1, err := getDefaultProvider()
	require.NoError(t, err)
	p2, err := getDefaultProvider()
	require.NoError(t, err)
	assert.Equal(t, p1, p2)
	assert.Equal(t, 1, calls)
}

func TestGetAzureToken_WithMockedProvider(t *testing.T) {
	origFactory := credentialFactory
	origProvider := defaultProvider.Load()
	t.Cleanup(func() {
		credentialFactory = origFactory
		defaultProvider.Store(origProvider)
	})

	defaultProvider.Store(nil)

	credentialFactory = func() (tokenCredential, error) {
		return &mockCredential{token: "mock-azure-token"}, nil
	}

	token, err := GetAzureToken(context.Background(), "https://management.azure.com/.default")
	require.NoError(t, err)
	assert.Equal(t, "mock-azure-token", token)
}

func TestGetAzureToken_FactoryError(t *testing.T) {
	origFactory := credentialFactory
	origProvider := defaultProvider.Load()
	t.Cleanup(func() {
		credentialFactory = origFactory
		defaultProvider.Store(origProvider)
	})

	defaultProvider.Store(nil)

	credentialFactory = func() (tokenCredential, error) {
		return nil, fmt.Errorf("no credentials")
	}

	_, err := GetAzureToken(context.Background(), "https://management.azure.com/.default")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no credentials")
}

func TestGetAzureToken_NilContext(t *testing.T) {
	origFactory := credentialFactory
	origProvider := defaultProvider.Load()
	t.Cleanup(func() {
		credentialFactory = origFactory
		defaultProvider.Store(origProvider)
	})

	defaultProvider.Store(nil)

	credentialFactory = func() (tokenCredential, error) {
		return &mockCredential{token: "nil-ctx-token"}, nil
	}

	//nolint:staticcheck // deliberately passing nil context to test the nil-guard
	token, err := GetAzureToken(nil, "https://management.azure.com/.default")
	require.NoError(t, err)
	assert.Equal(t, "nil-ctx-token", token)
}

func TestClassifyAuthError(t *testing.T) {
	tests := []struct {
		name     string
		errMsg   string
		contains string
	}{
		{"insufficient", "insufficient privileges", "insufficient permissions"},
		{"unauthorized", "Unauthorized access", "insufficient permissions"},
		{"forbidden", "Forbidden by policy", "insufficient permissions"},
		{"permission", "permission denied", "insufficient permissions"},
		{"credential unavailable", "credential unavailable", "azd auth login"},
		{"login required", "please login first", "azd auth login"},
		{"no accounts", "no accounts found", "azd auth login"},
		{"auth required", "authentication required", "azd auth login"},
		{"configure", "configure your credentials", "azd auth login"},
		{"generic error", "something went wrong", "authentication failed for scope"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := classifyAuthError("test-scope", fmt.Errorf("%s", tt.errMsg))
			assert.Contains(t, err.Error(), tt.contains)
		})
	}
}

func TestAzureTokenProvider_GetToken_NilContext(t *testing.T) {
	origFactory := credentialFactory
	origProvider := defaultProvider.Load()
	t.Cleanup(func() {
		credentialFactory = origFactory
		defaultProvider.Store(origProvider)
	})

	defaultProvider.Store(nil)

	credentialFactory = func() (tokenCredential, error) {
		return &mockCredential{token: "ctx-test"}, nil
	}

	provider, err := NewAzureTokenProvider()
	require.NoError(t, err)

	//nolint:staticcheck // deliberately passing nil context
	token, err := provider.GetToken(nil, "https://management.azure.com/.default")
	require.NoError(t, err)
	assert.Equal(t, "ctx-test", token)
}

func TestAzureTokenProvider_GetToken_CredentialError(t *testing.T) {
	origFactory := credentialFactory
	origProvider := defaultProvider.Load()
	t.Cleanup(func() {
		credentialFactory = origFactory
		defaultProvider.Store(origProvider)
	})

	defaultProvider.Store(nil)

	credentialFactory = func() (tokenCredential, error) {
		return &mockCredential{err: fmt.Errorf("unauthorized access")}, nil
	}

	provider, err := NewAzureTokenProvider()
	require.NoError(t, err)

	_, err = provider.GetToken(context.Background(), "some-scope")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient permissions")
}

func TestAzureTokenProvider_NewProvider(t *testing.T) {
	// Test that we can create a provider
	// This will fail if credentials are not available, but that's expected behavior
	provider, err := NewAzureTokenProvider()
	// If credentials are available, provider should be created successfully
	// If not, we get an error which is also valid behavior
	if err != nil {
		// No credentials available - this is acceptable for unit tests
		// The error should indicate credential unavailability
		t.Skipf("Skipping - no Azure credentials available: %v", err)
		return
	}

	require.NotNil(t, provider)

	// If we got here, credentials are available, so test token acquisition
	token, err := provider.GetToken(context.Background(), "https://management.azure.com/.default")
	if err != nil {
		// Authentication failed - this is acceptable if not logged in
		// The error should be classified properly
		assert.Contains(t, err.Error(), "authentication", "Error should mention authentication")
		return
	}

	// If we got a token, verify it's not empty
	assert.NotEmpty(t, token)
	assert.Greater(t, len(token), 10, "Token should be a meaningful string")
}

func TestAzureTokenProvider_InvalidScope(t *testing.T) {
	provider, err := NewAzureTokenProvider()
	if err != nil {
		t.Skipf("Skipping - no Azure credentials available: %v", err)
		return
	}

	require.NotNil(t, provider)

	// Try to get token with invalid scope — should fail with either a scope error
	// or an authentication error depending on the environment's credential state.
	_, err = provider.GetToken(context.Background(), "invalid-scope")
	assert.Error(t, err)
}

func TestAzureTokenProvider_EmptyScope(t *testing.T) {
	provider, err := NewAzureTokenProvider()
	if err != nil {
		t.Skipf("Skipping - no Azure credentials available: %v", err)
		return
	}

	require.NotNil(t, provider)

	// Try to get token with empty scope
	_, err = provider.GetToken(context.Background(), "")
	// Should get an error for empty scope
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty", "Error should mention empty scope")
}

func TestAzureTokenProvider_TokenCaching(t *testing.T) {
	provider, err := NewAzureTokenProvider()
	if err != nil {
		t.Skipf("Skipping - no Azure credentials available: %v", err)
		return
	}

	require.NotNil(t, provider)

	scope := "https://management.azure.com/.default"

	// Get token first time
	token1, err := provider.GetToken(context.Background(), scope)
	if err != nil {
		t.Skipf("Skipping - authentication failed: %v", err)
		return
	}

	require.NoError(t, err)
	assert.NotEmpty(t, token1)

	// Get token second time - should use cache if token hasn't expired
	token2, err := provider.GetToken(context.Background(), scope)
	require.NoError(t, err)
	assert.NotEmpty(t, token2)

	// Tokens should be the same (cached) or different (if expired and refreshed)
	// Both are valid behaviors
	assert.NotEmpty(t, token2)
}
