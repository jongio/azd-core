package env

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/jongio/azd-core/keyvault"
)

func TestMapToSliceAndBack(t *testing.T) {
	original := map[string]string{
		"A": "1",
		"B": "2",
	}

	slice := MapToSlice(original)
	roundTrip := SliceToMap(slice)

	if !reflect.DeepEqual(original, roundTrip) {
		t.Fatalf("round-trip env mismatch, got %v", roundTrip)
	}
}

func TestHasKeyVaultReferences(t *testing.T) {
	withRef := []string{
		"FOO=bar",
		"SECRET=@Microsoft.KeyVault(VaultName=vault;SecretName=name)",
	}
	withoutRef := []string{
		"FOO=bar",
		"BAZ=qux",
	}

	if !HasKeyVaultReferences(withRef) {
		t.Fatal("expected key vault reference to be detected")
	}

	if HasKeyVaultReferences(withoutRef) {
		t.Fatal("did not expect key vault reference to be detected")
	}
}

func TestResolveSkipsWhenResolverMissing(t *testing.T) {
	env := map[string]string{"FOO": "bar"}
	result, warnings, err := Resolve(context.Background(), env, nil, keyvault.ResolveEnvironmentOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if !reflect.DeepEqual(env, result) {
		t.Fatalf("expected env to be unchanged, got %v", result)
	}
}

func TestResolveUsesResolverForReferences(t *testing.T) {
	env := map[string]string{
		"FOO":    "bar",
		"SECRET": "@Microsoft.KeyVault(VaultName=vault;SecretName=name)",
	}

	fake := &fakeResolver{
		resolved: []string{"SECRET=resolved", "FOO=bar"},
	}

	result, warnings, err := Resolve(context.Background(), env, fake, keyvault.ResolveEnvironmentOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !fake.called {
		t.Fatal("expected resolver to be called")
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if result["SECRET"] != "resolved" {
		t.Fatalf("expected SECRET to be resolved, got %q", result["SECRET"])
	}
	if result["FOO"] != "bar" {
		t.Fatalf("expected FOO to remain intact, got %q", result["FOO"])
	}
}

func TestResolveSkipsWhenNoReferences(t *testing.T) {
	env := map[string]string{"FOO": "bar"}
	fake := &fakeResolver{resolved: MapToSlice(env)}

	result, warnings, err := Resolve(context.Background(), env, fake, keyvault.ResolveEnvironmentOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fake.called {
		t.Fatal("expected resolver not to be called")
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}
	if !reflect.DeepEqual(env, result) {
		t.Fatalf("expected env to be unchanged, got %v", result)
	}
}

func TestResolvePropagatesError(t *testing.T) {
	env := map[string]string{
		"SECRET": "@Microsoft.KeyVault(VaultName=vault;SecretName=name)",
	}

	warning := keyvault.KeyVaultResolutionWarning{Key: "SECRET", Err: errors.New("resolve failed")}
	fake := &fakeResolver{
		resolved: MapToSlice(env),
		warnings: []keyvault.KeyVaultResolutionWarning{warning},
		err:      warning.Err,
	}

	result, warnings, err := Resolve(context.Background(), env, fake, keyvault.ResolveEnvironmentOptions{})
	if err == nil {
		t.Fatal("expected error but got none")
	}
	if len(warnings) != 1 || warnings[0].Key != "SECRET" {
		t.Fatalf("expected propagated warnings, got %v", warnings)
	}
	if !reflect.DeepEqual(env, result) {
		t.Fatalf("expected original env on error, got %v", result)
	}
}

type fakeResolver struct {
	resolved []string
	warnings []keyvault.KeyVaultResolutionWarning
	err      error
	called   bool
}

func (f *fakeResolver) ResolveEnvironmentVariables(ctx context.Context, env []string, opts keyvault.ResolveEnvironmentOptions) ([]string, []keyvault.KeyVaultResolutionWarning, error) {
	f.called = true
	if f.resolved == nil {
		f.resolved = env
	}
	return f.resolved, f.warnings, f.err
}

func TestSliceToMap_SkipsMalformedRows(t *testing.T) {
	envSlice := []string{
		"VALID=value",
		"MISSING_EQUALS",
		"ANOTHER=valid",
		"ALSO_MISSING",
		"=empty_key",
	}

	result := SliceToMap(envSlice)

	// Only valid KEY=VALUE entries should be included
	if len(result) != 3 {
		t.Errorf("SliceToMap() expected 3 entries, got %d", len(result))
	}

	if result["VALID"] != "value" {
		t.Errorf("SliceToMap() VALID = %q, want %q", result["VALID"], "value")
	}

	if result["ANOTHER"] != "valid" {
		t.Errorf("SliceToMap() ANOTHER = %q, want %q", result["ANOTHER"], "valid")
	}

	// Malformed rows should be skipped
	if _, exists := result["MISSING_EQUALS"]; exists {
		t.Errorf("SliceToMap() should have skipped MISSING_EQUALS")
	}
}

func TestHasKeyVaultReferences_SkipsMalformedRows(t *testing.T) {
	envVars := []string{
		"NO_EQUALS",
		"FOO=bar",
		"ANOTHER_MISSING",
		"SECRET=@Microsoft.KeyVault(VaultName=vault;SecretName=name)",
	}

	if !HasKeyVaultReferences(envVars) {
		t.Fatal("expected key vault reference to be detected despite malformed rows")
	}
}

func TestHasKeyVaultReferences_AllMalformed(t *testing.T) {
	envVars := []string{
		"NO_EQUALS",
		"ANOTHER_MISSING",
		"AND_ANOTHER",
	}

	if HasKeyVaultReferences(envVars) {
		t.Fatal("did not expect key vault reference to be detected in all-malformed list")
	}
}

func TestResolveWithNilEnvironment(t *testing.T) {
	resolver := &fakeResolver{
		resolved: []string{},
	}

	result, warnings, err := Resolve(context.Background(), nil, resolver, keyvault.ResolveEnvironmentOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}

	if result == nil || len(result) != 0 {
		t.Fatalf("expected empty map, got %v", result)
	}
}

func TestMapToSlice_PreservesAllValues(t *testing.T) {
	env := map[string]string{
		"EMPTY":   "",
		"NORMAL":  "value",
		"SPACES":  "value with spaces",
		"EQUALS":  "value=with=equals",
		"SPECIAL": "value!@#$%^&*()",
	}

	result := MapToSlice(env)
	roundTrip := SliceToMap(result)

	if !reflect.DeepEqual(env, roundTrip) {
		t.Fatalf("round-trip mismatch: original %v, got %v", env, roundTrip)
	}
}

func TestCopyEnv(t *testing.T) {
	original := map[string]string{
		"A": "1",
		"B": "2",
		"C": "3",
	}

	clone := copyEnv(original)

	// Verify contents are equal
	if !reflect.DeepEqual(original, clone) {
		t.Fatalf("clone content mismatch: original %v, clone %v", original, clone)
	}

	// Verify it's a deep copy (modifying original doesn't affect clone)
	original["D"] = "4"
	if _, exists := clone["D"]; exists {
		t.Fatal("clone was modified when original was modified - not a deep copy")
	}
}

func TestResolveEnvironmentVariables_WithStopOnError(t *testing.T) {
	env := map[string]string{
		"SECRET": "@Microsoft.KeyVault(VaultName=vault;SecretName=name)",
	}

	warning := keyvault.KeyVaultResolutionWarning{Key: "SECRET", Err: errors.New("resolve failed")}
	fake := &fakeResolver{
		resolved: MapToSlice(env),
		warnings: []keyvault.KeyVaultResolutionWarning{warning},
		err:      warning.Err,
	}

	_, warnings, err := Resolve(context.Background(), env, fake, keyvault.ResolveEnvironmentOptions{StopOnError: true})
	if err == nil {
		t.Fatal("expected error with StopOnError=true")
	}

	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(warnings))
	}
}

func TestHasKeyVaultReferences_EmptyList(t *testing.T) {
	if HasKeyVaultReferences([]string{}) {
		t.Fatal("did not expect key vault reference in empty list")
	}
}

func TestResolveWithDifferentOptions(t *testing.T) {
	env := map[string]string{"FOO": "bar"}
	fake := &fakeResolver{resolved: MapToSlice(env)}

	// Test with default options
	_, _, err := Resolve(context.Background(), env, fake, keyvault.ResolveEnvironmentOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Test with StopOnError true
	_, _, err = Resolve(context.Background(), env, fake, keyvault.ResolveEnvironmentOptions{StopOnError: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Test with StopOnError false
	_, _, err = Resolve(context.Background(), env, fake, keyvault.ResolveEnvironmentOptions{StopOnError: false})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
// Tests for ResolveMap helper function

func TestResolveMap_WithReferences(t *testing.T) {
	env := map[string]string{
		"FOO":    "bar",
		"SECRET": "@Microsoft.KeyVault(VaultName=vault;SecretName=name)",
	}

	fake := &fakeResolver{
		resolved: []string{"SECRET=resolved-secret", "FOO=bar"},
	}

	result, warnings, err := ResolveMap(context.Background(), env, fake, keyvault.ResolveEnvironmentOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !fake.called {
		t.Fatal("expected resolver to be called")
	}

	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}

	if result["SECRET"] != "resolved-secret" {
		t.Fatalf("expected SECRET to be resolved, got %q", result["SECRET"])
	}

	if result["FOO"] != "bar" {
		t.Fatalf("expected FOO to remain intact, got %q", result["FOO"])
	}
}

func TestResolveMap_NoReferences(t *testing.T) {
	env := map[string]string{"FOO": "bar", "BAZ": "qux"}
	fake := &fakeResolver{resolved: MapToSlice(env)}

	result, warnings, err := ResolveMap(context.Background(), env, fake, keyvault.ResolveEnvironmentOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fake.called {
		t.Fatal("expected resolver not to be called when no references present")
	}

	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}

	if !reflect.DeepEqual(env, result) {
		t.Fatalf("expected env to be unchanged, got %v", result)
	}
}

func TestResolveMap_NilResolver(t *testing.T) {
	env := map[string]string{
		"SECRET": "@Microsoft.KeyVault(VaultName=vault;SecretName=name)",
	}

	result, warnings, err := ResolveMap(context.Background(), env, nil, keyvault.ResolveEnvironmentOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}

	if !reflect.DeepEqual(env, result) {
		t.Fatalf("expected env to be unchanged, got %v", result)
	}
}

func TestResolveMap_NilEnvironment(t *testing.T) {
	resolver := &fakeResolver{
		resolved: []string{},
	}

	result, warnings, err := ResolveMap(context.Background(), nil, resolver, keyvault.ResolveEnvironmentOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}

	if result == nil || len(result) != 0 {
		t.Fatalf("expected empty map, got %v", result)
	}
}

func TestResolveMap_WithError(t *testing.T) {
	env := map[string]string{
		"SECRET": "@Microsoft.KeyVault(VaultName=vault;SecretName=name)",
	}

	warning := keyvault.KeyVaultResolutionWarning{Key: "SECRET", Err: errors.New("resolve failed")}
	fake := &fakeResolver{
		resolved: MapToSlice(env),
		warnings: []keyvault.KeyVaultResolutionWarning{warning},
		err:      warning.Err,
	}

	result, warnings, err := ResolveMap(context.Background(), env, fake, keyvault.ResolveEnvironmentOptions{StopOnError: true})
	if err == nil {
		t.Fatal("expected error but got none")
	}

	if len(warnings) != 1 || warnings[0].Key != "SECRET" {
		t.Fatalf("expected propagated warnings, got %v", warnings)
	}

	if !reflect.DeepEqual(env, result) {
		t.Fatalf("expected original env on error, got %v", result)
	}
}

// Tests for ResolveSlice helper function

func TestResolveSlice_WithReferences(t *testing.T) {
	envSlice := []string{
		"FOO=bar",
		"SECRET=@Microsoft.KeyVault(VaultName=vault;SecretName=name)",
	}

	fake := &fakeResolver{
		resolved: []string{"FOO=bar", "SECRET=resolved-secret"},
	}

	result, warnings, err := ResolveSlice(context.Background(), envSlice, fake, keyvault.ResolveEnvironmentOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !fake.called {
		t.Fatal("expected resolver to be called")
	}

	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}

	expected := []string{"FOO=bar", "SECRET=resolved-secret"}
	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("expected %v, got %v", expected, result)
	}
}

func TestResolveSlice_NoReferences(t *testing.T) {
	envSlice := []string{"FOO=bar", "BAZ=qux"}
	fake := &fakeResolver{resolved: envSlice}

	result, warnings, err := ResolveSlice(context.Background(), envSlice, fake, keyvault.ResolveEnvironmentOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fake.called {
		t.Fatal("expected resolver not to be called when no references present")
	}

	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}

	if !reflect.DeepEqual(envSlice, result) {
		t.Fatalf("expected slice to be unchanged, got %v", result)
	}

	// Verify it's a copy, not the same slice
	if &envSlice[0] == &result[0] {
		t.Fatal("expected a copy of the slice, got the same slice")
	}
}

func TestResolveSlice_NilResolver(t *testing.T) {
	envSlice := []string{
		"SECRET=@Microsoft.KeyVault(VaultName=vault;SecretName=name)",
	}

	result, warnings, err := ResolveSlice(context.Background(), envSlice, nil, keyvault.ResolveEnvironmentOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}

	if !reflect.DeepEqual(envSlice, result) {
		t.Fatalf("expected slice to be unchanged, got %v", result)
	}
}

func TestResolveSlice_NilSlice(t *testing.T) {
	resolver := &fakeResolver{
		resolved: []string{},
	}

	result, warnings, err := ResolveSlice(context.Background(), nil, resolver, keyvault.ResolveEnvironmentOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}

	if result == nil || len(result) != 0 {
		t.Fatalf("expected empty slice, got %v", result)
	}
}

func TestResolveSlice_WithError(t *testing.T) {
	envSlice := []string{
		"SECRET=@Microsoft.KeyVault(VaultName=vault;SecretName=name)",
	}

	warning := keyvault.KeyVaultResolutionWarning{Key: "SECRET", Err: errors.New("resolve failed")}
	fake := &fakeResolver{
		resolved: envSlice,
		warnings: []keyvault.KeyVaultResolutionWarning{warning},
		err:      warning.Err,
	}

	result, warnings, err := ResolveSlice(context.Background(), envSlice, fake, keyvault.ResolveEnvironmentOptions{StopOnError: true})
	if err == nil {
		t.Fatal("expected error but got none")
	}

	if len(warnings) != 1 || warnings[0].Key != "SECRET" {
		t.Fatalf("expected propagated warnings, got %v", warnings)
	}

	if !reflect.DeepEqual(envSlice, result) {
		t.Fatalf("expected original slice on error, got %v", result)
	}
}

func TestResolveSlice_WithWarnings(t *testing.T) {
	envSlice := []string{
		"FOO=bar",
		"SECRET=@Microsoft.KeyVault(VaultName=vault;SecretName=name)",
	}

	warning := keyvault.KeyVaultResolutionWarning{Key: "SECRET", Err: errors.New("resolve failed")}
	fake := &fakeResolver{
		resolved: []string{"FOO=bar", "SECRET=@Microsoft.KeyVault(VaultName=vault;SecretName=name)"},
		warnings: []keyvault.KeyVaultResolutionWarning{warning},
		err:      nil, // No error, just warnings
	}

	result, warnings, err := ResolveSlice(context.Background(), envSlice, fake, keyvault.ResolveEnvironmentOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(warnings))
	}

	if warnings[0].Key != "SECRET" {
		t.Fatalf("expected warning for SECRET, got %v", warnings[0])
	}

	// Should return the resolver's result even with warnings
	expected := []string{"FOO=bar", "SECRET=@Microsoft.KeyVault(VaultName=vault;SecretName=name)"}
	if !reflect.DeepEqual(result, expected) {
		t.Fatalf("expected %v, got %v", expected, result)
	}
}

func TestCopySlice(t *testing.T) {
	original := []string{"A=1", "B=2", "C=3"}
	clone := copySlice(original)

	// Verify contents are equal
	if !reflect.DeepEqual(original, clone) {
		t.Fatalf("clone content mismatch: original %v, clone %v", original, clone)
	}

	// Verify it's a deep copy (modifying original doesn't affect clone)
	original[0] = "D=4"
	if clone[0] == "D=4" {
		t.Fatal("clone was modified when original was modified - not a deep copy")
	}

	if clone[0] != "A=1" {
		t.Fatalf("expected clone[0] to be A=1, got %s", clone[0])
	}
}

func TestResolveSlice_EmptySlice(t *testing.T) {
	envSlice := []string{}
	fake := &fakeResolver{resolved: []string{}}

	result, warnings, err := ResolveSlice(context.Background(), envSlice, fake, keyvault.ResolveEnvironmentOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if fake.called {
		t.Fatal("expected resolver not to be called for empty slice")
	}

	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %v", warnings)
	}

	if len(result) != 0 {
		t.Fatalf("expected empty result, got %v", result)
	}
}