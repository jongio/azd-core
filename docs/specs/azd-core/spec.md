# azd-core

## Goal
Provide a shared Go module (`github.com/jongio/azd-core`) for reusable azd CLI/extension logic. First deliverable: Azure Key Vault reference resolution utilities extracted from `jongio/azd-app` PR #103, packaged for reuse by `azd-app`, `azd-exec`, and future CLIs.

## Scope (initial cut)
- Key Vault reference resolution library with:
  - Detection of `@Microsoft.KeyVault(...)` and `akvs://` references.
  - Resolution via Azure Key Vault using `DefaultAzureCredential`.
  - Safe URL/name validation to prevent SSRF and invalid vault names.
  - Caching of `azsecrets.Client` instances per vault.
  - Configurable error-handling (`StopOnError`), warning surfacing.
- Environment resolution helper that wraps the resolver to process `KEY=VALUE` slices/maps.
- Minimal module scaffolding: go.mod, package docs, README for usage.

## Non-goals (now)
- Broader azd runtime helpers (logs, telemetry, config) — add later.
- Cross-language bindings.
- Release automation; manual tagging is fine for the first drop.

## References
- Source PR: https://github.com/jongio/azd-app/pull/103 (branch `kvres`).
- Files to port first:
  - `cli/src/internal/keyvault/keyvault.go`
  - `cli/src/internal/keyvault/keyvault_test.go`
  - `cli/src/internal/service/environment.go` (Key Vault resolution integration patterns)

## Design notes (KV resolver)
- Support three formats: `@Microsoft.KeyVault(SecretUri=...)`, `@Microsoft.KeyVault(VaultName=...;SecretName=...;SecretVersion=...)`, and `akvs://<guid>/<vault>/<secret>[/<version>]`.
- Use `azidentity.DefaultAzureCredential`; no globals; thread-safe client cache.
- Validate vault URLs/names before fetching to mitigate SSRF/invalid names.
- Normalize wrapped values (strip surrounding quotes/whitespace) before parsing.
- Return warnings for failed resolutions; allow graceful degradation by default.

## API shape (proposed)
- Package `keyvault`:
  - `func NewKeyVaultResolver() (*KeyVaultResolver, error)`
  - `func IsKeyVaultReference(string) bool`
  - `func (*KeyVaultResolver) ResolveReference(ctx context.Context, ref string) (string, error)`
  - `func (*KeyVaultResolver) ResolveEnvironmentVariables(ctx context.Context, env []string, opts ResolveEnvironmentOptions) ([]string, []KeyVaultResolutionWarning, error)`
- Package `env` (or `environ`, helper):
  - Adapter functions to map `map[string]string` ↔ `[]string` and apply resolver; mirrors `ResolveEnvironment` pattern from `azd-app` without pulling unrelated logic.

## Acceptance criteria
- Module builds with `go test ./...` on a clean checkout (network calls skipped/mocked where applicable).
- Key Vault resolver and tests are ported with no regression to behaviors in PR #103.
- Public README documents supported reference formats, auth expectations, and usage example for consumers.
- No global state; thread-safe client cache; validation guards intact.
