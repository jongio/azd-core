// Package keyvault provides Azure Key Vault reference resolution helpers.
//
// This file contains the adapter that bridges the keyvault types with the
// env package's Resolver interface, enabling the env package to resolve
// Key Vault references without importing keyvault directly.
package keyvault

import (
	"context"
)

// EnvResolveOptions configures environment resolution behavior for the adapter.
type EnvResolveOptions struct {
	StopOnError bool
}

// EnvResolutionWarning captures non-fatal resolution failures in the adapter.
type EnvResolutionWarning struct {
	Key string
	Err error
}

// EnvResolverAdapter adapts KeyVaultResolver to the env.Resolver interface,
// breaking the import cycle between env and keyvault packages.
type EnvResolverAdapter struct {
	resolver *KeyVaultResolver
}

// NewEnvResolverAdapter creates an adapter that satisfies the env.Resolver interface.
func NewEnvResolverAdapter(resolver *KeyVaultResolver) *EnvResolverAdapter {
	return &EnvResolverAdapter{resolver: resolver}
}

// IsSecretReference reports whether a value is a Key Vault reference.
// This satisfies the env.SecretReferenceChecker interface.
func (a *EnvResolverAdapter) IsSecretReference(value string) bool {
	return IsKeyVaultReference(value)
}

// ResolveEnvironmentVariables resolves Key Vault references in KEY=VALUE entries.
// The opts parameter uses a generic struct with StopOnError to avoid importing env.
//
// This method satisfies the env.Resolver interface via structural typing:
//
//	ResolveEnvironmentVariables(ctx context.Context, env []string, opts env.ResolveOptions) ([]string, []env.ResolutionWarning, error)
//
// Since Go uses structural typing for interfaces, this adapter works as long as
// the method signature matches. The env package defines its own ResolveOptions
// and ResolutionWarning types that this adapter is compatible with.
func (a *EnvResolverAdapter) ResolveEnvironmentVariables(ctx context.Context, envVars []string, opts EnvResolveOptions) ([]string, []EnvResolutionWarning, error) {
	return resolveEnvVars(ctx, a.resolver, envVars, opts.StopOnError, func(key string, err error) EnvResolutionWarning {
		return EnvResolutionWarning{Key: key, Err: err}
	})
}
