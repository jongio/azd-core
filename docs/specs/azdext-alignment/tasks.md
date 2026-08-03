# azdext Alignment: Tasks

Spec: [spec.md](./spec.md)

Legend: `[ ]` pending, `[~]` in progress, `[x]` done. Rows with no marker are pending. Repos: **C** azd-core, **A** azd-app, **P** azd-copilot, **R** azd-rest.

## Phase 0: Baseline

| # | Repo | Task | Depends on |
|---|---|---|---|
| [x] 0.1 | A P R | Bump `github.com/azure/azure-dev/cli/azd` to `v1.29.0`, `go mod tidy`, verify build and tests green with no code changes | |
| [x] 0.2 | R | Align `go` directive to `1.26.5` to match the other repos | 0.1 |
| [x] 0.3 | C | Move `EXTENSION_FRAMEWORK_GAP_ANALYSIS.md` to `docs/archive/` with a superseded-by note referencing azure-dev PR #6856 | |
| [x] 0.4 | C | Add `docs/sdk-upgrade-policy.md`: track latest stable `cli/azd/vX.Y.Z` tag, never pseudo-versions, never `main` | |

Phase 0 notes:

- 0.1: azd-copilot and azd-rest were already on `v1.29.0`. azd-app moved `v1.28.1` to `v1.29.0` with no source changes; build, tests, and the coverage gate stayed green at 65.0%.
- 0.2: already satisfied. All four repos declare `go 1.26.5`.
- 0.3: archived to `docs/archive/extension-framework-gap-analysis.md` with a superseded-by banner. `spec.md` now points at the new path.
- 0.4: `docs/sdk-upgrade-policy.md`. All four repos move the SDK pin together; `azd-core` does not depend on the SDK directly yet.

## Phase 1: Entry points and structured errors

Order: R, then P, then A.

| # | Repo | Task | Depends on |
|---|---|---|---|
| 1.1 | R | Replace `main()` with `azdext.Run(cmd.NewRootCmd())`. Delete `ExitCoder` and manual error printing in `src/cmd/rest/main.go` and `src/internal/cmd/env.go` | 0.1 |
| 1.2 | R | Add `src/internal/cmd/errors.go` with `const` snake_case error codes. Convert command-handler failures to `azdext.LocalError` / `azdext.ServiceError` with categories and suggestions | 1.1 |
| 1.3 | R | Add a startup test asserting `azdext.ValidateNoReservedFlagConflicts(NewRootCmd())` returns nil | 1.1 |
| 1.4 | P | Replace `main()` with `azdext.Run(newRootCmd())`. Delete manual `Execute()` error handling in `src/cmd/copilot/main.go` | 0.1 |
| 1.5 | P | Add error codes and convert handler failures to structured errors | 1.4 |
| 1.6 | P | Add reserved-flag conflict test | 1.4 |
| 1.7 | P | Add `azdext.NewVersionCommand("jongio.azd.copilot", ...)` | 1.4 |
| 1.8 | A | Replace `main()` with `azdext.Run(rootCmd)`. Preserve the chained `PersistentPreRunE` behavior | 0.1 |
| 1.9 | A | Add error codes and convert handler failures to structured errors | 1.8 |
| 1.10 | A | Add reserved-flag conflict test | 1.8 |
| 1.11 | A | Add `azdext.NewVersionCommand("jongio.azd.app", ...)` | 1.8 |

## Phase 2: azd-core rebase

Each task must land with all three extensions still building against the prior `azd-core` tag.

| # | Repo | Task | Depends on |
|---|---|---|---|
| 2.0 | C | Add `github.com/azure/azure-dev/cli/azd v1.29.0` to `go.mod` | 0.1 |
| 2.1 | C | Delete `procutil`. Callers move to `azdext.IsProcessRunning` / `GetProcessInfo` | 2.0 |
| 2.2 | C | Delete `shellutil`. Callers move to `azdext.DetectShell` / `ShellCommand` | 2.0 |
| 2.3 | C | Delete `pathutil`. Callers move to `azdext.LookupTool` / `RequireTools` / `PrependPATH` | 2.0 |
| 2.4 | C | Delete `azdextutil` (`RateLimiter`, `ValidateShellName`, `GetProjectDir`). Callers move to `MCPServerBuilder.WithRateLimit`, `azdext.ValidateScriptName`, `azdext.GetProjectDir` | 2.0 |
| 2.5 | C | Delete `version`. Callers move to `azdext.NewVersionCommand` | 1.7, 1.11 |
| 2.6 | C | Delete `urlutil`. Callers move to `azdext.SSRFGuard` / `MCPSecurityPolicy` | 2.0 |
| 2.7 | C | Trim `fileutil`: delete `atomicWrite`, `renameWithRetry`, `AtomicWriteFile`, `EnsureDir`. Keep project-detection predicates and JSON cache helpers | 2.0 |
| 2.8 | C | Trim `security`: keep only `ValidateFilePermissions` and `ValidatePackageManager` (no azdext equivalent). Delete the rest | 2.6 |
| 2.9 | C | Rebase `keyvault` internals on `azdext.KeyVaultResolver`. Keep the warning-collecting façade in `env` | 2.0 |
| 2.10 | C | Rebase `httpclient.Client` on `azdext.ResilientClient` and `azdext.Pager`. Keep `Formatter`, redaction helpers, `DetectContentType`. Resolve open question 2 on `gobreaker` | 2.0 |
| 2.11 | C | Reduce `auth` to a host-aware selector: `azdext.TokenProvider` when a host is present, `azidentity` chain otherwise. Delete `DetectScope` / `IsAzureHost` in favor of `azdext.ScopeDetector` | 2.10 |
| 2.12 | C | Replace `logutil` internals with `azdext.Logger`. Keep signatures, mark deprecated | 2.0 |
| 2.13 | C | Delegate `cliout.PrintJSON` / `Table` to `azdext.Output`, and color/prompt gating to `azdext.DetectInteractive()`. Keep the rest of the `cliout` API | 2.0 |
| 2.14 | C | Switch `env.LoadAzdEnvironment` to `azdext.LoadAzdEnvironment`. Add an explicit `LoadAzdEnvironmentFromSubprocess` for detached children only | 2.0 |
| 2.15 | C | Release `azd-core v0.6.0` with a `CHANGELOG.md` migration table for every deleted symbol | 2.1-2.14 |
| 2.16 | A P R | Bump `azd-core` to `v0.6.0` and fix all call sites | 2.15 |

## Phase 3: Capability uplift

### azd-app

| # | Task | Depends on |
|---|---|---|
| 3.1 | Remove the detached-child env workaround in `src/cmd/app/main.go:44-56`. Add a regression test that a detached child starts without blocking | 2.16 |
| 3.2 | Replace `src/internal/azdconfig` with `azdext.ConfigHelper` | 2.16 |
| 3.3 | Embed `azdext.BaseServiceTargetProvider` in `src/internal/servicetarget/local_provider.go`, delete no-op methods | 2.16 |
| 3.4 | Add `providers_manifest_test.go` using `azdext.VerifyProvidersMatchManifest` | 2.16 |
| 3.5 | Add an MCP security policy to the MCP server and audit every tool against it | 2.16 |
| 3.6 | Add configuration JSON Schema and `EnvironmentVariables` to the metadata command | 2.16 |
| 3.7 | Evaluate `WithValidationCheck` for pre-provision project validation, and `WithProvisioningProvider` for local orchestration. Decide and record | 3.4 |

### azd-copilot

| # | Task | Depends on |
|---|---|---|
| 3.8 | Prototype `client.Copilot()` for non-interactive paths. Measure latency against the subprocess launcher. Resolve open question 1 | 2.16 |
| 3.9 | If 3.8 is favorable, route `--prompt` / autopilot / MCP tool paths through gRPC. Keep the subprocess path for interactive TTY sessions | 3.8 |
| 3.10 | Use `client.Ai()` for model and quota lookups | 2.16 |
| 3.11 | Add an MCP security policy | 2.16 |
| 3.12 | Add configuration JSON Schema and `EnvironmentVariables` to the metadata command | 2.16 |

### azd-rest

| # | Task | Depends on |
|---|---|---|
| 3.13 | Delete custom pagination in `src/internal/client`, use `azdext.Pager` | 2.16 |
| 3.14 | Delete custom retry and token caching in `src/internal/cmd/mcp.go`, use `azdext.ResilientClient` and `azdext.TokenProvider` | 2.16 |
| 3.15 | Use `azdext.ScopeDetector` for scope detection | 2.16 |
| 3.16 | Extend the MCP security policy with `ValidatePathsWithinBase` for file-reading tools | 2.16 |
| 3.17 | Add configuration JSON Schema and `EnvironmentVariables` to the metadata command | 2.16 |

## Phase 4: Distribution

| # | Repo | Task | Depends on |
|---|---|---|---|
| 4.1 | A P R | Replace `minAzdVersion` (azd-rest, not a schema field) and add `requiredAzdVersion: ">= 1.29.0"` to all three manifests. Validate against `extension.schema.json` | 3.x |
| 4.2 | A P | Standardize magefiles on `azd x build` / `pack` / `publish` / `release`, matching azd-rest | 4.1 |
| 4.3 | new | Create a `jongio.azd` extension pack manifest with dependencies on the three extensions | 4.1 |
| 4.4 | A P R | Submit entries to `registry.dev.json` in Azure/azure-dev with SHA256/SHA512 checksums | 4.3 |

## Verification

Run after each phase, not just at the end.

- `mage test` green in all four repos, zero warnings.
- `mage lint` / `golangci-lint` clean.
- `azdext.ValidateNoReservedFlagConflicts` and `VerifyProvidersMatchManifest` tests pass.
- Manual smoke on Windows, Linux, macOS: `azd app run`, `azd rest get <url>`, `azd copilot -p "hello"`.
- `go build` binary size delta under 10 percent per extension.
