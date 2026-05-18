package env

import (
	"context"
	"errors"
	"reflect"
	"testing"
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

func TestHasSecretReferences(t *testing.T) {
	checker := &fakeChecker{refs: map[string]bool{
		"@Microsoft.KeyVault(VaultName=vault;SecretName=name)": true,
	}}
	withRef := []string{
		"FOO=bar",
		"SECRET=@Microsoft.KeyVault(VaultName=vault;SecretName=name)",
	}
	withoutRef := []string{
		"FOO=bar",
		"BAZ=qux",
	}

	if !HasSecretReferences(withRef, checker) {
		t.Fatal("expected secret reference to be detected")
	}

	if HasSecretReferences(withoutRef, checker) {
		t.Fatal("did not expect secret reference to be detected")
	}
}

func TestResolveSkipsWhenResolverMissing(t *testing.T) {
	env := map[string]string{"FOO": "bar"}
	result, warnings, err := Resolve(context.Background(), env, nil, ResolveOptions{})
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

func TestResolveUsesResolver(t *testing.T) {
	env := map[string]string{
		"FOO":    "bar",
		"SECRET": "@Microsoft.KeyVault(VaultName=vault;SecretName=name)",
	}

	fake := &fakeResolver{
		resolved: []string{"SECRET=resolved", "FOO=bar"},
	}

	result, warnings, err := Resolve(context.Background(), env, fake, ResolveOptions{})
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

func TestResolvePropagatesError(t *testing.T) {
	env := map[string]string{
		"SECRET": "@Microsoft.KeyVault(VaultName=vault;SecretName=name)",
	}

	warning := ResolutionWarning{Key: "SECRET", Err: errors.New("resolve failed")}
	fake := &fakeResolver{
		resolved: MapToSlice(env),
		warnings: []ResolutionWarning{warning},
		err:      warning.Err,
	}

	result, warnings, err := Resolve(context.Background(), env, fake, ResolveOptions{})
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

type fakeChecker struct {
	refs map[string]bool
}

func (f *fakeChecker) IsSecretReference(value string) bool {
	return f.refs[value]
}

type fakeResolver struct {
	resolved []string
	warnings []ResolutionWarning
	err      error
	called   bool
}

func (f *fakeResolver) ResolveEnvironmentVariables(ctx context.Context, env []string, opts ResolveOptions) ([]string, []ResolutionWarning, error) {
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

	if len(result) != 3 {
		t.Errorf("SliceToMap() expected 3 entries, got %d", len(result))
	}

	if result["VALID"] != "value" {
		t.Errorf("SliceToMap() VALID = %q, want %q", result["VALID"], "value")
	}

	if result["ANOTHER"] != "valid" {
		t.Errorf("SliceToMap() ANOTHER = %q, want %q", result["ANOTHER"], "valid")
	}

	if _, exists := result["MISSING_EQUALS"]; exists {
		t.Errorf("SliceToMap() should have skipped MISSING_EQUALS")
	}
}

func TestResolveWithNilEnvironment(t *testing.T) {
	resolver := &fakeResolver{
		resolved: []string{},
	}

	result, warnings, err := Resolve(context.Background(), nil, resolver, ResolveOptions{})
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

	if !reflect.DeepEqual(original, clone) {
		t.Fatalf("clone content mismatch: original %v, clone %v", original, clone)
	}

	original["D"] = "4"
	if _, exists := clone["D"]; exists {
		t.Fatal("clone was modified when original was modified - not a deep copy")
	}
}

func TestResolveEnvironmentVariables_WithStopOnError(t *testing.T) {
	env := map[string]string{
		"SECRET": "@Microsoft.KeyVault(VaultName=vault;SecretName=name)",
	}

	warning := ResolutionWarning{Key: "SECRET", Err: errors.New("resolve failed")}
	fake := &fakeResolver{
		resolved: MapToSlice(env),
		warnings: []ResolutionWarning{warning},
		err:      warning.Err,
	}

	_, warnings, err := Resolve(context.Background(), env, fake, ResolveOptions{StopOnError: true})
	if err == nil {
		t.Fatal("expected error with StopOnError=true")
	}

	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(warnings))
	}
}

func TestResolveMap_WithReferences(t *testing.T) {
	env := map[string]string{
		"FOO":    "bar",
		"SECRET": "@Microsoft.KeyVault(VaultName=vault;SecretName=name)",
	}

	fake := &fakeResolver{
		resolved: []string{"SECRET=resolved-secret", "FOO=bar"},
	}

	result, warnings, err := ResolveMap(context.Background(), env, fake, ResolveOptions{})
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
}

func TestResolveMap_NilResolver(t *testing.T) {
	env := map[string]string{
		"SECRET": "@Microsoft.KeyVault(VaultName=vault;SecretName=name)",
	}

	result, warnings, err := ResolveMap(context.Background(), env, nil, ResolveOptions{})
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

func TestResolveSlice_WithReferences(t *testing.T) {
	envSlice := []string{
		"FOO=bar",
		"SECRET=@Microsoft.KeyVault(VaultName=vault;SecretName=name)",
	}

	fake := &fakeResolver{
		resolved: []string{"FOO=bar", "SECRET=resolved-secret"},
	}

	result, warnings, err := ResolveSlice(context.Background(), envSlice, fake, ResolveOptions{})
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

func TestResolveSlice_NilResolver(t *testing.T) {
	envSlice := []string{
		"SECRET=@Microsoft.KeyVault(VaultName=vault;SecretName=name)",
	}

	result, warnings, err := ResolveSlice(context.Background(), envSlice, nil, ResolveOptions{})
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

	result, warnings, err := ResolveSlice(context.Background(), nil, resolver, ResolveOptions{})
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

func TestCopySlice(t *testing.T) {
	original := []string{"A=1", "B=2", "C=3"}
	clone := copySlice(original)

	if !reflect.DeepEqual(original, clone) {
		t.Fatalf("clone content mismatch: original %v, clone %v", original, clone)
	}

	original[0] = "D=4"
	if clone[0] == "D=4" {
		t.Fatal("clone was modified when original was modified - not a deep copy")
	}
}

