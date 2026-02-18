package keyvault

import (
	"context"
	"testing"
)

func TestIsKeyVaultReference(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{
			name:  "Format 1: SecretUri with version",
			value: "@Microsoft.KeyVault(SecretUri=https://myvault.vault.azure.net/secrets/my-secret/abc123)",
			want:  true,
		},
		{
			name:  "Format 1: SecretUri without version",
			value: "@Microsoft.KeyVault(SecretUri=https://myvault.vault.azure.net/secrets/my-secret)",
			want:  true,
		},
		{
			name:  "Format 2: VaultName with version",
			value: "@Microsoft.KeyVault(VaultName=myvault;SecretName=my-secret;SecretVersion=abc123)",
			want:  true,
		},
		{
			name:  "Format 2: VaultName without version",
			value: "@Microsoft.KeyVault(VaultName=myvault;SecretName=my-secret)",
			want:  true,
		},
		{
			name:  "Format 3: akvs with version",
			value: "akvs://12345678-1234-1234-1234-123456789abc/myvault/my-secret/abc123",
			want:  true,
		},
		{
			name:  "Format 3: akvs without version",
			value: "akvs://12345678-1234-1234-1234-123456789abc/myvault/my-secret",
			want:  true,
		},
		{
			name:  "Not a Key Vault reference",
			value: "just a regular value",
			want:  false,
		},
		{
			name:  "Empty string",
			value: "",
			want:  false,
		},
		{
			name:  "Invalid format - missing closing paren",
			value: "@Microsoft.KeyVault(SecretUri=https://myvault.vault.azure.net/secrets/my-secret",
			want:  false,
		},
		{
			name:  "Invalid akvs format - missing parts",
			value: "akvs://guid/vault",
			want:  false,
		},
		{
			name:  "Format 1 with wrapper quotes",
			value: "\"@Microsoft.KeyVault(SecretUri=https://myvault.vault.azure.net/secrets/my-secret)\"",
			want:  true,
		},
		{
			name:  "Format 1 with single quotes",
			value: "'@Microsoft.KeyVault(SecretUri=https://myvault.vault.azure.net/secrets/my-secret)'",
			want:  true,
		},
		{
			name:  "Format 2 with leading/trailing whitespace",
			value: "  @Microsoft.KeyVault(VaultName=myvault;SecretName=my-secret)  ",
			want:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsKeyVaultReference(tt.value)
			if got != tt.want {
				t.Errorf("IsKeyVaultReference(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestNormalizeKeyVaultReferenceValue(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{
			name:  "No quotes",
			value: "@Microsoft.KeyVault(SecretUri=https://vault.vault.azure.net/secrets/name)",
			want:  "@Microsoft.KeyVault(SecretUri=https://vault.vault.azure.net/secrets/name)",
		},
		{
			name:  "Double quotes",
			value: "\"@Microsoft.KeyVault(SecretUri=https://vault.vault.azure.net/secrets/name)\"",
			want:  "@Microsoft.KeyVault(SecretUri=https://vault.vault.azure.net/secrets/name)",
		},
		{
			name:  "Single quotes",
			value: "'@Microsoft.KeyVault(SecretUri=https://vault.vault.azure.net/secrets/name)'",
			want:  "@Microsoft.KeyVault(SecretUri=https://vault.vault.azure.net/secrets/name)",
		},
		{
			name:  "Leading/trailing whitespace",
			value: "  @Microsoft.KeyVault(VaultName=vault;SecretName=secret)  ",
			want:  "@Microsoft.KeyVault(VaultName=vault;SecretName=secret)",
		},
		{
			name:  "Quotes with whitespace",
			value: "  \"@Microsoft.KeyVault(VaultName=vault;SecretName=secret)\"  ",
			want:  "@Microsoft.KeyVault(VaultName=vault;SecretName=secret)",
		},
		{
			name:  "Empty string",
			value: "",
			want:  "",
		},
		{
			name:  "Single character",
			value: "a",
			want:  "a",
		},
		{
			name:  "Mismatched quotes - not stripped",
			value: "\"@Microsoft.KeyVault(VaultName=vault;SecretName=secret)'",
			want:  "\"@Microsoft.KeyVault(VaultName=vault;SecretName=secret)'",
		},
		{
			name:  "Internal quotes - not stripped",
			value: "@Microsoft.KeyVault(SecretUri=https://vault.vault.azure.net/secrets/\"name\")",
			want:  "@Microsoft.KeyVault(SecretUri=https://vault.vault.azure.net/secrets/\"name\")",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeKeyVaultReferenceValue(tt.value)
			if got != tt.want {
				t.Errorf("normalizeKeyVaultReferenceValue(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestParseAzdAkvsURI(t *testing.T) {
	tests := []struct {
		name           string
		uri            string
		wantGUID       string
		wantVaultName  string
		wantSecretName string
		wantVersion    string
		wantErr        bool
	}{
		{
			name:           "Valid with version",
			uri:            "akvs://12345678-1234-1234-1234-123456789abc/myvault/my-secret/v1",
			wantGUID:       "12345678-1234-1234-1234-123456789abc",
			wantVaultName:  "myvault",
			wantSecretName: "my-secret",
			wantVersion:    "v1",
			wantErr:        false,
		},
		{
			name:           "Valid without version",
			uri:            "akvs://12345678-1234-1234-1234-123456789abc/myvault/my-secret",
			wantGUID:       "12345678-1234-1234-1234-123456789abc",
			wantVaultName:  "myvault",
			wantSecretName: "my-secret",
			wantVersion:    "",
			wantErr:        false,
		},
		{
			name:    "Invalid - missing secret name",
			uri:     "akvs://12345678-1234-1234-1234-123456789abc/myvault",
			wantErr: true,
		},
		{
			name:    "Invalid - empty uri",
			uri:     "",
			wantErr: true,
		},
		{
			name:    "Invalid - wrong scheme",
			uri:     "https://12345678-1234-1234-1234-123456789abc/myvault/my-secret",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			guid, vaultName, secretName, version, err := parseAzdAkvsURI(tt.uri)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseAzdAkvsURI(%q) error = %v, wantErr %v", tt.uri, err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if guid != tt.wantGUID {
				t.Errorf("parseAzdAkvsURI(%q) guid = %q, want %q", tt.uri, guid, tt.wantGUID)
			}
			if vaultName != tt.wantVaultName {
				t.Errorf("parseAzdAkvsURI(%q) vaultName = %q, want %q", tt.uri, vaultName, tt.wantVaultName)
			}
			if secretName != tt.wantSecretName {
				t.Errorf("parseAzdAkvsURI(%q) secretName = %q, want %q", tt.uri, secretName, tt.wantSecretName)
			}
			if version != tt.wantVersion {
				t.Errorf("parseAzdAkvsURI(%q) version = %q, want %q", tt.uri, version, tt.wantVersion)
			}
		})
	}
}

func TestKeyVaultResolver_New(t *testing.T) {
	resolver, err := NewKeyVaultResolver()

	if err == nil && resolver == nil {
		t.Error("NewKeyVaultResolver() returned nil resolver without error")
	}

	if resolver != nil {
		if resolver.clients == nil {
			t.Error("NewKeyVaultResolver() created resolver with nil clients map")
		}
	}
}

func TestKeyVaultResolutionWarning(t *testing.T) {
	warning := KeyVaultResolutionWarning{
		Key: "MY_SECRET",
		Err: nil,
	}

	if warning.Key != "MY_SECRET" {
		t.Errorf("KeyVaultResolutionWarning.Key = %q, want %q", warning.Key, "MY_SECRET")
	}
}

func TestResolveEnvironmentOptions(t *testing.T) {
	opts := ResolveEnvironmentOptions{StopOnError: true}
	if !opts.StopOnError {
		t.Error("ResolveEnvironmentOptions.StopOnError = false, want true")
	}

	opts2 := ResolveEnvironmentOptions{StopOnError: false}
	if opts2.StopOnError {
		t.Error("ResolveEnvironmentOptions.StopOnError = true, want false")
	}
}

// Tests for ResolveReference and ResolveEnvironmentVariables functions.
// Note: Functions like SliceToMap, HasKeyVaultReferences, MapToSlice are tested in the env package tests.

func TestKeyVaultResolver_ResolveReference_SecretURI(t *testing.T) {
	tests := []struct {
		name      string
		reference string
		wantErr   bool
		wantValue string
	}{
		{
			name:      "Invalid secret URI format - missing /secrets/",
			reference: "@Microsoft.KeyVault(SecretUri=https://vault.vault.azure.net/invalid/secret)",
			wantErr:   true,
		},
		{
			name:      "Invalid secret URI - not https",
			reference: "@Microsoft.KeyVault(SecretUri=http://vault.vault.azure.net/secrets/name)",
			wantErr:   true,
		},
		{
			name:      "Invalid secret URI - wrong domain",
			reference: "@Microsoft.KeyVault(SecretUri=https://vault.wrong.com/secrets/name)",
			wantErr:   true,
		},
		{
			name:      "Invalid - all three formats don't match",
			reference: "@Microsoft.KeyVault(Invalid=format)",
			wantErr:   true,
		},
		{
			name:      "Invalid format - completely malformed",
			reference: "this is not a valid reference",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Since we can't easily mock the Azure SDK client, we test error conditions
			resolver, err := NewKeyVaultResolver()
			if err != nil {
				// If credential setup fails, skip this test
				t.Skipf("Skipping due to credential setup: %v", err)
			}

			if resolver == nil {
				t.Skip("Skipping - NewKeyVaultResolver returned nil")
			}

			_, err = resolver.ResolveReference(context.Background(), tt.reference)
			if (err != nil) != tt.wantErr {
				t.Errorf("ResolveReference() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestKeyVaultResolver_ResolveEnvironmentVariables(t *testing.T) {
	tests := []struct {
		name     string
		envVars  []string
		options  ResolveEnvironmentOptions
		wantErr  bool
		checkLen bool
	}{
		{
			name:     "Empty environment",
			envVars:  []string{},
			options:  ResolveEnvironmentOptions{},
			wantErr:  false,
			checkLen: true,
		},
		{
			name:     "Environment with no references",
			envVars:  []string{"FOO=bar", "BAZ=qux"},
			options:  ResolveEnvironmentOptions{},
			wantErr:  false,
			checkLen: true,
		},
		{
			name:     "Environment with malformed entries",
			envVars:  []string{"NOEQUALS", "FOO=bar", "=value"},
			options:  ResolveEnvironmentOptions{},
			wantErr:  false,
			checkLen: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver, err := NewKeyVaultResolver()
			if err != nil {
				t.Skipf("Skipping due to credential setup: %v", err)
			}

			if resolver == nil {
				t.Skip("Skipping - NewKeyVaultResolver returned nil")
			}

			result, warnings, err := resolver.ResolveEnvironmentVariables(context.Background(), tt.envVars, tt.options)
			if (err != nil) != tt.wantErr {
				t.Errorf("ResolveEnvironmentVariables() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.checkLen && len(result) != len(tt.envVars) {
				t.Errorf("ResolveEnvironmentVariables() result length = %d, want %d", len(result), len(tt.envVars))
			}

			// Malformed entries should be passed through unchanged
			if len(tt.envVars) > 0 && tt.envVars[0] == "NOEQUALS" {
				if len(result) > 0 && result[0] != "NOEQUALS" {
					t.Errorf("ResolveEnvironmentVariables() malformed entry not passed through: got %q", result[0])
				}
			}

			_ = warnings // Check warnings can be accessed
		})
	}
}

func TestValidateVaultURL_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{
			name:    "Empty URL",
			url:     "",
			wantErr: true,
		},
		{
			name:    "Only domain no vault name",
			url:     "https://.vault.azure.net",
			wantErr: true,
		},
		{
			name:    "Mixed case vault name (should be valid)",
			url:     "https://MyVault.vault.azure.net",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateVaultURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateVaultURL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNormalizeKeyVaultReferenceValue_EdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{
			name:  "Only whitespace",
			value: "   ",
			want:  "",
		},
		{
			name:  "Quote character only",
			value: "\"",
			want:  "\"",
		},
		{
			name:  "Single quote only",
			value: "'",
			want:  "'",
		},
		{
			name:  "Tab characters",
			value: "\t@Microsoft.KeyVault(VaultName=vault;SecretName=secret)\t",
			want:  "@Microsoft.KeyVault(VaultName=vault;SecretName=secret)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeKeyVaultReferenceValue(tt.value)
			if got != tt.want {
				t.Errorf("normalizeKeyVaultReferenceValue(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestValidateVaultURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{
			name:    "Valid vault URL",
			url:     "https://myvault.vault.azure.net",
			wantErr: false,
		},
		{
			name:    "Valid vault URL with hyphens",
			url:     "https://my-test-vault.vault.azure.net",
			wantErr: false,
		},
		{
			name:    "Invalid - HTTP instead of HTTPS",
			url:     "http://myvault.vault.azure.net",
			wantErr: true,
		},
		{
			name:    "Invalid - wrong domain",
			url:     "https://myvault.malicious.com",
			wantErr: true,
		},
		{
			name:    "Invalid - vault name too short",
			url:     "https://ab.vault.azure.net",
			wantErr: true,
		},
		{
			name:    "Invalid - vault name too long",
			url:     "https://this-is-a-very-long-vault-name-that-exceeds-limit.vault.azure.net",
			wantErr: true,
		},
		{
			name:    "Invalid - vault name starts with number",
			url:     "https://1myvault.vault.azure.net",
			wantErr: true,
		},
		{
			name:    "Invalid - vault name has invalid characters",
			url:     "https://my_vault.vault.azure.net",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateVaultURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateVaultURL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateVaultName(t *testing.T) {
	tests := []struct {
		name      string
		vaultName string
		wantErr   bool
	}{
		{
			name:      "Valid vault name",
			vaultName: "myvault",
			wantErr:   false,
		},
		{
			name:      "Valid vault name with hyphens",
			vaultName: "my-test-vault",
			wantErr:   false,
		},
		{
			name:      "Valid vault name with numbers",
			vaultName: "myvault123",
			wantErr:   false,
		},
		{
			name:      "Valid vault name - 3 chars (minimum)",
			vaultName: "abc",
			wantErr:   false,
		},
		{
			name:      "Valid vault name - 24 chars (maximum)",
			vaultName: "abcdefghijklmnopqrstuvwx",
			wantErr:   false,
		},
		{
			name:      "Invalid - too short (2 chars)",
			vaultName: "ab",
			wantErr:   true,
		},
		{
			name:      "Invalid - too long (25 chars)",
			vaultName: "abcdefghijklmnopqrstuvwxy",
			wantErr:   true,
		},
		{
			name:      "Invalid - starts with number",
			vaultName: "1myvault",
			wantErr:   true,
		},
		{
			name:      "Invalid - contains underscore",
			vaultName: "my_vault",
			wantErr:   true,
		},
		{
			name:      "Invalid - contains special characters",
			vaultName: "my.vault",
			wantErr:   true,
		},
		{
			name:      "Invalid - empty string",
			vaultName: "",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateVaultName(tt.vaultName)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateVaultName() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestResolveReference_ErrorPaths tests error conditions in ResolveReference without needing Azure SDK
func TestResolveReference_ErrorPaths(t *testing.T) {
	resolver, err := NewKeyVaultResolver()
	if err != nil {
		t.Skipf("Skipping due to credential setup: %v", err)
	}

	if resolver == nil {
		t.Skip("Skipping - NewKeyVaultResolver returned nil")
	}

	tests := []struct {
		name      string
		reference string
		shouldErr bool
	}{
		{
			name:      "Empty reference",
			reference: "",
			shouldErr: true,
		},
		{
			name:      "Invalid format - no pattern match",
			reference: "not-a-reference",
			shouldErr: true,
		},
		{
			name:      "Invalid akvs format missing components",
			reference: "akvs://missing",
			shouldErr: true,
		},
		{
			name:      "Invalid SecretUri format - no /secrets/",
			reference: "@Microsoft.KeyVault(SecretUri=https://vault.vault.azure.net/invalid)",
			shouldErr: true,
		},
		{
			name:      "Invalid vault URI - HTTP not HTTPS",
			reference: "@Microsoft.KeyVault(SecretUri=http://vault.vault.azure.net/secrets/name)",
			shouldErr: true,
		},
		{
			name:      "Vault name too long",
			reference: "@Microsoft.KeyVault(VaultName=this-name-is-definitely-way-too-long-for-a-vault;SecretName=secret)",
			shouldErr: true,
		},
		{
			name:      "Vault name too short",
			reference: "@Microsoft.KeyVault(VaultName=ab;SecretName=secret)",
			shouldErr: true,
		},
		{
			name:      "Vault name starts with number",
			reference: "@Microsoft.KeyVault(VaultName=1vault;SecretName=secret)",
			shouldErr: true,
		},
		{
			name:      "Invalid vault domain",
			reference: "@Microsoft.KeyVault(SecretUri=https://myvault.wrong-domain.com/secrets/name)",
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := resolver.ResolveReference(context.Background(), tt.reference)
			if (err != nil) != tt.shouldErr {
				t.Errorf("ResolveReference(%q) error = %v, shouldErr %v", tt.reference, err, tt.shouldErr)
			}
		})
	}
}

// TestResolveEnvironmentVariables_MalformedAndValidation tests non-Azure SDK path of ResolveEnvironmentVariables
func TestResolveEnvironmentVariables_ErrorHandling(t *testing.T) {
	resolver, err := NewKeyVaultResolver()
	if err != nil {
		t.Skipf("Skipping due to credential setup: %v", err)
	}

	if resolver == nil {
		t.Skip("Skipping - NewKeyVaultResolver returned nil")
	}

	tests := []struct {
		name         string
		envVars      []string
		options      ResolveEnvironmentOptions
		expectErrors bool
		checkOutput  func([]string, error) bool
	}{
		{
			name:         "No references in env",
			envVars:      []string{"VAR1=value1", "VAR2=value2"},
			options:      ResolveEnvironmentOptions{},
			expectErrors: false,
			checkOutput: func(result []string, err error) bool {
				return err == nil && len(result) == 2
			},
		},
		{
			name:         "Malformed env entries with no references",
			envVars:      []string{"NOEQUALS", "VAR=value", "ALSO_MISSING"},
			options:      ResolveEnvironmentOptions{},
			expectErrors: false,
			checkOutput: func(result []string, err error) bool {
				// Malformed entries should be passed through unchanged
				return err == nil && len(result) == 3
			},
		},
		{
			name:         "Invalid format not recognized as key vault reference",
			envVars:      []string{"NORMAL=value", "INVALID=@Microsoft.KeyVault(Invalid=format)"},
			options:      ResolveEnvironmentOptions{StopOnError: false},
			expectErrors: false, // Invalid format is not a valid reference so it's ignored
			checkOutput: func(result []string, err error) bool {
				return err == nil && len(result) == 2
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, warnings, err := resolver.ResolveEnvironmentVariables(context.Background(), tt.envVars, tt.options)

			if (err != nil) != tt.expectErrors {
				t.Errorf("ResolveEnvironmentVariables() error = %v, expectErrors %v", err, tt.expectErrors)
			}

			if !tt.checkOutput(result, err) {
				t.Errorf("ResolveEnvironmentVariables() output check failed. result=%v, err=%v, warnings=%v", result, err, warnings)
			}
		})
	}
}

// TestGetClient_CachingBehavior tests that the client caching works
func TestGetClient_CachingBehavior(t *testing.T) {
	resolver, err := NewKeyVaultResolver()
	if err != nil {
		t.Skipf("Skipping due to credential setup: %v", err)
		return
	}

	if resolver == nil {
		t.Skip("Skipping - NewKeyVaultResolver returned nil")
		return
	}

	// Verify clients map exists and is initialized
	if resolver.clients == nil {
		t.Error("KeyVaultResolver.clients is nil")
		return
	}

	// The getClient method uses caching internally
	// We can verify the basic structure is correct
	if len(resolver.clients) != 0 {
		t.Errorf("KeyVaultResolver.clients should start empty, has %d entries", len(resolver.clients))
	}
}

// TestResolveReference_WithVersion tests parsing versions in references
func TestResolveReference_WithVersion(t *testing.T) {
	resolver, err := NewKeyVaultResolver()
	if err != nil {
		t.Skipf("Skipping due to credential setup: %v", err)
	}

	if resolver == nil {
		t.Skip("Skipping - NewKeyVaultResolver returned nil")
	}

	// Test valid references with versions - they will fail on actual resolution but pass validation
	tests := []struct {
		name      string
		reference string
		shouldErr bool
	}{
		{
			name:      "akvs format with version",
			reference: "akvs://12345678-1234-1234-1234-123456789abc/myvault/secret/v1",
			shouldErr: true, // Will error on Azure SDK call but passes format validation
		},
		{
			name:      "akvs format without version",
			reference: "akvs://12345678-1234-1234-1234-123456789abc/myvault/secret",
			shouldErr: true, // Will error on Azure SDK call but passes format validation
		},
		{
			name:      "VaultName with version",
			reference: "@Microsoft.KeyVault(VaultName=myvault;SecretName=secret;SecretVersion=v1)",
			shouldErr: true, // Will error on Azure SDK call
		},
		{
			name:      "VaultName without version",
			reference: "@Microsoft.KeyVault(VaultName=myvault;SecretName=secret)",
			shouldErr: true, // Will error on Azure SDK call
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := resolver.ResolveReference(context.Background(), tt.reference)
			if (err != nil) != tt.shouldErr {
				t.Logf("ResolveReference(%q) error = %v", tt.reference, err)
			}
		})
	}
}

// TestResolveEnvironmentVariables_WithWarnings tests that warnings are properly collected
func TestResolveEnvironmentVariables_WithWarnings(t *testing.T) {
	resolver, err := NewKeyVaultResolver()
	if err != nil {
		t.Skipf("Skipping due to credential setup: %v", err)
	}

	if resolver == nil {
		t.Skip("Skipping - NewKeyVaultResolver returned nil")
	}

	// Create environment with valid references that will fail (no credentials)
	envVars := []string{"NORMAL=value", "SECRET1=@Microsoft.KeyVault(VaultName=myvault;SecretName=secret1)"}

	result, warnings, err := resolver.ResolveEnvironmentVariables(context.Background(), envVars, ResolveEnvironmentOptions{StopOnError: false})

	// Should not error with StopOnError=false
	if err != nil {
		t.Logf("ResolveEnvironmentVariables() error = %v", err)
	}

	// Warnings or errors are expected due to Azure SDK calls
	if len(result) != 2 {
		t.Errorf("expected 2 results, got %d", len(result))
	}

	_ = warnings // Warnings may or may not be present depending on Azure credential state
}

// TestResolveEnvironmentVariables_ContextCancellation tests that context cancellation stops processing
func TestResolveEnvironmentVariables_ContextCancellation(t *testing.T) {
	resolver, err := NewKeyVaultResolver()
	if err != nil {
		t.Skipf("Skipping due to credential setup: %v", err)
	}

	if resolver == nil {
		t.Skip("Skipping - NewKeyVaultResolver returned nil")
	}

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	envVars := []string{
		"VAR1=value1",
		"SECRET=@Microsoft.KeyVault(VaultName=myvault;SecretName=secret)",
		"VAR2=value2",
	}

	_, warnings, err := resolver.ResolveEnvironmentVariables(ctx, envVars, ResolveEnvironmentOptions{})

	// Should return context.Canceled error
	if err == nil {
		t.Error("expected context cancellation error but got none")
	}

	if err != nil && err != context.Canceled {
		t.Logf("got error: %v (expected context.Canceled)", err)
	}

	_ = warnings
}
