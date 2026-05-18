package keyvault

import (
	"context"

	"github.com/jongio/azd-core/env"
)

// EnvResolver adapts KeyVaultResolver to the env.Resolver interface.
type EnvResolver struct {
	Resolver *KeyVaultResolver
}

// NewEnvResolver creates an adapter that satisfies env.Resolver.
func NewEnvResolver(r *KeyVaultResolver) *EnvResolver {
	return &EnvResolver{Resolver: r}
}

// ResolveEnvironmentVariables bridges between env and keyvault type systems.
func (a *EnvResolver) ResolveEnvironmentVariables(ctx context.Context, envVars []string, opts env.ResolveOptions) ([]string, []env.ResolutionWarning, error) {
	kvOpts := ResolveEnvironmentOptions{
		StopOnError: opts.StopOnError,
	}
	resolved, kvWarnings, err := a.Resolver.ResolveEnvironmentVariables(ctx, envVars, kvOpts)
	warnings := make([]env.ResolutionWarning, len(kvWarnings))
	for i, w := range kvWarnings {
		warnings[i] = env.ResolutionWarning{Key: w.Key, Err: w.Err}
	}
	return resolved, warnings, err
}
