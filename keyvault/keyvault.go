// Package keyvault provides Azure Key Vault reference resolution helpers.
package keyvault

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/azure/azure-dev/cli/azd/pkg/azdext"
)

// kvRefAkvsVersionedPattern matches the four segment akvs form that carries an
// explicit secret version:
//
//	akvs://<subscription-id>/<vault-name>/<secret-name>/<version>
//
// azdext.ParseSecretReference accepts only the three segment form, so this
// package rewrites the four segment form into the equivalent
// @Microsoft.KeyVault(VaultName=...;SecretName=...;SecretVersion=...) reference,
// which azdext does understand. See normalizeReference.
var kvRefAkvsVersionedPattern = regexp.MustCompile(`^akvs://([^/]+)/([^/]+)/([^/]+)/([^/]+)$`)

// KeyVaultResolver resolves Azure Key Vault references to secret values.
//
// Reference parsing, client construction, per-vault client caching, and secret
// retrieval are all provided by azdext.KeyVaultResolver. This type adds the
// KEY=VALUE environment slice API that azd extensions use, and support for the
// versioned akvs form.
type KeyVaultResolver struct {
	inner *azdext.KeyVaultResolver
}

// KeyVaultResolutionWarning captures non-fatal resolution failures.
type KeyVaultResolutionWarning struct {
	Key string
	Err error
}

// ResolveEnvironmentOptions configures environment resolution behavior.
type ResolveEnvironmentOptions struct {
	StopOnError bool
}

// NewKeyVaultResolver builds a resolver using DefaultAzureCredential.
func NewKeyVaultResolver() (*KeyVaultResolver, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create DefaultAzureCredential: %w", err)
	}

	return NewKeyVaultResolverWithCredential(cred, nil)
}

// NewKeyVaultResolverWithCredential builds a resolver from an explicit credential.
//
// Use this to supply an azdext.TokenProvider so secret resolution rides the
// extension's own azd issued token rather than a separate credential chain, to
// target a sovereign cloud through KeyVaultResolverOptions.VaultSuffix, or to
// inject a secret client in tests through KeyVaultResolverOptions.ClientFactory.
//
// Passing nil opts selects the Azure public cloud and the real secret client.
func NewKeyVaultResolverWithCredential(
	credential azcore.TokenCredential,
	opts *azdext.KeyVaultResolverOptions,
) (*KeyVaultResolver, error) {
	inner, err := azdext.NewKeyVaultResolver(credential, opts)
	if err != nil {
		return nil, err
	}

	return &KeyVaultResolver{inner: inner}, nil
}

// IsKeyVaultReference reports whether the value matches a supported reference format.
//
// Surrounding whitespace and a single layer of matching quotes are ignored, so a
// value read straight out of a .env file is recognized.
func IsKeyVaultReference(value string) bool {
	return azdext.IsSecretReference(value)
}

// ResolveReference resolves a single Key Vault reference to its secret value.
//
// Failures are returned as *azdext.KeyVaultResolveError, so callers can inspect
// Reason to distinguish a malformed reference from a missing secret, an access
// denial, or a service error.
func (r *KeyVaultResolver) ResolveReference(ctx context.Context, reference string) (string, error) {
	return r.inner.Resolve(ctx, normalizeReference(reference))
}

// ResolveEnvironmentVariables resolves references in KEY=VALUE entries.
//
// Entries that are not KEY=VALUE, and values that are not Key Vault references,
// are passed through untouched. When a reference fails to resolve, the original
// entry is preserved and a warning is recorded. Set StopOnError to abort on the
// first failure instead.
func (r *KeyVaultResolver) ResolveEnvironmentVariables(
	ctx context.Context,
	envVars []string,
	options ResolveEnvironmentOptions,
) ([]string, []KeyVaultResolutionWarning, error) {
	return resolveEnvVars(ctx, r, envVars, options.StopOnError, func(key string, err error) KeyVaultResolutionWarning {
		return KeyVaultResolutionWarning{Key: key, Err: err}
	})
}

// resolveEnvVars implements the KEY=VALUE resolution loop shared by
// KeyVaultResolver.ResolveEnvironmentVariables and EnvResolverAdapter. The
// warning type is a parameter because the adapter deliberately declares its own
// so the env package does not have to import this one.
func resolveEnvVars[W any](
	ctx context.Context,
	r *KeyVaultResolver,
	envVars []string,
	stopOnError bool,
	newWarning func(key string, err error) W,
) ([]string, []W, error) {
	resolved := make([]string, 0, len(envVars))

	var warnings []W

	for _, envVar := range envVars {
		// Check for context cancellation to allow early termination
		select {
		case <-ctx.Done():
			return nil, warnings, ctx.Err()
		default:
		}

		key, value, ok := strings.Cut(envVar, "=")
		if !ok || !IsKeyVaultReference(value) {
			resolved = append(resolved, envVar)
			continue
		}

		secretValue, err := r.ResolveReference(ctx, value)
		if err != nil {
			warnings = append(warnings, newWarning(key, err))

			if stopOnError {
				return nil, warnings, fmt.Errorf("failed to resolve Key Vault reference for %s: %w", key, err)
			}

			resolved = append(resolved, envVar)
			continue
		}

		resolved = append(resolved, key+"="+secretValue)
	}

	return resolved, warnings, nil
}

// normalizeReference rewrites the versioned akvs form into the equivalent
// @Microsoft.KeyVault(VaultName=...;SecretName=...;SecretVersion=...) reference.
// Every other input is returned unchanged for azdext to parse.
func normalizeReference(reference string) string {
	matches := kvRefAkvsVersionedPattern.FindStringSubmatch(stripQuotes(reference))
	if matches == nil {
		return reference
	}

	return fmt.Sprintf(
		"@Microsoft.KeyVault(VaultName=%s;SecretName=%s;SecretVersion=%s)",
		matches[2], matches[3], matches[4],
	)
}

// stripQuotes removes surrounding whitespace and a single layer of matching
// single or double quotes. It mirrors what azdext does to every reference it is
// given, so the versioned akvs check sees the same string azdext would.
func stripQuotes(value string) string {
	normalized := strings.TrimSpace(value)
	if len(normalized) < 2 {
		return normalized
	}

	first := normalized[0]
	last := normalized[len(normalized)-1]

	if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
		normalized = strings.TrimSpace(normalized[1 : len(normalized)-1])
	}

	return normalized
}
