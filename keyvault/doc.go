// Package keyvault resolves Azure Key Vault references found in environment
// variables.
//
// Reference parsing, client construction, per-vault client caching, and secret
// retrieval are delegated to azdext.KeyVaultResolver. This package adds the
// KEY=VALUE slice API used by azd extensions and support for the versioned
// akvs://<subscription>/<vault>/<secret>/<version> form, which azdext does not
// parse on its own.
//
// Three reference formats are accepted:
//
//	akvs://<subscription-id>/<vault-name>/<secret-name>[/<version>]
//	@Microsoft.KeyVault(SecretUri=https://<vault>.vault.azure.net/secrets/<name>[/<version>])
//	@Microsoft.KeyVault(VaultName=<vault>;SecretName=<name>[;SecretVersion=<version>])
//
// NewKeyVaultResolver authenticates with azidentity.DefaultAzureCredential.
// NewKeyVaultResolverWithCredential takes an explicit credential and options,
// which is how callers reach sovereign clouds or inject a fake secret client
// in tests.
package keyvault
