// Package env provides environment variable utilities for Azure Developer CLI (azd) extensions.
//
// This package includes:
//   - Key Vault reference resolution (ResolveMap, ResolveSlice)
//   - Format conversion (MapToSlice, SliceToMap)
//   - Pattern-based extraction (FilterByPrefix, ExtractPattern)
//   - Service name normalization (NormalizeServiceName)
//
// # Key Vault Resolution
//
// Use ResolveMap to resolve Azure Key Vault references in environment maps:
//
//	import (
//		"github.com/jongio/azd-core/env"
//		"github.com/jongio/azd-core/keyvault"
//	)
//
//	resolver, err := keyvault.NewKeyVaultResolver()
//	if err != nil {
//		return err
//	}
//
//	envMap := map[string]string{
//		"DATABASE_URL": "postgres://localhost/db",
//		"API_KEY": "@Microsoft.KeyVault(VaultName=myvault;SecretName=api-key)",
//	}
//
//	resolved, warnings, err := env.ResolveMap(ctx, envMap, resolver, keyvault.ResolveEnvironmentOptions{})
//	if err != nil {
//		return err
//	}
//	// resolved["API_KEY"] now contains the actual secret value from Key Vault
//
// # Pattern-Based Extraction
//
// Extract environment variables matching specific patterns:
//
//	// Filter by prefix (case-insensitive)
//	azureVars := env.FilterByPrefix(envVars, "AZURE_")
//	// Returns: {"AZURE_TENANT_ID": "xyz", "AZURE_CLIENT_ID": "abc"}
//
//	// Extract SERVICE_*_URL with normalization
//	serviceURLs := env.ExtractPattern(envVars, env.PatternOptions{
//		Prefix:       "SERVICE_",
//		Suffix:       "_URL",
//		TrimPrefix:   true,
//		TrimSuffix:   true,
//		Transform:    env.NormalizeServiceName,
//	})
//	// Returns: {"my-api": "https://...", "web-app": "https://..."}
//
// # Service Name Normalization
//
// Convert environment variable naming to service naming conventions:
//
//	serviceName := env.NormalizeServiceName("MY_API_SERVICE")
//	// Returns: "my-api-service"
//
// This is useful for converting uppercase underscore-separated names
// (common in environment variables) to lowercase hyphen-separated names
// (common in service identifiers, DNS labels, and container names).
//
// # Supported Key Vault Reference Formats
//
//   - @Microsoft.KeyVault(SecretUri=https://...)
//   - @Microsoft.KeyVault(VaultName=...;SecretName=...;SecretVersion=...)
//   - akvs://<subscription-id>/<vault-name>/<secret-name>[/<version>]
//
// # Authentication
//
// Key Vault resolution uses azidentity.DefaultAzureCredential, which supports:
//   - Environment variables (AZURE_TENANT_ID, AZURE_CLIENT_ID, AZURE_CLIENT_SECRET)
//   - Managed identity (Azure VM, App Service, Container Apps)
//   - Azure CLI (az login)
//   - Azure PowerShell
//   - Interactive browser authentication
//
// # Error Handling
//
// By default, resolution continues even if individual references fail (warnings are collected).
// Use StopOnError to fail fast:
//
//	opts := keyvault.ResolveEnvironmentOptions{StopOnError: true}
//	resolved, warnings, err := env.ResolveMap(ctx, envMap, resolver, opts)
//	if err != nil {
//		// First error encountered
//	}
package env
