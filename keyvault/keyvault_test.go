package keyvault

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
	"github.com/azure/azure-dev/cli/azd/pkg/azdext"
)

// getSecretCall records one call into the fake secret client.
type getSecretCall struct {
	VaultURL string
	Name     string
	Version  string
}

// fakeSecretClient is a stand in for the Azure secrets data plane client.
type fakeSecretClient struct {
	vaultURL string
	recorder *callRecorder
	value    *string
	err      error
}

func (f *fakeSecretClient) GetSecret(
	_ context.Context,
	name string,
	version string,
	_ *azsecrets.GetSecretOptions,
) (azsecrets.GetSecretResponse, error) {
	f.recorder.record(getSecretCall{VaultURL: f.vaultURL, Name: name, Version: version})

	if f.err != nil {
		return azsecrets.GetSecretResponse{}, f.err
	}

	return azsecrets.GetSecretResponse{Secret: azsecrets.Secret{Value: f.value}}, nil
}

// callRecorder collects fake client activity across goroutines.
type callRecorder struct {
	mu            sync.Mutex
	calls         []getSecretCall
	factoryVaults []string
}

func (c *callRecorder) record(call getSecretCall) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, call)
}

func (c *callRecorder) recordFactory(vaultURL string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.factoryVaults = append(c.factoryVaults, vaultURL)
}

func (c *callRecorder) snapshot() ([]getSecretCall, []string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return append([]getSecretCall(nil), c.calls...), append([]string(nil), c.factoryVaults...)
}

// fakeCredential satisfies azcore.TokenCredential without contacting anything.
type fakeCredential struct{}

func (fakeCredential) GetToken(
	_ context.Context,
	_ policy.TokenRequestOptions,
) (azcore.AccessToken, error) {
	return azcore.AccessToken{Token: "fake"}, nil
}

// newTestResolver builds a resolver whose secret client is the fake above.
// secretValue is returned by every successful GetSecret call; getSecretErr, when
// non nil, is returned instead.
func newTestResolver(t *testing.T, secretValue string, getSecretErr error) (*KeyVaultResolver, *callRecorder) {
	t.Helper()

	rec := &callRecorder{}
	value := secretValue

	resolver, err := NewKeyVaultResolverWithCredential(fakeCredential{}, &azdext.KeyVaultResolverOptions{
		ClientFactory: func(vaultURL string, _ azcore.TokenCredential) (azdext.SecretGetter, error) {
			rec.recordFactory(vaultURL)
			return &fakeSecretClient{vaultURL: vaultURL, recorder: rec, value: &value, err: getSecretErr}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewKeyVaultResolverWithCredential() error = %v", err)
	}

	return resolver, rec
}

func TestIsKeyVaultReference(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{"SecretUri with version", "@Microsoft.KeyVault(SecretUri=https://myvault.vault.azure.net/secrets/my-secret/abc123)", true},
		{"SecretUri without version", "@Microsoft.KeyVault(SecretUri=https://myvault.vault.azure.net/secrets/my-secret)", true},
		{"VaultName with version", "@Microsoft.KeyVault(VaultName=myvault;SecretName=my-secret;SecretVersion=abc123)", true},
		{"VaultName without version", "@Microsoft.KeyVault(VaultName=myvault;SecretName=my-secret)", true},
		{"akvs with version", "akvs://12345678-1234-1234-1234-123456789abc/myvault/my-secret/abc123", true},
		{"akvs without version", "akvs://12345678-1234-1234-1234-123456789abc/myvault/my-secret", true},
		{"lowercase app reference prefix", "@microsoft.keyvault(VaultName=myvault;SecretName=my-secret)", true},
		{"sovereign cloud SecretUri", "@Microsoft.KeyVault(SecretUri=https://myvault.vault.azure.cn/secrets/s)", true},
		{"plain value", "just a regular value", false},
		{"empty string", "", false},
		{"missing closing paren", "@Microsoft.KeyVault(SecretUri=https://myvault.vault.azure.net/secrets/my-secret", false},
		{"app reference without SecretName", "@Microsoft.KeyVault(VaultName=myvault)", false},
		{"empty app reference", "@Microsoft.KeyVault()", false},
		// The akvs prefix alone is enough to be treated as a reference. A
		// malformed one surfaces as a resolution warning naming the key rather
		// than passing silently through as a literal value.
		{"malformed akvs is still a reference", "akvs://guid/vault", true},
		{"wrapper double quotes", "\"@Microsoft.KeyVault(SecretUri=https://myvault.vault.azure.net/secrets/my-secret)\"", true},
		{"wrapper single quotes", "'@Microsoft.KeyVault(SecretUri=https://myvault.vault.azure.net/secrets/my-secret)'", true},
		{"surrounding whitespace", "  @Microsoft.KeyVault(VaultName=myvault;SecretName=my-secret)  ", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsKeyVaultReference(tt.value); got != tt.want {
				t.Errorf("IsKeyVaultReference(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestStripQuotes(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{"plain", "value", "value"},
		{"whitespace", "  value  ", "value"},
		{"double quoted", "\"value\"", "value"},
		{"single quoted", "'value'", "value"},
		{"quoted with inner whitespace", "'  value  '", "value"},
		{"mismatched quotes are left alone", "\"value'", "\"value'"},
		{"single character is left alone", "\"", "\""},
		{"empty", "", ""},
		{"only whitespace", "   ", ""},
		{"inner quotes are left alone", "a\"b", "a\"b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := stripQuotes(tt.value); got != tt.want {
				t.Errorf("stripQuotes(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestNormalizeReference(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{
			name:  "versioned akvs is rewritten",
			value: "akvs://sub-id/myvault/my-secret/v1",
			want:  "@Microsoft.KeyVault(VaultName=myvault;SecretName=my-secret;SecretVersion=v1)",
		},
		{
			name:  "quoted versioned akvs is rewritten",
			value: "'akvs://sub-id/myvault/my-secret/v1'",
			want:  "@Microsoft.KeyVault(VaultName=myvault;SecretName=my-secret;SecretVersion=v1)",
		},
		{
			name:  "unversioned akvs is untouched",
			value: "akvs://sub-id/myvault/my-secret",
			want:  "akvs://sub-id/myvault/my-secret",
		},
		{
			name:  "five segments are untouched",
			value: "akvs://sub-id/myvault/my-secret/v1/extra",
			want:  "akvs://sub-id/myvault/my-secret/v1/extra",
		},
		{
			name:  "app reference is untouched",
			value: "@Microsoft.KeyVault(VaultName=myvault;SecretName=my-secret)",
			want:  "@Microsoft.KeyVault(VaultName=myvault;SecretName=my-secret)",
		},
		{
			name:  "plain value is untouched",
			value: "plain",
			want:  "plain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeReference(tt.value); got != tt.want {
				t.Errorf("normalizeReference(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestNewKeyVaultResolverWithCredential_NilCredential(t *testing.T) {
	if _, err := NewKeyVaultResolverWithCredential(nil, nil); err == nil {
		t.Fatal("NewKeyVaultResolverWithCredential(nil, nil) expected an error, got nil")
	}
}

func TestNewKeyVaultResolver(t *testing.T) {
	// DefaultAzureCredential construction does no IO, so this succeeds on a
	// machine with no Azure login. Resolution would fail later, not here.
	resolver, err := NewKeyVaultResolver()
	if err != nil {
		t.Skipf("DefaultAzureCredential unavailable in this environment: %v", err)
	}
	if resolver == nil {
		t.Fatal("NewKeyVaultResolver() returned a nil resolver with a nil error")
	}
}

func TestResolveReference_AllFormats(t *testing.T) {
	tests := []struct {
		name        string
		reference   string
		wantVault   string
		wantSecret  string
		wantVersion string
	}{
		{
			name:       "akvs",
			reference:  "akvs://sub-id/myvault/my-secret",
			wantVault:  "https://myvault.vault.azure.net",
			wantSecret: "my-secret",
		},
		{
			name:        "akvs with version",
			reference:   "akvs://sub-id/myvault/my-secret/v1",
			wantVault:   "https://myvault.vault.azure.net",
			wantSecret:  "my-secret",
			wantVersion: "v1",
		},
		{
			name:       "SecretUri",
			reference:  "@Microsoft.KeyVault(SecretUri=https://myvault.vault.azure.net/secrets/my-secret)",
			wantVault:  "https://myvault.vault.azure.net",
			wantSecret: "my-secret",
		},
		{
			name:        "SecretUri with version",
			reference:   "@Microsoft.KeyVault(SecretUri=https://myvault.vault.azure.net/secrets/my-secret/v2)",
			wantVault:   "https://myvault.vault.azure.net",
			wantSecret:  "my-secret",
			wantVersion: "v2",
		},
		{
			name:       "VaultName",
			reference:  "@Microsoft.KeyVault(VaultName=myvault;SecretName=my-secret)",
			wantVault:  "https://myvault.vault.azure.net",
			wantSecret: "my-secret",
		},
		{
			name:        "VaultName with version",
			reference:   "@Microsoft.KeyVault(VaultName=myvault;SecretName=my-secret;SecretVersion=v3)",
			wantVault:   "https://myvault.vault.azure.net",
			wantSecret:  "my-secret",
			wantVersion: "v3",
		},
		{
			name:       "quoted reference",
			reference:  "  'akvs://sub-id/myvault/my-secret'  ",
			wantVault:  "https://myvault.vault.azure.net",
			wantSecret: "my-secret",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver, rec := newTestResolver(t, "s3cret", nil)

			got, err := resolver.ResolveReference(context.Background(), tt.reference)
			if err != nil {
				t.Fatalf("ResolveReference(%q) error = %v", tt.reference, err)
			}
			if got != "s3cret" {
				t.Errorf("ResolveReference(%q) = %q, want %q", tt.reference, got, "s3cret")
			}

			calls, _ := rec.snapshot()
			if len(calls) != 1 {
				t.Fatalf("GetSecret called %d times, want 1", len(calls))
			}
			if calls[0].VaultURL != tt.wantVault {
				t.Errorf("vault URL = %q, want %q", calls[0].VaultURL, tt.wantVault)
			}
			if calls[0].Name != tt.wantSecret {
				t.Errorf("secret name = %q, want %q", calls[0].Name, tt.wantSecret)
			}
			if calls[0].Version != tt.wantVersion {
				t.Errorf("secret version = %q, want %q", calls[0].Version, tt.wantVersion)
			}
		})
	}
}

func TestResolveReference_InvalidReferences(t *testing.T) {
	tests := []struct {
		name      string
		reference string
	}{
		{"empty", ""},
		{"plain value", "not-a-reference"},
		{"akvs missing secret", "akvs://sub-id/myvault"},
		{"akvs empty", "akvs://"},
		{"akvs vault name too short", "akvs://sub-id/ab/my-secret"},
		{"akvs vault name leading hyphen", "akvs://sub-id/-myvault/my-secret"},
		{"akvs vault name trailing hyphen", "akvs://sub-id/myvault-/my-secret"},
		{"SecretUri on a foreign host", "@Microsoft.KeyVault(SecretUri=https://evil.example.com/secrets/s)"},
		{"app reference missing SecretName", "@Microsoft.KeyVault(VaultName=myvault;SecretVersion=v1)"},
		{"app reference duplicate parameter", "@Microsoft.KeyVault(VaultName=a;VaultName=b;SecretName=s)"},
		{"app reference malformed parameter", "@Microsoft.KeyVault(VaultName=myvault;SecretName)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver, rec := newTestResolver(t, "s3cret", nil)

			if _, err := resolver.ResolveReference(context.Background(), tt.reference); err == nil {
				t.Fatalf("ResolveReference(%q) expected an error, got nil", tt.reference)
			} else if reason := resolveReason(t, err); reason != azdext.ResolveReasonInvalidReference {
				t.Errorf("reason = %v, want %v", reason, azdext.ResolveReasonInvalidReference)
			}

			if calls, _ := rec.snapshot(); len(calls) != 0 {
				t.Errorf("GetSecret called %d times for an invalid reference, want 0", len(calls))
			}
		})
	}
}

// resolveReason extracts the classification from a resolver error.
func resolveReason(t *testing.T, err error) azdext.ResolveReason {
	t.Helper()

	var kvErr *azdext.KeyVaultResolveError
	if !errors.As(err, &kvErr) {
		t.Fatalf("error type = %T, want *azdext.KeyVaultResolveError (%v)", err, err)
	}

	return kvErr.Reason
}

func TestResolveReference_ServiceErrorsAreClassified(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		want       azdext.ResolveReason
	}{
		{"not found", http.StatusNotFound, azdext.ResolveReasonNotFound},
		{"forbidden", http.StatusForbidden, azdext.ResolveReasonAccessDenied},
		{"unauthorized", http.StatusUnauthorized, azdext.ResolveReasonAccessDenied},
		{"server error", http.StatusInternalServerError, azdext.ResolveReasonServiceError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver, _ := newTestResolver(t, "", &azcore.ResponseError{StatusCode: tt.statusCode})

			_, err := resolver.ResolveReference(context.Background(), "akvs://sub-id/myvault/my-secret")
			if err == nil {
				t.Fatal("ResolveReference() expected an error, got nil")
			}
			if reason := resolveReason(t, err); reason != tt.want {
				t.Errorf("reason = %v, want %v", reason, tt.want)
			}
		})
	}
}

func TestResolveReference_NonResponseErrorIsServiceError(t *testing.T) {
	resolver, _ := newTestResolver(t, "", errors.New("dial tcp: lookup failed"))

	_, err := resolver.ResolveReference(context.Background(), "akvs://sub-id/myvault/my-secret")
	if err == nil {
		t.Fatal("ResolveReference() expected an error, got nil")
	}
	if reason := resolveReason(t, err); reason != azdext.ResolveReasonServiceError {
		t.Errorf("reason = %v, want %v", reason, azdext.ResolveReasonServiceError)
	}
}

func TestResolveReference_ClientFactoryFailure(t *testing.T) {
	resolver, err := NewKeyVaultResolverWithCredential(fakeCredential{}, &azdext.KeyVaultResolverOptions{
		ClientFactory: func(string, azcore.TokenCredential) (azdext.SecretGetter, error) {
			return nil, errors.New("no client for you")
		},
	})
	if err != nil {
		t.Fatalf("NewKeyVaultResolverWithCredential() error = %v", err)
	}

	_, err = resolver.ResolveReference(context.Background(), "akvs://sub-id/myvault/my-secret")
	if err == nil {
		t.Fatal("ResolveReference() expected an error, got nil")
	}
	if reason := resolveReason(t, err); reason != azdext.ResolveReasonClientCreation {
		t.Errorf("reason = %v, want %v", reason, azdext.ResolveReasonClientCreation)
	}
}

func TestResolveReference_ClientIsCachedPerVault(t *testing.T) {
	resolver, rec := newTestResolver(t, "s3cret", nil)
	ctx := context.Background()

	for _, ref := range []string{
		"akvs://sub-id/myvault/first",
		"akvs://sub-id/myvault/second",
		"akvs://sub-id/othervault/third",
	} {
		if _, err := resolver.ResolveReference(ctx, ref); err != nil {
			t.Fatalf("ResolveReference(%q) error = %v", ref, err)
		}
	}

	_, vaults := rec.snapshot()
	if len(vaults) != 2 {
		t.Errorf("client factory called %d times for 2 distinct vaults: %v", len(vaults), vaults)
	}
}

func TestResolveReference_VaultSuffixOverride(t *testing.T) {
	rec := &callRecorder{}
	value := "s3cret"

	resolver, err := NewKeyVaultResolverWithCredential(fakeCredential{}, &azdext.KeyVaultResolverOptions{
		VaultSuffix: "vault.usgovcloudapi.net",
		ClientFactory: func(vaultURL string, _ azcore.TokenCredential) (azdext.SecretGetter, error) {
			rec.recordFactory(vaultURL)
			return &fakeSecretClient{vaultURL: vaultURL, recorder: rec, value: &value}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewKeyVaultResolverWithCredential() error = %v", err)
	}

	if _, err := resolver.ResolveReference(context.Background(), "akvs://sub-id/myvault/my-secret"); err != nil {
		t.Fatalf("ResolveReference() error = %v", err)
	}

	calls, _ := rec.snapshot()
	want := "https://myvault.vault.usgovcloudapi.net"
	if len(calls) != 1 || calls[0].VaultURL != want {
		t.Errorf("vault URL = %v, want %q", calls, want)
	}
}

func TestResolveEnvironmentVariables(t *testing.T) {
	resolver, _ := newTestResolver(t, "s3cret", nil)

	input := []string{
		"PLAIN=value",
		"SECRET=akvs://sub-id/myvault/my-secret",
		"NOT_A_PAIR",
		"EMPTY=",
		"QUOTED='akvs://sub-id/myvault/other'",
		"VERSIONED=akvs://sub-id/myvault/my-secret/v1",
	}

	got, warnings, err := resolver.ResolveEnvironmentVariables(context.Background(), input, ResolveEnvironmentOptions{})
	if err != nil {
		t.Fatalf("ResolveEnvironmentVariables() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v, want none", warnings)
	}

	want := []string{
		"PLAIN=value",
		"SECRET=s3cret",
		"NOT_A_PAIR",
		"EMPTY=",
		"QUOTED=s3cret",
		"VERSIONED=s3cret",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d entries, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestResolveEnvironmentVariables_WarnsAndPreservesOnFailure(t *testing.T) {
	resolver, _ := newTestResolver(t, "", &azcore.ResponseError{StatusCode: http.StatusNotFound})

	input := []string{"PLAIN=value", "SECRET=akvs://sub-id/myvault/my-secret"}

	got, warnings, err := resolver.ResolveEnvironmentVariables(context.Background(), input, ResolveEnvironmentOptions{})
	if err != nil {
		t.Fatalf("ResolveEnvironmentVariables() error = %v, want nil with StopOnError false", err)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings = %v, want exactly 1", warnings)
	}
	if warnings[0].Key != "SECRET" {
		t.Errorf("warning key = %q, want %q", warnings[0].Key, "SECRET")
	}
	if got[1] != input[1] {
		t.Errorf("failed entry = %q, want the original %q", got[1], input[1])
	}
}

func TestResolveEnvironmentVariables_StopOnError(t *testing.T) {
	resolver, _ := newTestResolver(t, "", &azcore.ResponseError{StatusCode: http.StatusNotFound})

	input := []string{"SECRET=akvs://sub-id/myvault/my-secret", "OTHER=akvs://sub-id/myvault/other"}

	got, warnings, err := resolver.ResolveEnvironmentVariables(
		context.Background(), input, ResolveEnvironmentOptions{StopOnError: true},
	)
	if err == nil {
		t.Fatal("ResolveEnvironmentVariables() expected an error with StopOnError true, got nil")
	}
	if got != nil {
		t.Errorf("resolved = %v, want nil on abort", got)
	}
	if len(warnings) != 1 {
		t.Errorf("warnings = %v, want exactly 1 before aborting", warnings)
	}
	if !strings.Contains(err.Error(), "SECRET") {
		t.Errorf("error %q does not name the failing key", err)
	}
}

func TestResolveEnvironmentVariables_ContextCancellation(t *testing.T) {
	resolver, _ := newTestResolver(t, "s3cret", nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := resolver.ResolveEnvironmentVariables(
		ctx, []string{"SECRET=akvs://sub-id/myvault/my-secret"}, ResolveEnvironmentOptions{},
	)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}

func TestResolveEnvironmentVariables_Empty(t *testing.T) {
	resolver, _ := newTestResolver(t, "s3cret", nil)

	got, warnings, err := resolver.ResolveEnvironmentVariables(context.Background(), nil, ResolveEnvironmentOptions{})
	if err != nil {
		t.Fatalf("ResolveEnvironmentVariables() error = %v", err)
	}
	if len(got) != 0 || len(warnings) != 0 {
		t.Errorf("got = %v, warnings = %v, want both empty", got, warnings)
	}
}

func TestEnvResolverAdapter(t *testing.T) {
	resolver, _ := newTestResolver(t, "s3cret", nil)
	adapter := NewEnvResolverAdapter(resolver)

	if !adapter.IsSecretReference("akvs://sub-id/myvault/my-secret") {
		t.Error("IsSecretReference() = false for a valid reference")
	}
	if adapter.IsSecretReference("plain") {
		t.Error("IsSecretReference() = true for a plain value")
	}

	got, warnings, err := adapter.ResolveEnvironmentVariables(
		context.Background(),
		[]string{"PLAIN=value", "SECRET=akvs://sub-id/myvault/my-secret"},
		EnvResolveOptions{},
	)
	if err != nil {
		t.Fatalf("ResolveEnvironmentVariables() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v, want none", warnings)
	}
	if got[1] != "SECRET=s3cret" {
		t.Errorf("resolved = %q, want %q", got[1], "SECRET=s3cret")
	}
}

func TestEnvResolverAdapter_WarningsCarryTheKey(t *testing.T) {
	resolver, _ := newTestResolver(t, "", &azcore.ResponseError{StatusCode: http.StatusForbidden})
	adapter := NewEnvResolverAdapter(resolver)

	_, warnings, err := adapter.ResolveEnvironmentVariables(
		context.Background(),
		[]string{"SECRET=akvs://sub-id/myvault/my-secret"},
		EnvResolveOptions{},
	)
	if err != nil {
		t.Fatalf("ResolveEnvironmentVariables() error = %v", err)
	}
	if len(warnings) != 1 || warnings[0].Key != "SECRET" {
		t.Fatalf("warnings = %v, want exactly one for SECRET", warnings)
	}
	if reason := resolveReason(t, warnings[0].Err); reason != azdext.ResolveReasonAccessDenied {
		t.Errorf("reason = %v, want %v", reason, azdext.ResolveReasonAccessDenied)
	}
}

func TestKeyVaultResolutionWarning(t *testing.T) {
	warning := KeyVaultResolutionWarning{Key: "MY_VAR", Err: errors.New("boom")}

	if warning.Key != "MY_VAR" {
		t.Errorf("Key = %q, want %q", warning.Key, "MY_VAR")
	}
	if warning.Err == nil || warning.Err.Error() != "boom" {
		t.Errorf("Err = %v, want boom", warning.Err)
	}
}

func TestResolveEnvironmentOptions(t *testing.T) {
	if (ResolveEnvironmentOptions{}).StopOnError {
		t.Error("StopOnError should default to false")
	}
	if !(ResolveEnvironmentOptions{StopOnError: true}).StopOnError {
		t.Error("StopOnError should be settable to true")
	}
}
