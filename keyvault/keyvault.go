// Package keyvault provides Azure Key Vault reference resolution helpers.
package keyvault

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
)

const (
	// Azure Key Vault naming constraints
	minVaultNameLength = 3
	maxVaultNameLength = 24
)

var (
	kvRefSecretURIPattern = regexp.MustCompile(`^@Microsoft\.KeyVault\(SecretUri=(.+)\)$`)
	kvRefVaultNamePattern = regexp.MustCompile(`^@Microsoft\.KeyVault\(VaultName=([^;]+);SecretName=([^;)]+)(?:;SecretVersion=([^;)]+))?\)$`)
	kvRefAzdAkvsPattern   = regexp.MustCompile(`^akvs://([^/]+)/([^/]+)/([^/]+)(?:/([^/]+))?$`)
)

// KeyVaultResolver resolves Azure Key Vault references to secret values.
type KeyVaultResolver struct {
	credential *azidentity.DefaultAzureCredential
	clients    map[string]*azsecrets.Client
	mu         sync.RWMutex
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

	return &KeyVaultResolver{
		credential: cred,
		clients:    make(map[string]*azsecrets.Client),
	}, nil
}

// IsKeyVaultReference reports whether the value matches a supported reference format.
func IsKeyVaultReference(value string) bool {
	normalized := normalizeKeyVaultReferenceValue(value)

	if kvRefSecretURIPattern.MatchString(normalized) {
		return true
	}

	if kvRefVaultNamePattern.MatchString(normalized) {
		return true
	}

	if strings.HasPrefix(normalized, "akvs://") {
		return kvRefAzdAkvsPattern.MatchString(normalized)
	}

	return false
}

// ResolveReference resolves a single Key Vault reference to its secret value.
func (r *KeyVaultResolver) ResolveReference(ctx context.Context, reference string) (string, error) {
	reference = normalizeKeyVaultReferenceValue(reference)

	if matches := kvRefSecretURIPattern.FindStringSubmatch(reference); matches != nil {
		secretURI := strings.TrimSpace(matches[1])
		return r.resolveBySecretURI(ctx, secretURI)
	}

	if matches := kvRefVaultNamePattern.FindStringSubmatch(reference); matches != nil {
		vaultName := matches[1]
		secretName := matches[2]
		version := ""
		if len(matches) > 3 && matches[3] != "" {
			version = matches[3]
		}
		return r.resolveByVaultNameAndSecret(ctx, vaultName, secretName, version)
	}

	if strings.HasPrefix(reference, "akvs://") {
		if !kvRefAzdAkvsPattern.MatchString(reference) {
			return "", fmt.Errorf("invalid akvs URI format")
		}

		guid, vaultName, secretName, version, err := parseAzdAkvsURI(reference)
		_ = guid
		if err != nil {
			return "", err
		}
		return r.resolveByVaultNameAndSecret(ctx, vaultName, secretName, version)
	}

	return "", fmt.Errorf("invalid Key Vault reference format")
}

// ResolveEnvironmentVariables resolves references in KEY=VALUE entries.
func (r *KeyVaultResolver) ResolveEnvironmentVariables(ctx context.Context, envVars []string, options ResolveEnvironmentOptions) ([]string, []KeyVaultResolutionWarning, error) {
	resolved := make([]string, 0, len(envVars))
	var warnings []KeyVaultResolutionWarning

	for _, envVar := range envVars {
		// Check for context cancellation to allow early termination
		select {
		case <-ctx.Done():
			return nil, warnings, ctx.Err()
		default:
		}

		parts := strings.SplitN(envVar, "=", 2)
		if len(parts) != 2 {
			resolved = append(resolved, envVar)
			continue
		}

		key := parts[0]
		value := parts[1]

		if !IsKeyVaultReference(value) {
			resolved = append(resolved, envVar)
			continue
		}

		secretValue, err := r.ResolveReference(ctx, value)
		if err != nil {
			warning := KeyVaultResolutionWarning{
				Key: key,
				Err: err,
			}
			warnings = append(warnings, warning)

			if options.StopOnError {
				return nil, warnings, fmt.Errorf("failed to resolve Key Vault reference for %s: %w", key, err)
			}

			resolved = append(resolved, envVar)
			continue
		}

		resolved = append(resolved, fmt.Sprintf("%s=%s", key, secretValue))
	}

	return resolved, warnings, nil
}

func (r *KeyVaultResolver) getClient(vaultURL string) (*azsecrets.Client, error) {
	// Double-checked locking pattern: check without lock first for performance,
	// then acquire write lock only if client doesn't exist
	r.mu.RLock()
	if client, ok := r.clients[vaultURL]; ok {
		r.mu.RUnlock()
		return client, nil
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()

	// Re-check after acquiring write lock in case another goroutine created it
	if client, ok := r.clients[vaultURL]; ok {
		return client, nil
	}

	client, err := azsecrets.NewClient(vaultURL, r.credential, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Key Vault client: %w", err)
	}

	r.clients[vaultURL] = client
	return client, nil
}

func (r *KeyVaultResolver) resolveBySecretURI(ctx context.Context, secretURI string) (string, error) {
	parts := strings.Split(secretURI, "/secrets/")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid secret URI format")
	}

	vaultURL := parts[0]
	secretPath := parts[1]

	if err := validateVaultURL(vaultURL); err != nil {
		return "", err
	}

	client, err := r.getClient(vaultURL)
	if err != nil {
		return "", err
	}

	secretParts := strings.Split(secretPath, "/")
	secretName := secretParts[0]
	version := ""
	if len(secretParts) > 1 {
		version = secretParts[1]
	}

	var resp azsecrets.GetSecretResponse
	if version != "" {
		resp, err = client.GetSecret(ctx, secretName, version, nil)
	} else {
		resp, err = client.GetSecret(ctx, secretName, "", nil)
	}

	if err != nil {
		// Don't include vault URL in error to avoid information disclosure in logs
		return "", fmt.Errorf("failed to get secret from Key Vault: %w", err)
	}

	if resp.Value == nil {
		return "", fmt.Errorf("secret has no value")
	}

	return *resp.Value, nil
}

func (r *KeyVaultResolver) resolveByVaultNameAndSecret(ctx context.Context, vaultName, secretName, version string) (string, error) {
	if err := validateVaultName(vaultName); err != nil {
		return "", err
	}

	vaultURL := fmt.Sprintf("https://%s.vault.azure.net", vaultName)

	client, err := r.getClient(vaultURL)
	if err != nil {
		return "", err
	}

	var resp azsecrets.GetSecretResponse
	if version != "" {
		resp, err = client.GetSecret(ctx, secretName, version, nil)
	} else {
		resp, err = client.GetSecret(ctx, secretName, "", nil)
	}

	if err != nil {
		// Don't include vault name or secret name in error to avoid information disclosure
		return "", fmt.Errorf("failed to get secret from Key Vault: %w", err)
	}

	if resp.Value == nil {
		return "", fmt.Errorf("secret has no value")
	}

	return *resp.Value, nil
}

func parseAzdAkvsURI(uri string) (guid, vaultName, secretName, version string, err error) {
	matches := kvRefAzdAkvsPattern.FindStringSubmatch(uri)
	if matches == nil {
		return "", "", "", "", fmt.Errorf("invalid akvs URI format: %s", uri)
	}

	guid = matches[1]
	vaultName = matches[2]
	secretName = matches[3]
	if len(matches) > 4 {
		version = matches[4]
	}

	return guid, vaultName, secretName, version, nil
}

func normalizeKeyVaultReferenceValue(value string) string {
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

func validateVaultURL(vaultURL string) error {
	if !strings.HasPrefix(vaultURL, "https://") {
		return fmt.Errorf("vault URI must use https scheme")
	}

	if !strings.HasSuffix(vaultURL, ".vault.azure.net") {
		return fmt.Errorf("vault URI must be in *.vault.azure.net domain")
	}

	vaultName := strings.TrimPrefix(vaultURL, "https://")
	vaultName = strings.TrimSuffix(vaultName, ".vault.azure.net")

	return validateVaultName(vaultName)
}

func validateVaultName(vaultName string) error {
	if len(vaultName) < minVaultNameLength || len(vaultName) > maxVaultNameLength {
		return fmt.Errorf("vault name must be %d-%d characters, got %d", minVaultNameLength, maxVaultNameLength, len(vaultName))
	}

	for i, ch := range vaultName {
		if (ch < 'a' || ch > 'z') && (ch < 'A' || ch > 'Z') && (ch < '0' || ch > '9') && ch != '-' {
			return fmt.Errorf("vault name contains invalid character: %c", ch)
		}
		if i == 0 && ch >= '0' && ch <= '9' {
			return fmt.Errorf("vault name cannot start with a number")
		}
	}

	return nil
}
