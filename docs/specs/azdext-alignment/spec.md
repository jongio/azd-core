# azdext Alignment

Align `azd-core` and every `azd-*` extension with the current Azure Developer CLI extension framework SDK (`github.com/azure/azure-dev/cli/azd/pkg/azdext`).

## Problem

Our extensions were built against an early version of the extension framework. Since then the SDK has absorbed most of the infrastructure we hand-rolled. The result is a parallel SDK (`azd-core`, 27,608 Go LOC across 27 packages) that reimplements auth, HTTP retry, pagination, Key Vault resolution, SSRF guarding, shell detection, tool discovery, atomic file writes, structured logging, process inspection, and azd environment loading. All of these now ship in `azdext`.

Three concrete symptoms:

1. **No structured error telemetry.** No extension calls `azdext.Run()`, so no extension reports errors to the azd host via `ExtensionService.ReportError`. All three hand-roll `main()` error printing and exit codes. We get `ext.run.failed` for every failure instead of classified `ext.<category>.<code>` telemetry.
2. **Independent authentication.** `azd-core/auth` builds its own `azidentity` credential chain instead of using `azdext.TokenProvider`, which brokers tokens through the azd host session. Users can be logged into `azd` and still hit a credential prompt from our extensions.
3. **Shelling out to azd.** `azd-core/env.LoadAzdEnvironment` runs `azd env get-values` as a subprocess. `azdext.LoadAzdEnvironment` reads the values already injected into the process. The subprocess approach is the documented cause of the detached-child hang worked around in `azd-app/cli/src/cmd/app/main.go:44-56`.

A prior analysis (`azd-core/EXTENSION_FRAMEWORK_GAP_ANALYSIS.md`, dated 2025-06-01) recommended building listen-command factories, MCP scaffolding, and rate limiters into `azd-core`. Upstream shipped all of those in PR #6856. That document is now actively misleading and should be archived.

## Current state

| Repo | Module | azd SDK pin | azd-core pin | Capabilities declared |
|---|---|---|---|---|
| azd-core | `github.com/jongio/azd-core` | **none** | n/a | n/a (library) |
| azd-app | `github.com/jongio/azd-app/cli` | v1.28.1 | v0.5.7 | custom-commands, lifecycle-events, mcp-server, service-target-provider, metadata |
| azd-copilot | `github.com/jongio/azd-copilot/cli` | v1.28.1 | v0.5.7 | custom-commands, lifecycle-events, mcp-server, metadata |
| azd-rest | `github.com/jongio/azd-rest` | v1.28.1 | v0.5.7 | custom-commands, lifecycle-events, mcp-server, metadata |
| azd-web-core | npm package | n/a | n/a | not an extension |
| azd-app-demo | sample app | n/a | n/a | not an extension |

Latest published SDK module tag: `cli/azd/v1.29.0`. Local `azure-dev` main is at `1.30.0-beta.1`.

### What we already use correctly

`azdext.NewExtensionRootCommand`, `NewListenCommand`, `NewMetadataCommand` (all three repos), `NewVersionCommand` (azd-rest), `NewMCPServerBuilder` + `ParseToolArgs` + `MCPJSONResult`/`MCPErrorResult` (all three), `DefaultMCPSecurityPolicy` (azd-rest only), `ServiceTargetProvider` (azd-app).

### What we are missing

Not a single call site exists for: `azdext.Run`, `LocalError`, `ServiceError`, `ReportError`, `TokenProvider`, `ResilientClient`, `NewPager`, `KeyVaultResolver`, `SSRFGuard`, `ScopeDetector`, `Logger`, `Output`, `ConfigHelper`, `LoadAzdEnvironment`, `DetectShell`, `ShellCommand`, `LookupTool`, `RequireTools`, `WriteFileAtomic`, `DetectInteractive`, `BaseServiceTargetProvider`, `ValidateNoReservedFlagConflicts`, `VerifyProvidersMatchManifest`, `WaitForDebugger`, `WithProvisioningProvider`, `WithValidationCheck`, `client.Copilot()`, `client.Ai()`, `client.Container()`.

## Duplication matrix

Verified by symbol extraction from `cli/azd/pkg/azdext/*.go` and `azd-core/*/*.go`.

### Tier 1: replace outright (azdext is strictly better)

| azdext API | azd-core duplicate | Why azdext wins |
|---|---|---|
| `Run`, `WrapError`, `ReportError`, `LocalError`, `ServiceError`, `LocalErrorCategory` | hand-rolled `main()`, `cmd.ExitCoder` | Structured telemetry classification, suggestion rendering, error links, standard exit codes |
| `Pager[T]`, `PageResponse[T]`, `PaginationError` | `httpclient.handlePagination`, `parseLinkHeader`, `extractNextLinkFromBody` | Generic, typed, URL redaction, control-char sanitization |
| `KeyVaultResolver`, `IsSecretReference`, `ParseSecretReference`, `ResolveMap`, `ResolveEnvironment` | `keyvault.KeyVaultResolver`, `IsKeyVaultReference`, `env.Resolve*` | Same feature set, maintained upstream, typed `KeyVaultResolveError` with `ResolveReason` |
| `SSRFGuard`, `MCPSecurityPolicy`, `SSRFSafeRedirect` | `security.ValidatePath*`, `urlutil.Validate*` | Covers CGNAT (RFC 6598), IPv6 transition (6to4/Teredo/NAT64), symlink resolution, redirect-time re-validation, header redaction. Ours covers none of these. |
| `ValidateServiceName`, `ValidateHostname`, `ValidateScriptName`, `IsContainerEnvironment`, `ContainerRuntime` | `security.ValidateServiceName`, `SanitizeScriptName`, `IsContainerEnvironment`, `azdextutil.ValidateShellName` | Shared regex definitions with the host, so validation cannot drift |
| `DetectShell`, `ShellCommand`, `ShellCommandWith`, `ExecCommand` | `shellutil.DetectShell`, `cmdutil.GetDefaultShell`, `cmdutil.prepareHookCommand` | Handles `PSModulePath`/`ComSpec` detection and injectable `ShellInfo` for tests |
| `LookupTool`, `LookupTools`, `RequireTools`, `ToolsNotFoundError`, `PrependPATH`, `AppendPATH`, `PATHContains` | `pathutil.FindToolInPath`, `SearchToolInSystemPath`, `RefreshPATH`, `GetInstallSuggestion` | Finds project-local wrappers (`./mvnw`, `./gradlew`), typed missing-tool error |
| `WriteFileAtomic`, `CopyFileAtomic`, `BackupFile`, `EnsureDir` | `fileutil.atomicWrite`, `renameWithRetry`, `AtomicWriteFile`, `EnsureDir` | Identical semantics including Windows rename retry |
| `IsProcessRunning`, `GetProcessInfo`, `CurrentProcessInfo`, `ParentProcessInfo`, `FindProcessByName`, `GetProcessEnvironment` | `procutil.IsProcessRunning` | Superset, no `gopsutil` dependency |
| `ScopeDetector.ScopesForURL` | `auth.DetectScope`, `auth.IsAzureHost` | Rule-driven and extensible |
| `GetProjectDir`, `FindFileUpward`, `ErrProjectNotFound` | `azdextutil.GetProjectDir(envVar)` | Upward `azure.yaml` search instead of env-var-only |
| `MCPServerBuilder.WithRateLimit` | `azdextutil.RateLimiter` | Already wired into every tool handler by the builder |
| `NewVersionCommand` | `version.NewCommand` | Identical, plus JSON output |
| `Logger`, `SetupLogging` | `logutil.ComponentLogger` and package globals | Component/operation tagging, no global mutable state |

### Tier 2: replace with fallback (azdext requires a live azd host)

These need a host connection (`AZD_SERVER` + `AZD_ACCESS_TOKEN`). Some of our code paths run detached from the host, so a fallback must stay.

| azdext API | azd-core duplicate | Decision |
|---|---|---|
| `TokenProvider` | `auth.AzureTokenProvider` | Prefer `azdext.TokenProvider` when a host is present. Keep the `azidentity` chain as an explicit documented fallback behind one selector function. |
| `LoadAzdEnvironment`, `ParseEnvironmentVariables` | `env.LoadAzdEnvironment` (shells out to `azd env get-values`) | Prefer the SDK. Keep the subprocess path only for the detached-child case, and delete the workaround in `azd-app` once the SDK path is in. |
| `ConfigHelper` | `azd-app/cli/src/internal/azdconfig` | Replace outright. This code only runs under a host. |
| `ResilientClient` | `httpclient.Client` | Rebase on `ResilientClient`. Keep the `gobreaker` circuit breaker as a thin decorator only if a consumer actually needs it, otherwise delete. |

### Tier 3: keep in azd-core (no azdext equivalent)

`cliout` (rich presentation: headers, sections, icons, hints, progress bars, colored labels), `progress` (multi-task spinners), `healthcheck` (+ Prometheus metrics), `notify` (desktop notifications), `browser`, `editor`, `projecttype` (language and framework detection), `yamlutil`, `registry`, `copilotskills`, `cache` (content-hash caching), `cmdutil` hook execution with output monitoring, `testutil`.

`cliout` should keep its own presentation API but delegate JSON and table rendering to `azdext.Output`, and delegate its color and prompt gating to `azdext.DetectInteractive()`.

### Tier 4: adopt (new capability, nothing to replace)

| azdext API | Value |
|---|---|
| `ValidateNoReservedFlagConflicts` | Catches flag collisions with azd's reserved globals (`-e`, `-C`, `-o`, `--debug`, ...) at startup instead of at user-report time |
| `VerifyProvidersMatchManifest` | Unit test that `extension.yaml` `providers:` matches what the host actually registers. Pattern is in `microsoft.azd.demo/internal/cmd/providers_manifest_test.go` |
| `WaitForDebugger` | Attach a debugger to a spawned extension process |
| `RegisterFlagOptions` | Shell completion and validation for enum-valued flags |
| `GenerateExtensionMetadata` + `extensions.ConfigurationMetadata` | We emit command metadata but no config JSON Schema and no documented env vars. Adding these gives IntelliSense and config validation. |
| `requiredAzdVersion` manifest field | Replaces azd-rest's non-schema `minAzdVersion: 1.10.0`, which azd ignores |
| `WithValidationCheck` (`validation-provider`) | Contribute checks to `azd provision` validation |
| `WithProvisioningProvider` (`provisioning-provider`) | Custom provisioning, relevant to azd-app |
| `client.Copilot()` | azd-copilot currently shells out to the `copilot` CLI. The host exposes sessions, usage metrics, and file changes over gRPC. |
| `client.Ai()`, `client.Container()`, `client.Compose()`, `client.Workflow()` | Unused or under-used services |
| Extension pack manifest (`dependencies`, no `capabilities`) | Publish a `jongio.azd` pack so users install all extensions with one command |

## Target architecture

```
azdext (upstream SDK)          <- infrastructure: auth, http, security, shell, files, logging, config, errors
   ^
   |  depends on
azd-core (shared extras)       <- presentation (cliout, progress), domain (projecttype, registry,
   ^                              healthcheck, copilotskills, cache, notify, editor, browser, yamlutil)
   |  depends on
azd-app / azd-copilot / azd-rest
```

Rules:

1. `azd-core` gains a direct dependency on `github.com/azure/azure-dev/cli/azd`.
2. `azd-core` never reimplements something `azdext` provides. It may wrap for ergonomics, but the wrapper must add behavior, not just delegate.
3. Extensions import `azdext` directly for anything `azdext` provides. They do not route through `azd-core` for it.
4. Every extension entry point is `azdext.Run(rootCmd)`.
5. Every user-facing failure is a `LocalError` or `ServiceError` with a category, a `snake_case` code, and a suggestion.

Expected reduction in `azd-core`: roughly 8,000 to 11,000 LOC removed across `auth`, `httpclient`, `keyvault`, `security`, `shellutil`, `procutil`, `fileutil`, `logutil`, `pathutil`, `azdextutil`, `env`, `version`, plus their tests.

## Plan

### Phase 0: baseline (blocking, do first)

- Upgrade all three extensions to `github.com/azure/azure-dev/cli/azd v1.29.0` and run `go mod tidy`. Confirm green build and tests before changing any code. This isolates SDK-upgrade breakage from refactor breakage.
- Archive `azd-core/EXTENSION_FRAMEWORK_GAP_ANALYSIS.md` to `docs/archive/` with a note that upstream PR #6856 superseded it.
- Add a `docs/` note pinning the SDK upgrade policy: track the latest stable `cli/azd/vX.Y.Z` tag, never a pseudo-version.

### Phase 1: error handling and entry points (highest value, lowest risk)

Per extension, in this order: azd-rest (smallest, 10.4k LOC), azd-copilot (6.9k), azd-app (157k).

- Replace `main()` with `azdext.Run(rootCmd)`. Delete hand-rolled error printing, `ExitCoder`, and `os.Exit` paths.
- Introduce a per-extension `errors.go` with `const` error codes.
- Convert user-facing failures at the command-handler layer to `azdext.LocalError` (category `validation`, `auth`, `dependency`, `compatibility`, `user`, `internal`) or `azdext.ServiceError` for Azure API failures. Leave lower-level helpers returning wrapped plain errors.
- Add `azdext.ValidateNoReservedFlagConflicts(rootCmd)` to a startup test in each repo.
- Add `azdext.NewVersionCommand` to azd-app and azd-copilot (azd-rest already has it).

### Phase 2: azd-core rebase onto azdext

Order matters: each step must land with all three extensions still building against the previous `azd-core` tag.

1. Add the `azdext` dependency to `azd-core/go.mod`.
2. **Delete and forward:** `procutil`, `shellutil`, `pathutil`, `azdextutil`, `version`, `urlutil`. Replace call sites in extensions with direct `azdext` calls. These are small and have few consumers.
3. **`fileutil`:** delete `atomicWrite`, `renameWithRetry`, `AtomicWriteFile`, `EnsureDir`. Keep the project-detection predicates (`FileExistsAny`, `HasAnyFileWithExts`, `ContainsTextInFile`) and the JSON cache helpers.
4. **`security`:** delete everything except anything not covered by `azdext.ValidateServiceName`/`ValidateScriptName`/`SSRFGuard`. Audit `ValidateFilePermissions` and `ValidatePackageManager`, which have no azdext equivalent, and keep only those.
5. **`keyvault` + `env` secret resolution:** replace the resolver internals with `azdext.KeyVaultResolver`. Keep `env.Resolve*` as the batch/warning-collecting façade if consumers depend on the warning shape.
6. **`httpclient`:** rebase `Client` on `azdext.ResilientClient` and `azdext.Pager`. Keep `Formatter`, `RedactSensitiveHeader`, `RedactURL`, `DetectContentType`.
7. **`auth`:** reduce to a selector that returns `azdext.TokenProvider` when a host is present and the `azidentity` chain otherwise. Delete `DetectScope`/`IsAzureHost` in favor of `azdext.ScopeDetector`.
8. **`logutil`:** replace the implementation with `azdext.Logger`. Keep the package name and function signatures for one minor release, marked deprecated, then delete.
9. **`cliout`:** keep the API. Delegate `PrintJSON`/`Table` to `azdext.Output` and color/prompt gating to `azdext.DetectInteractive()`.
10. **`env.LoadAzdEnvironment`:** switch to `azdext.LoadAzdEnvironment`. Keep the subprocess path behind an explicit `FromSubprocess` variant used only by detached children.

Release `azd-core v0.6.0` as a breaking change with a migration table in `CHANGELOG.md`.

### Phase 3: per-extension capability uplift

**azd-app**
- Remove the detached-child environment workaround in `main.go` once `azdext.LoadAzdEnvironment` is in place. Verify the original hang does not return.
- Replace `internal/azdconfig` with `azdext.ConfigHelper`.
- Embed `azdext.BaseServiceTargetProvider` in `internal/servicetarget/local_provider.go` and delete no-op methods.
- Add `providers_manifest_test.go` using `azdext.VerifyProvidersMatchManifest`.
- Evaluate `WithProvisioningProvider` for local orchestration, and `WithValidationCheck` for pre-provision project validation.
- Add MCP security policy (currently only azd-rest has one) and audit MCP tools against `azdext.MCPSecurityPolicy`.
- Add configuration JSON Schema to the metadata command for `azure.yaml` app settings.

**azd-copilot**
- Evaluate replacing the shelled-out `copilot` CLI launcher with `client.Copilot()` for non-interactive paths (`--prompt`, autopilot, MCP tools). Keep the subprocess path for interactive TTY sessions. Measure before committing: the gRPC path gives session reuse, usage metrics, and file-change tracking for free.
- Use `client.Ai()` for model and quota lookups instead of custom code.
- Add MCP security policy.
- Add `requiredAzdVersion` to the manifest.

**azd-rest**
- Delete custom pagination, retry, and token caching in `internal/client` and `internal/cmd/mcp.go` in favor of `azdext.Pager`, `ResilientClient`, and `TokenProvider`.
- Replace `minAzdVersion: 1.10.0` (not a schema field, silently ignored) with `requiredAzdVersion: ">= 1.29.0"`.
- Use `azdext.ScopeDetector` instead of `azd-core/auth.DetectScope`.
- Keep the existing `MCPSecurityPolicy` and extend it with `ValidatePathsWithinBase` for any file-reading tools.

### Phase 4: distribution and consistency

- Publish `jongio.azd` as an extension pack with `dependencies` on `jongio.azd.app`, `jongio.azd.copilot`, `jongio.azd.rest`.
- Standardize on `azd x build` / `azd x pack` / `azd x publish` / `azd x release` in all three magefiles. azd-rest already does this; azd-app and azd-copilot are inconsistent.
- Align `go` directives: azd-rest is on `go 1.26.4`, the others on `1.26.5`.
- Add `azdext.GenerateExtensionMetadata` configuration schemas and `EnvironmentVariables` documentation to all three metadata commands.
- Submit all three to the azd dev registry (`registry.dev.json`) so users can `azd extension install --source dev`.

## Risks

| Risk | Mitigation |
|---|---|
| `azdext.TokenProvider` requires a live host; some azd-app paths run detached | Tier 2 selector with documented fallback. Integration test both paths. |
| Adding `azdext` to `azd-core` pulls in gRPC and a large dependency tree | Every consumer already depends on it. Verify no binary size regression beyond 10 percent. |
| azd-app is 157k LOC; a wide refactor risks regressions | Phase 1 first (entry point and errors only), then package-by-package in Phase 2 order. Each step is independently shippable. |
| `azd-core v0.6.0` is breaking for external consumers | There are no external consumers. Publish a migration table anyway. |
| SDK 1.29 to 1.30-beta drift | Pin to the latest stable tag only. Do not track `main`. |
| Copilot gRPC path may be slower or less capable than the CLI subprocess | Prototype and measure before removing the subprocess path. Do not delete the fallback. |

## Quality gates

- Zero build errors and zero warnings in all four Go repos.
- 100 percent test pass. No skipped tests introduced.
- Coverage on modified packages at or above the pre-change baseline, minimum 80 percent for new code.
- `azdext.ValidateNoReservedFlagConflicts` passes in each extension.
- `azdext.VerifyProvidersMatchManifest` passes wherever providers are declared.
- End-to-end smoke: `azd app run`, `azd rest get`, `azd copilot -p "..."` against a real azd host on Windows, Linux, and macOS.
- Binary size regression under 10 percent per extension.

## Done definition

1. All three extensions pin `github.com/azure/azure-dev/cli/azd v1.29.0` or later, no pseudo-versions.
2. All three entry points are `azdext.Run`.
3. Structured errors with categories, codes, and suggestions on every user-facing failure path.
4. `azd-core` depends on `azdext` and contains no reimplementation of a Tier 1 API.
5. Every Tier 1 duplicate is deleted, not merely deprecated.
6. `azd-app`'s detached-child environment workaround is removed and verified.
7. `azd-rest` uses SDK pagination, retry, and token providers.
8. All three manifests declare `requiredAzdVersion` and pass schema validation.
9. Metadata commands emit configuration schemas and environment variable docs.
10. `EXTENSION_FRAMEWORK_GAP_ANALYSIS.md` is archived.
11. All quality gates green.

## Open questions

Resolve before Phase 3 starts.

1. Should `azd-copilot` move to `client.Copilot()` for non-interactive paths, or keep the subprocess launcher everywhere? Needs a prototype and a latency measurement.
2. Is the `gobreaker` circuit breaker in `azd-core/httpclient` used by any consumer? If not, delete it rather than layering it on `ResilientClient`.
3. Should `azd-core` keep a standalone (no-host) mode as a supported contract, or declare azd-host-required and delete every fallback? This determines whether Tier 2 exists at all.
4. Do we publish `azd-core` docs as a stable public API, or mark it internal to the `jongio.azd` family so we can break it freely?
