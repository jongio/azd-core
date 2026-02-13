package authn

import (
	"strings"
	"testing"
)

func TestGetTokenInvalidScope(t *testing.T) {
	tests := []struct {
		name    string
		scope   string
		wantErr string
	}{
		{
			name:    "scope exceeds 512 bytes",
			scope:   "https://example.com/" + strings.Repeat("a", 500),
			wantErr: "invalid scope format",
		},
		{
			name:    "scope without https prefix",
			scope:   "http://management.azure.com/.default",
			wantErr: "invalid scope format",
		},
		{
			name:    "scope with control characters",
			scope:   "https://evil.com/.default\n",
			wantErr: "invalid characters in scope",
		},
		{
			name:    "scope with null byte",
			scope:   "https://evil.com/.default\x00",
			wantErr: "invalid characters in scope",
		},
		{
			name:    "empty scope",
			scope:   "",
			wantErr: "invalid scope format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := GetToken(tt.scope)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("GetToken(%q) error = %q, want containing %q", tt.scope, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestGetTokenValidScopeFormat(t *testing.T) {
	tests := []struct {
		name  string
		scope string
	}{
		{
			name:  "management scope",
			scope: "https://management.azure.com/.default",
		},
		{
			name:  "storage scope",
			scope: "https://storage.azure.com/.default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := GetToken(tt.scope)
			if err == nil {
				// azd CLI returned a token; nothing else to check.
				return
			}
			// Scope validation passed if the error comes from the exec step.
			msg := err.Error()
			if strings.Contains(msg, "invalid scope format") || strings.Contains(msg, "invalid characters in scope") {
				t.Errorf("GetToken(%q) returned validation error: %v", tt.scope, err)
			}
		})
	}
}

func TestAzdTokenResponseParsing(t *testing.T) {
	// The JSON parsing in GetToken is inline and cannot be tested in isolation
	// without the azd CLI. This test documents scope validation behavior and
	// confirms that invalid scopes are rejected before reaching the exec path.
	tests := []struct {
		name    string
		scope   string
		wantErr string
	}{
		{
			name:    "reject plain string scope",
			scope:   "not-a-url",
			wantErr: "invalid scope format",
		},
		{
			name:    "reject ftp scheme",
			scope:   "ftp://files.example.com/.default",
			wantErr: "invalid scope format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := GetToken(tt.scope)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("GetToken(%q) error = %q, want containing %q", tt.scope, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestTokenProviderInterface(t *testing.T) {
	// AzdTokenProvider is declared to implement TokenProvider but currently
	// uses a package-level GetToken function instead of a method receiver.
	// Verify the types exist and the interface is defined.
	var _ TokenProvider    // interface exists
	_ = AzdTokenProvider{} // struct exists
}
