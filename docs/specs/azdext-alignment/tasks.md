# azdext Alignment: Tasks

Spec: [spec.md](./spec.md)

Legend: `[ ]` pending, `[~]` in progress, `[x]` done, `[-]` deliberately not done with a documented reason. Rows with no marker are pending. Repos: **C** azd-core, **A** azd-app, **P** azd-copilot, **R** azd-rest.

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
| [x] 1.1 | R | Replace `main()` with `azdext.Run(cmd.NewRootCmd())`. Delete `ExitCoder` and manual error printing in `src/cmd/rest/main.go` and `src/internal/cmd/env.go` | 0.1 |
| [x] 1.2 | R | Add `src/internal/cmd/errors.go` with `const` snake_case error codes. Convert command-handler failures to `azdext.LocalError` / `azdext.ServiceError` with categories and suggestions | 1.1 |
| [x] 1.3 | R | Add a startup test asserting `azdext.ValidateNoReservedFlagConflicts(NewRootCmd())` returns nil | 1.1 |
| [x] 1.4 | P | Replace `main()` with `azdext.Run(newRootCmd())`. Delete manual `Execute()` error handling in `src/cmd/copilot/main.go` | 0.1 |
| [x] 1.5 | P | Add error codes and convert handler failures to structured errors | 1.4 |
| [x] 1.6 | P | Add reserved-flag conflict test | 1.4 |
| [-] 1.7 | P | Add `azdext.NewVersionCommand("jongio.azd.copilot", ...)` | 1.4 |
| [x] 1.8 | A | Replace `main()` with `azdext.Run(rootCmd)`. Preserve the chained `PersistentPreRunE` behavior | 0.1 |
| [x] 1.9 | A | Add error codes and convert handler failures to structured errors | 1.8 |
| [x] 1.10 | A | Add reserved-flag conflict test | 1.8 |
| [-] 1.11 | A | Add `azdext.NewVersionCommand("jongio.azd.app", ...)` | 1.8 |

Phase 1 notes:

- 1.1-1.3: azd-rest, commit `5499505`. Six snake_case codes in `src/internal/service/errors.go`, `fail.go` deleted, five test files moved off `ExitCode()` assertions. `RequestOptions.URL` gave a real host for the `serviceName` telemetry argument via a new `hostFromURL` helper, so nothing passes an empty string. Coverage 86.3% -> 87.0%.
- 1.4-1.6: azd-copilot, commit `9a96db4`. Five codes in `commands/errors.go`. Coverage 30.0% -> 31.4%.
- 1.8-1.10: azd-app, commit `9af66050`. Five codes in `commands/errors.go`. Coverage 65.0% -> 65.1%, and `cmd/app` went 0.0% -> 24.0% because extracting `newRootCmd` made the command tree testable.
- 1.7 and 1.11 rejected. `azdext.NewVersionCommand` is a regression against azd-core's `version.NewCommand`, not an upgrade. The SDK command emits only `{name, version}` in JSON, prints `id version` as text, and has no `--quiet`. azd-core's emits the full `Info` (version, buildDate, gitCommit, extensionId, name), prints four labeled fields, and has `--quiet`/`-q`. The SDK command's one real advantage is that it calls `RegisterFlagOptions` so azd can discover `--output json`. Task 2.5 folds that advantage into azd-core's command instead, and moves azd-rest back onto it. `spec.md` line 58 previously claimed the two were equivalent; that claim is corrected.
- Adopting `azdext.Run` was not cosmetic. azd-copilot and azd-app both called `rootCmd.Execute()`, which runs the command tree on `context.Background()` with no azd trace propagation and no gRPC access token. `azdext.Run` uses `ExecuteContext(NewContext())` with `WithAccessToken`.
- 1.10 found a shipping bug rather than passing. Five azd-app subcommands defined local flags that shadowed azd reserved globals, so `azdext.Run` would have refused to start. The collisions were already breaking `--output` and `-e` silently, because cobra resolves a local flag ahead of an inherited one. Fixed as a breaking flag rename; see the azd-app CHANGELOG.
## Phase 2: azd-core rebase

Each task must land with all three extensions still building against the prior `azd-core` tag.

| # | Repo | Task | Depends on |
|---|---|---|---|
| [x] 2.0 | C | Add `github.com/azure/azure-dev/cli/azd v1.29.0` to `go.mod` | 0.1 |
| [x] 2.1 | C | Delete `procutil`. Callers move to `azdext.IsProcessRunning` / `GetProcessInfo` | 2.0 |
| [x] 2.2 | C | Delete `shellutil`, but **not** for the reason originally stated. `azdext.DetectShell()` answers a different question (which shell the user is in) than `shellutil.DetectShell(scriptPath)` (which interpreter runs this script file), so it is not a replacement. `shellutil` is deleted because it has zero callers in azd-core and all three extensions. `cmdutil.GetDefaultShell` / `prepareHookCommand` are **kept** for the same non-equivalence reason. See `migration.md` | 2.0 |
| [x] 2.3 | C | **Trim** `pathutil` rather than delete it. Only `FindToolInPath` has an SDK equivalent, and moving to `azdext.LookupTool` fixes a real Windows bug (the old helper appended `.exe`, so `npm`, `pnpm`, `az`, and `func` were reported missing). `SearchToolInSystemPath`, `GetInstallSuggestion`, and `RefreshPATH` have no SDK equivalent and are kept. See `migration.md` | 2.0 |
| 2.4 | [x] C | Delete `azdextutil`. **Verified: zero callers** in azd-core and all three extensions, and the package was already deprecated in v0.5.3, so nothing migrates. The plan's stated replacements do not hold on inspection: `azdext.ValidateScriptName` is a shell-metacharacter denylist for script filenames, not a shell-name allowlist; `MCPServerBuilder.WithRateLimit` is bound to a builder and is not a standalone limiter; `azdext.GetProjectDir` fails with `ErrProjectNotFound` where the azd-core version fell back to the working directory. Recorded in `migration.md`. | 2.0 |
| 2.5 | [x] C | Keep `version`. `NewCommand` now calls `azdext.RegisterFlagOptions` so azd gets completion, help text, metadata, and parse-time validation for the output flag, which is the one thing `azdext.NewVersionCommand` did better. Signature note: `RegisterFlagOptions` takes two arguments, not three. azd-rest moved back onto `version.NewCommand`, recovering `--quiet`, `buildDate`, and `gitCommit`. Two latent defects surfaced doing it, both fixed: `NewCommand` hardcoded the `-q` shorthand, which panics in any extension that already binds `-q` (azd-rest uses it for `--query`), and it hardcoded the flag name `output`, which is wrong for an extension whose output is driven by its own flag. Both are now `Option` values. | 2.0 |
| 2.6 | [x] C | **Keep `urlutil`.** Verified: it is not an SSRF control and `azdext.SSRFGuard` is not a URL validator, so the two are layered rather than interchangeable. `urlutil` is syntactic and offline; `SSRFGuard` resolves hosts and blocks private ranges. Its only consumer is azd-app's service config validation, where the URLs are local dev endpoints that `SSRFGuard` would reject by design, and where DNS lookups during config parsing would be wrong. `ValidateDomain` was also checked against `azdext.ValidateHostname`: the label rules match, but `ValidateDomain` additionally rejects a port and requires at least one dot, and reports which character failed. Delegating would loosen validation and degrade the message. SSRF adoption belongs in task 2.10 (`httpclient`), where outbound requests are actually made. | 2.0 |
| 2.7 | [x] C | **Trimmed `fileutil` internals, kept the exported API.** `atomicWrite` and `renameWithRetry` deleted; `AtomicWriteJSON`, `AtomicWriteFile`, and `EnsureDir` now delegate to `azdext.WriteFileAtomic` / `azdext.EnsureDir`. The plan also said to delete `AtomicWriteFile` and `EnsureDir`, which verification rejected: `fileutil.DirPermission` is `0750` and `azdext.EnsureDir` falls back to `0755`, so deleting the wrapper would have loosened directory permissions at 24 call sites across three repos, each of which would have had to remember the policy. Keeping one-line wrappers that carry a security-relevant default is cheaper and safer than spreading it. Net win is real: the SDK's `osutil.Rename` retries the actual Windows lock errors (`ERROR_SHARING_VIOLATION`, `ERROR_ACCESS_DENIED`) for up to 10s, where `renameWithRetry` retried *any* error for 200ms, and `WriteFileAtomic` handles `perm == 0` instead of chmod'ing the file to `0000`. Whole existing suite passes unchanged. fileutil 89.0% -> 95.5%. | 2.0 |
| 2.8 | C | [x] Trim `security`. **Plan claim refuted.** The SDK has no standalone `ValidatePath` (~70 call sites), `ValidatePathWithinBases` (returns a resolved path `MCPSecurityPolicy.CheckPath` does not), `ValidatePackageManager`, or `ValidateFilePermissions`. Only 3 of 7 functions could delegate: `ValidateServiceName` (format rule only, keeps `allowEmpty`, the length message, the sentinel, and the `..` check the SDK regex accepts), `SanitizeScriptName` renamed to `ValidateScriptName`, and `IsContainerEnvironment` | 2.6 |
| 2.9 | C | [x] Rebase `keyvault` internals on `azdext.KeyVaultResolver`. Kept the warning-collecting façade. **One gap the plan missed:** `azdext.ParseSecretReference` accepts only the three segment `akvs://` form, but azd-core documents a four segment form carrying a version, so `normalizeReference` rewrites it into the equivalent `@Microsoft.KeyVault(VaultName=...;SecretName=...;SecretVersion=...)` reference | 2.0 |
| 2.10 | C | [x] **Keep `httpclient.Client` and `handlePagination`; harvest the SDK behavior they were missing.** Verified: neither `azdext.ResilientClient` nor `azdext.Pager` can express what azd-rest needs. `ResilientClient.Do` has no per-request headers, no TLS opt out, no redirect policy, no response size limit and no verbose logging, it takes an `azcore.TokenCredential` rather than azd-core's scope-string `TokenProvider`, and it derives the scope from the URL instead of honoring `--scope`. `azdext.Pager[T]` only reads `nextLink`, so adopting it would delete `@odata.nextLink` (Microsoft Graph), `@odata.next`, and RFC 5988 `Link` header pagination, and its `HTTPDoer` cannot carry the auth or custom headers a page request needs. What was adopted: `azdext.SSRFSafeRedirect` on the redirect path (the redirect target is chosen by the server, so it belongs under an SSRF policy), plus retry on 429 and 408, `Retry-After` support, jittered backoff, and a bounded body drain before each retry | 2.0 |
| 2.11 | C | Reduce `auth` to a host-aware selector: `azdext.TokenProvider` when a host is present, `azidentity` chain otherwise. Delete `DetectScope` / `IsAzureHost` in favor of `azdext.ScopeDetector` | 2.10 |
| 2.12 | C | Replace `logutil` internals with `azdext.Logger`. Keep signatures, mark deprecated | 2.0 |
| 2.13 | C | Delegate `cliout.PrintJSON` / `Table` to `azdext.Output`, and color/prompt gating to `azdext.DetectInteractive()`. Keep the rest of the `cliout` API | 2.0 |
| 2.14 | C | Switch `env.LoadAzdEnvironment` to `azdext.LoadAzdEnvironment`. Add an explicit `LoadAzdEnvironmentFromSubprocess` for detached children only | 2.0 |
| 2.15 | C | Release `azd-core v0.6.0`, copying the accumulated `docs/specs/azdext-alignment/migration.md` table into `CHANGELOG.md` | 2.1-2.14 |
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
