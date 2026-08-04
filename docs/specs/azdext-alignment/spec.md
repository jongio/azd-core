# azdext Alignment

Align `azd-core` and every `azd-*` extension with the current Azure Developer CLI extension framework SDK (`github.com/azure/azure-dev/cli/azd/pkg/azdext`).

## Problem

Our extensions were built against an early version of the extension framework. Since then the SDK has absorbed most of the infrastructure we hand-rolled. The result is a parallel SDK (`azd-core`, 27,608 Go LOC across 27 packages) that reimplements auth, HTTP retry, pagination, Key Vault resolution, SSRF guarding, shell detection, tool discovery, atomic file writes, structured logging, process inspection, and azd environment loading. All of these now ship in `azdext`.

Three concrete symptoms:

1. **No structured error telemetry.** No extension calls `azdext.Run()`, so no extension reports errors to the azd host via `ExtensionService.ReportError`. All three hand-roll `main()` error printing and exit codes. We get `ext.run.failed` for every failure instead of classified `ext.<category>.<code>` telemetry.
2. **Independent authentication.** `azd-core/auth` builds its own `azidentity` credential chain instead of using `azdext.TokenProvider`, which brokers tokens through the azd host session. Users can be logged into `azd` and still hit a credential prompt from our extensions.
3. **Shelling out to azd.** `azd-core/env.LoadAzdEnvironment` runs `azd env get-values` as a subprocess. `azdext.LoadAzdEnvironment` reads the values already injected into the process. The subprocess approach is the documented cause of the detached-child hang worked around in `azd-app/cli/src/cmd/app/main.go:44-56`.

A prior analysis (now archived at `azd-core/docs/archive/extension-framework-gap-analysis.md`, dated 2025-06-01) recommended building listen-command factories, MCP scaffolding, and rate limiters into `azd-core`. Upstream shipped all of those in PR #6856. That document is now actively misleading, so it carries a superseded-by banner and must not be used to plan work.

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
| `SSRFGuard`, `MCPSecurityPolicy`, `SSRFSafeRedirect` | `security.ValidatePath*` (partial); **not** `urlutil.Validate*` | Covers CGNAT (RFC 6598), IPv6 transition (6to4/Teredo/NAT64), symlink resolution, redirect-time re-validation, header redaction. Ours covers none of these. **Corrected in task 2.6:** `SSRFGuard` is not a replacement for `urlutil`. `urlutil` is syntactic and offline; `SSRFGuard` resolves the host and blocks private ranges, which would reject the local dev URLs `urlutil` exists to validate. Layer them, do not swap them. Adopt `SSRFGuard` in `httpclient` (task 2.10), at the point of outbound request. |
| `ValidateServiceName`, `ValidateHostname`, `ValidateScriptName`, `IsContainerEnvironment`, `ContainerRuntime` | `security.ValidateServiceName`, `SanitizeScriptName`, `IsContainerEnvironment` | Shared regex definitions with the host, so validation cannot drift. Note: `azdext.ValidateScriptName` is **not** a replacement for `azdextutil.ValidateShellName`. The SDK function rejects shell metacharacters in a script filename; the azd-core function checked a shell name against an allowlist of six. Different inputs, different purpose. |
| `DetectShell`, `ShellCommand`, `ShellCommandWith`, `ExecCommand` | (no azd-core equivalent) | Detects the **user's current interactive shell** from `SHELL`/`PSModulePath`/`ComSpec`, with injectable `ShellInfo` for tests. **This is not the same problem `shellutil` or `cmdutil` solve.** `shellutil.DetectShell(scriptPath)` picks an **interpreter for a script file** from its extension and shebang, so it can return `python3` or `node`; the SDK never reads the file and cannot. `cmdutil.GetDefaultShell()` picks the **best available** shell by `LookPath` (pwsh on Windows, bash on Unix) so hook scripts get a predictable interpreter; the SDK would return `fish` for a fish user and break bash hooks. `cmdutil.prepareHookCommand` additionally handles `-File` for `.ps1` paths and a UTF-8 console preamble that `ShellCommandWith` has no equivalent for. See task 2.2. |
| `LookupTool`, `LookupTools`, `RequireTools`, `ToolsNotFoundError`, `PrependPATH`, `AppendPATH`, `PATHContains` | `pathutil.FindToolInPath` **only** | `LookupTool` finds project-local wrappers (`./mvnw`, `./gradlew`) and gives a typed missing-tool error. It is also a **bug fix**: `pathutil.FindToolInPath` appended `.exe` on Windows before calling `exec.LookPath`, so it failed to find `.cmd` shims. Verified on Windows: `npm`, `pnpm`, `az`, and `func` all resolve with a plain lookup and all fail with the `.exe` suffix. The SDK does **not** replace `SearchToolInSystemPath` (well-known install dirs off PATH), `GetInstallSuggestion` (curated install URLs), or `RefreshPATH` (re-reads Machine and User PATH from the Windows registry). Those three stay. See task 2.3. |
| `WriteFileAtomic`, `CopyFileAtomic`, `BackupFile`, `EnsureDir` | `fileutil.atomicWrite`, `renameWithRetry`, `AtomicWriteFile`, `EnsureDir` | Identical semantics including Windows rename retry |
| `IsProcessRunning`, `GetProcessInfo`, `CurrentProcessInfo`, `ParentProcessInfo`, `FindProcessByName`, `GetProcessEnvironment` | `procutil.IsProcessRunning` | Superset, no `gopsutil` dependency |
| `ScopeDetector.ScopesForURL` | `auth.DetectScope`, `auth.IsAzureHost` | Rule-driven and extensible |
| `GetProjectDir`, `FindFileUpward`, `ErrProjectNotFound` | `azdextutil.GetProjectDir(envVar)` | Upward `azure.yaml` search instead of env-var-only. Contract differs: the azd-core version always returns a directory (falling back to the working directory), the SDK version returns `ErrProjectNotFound` when there is no azd project. |
| `RegisterFlagOptions` | none, this is new | Declares supported flag values once and gets completion, help text, extension metadata, and parse-time validation from it. Adopted by `version.NewCommand`. |
| `MCPServerBuilder.WithRateLimit` | `azdextutil.RateLimiter` | Already wired into every tool handler by the builder. The SDK has no standalone limiter type, so this only helps code that is already using `MCPServerBuilder`. |
| `NewVersionCommand` | `version.NewCommand` | Not a replacement. The SDK command emits only `{name, version}` in JSON, prints `id version` as text, and has no `--quiet`. Ours emits the full `Info` (version, buildDate, gitCommit, extensionId, name) and has `--quiet`/`-q`. The SDK's only advantage is that it calls `RegisterFlagOptions` so azd can discover `--output json`. See task 2.5 |
| `Logger`, `SetupLogging` | `logutil.ComponentLogger` and package globals | Component/operation tagging, no global mutable state |

### Tier 2: replace with fallback (azdext requires a live azd host)

These need a host connection (`AZD_SERVER` + `AZD_ACCESS_TOKEN`). Some of our code paths run detached from the host, so a fallback must stay.

| azdext API | azd-core duplicate | Decision |
|---|---|---|
| `TokenProvider` | `auth.AzureTokenProvider` | Prefer `azdext.TokenProvider` when a host is present. Keep the `azidentity` chain as an explicit documented fallback behind one selector function. |
| `LoadAzdEnvironment`, `ParseEnvironmentVariables` | `env.LoadAzdEnvironment` (shells out to `azd env get-values`) | Prefer the SDK. Keep the subprocess path only for the detached-child case, and delete the workaround in `azd-app` once the SDK path is in. |
| `ConfigHelper` | `azd-app/cli/src/internal/azdconfig` | Replace outright. This code only runs under a host. |
| `ResilientClient` | `httpclient.Client` | Not equivalent. Harvest the retry and redirect behavior instead of rebasing; see task 2.10. `gobreaker` is not in `httpclient` at all, it belongs to `healthcheck`. |

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
- Expect that test to fail somewhere. In azd-app it found five subcommands whose local flags shadowed azd reserved globals, which broke `--output` and `-e` silently and would have stopped `azdext.Run` from starting at all. Fixing it is a breaking flag rename, so budget for docs and a CHANGELOG entry.
- Do not adopt `azdext.NewVersionCommand`. It is a regression against azd-core's `version.NewCommand`, so azd-app and azd-copilot keep the azd-core command and azd-rest moves back onto it in phase 2. See task 2.5.

### Phase 2: azd-core rebase onto azdext

Order matters: each step must land with all three extensions still building against the previous `azd-core` tag.

1. Add the `azdext` dependency to `azd-core/go.mod`.
2. **Delete and forward:** `procutil` and `azdextutil` were deleted outright. `shellutil`, `pathutil`, and `urlutil` were **kept** after the claimed equivalences failed verification (tasks 2.2, 2.3, 2.6); only `pathutil.FindToolInPath` was removed. `version` stays and instead gains the SDK's `RegisterFlagOptions` call, plus caller options for the pieces a shared library cannot know (task 2.5). Replace call sites in extensions with direct `azdext` calls only where the semantics actually match.
3. **`fileutil`:** delete the private `atomicWrite` and `renameWithRetry` and delegate `AtomicWriteJSON`, `AtomicWriteFile`, and `EnsureDir` to `azdext.WriteFileAtomic` / `azdext.EnsureDir`. **Corrected in task 2.7:** the exported wrappers are retained, not deleted, because `fileutil.DirPermission` is `0750` while `azdext.EnsureDir` defaults to `0755`; the wrapper is where that policy lives. Keep the project-detection predicates (`FileExistsAny`, `HasAnyFileWithExts`, `ContainsTextInFile`) and the JSON cache helpers.
4. **`security`:** delegate where a true equivalent exists and keep the rest. **Corrected in task 2.8:** the SDK has no standalone equivalent for `ValidatePath`, `ValidatePathWithinBases`, `ValidatePackageManager`, or `ValidateFilePermissions`, so all four are retained. `MCPSecurityPolicy.CheckPath` is a method on a configured policy builder and returns only an error, never the resolved path that `ValidatePathWithinBases` callers need. Three functions delegate: `ValidateServiceName` forwards the format rule to `azdext.ValidateServiceName` (identical regex) while keeping `allowEmpty`, the length message, the `ErrInvalidServiceName` sentinel, and an explicit `..` traversal check the SDK regex accepts; `SanitizeScriptName` is renamed to `ValidateScriptName` and forwards to `azdext.ValidateScriptName`; `IsContainerEnvironment` forwards to `azdext.IsContainerEnvironment`.
5. **`keyvault` + `env` secret resolution:** replace the resolver internals with `azdext.KeyVaultResolver`. Keep `env.Resolve*` as the batch/warning-collecting façade if consumers depend on the warning shape. **Refined in task 2.9:** `azdext.ParseSecretReference` accepts only the three segment `akvs://<subscription>/<vault>/<secret>` form. azd-core has documented a four segment form with a trailing version since v0.1, in the README, the spec, `env/doc.go`, and azd-app's user docs, so the package keeps a `normalizeReference` step that rewrites the four segment form into the `@Microsoft.KeyVault(VaultName=...;SecretName=...;SecretVersion=...)` reference `azdext` does parse. Everything else, including all three parsers, the vault host allowlist, per-vault client caching, and error classification, comes from the SDK.
6. **`httpclient`:** **Corrected in task 2.10.** `Client` is not rebased on `azdext.ResilientClient`, and `handlePagination` is not rebased on `azdext.Pager`. Both SDK types are built for an extension calling a known Azure endpoint, while `httpclient` backs `azd rest`, a general purpose REST client driven entirely by user flags. `ResilientClient` cannot carry `--header`, `--insecure`, `--follow-redirects`, `--max-redirects`, `--scope`, `--verbose`, or a response size limit, and `Pager` understands only `nextLink`, so it would drop Microsoft Graph's `@odata.nextLink` and the RFC 5988 `Link` header. The SDK behavior that `httpclient` was genuinely missing was harvested instead: `azdext.SSRFSafeRedirect` guards the server chosen redirect target, and the retry loop now retries 429 and 408, honors `Retry-After`, jitters its backoff, and drains the discarded body. `Formatter`, `RedactSensitiveHeader`, `RedactURL`, and `DetectContentType` are unchanged.
7. **`auth`:** layer, do not replace. `DetectScope` keeps its signature and its empty-scope-for-unknown-host contract, resolves Kusto and Service Bus locally because neither fits a static host to scope map, and delegates everything else to `azdext.ScopeDetector` with `CustomRules` carrying the 10 mappings the SDK lacks and the `.azurecr.io` override. `IsAzureHost` stays: it answers a different question than `DetectScope` and has no SDK equivalent. `AzureTokenProvider` stays as the caching and error-classifying front end; `NewAzureTokenProviderForHost` selects `azdext.TokenProvider` when a host is present so the tenant comes from the deployment context.
8. **`logutil`:** layer, do not deprecate. `ComponentLogger` wraps `*azdext.Logger` and exposes it via `AzdextLogger`, but level filtering and the writer stay here because `azdext.LoggerOptions` has only a `Debug` boolean and `azdext.NewLogger` does not inherit `slog.Default`. Setup delegates to `azdext.SetupLogging` where the level allows. Adopt the SDK `AZD_DEBUG` parsing.
9. **`cliout`:** DONE. All 43 exported functions kept. `PrintJSON` and `Table` delegate to `azdext.Output`, with `Table` flattening its `TableRow` (`map[string]string`) rows into the SDK's positional `[][]string` shape. Color gating is seeded from `azdext.DetectInteractive().CanColorize()` and applied at a single print choke point, so `NO_COLOR`, `FORCE_COLOR`, and a redirected stdout are all honored. `Confirm` additionally consults `CanPrompt()`.

   Three defects were found and fixed while doing this, none of which the plan anticipated:

   - `noColor` was initialized to a hardcoded `false` and `getNoColor()` was defined but never called anywhere. `NoColor()` was therefore a no-op and every command leaked ANSI escapes into redirected output.
   - `Table` had no JSON mode check at all, so `--output json` printed a text table into the JSON stream. Delegating to `azdext.Output` fixes this.
   - `Confirm` gated only on `IsJSON()`, so in default format it called `fmt.Scanln` unconditionally and blocked under CI, `AZD_NO_PROMPT`, and AI agent hosts.

   The exported string helpers (`Highlight`, `Emphasize`, `Muted`, `URL`, `Count`, `Status`) intentionally still return ANSI codes. Stripping happens where the package writes, not where it composes, which keeps those helpers pure and their callers free to route the result anywhere.
10. **`env.LoadAzdEnvironment`:** DONE, as a rejected delegation. Nothing changed at runtime; the reasoning is now enforced by tests so a future contributor cannot make the swap silently.

    `azdext.LoadAzdEnvironment(ctx)` runs `azd env get-values` with no `-e` flag. It reads whichever environment azd currently considers default and offers no way to name one. `env.LoadAzdEnvironment(ctx, envName)` exists precisely because azd injects the default environment into the extension process and only then passes `-e` through, so the extension has to reload. Substituting the SDK function would have turned `azd app <cmd> -e staging` into a silent read of the default environment: a correctness bug with a plausible blast radius of writing staging output using production connection strings.

    Three further capabilities would have been lost: the `--output json` path (the KEY=VALUE fallback cannot represent a value containing a newline), the environment name allowlist (the name reaches an `exec` argv), and the `CommandRunner` seam that all eleven existing loader tests depend on.

    A probe also compared `ParseKeyValueFormat` against `azdext.ParseEnvironmentVariables` across twelve input shapes. They agree on ten. They differ on two, and both differences change the value the caller receives: the SDK applies `strings.TrimSpace` to the value (losing significant leading whitespace) and strips only double quotes (so `KEY='v'` keeps its quotes). So that parser is not a drop-in either.

    No `LoadAzdEnvironmentFromSubprocess` was added. The detached-child skip depends on azd-app's own spawn marker environment variable and its once-and-unset semantics, which exist so that services and hooks spawned from `os.Environ()` do not inherit the marker. That is azd-app's detach protocol, not a shared concept, and hoisting it into azd-core would put azd-app internals in a general purpose package for no caller.

    Pinned by `env/azdext_divergence_test.go`. If the SDK later gains `-e` targeting or matches the parser behavior, those tests fail and the delegation can be reconsidered.

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
2. ~~Is the `gobreaker` circuit breaker in `azd-core/httpclient` used by any consumer?~~ **Resolved: the premise was false.** There is no circuit breaker in `httpclient`. `gobreaker/v2` is used throughout `healthcheck` (`checker.go`, `components.go`, `profiles.go`, `types.go`, `metrics.go`) where it is core to the package. The dependency stays.
3. Should `azd-core` keep a standalone (no-host) mode as a supported contract, or declare azd-host-required and delete every fallback? This determines whether Tier 2 exists at all.
4. Do we publish `azd-core` docs as a stable public API, or mark it internal to the `jongio.azd` family so we can break it freely?
