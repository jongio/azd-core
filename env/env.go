// Package env provides environment variable resolution utilities with Azure Key Vault integration.
//
// This package makes it easy for consumers (like azd-app and azd-exec) to resolve
// Azure Key Vault references in environment variables. It provides adapter functions
// to work with both map[string]string and []string representations of environment variables.
//
// # Usage with Environment Maps
//
// Use ResolveMap when working with environment maps:
//
//	resolver, err := keyvault.NewKeyVaultResolver()
//	if err != nil {
//		// handle error
//	}
//
//	envMap := map[string]string{
//		"DATABASE_URL": "postgres://localhost/db",
//		"API_KEY": "@Microsoft.KeyVault(VaultName=myvault;SecretName=api-key)",
//	}
//
//	adapter := keyvault.NewEnvResolverAdapter(resolver)
//	resolved, warnings, err := env.ResolveMap(ctx, envMap, adapter, env.ResolveOptions{})
//	if err != nil {
//		// handle error
//	}
//	for _, w := range warnings {
//		// log warning: w.Key, w.Err
//	}
//	// resolved["API_KEY"] now contains the actual secret value from Key Vault
//
// # Usage with Environment Slices
//
// Use ResolveSlice when working with KEY=VALUE slices (e.g., from os.Environ()):
//
//	resolver, err := keyvault.NewKeyVaultResolver()
//	if err != nil {
//		// handle error
//	}
//
//	adapter := keyvault.NewEnvResolverAdapter(resolver)
//	envSlice := os.Environ() // or []string{"KEY=value", ...}
//	resolved, warnings, err := env.ResolveSlice(ctx, envSlice, adapter, env.ResolveOptions{})
//	if err != nil {
//		// handle error
//	}
//	// Use resolved with exec.Cmd: cmd.Env = resolved
//
// # Error Handling Options
//
// By default, resolution continues even if individual references fail (warnings are collected).
// Use StopOnError to fail fast:
//
//	opts := env.ResolveOptions{StopOnError: true}
//	resolved, warnings, err := env.ResolveMap(ctx, envMap, adapter, opts)
//	if err != nil {
//		// Resolution failed, warnings contains details
//	}
//
// # Supported Key Vault Reference Formats
//
// The package supports three Key Vault reference formats:
//   - @Microsoft.KeyVault(SecretUri=https://vault.vault.azure.net/secrets/name/version)
//   - @Microsoft.KeyVault(VaultName=vault;SecretName=name;SecretVersion=version)
//   - akvs://guid/vault/secret/version
//
// # Helper Functions
//
// The package also provides utility functions for working with environment variables:
//   - MapToSlice: Convert map[string]string to []string (KEY=VALUE format)
//   - SliceToMap: Convert []string to map[string]string (skips malformed entries)
//   - HasSecretReferences: Check if any secret references exist
package env

import (
	"context"
	"fmt"
	"strings"
)

// ResolveOptions configures environment resolution behavior.
type ResolveOptions struct {
	StopOnError bool
}

// ResolutionWarning captures non-fatal resolution failures.
type ResolutionWarning struct {
	Key string
	Err error
}

// SecretReferenceChecker determines whether a value is a secret reference
// that needs resolution.
type SecretReferenceChecker interface {
	IsSecretReference(value string) bool
}

// Resolver abstracts secret environment resolution to ease testing and
// break the dependency on specific secret store packages.
type Resolver interface {
	ResolveEnvironmentVariables(ctx context.Context, env []string, opts ResolveOptions) ([]string, []ResolutionWarning, error)
}

// MapToSlice converts an env map into KEY=VALUE entries.
func MapToSlice(env map[string]string) []string {
	result := make([]string, 0, len(env))
	for k, v := range env {
		result = append(result, fmt.Sprintf("%s=%s", k, v))
	}
	return result
}

// SliceToMap converts KEY=VALUE entries into a map, skipping malformed rows.
func SliceToMap(envSlice []string) map[string]string {
	result := make(map[string]string, len(envSlice))
	for _, envVar := range envSlice {
		parts := strings.SplitN(envVar, "=", 2)
		if len(parts) != 2 {
			continue
		}
		result[parts[0]] = parts[1]
	}
	return result
}

// HasSecretReferences quickly checks for any secret-formatted values using the
// provided checker. This avoids the env package needing to know about specific
// secret reference formats.
func HasSecretReferences(envVars []string, checker SecretReferenceChecker) bool {
	for _, envVar := range envVars {
		parts := strings.SplitN(envVar, "=", 2)
		if len(parts) != 2 {
			continue
		}
		if checker.IsSecretReference(parts[1]) {
			return true
		}
	}
	return false
}

// Resolve applies a secret resolver to the provided env map if needed.
func Resolve(ctx context.Context, env map[string]string, resolver Resolver, opts ResolveOptions) (map[string]string, []ResolutionWarning, error) {
	if env == nil {
		env = map[string]string{}
	}

	if resolver == nil {
		return copyEnv(env), nil, nil
	}

	envSlice := MapToSlice(env)

	resolvedSlice, warnings, err := resolver.ResolveEnvironmentVariables(ctx, envSlice, opts)
	if err != nil {
		return copyEnv(env), warnings, err
	}

	return SliceToMap(resolvedSlice), warnings, nil
}

// ResolveMap applies the secret resolver to an environment map.
// It converts the map to a slice, resolves any secret references,
// and returns a new map with the resolved values.
// If resolver is nil, the original map is returned (as a copy).
func ResolveMap(ctx context.Context, envMap map[string]string, resolver Resolver, opts ResolveOptions) (map[string]string, []ResolutionWarning, error) {
	return Resolve(ctx, envMap, resolver, opts)
}

// ResolveSlice applies the secret resolver to an environment slice.
// It takes KEY=VALUE entries, resolves any secret references,
// and returns a new slice with the resolved values.
// If resolver is nil, the original slice is returned (as a copy).
func ResolveSlice(ctx context.Context, envSlice []string, resolver Resolver, opts ResolveOptions) ([]string, []ResolutionWarning, error) {
	if envSlice == nil {
		return []string{}, nil, nil
	}

	if resolver == nil {
		return copySlice(envSlice), nil, nil
	}

	return resolver.ResolveEnvironmentVariables(ctx, envSlice, opts)
}

func copySlice(envSlice []string) []string {
	clone := make([]string, len(envSlice))
	copy(clone, envSlice)
	return clone
}

func copyEnv(env map[string]string) map[string]string {
	clone := make(map[string]string, len(env))
	for k, v := range env {
		clone[k] = v
	}
	return clone
}
