# Migrating to azd-core v0.6.0

v0.6.0 rebases azd-core onto the `azdext` SDK that ships with Azure Developer
CLI v1.29.0. Where the SDK is genuinely equivalent, azd-core now delegates to it
so extensions and the azd host agree on behavior. Where it is not equivalent,
azd-core keeps its own implementation and this guide says why.

**Requires:** `github.com/azure/azure-dev/cli/azd v1.29.0` or later.

`azdext` below means `github.com/azure/azure-dev/cli/azd/pkg/azdext`.

## What you have to change

Only three things can break a build:

1. `procutil`, `shellutil`, and `azdextutil` are deleted. See "Removed
   packages" for the per-symbol replacement, and read the notes: three of those
   symbols have an SDK function with a similar name that does something
   different.
2. `pathutil.FindToolInPath` and `security.SanitizeScriptName` are removed
   from packages that otherwise stay.
3. A small number of signatures changed. See "Changed signatures in retained
   packages" at the end.

Everything else is a behavior change under an unchanged signature. Those are
listed too, because several of them fix bugs and one of them can change whether
a prompt appears.

## Removed packages

| Removed | Replacement | Notes |
|---|---|---|
| `procutil.IsProcessRunning(pid int) bool` | `azdext.IsProcessRunning(pid int) bool` | Identical signature and behavior. Drop-in. |
| `procutil` (package) | `azdext` | The SDK is a superset: it adds `GetProcessInfo`, `CurrentProcessInfo`, `ParentProcessInfo`, `FindProcessByName`, `GetProcessEnvironment`. Removing this package also drops `github.com/shirou/gopsutil/v4` from the dependency graph. |
| `shellutil.DetectShell(scriptPath string) string` | none | **No drop-in replacement.** See "Not equivalent" below. Copy the function if you need it. |
| `shellutil.ReadShebang(scriptPath string) string` | none | **No drop-in replacement.** The SDK never reads script files. |
| `azdextutil.NewRateLimiter`, `RateLimiter.Allow`, `RateLimiter.CheckRateLimit` | `MCPServerBuilder.WithRateLimit(burst int, refillRate float64)` | Only if you are already building your server with `MCPServerBuilder`. The SDK exposes no standalone limiter type. If you need one outside a builder, use `golang.org/x/time/rate` directly, which is all `azdextutil.RateLimiter` wrapped. |
| `azdextutil.ValidateShellName(shell string) error` | none | **Not** `azdext.ValidateScriptName`. See "Not equivalent" below. |
| `azdextutil.GetProjectDir(envVar string) (string, error)` | `azdext.GetProjectDir() (string, error)` | Behavior change. The azd-core version read the named environment variable and fell back to the working directory, so it effectively always succeeded. The SDK version walks up looking for `azure.yaml` and returns `azdext.ErrProjectNotFound` when there is no azd project. Handle that error. |
| `azdextutil` (package) | `azdext` | Deprecated in v0.5.3 and had no remaining callers in azd-core or any extension at removal time. |
| `shellutil.Shell*` constants | `azdext.ShellType*` | Similar names, different type. `shellutil` used untyped `string`; the SDK uses a typed `azdext.ShellType`. `shellutil.ShellPwsh` and `ShellPowerShell` both map to `azdext.ShellTypePowerShell`. |

## Removed functions from retained packages

| Removed | Replacement | Notes |
|---|---|---|
| `pathutil.FindToolInPath(name string) string` | `azdext.LookupTool(name).Path` | Also a **bug fix**, see below. `LookupTool` additionally returns `ToolInfo.Found` and prefers a project-local wrapper such as `./mvnw` over the PATH copy. |
| `security.SanitizeScriptName(name string) error` | `security.ValidateScriptName(name string) error` | Renamed to match what it does: it validates and never sanitizes. Now delegates to `azdext.ValidateScriptName`, which rejects a strict superset of the old metacharacters (adds `!`, `~`, `*`, `?`, `%`, and NUL), also rejects `..`, and rejects the empty string, which the old function accepted. The error text is now "script name contains forbidden shell metacharacter at position N" rather than "dangerous character". |

`pathutil` itself is **retained**. `SearchToolInSystemPath`, `GetInstallSuggestion`, and
`RefreshPATH` have no SDK equivalent:

- `SearchToolInSystemPath` scans well-known install directories that are **not** on PATH. `LookupTool` searches PATH and the working directory only.
- `GetInstallSuggestion` returns curated install URLs for roughly 22 tools. `azdext.ToolsNotFoundError` only lists missing names.
- `RefreshPATH` re-reads the Machine and User PATH from the Windows registry and applies it to the process. `azdext.PrependPATH` adds directories you already know about, which is a different operation.

### Why `FindToolInPath` was a bug

It appended `.exe` on Windows before calling `exec.LookPath`, which defeats
`PATHEXT` resolution and so never matched `.cmd` shims. Measured on Windows:

| Tool | `LookPath(name)` | `LookPath(name + ".exe")` |
|---|---|---|
| `npm` | found (`npm.cmd`) | not found |
| `pnpm` | found (`pnpm.cmd`) | not found |
| `az` | found (`az.cmd`) | not found |
| `func` | found (`func.cmd`) | not found |
| `node`, `git`, `docker` | found | found |

So `azd app reqs fix` reported npm, pnpm, az, and func as missing on Windows
even when they were installed. `azdext.LookupTool` calls `exec.LookPath(name)`
without a suffix and resolves all of them.


## Not equivalent, despite the shared name

`azdext.DetectShell()` is **not** a replacement for `shellutil.DetectShell(scriptPath)`.

| | `shellutil.DetectShell(path)` | `azdext.DetectShell()` |
|---|---|---|
| Question answered | Which interpreter should run *this script file*? | Which shell is *the user* running right now? |
| Input | A script path | Nothing |
| Method | File extension, then shebang line | `SHELL`, then `PSModulePath`, then `ComSpec` |
| Can return `python3` / `node` | Yes, via shebang | No |
| Result for `deploy.ps1` | `powershell` or `pwsh` | Whatever shell the user launched from |

Substituting one for the other silently breaks script execution: `.ps1` files
would be handed to the user's shell, and shebang interpreters would be ignored
entirely.

`shellutil` was removed because it had **zero callers** in `azd-core` or in any
extension, not because the SDK supersedes it. Anyone depending on it should
vendor the two functions rather than reach for `azdext.DetectShell`.

`cmdutil.GetDefaultShell` and `cmdutil.prepareHookCommand` are **retained** for
the same reason. `GetDefaultShell` picks the best *available* shell by
`LookPath` so hooks get a predictable interpreter; `azdext.DetectShell` would
return `fish` for a fish user and break hooks written for bash.
`prepareHookCommand` also handles `-File` for `.ps1` paths and emits a UTF-8
console preamble, neither of which `azdext.ShellCommandWith` provides.

`azdext.ValidateScriptName` is **not** a replacement for
`azdextutil.ValidateShellName`.

| | `azdextutil.ValidateShellName(shell)` | `azdext.ValidateScriptName(name)` |
|---|---|---|
| Question answered | Is this one of the shells we support? | Could this filename be used for command injection? |
| Strategy | Allowlist of six names: bash, sh, zsh, pwsh, powershell, cmd | Denylist of shell metacharacters, plus `..` and null bytes |
| Verdict on `"fish"` | Rejected, not in the allowlist | Accepted, contains no metacharacters |
| Verdict on `"deploy.sh"` | Rejected, not a shell name | Accepted |
| Empty input | Accepted, treated as "unset" | Rejected |

They agree on almost no input. `azdextutil` was removed because it had zero
callers and had already been deprecated in v0.5.3, not because the SDK
supersedes it. Code needing a shell allowlist should keep its own.

`azdext.SSRFGuard` is **not** a replacement for `urlutil`.

| | `urlutil.Validate(rawURL)` | `azdext.SSRFGuard.Check(rawURL)` |
|---|---|---|
| Question answered | Is this string a well-formed http(s) URL? | Is it safe to send a request to this URL? |
| Touches the network | Never | Yes, resolves the host when private-network blocking is on |
| Verdict on `http://localhost:3000` | Accepted | Accepted (explicit loopback bypass) |
| Verdict on `http://10.0.0.5/health` | Accepted | Blocked by `DefaultSSRFGuard` (RFC 1918) |
| Verdict on `http://internal.corp/health` | Accepted | Blocked by `DefaultSSRFGuard` (HTTPS required) |
| Enforces a max URL length | Yes, 2048 | No |
| Requires a non-empty host | Yes, explicitly | Only as a side effect of resolution failing |

`urlutil` is retained. Its only consumer validates service URLs from user
config, which routinely point at localhost or an internal host, so an SSRF
guard would reject exactly the input the validator exists to accept, and would
perform DNS lookups while parsing a config file. The two are meant to be
layered: validate the string with `urlutil`, then guard the request with
`azdext.SSRFGuard` at the point of outbound call.

`azdext.ValidateHostname` is **not** a replacement for `urlutil.ValidateDomain`
either. The per-label rules are identical (alphanumeric start and end, `-`
allowed inside, 1 to 63 characters, 253 total), but `ValidateDomain` also
rejects an embedded protocol and an embedded port, requires at least one dot
unless the value is exactly `localhost`, and names the offending character in
its error. Delegating would loosen validation for a custom-domain config field
and replace a specific message with a generic one.
## Behavior changes with no signature change

`fileutil.AtomicWriteJSON`, `fileutil.AtomicWriteFile`, and `fileutil.EnsureDir`
keep their signatures but now delegate to `azdext.WriteFileAtomic` and
`azdext.EnsureDir`. No call site needs to change. Three behaviors differ:

| Behavior | Before (v0.5.x) | After (v0.6.0) |
|---|---|---|
| Rename retry on Windows | Any error, 5 attempts, 200ms total | `ERROR_SHARING_VIOLATION` and `ERROR_ACCESS_DENIED` only, 10 attempts, up to 10s |
| Rename retry on Unix | Any error, 5 attempts, 200ms total | None, `os.Rename` is already atomic |
| `AtomicWriteFile` with `perm == 0` | Chmods the file to `0000`, leaving it unreadable | Preserves the existing file's permissions, or `0644` for a new file |

The Windows change means a write that previously failed under a transient
antivirus or indexer lock now waits and succeeds. The trade is that a
genuinely stuck rename takes up to 10s to report failure instead of 200ms.

`fileutil.EnsureDir` still creates directories with `fileutil.DirPermission`
(`0750`). Do not swap it for `azdext.EnsureDir` at call sites: that function
falls back to `0755` when passed a zero permission, which would grant
world read and execute.
`security.IsContainerEnvironment` keeps its signature but now delegates to
`azdext.IsContainerEnvironment`. Detection changed from value-based to
presence-based, and one more variable is consulted:

| Behavior | Before (v0.5.x) | After (v0.6.0) |
|---|---|---|
| `CODESPACES` / `REMOTE_CONTAINERS` | Must equal the literal `"true"` | Any non-empty value counts as present |
| `REMOTE_CONTAINERS_IPC` | Not consulted | Counted as present when non-empty |
| `KUBERNETES_SERVICE_HOST`, `/.dockerenv` | Consulted | Unchanged |

Presence-based detection is the convention these tools actually use. GitHub
never sets `CODESPACES` to anything but `"true"`, and adding
`REMOTE_CONTAINERS_IPC` closes a real gap: some VS Code Dev Container versions
set only that variable, so azd-core previously failed to detect them. If you
have a test that sets one of these to `"false"` and expects `false`, invert it.
`azdext` also exposes `ContainerRuntime()` returning `codespaces`,
`kubernetes`, `devcontainer`, `docker`, or the empty string, if you need to
distinguish them.

`security.ValidateServiceName` keeps its signature, including the `allowEmpty`
parameter, the length error message, and the `ErrInvalidServiceName` sentinel.
Only the format rule now comes from `azdext.ValidateServiceName`, whose regex
is byte-identical to the one it replaces. The explicit `..`, `/`, and `\`
traversal check is retained because the SDK regex accepts `a..b`.

### keyvault

The package is now a thin layer over `azdext.KeyVaultResolver`. Parsing, client
construction, per-vault caching, and secret retrieval all move to the SDK. The
exported API is unchanged, but the behavior behind it is not.

| Behavior | Before | Now |
|---|---|---|
| `IsKeyVaultReference` on a malformed `akvs://` value | Strict regex, so `akvs://a/b` was not a reference and passed through as a literal value | Prefix check, so anything starting with `akvs://` is a reference. A typo now surfaces as a resolution warning naming the variable instead of silently shipping a broken literal into the environment |
| `@Microsoft.KeyVault(` prefix casing | Case sensitive, so `@microsoft.keyvault(...)` was not recognized | Case insensitive, matching how Azure App Service reads the same syntax. This was a bug |
| Vault name rules | Leading and trailing hyphens were accepted | `^[a-zA-Z][a-zA-Z0-9-]{1,22}[a-zA-Z0-9]$`, matching the Azure naming rule. A name that Azure would reject is now rejected before the network call |
| `SecretUri` host | Only `.vault.azure.net` | `.vault.azure.net`, `.vault.azure.cn`, `.vault.usgovcloudapi.net`, `.vault.microsoftazure.de`, and `.managedhsm.azure.net`. Sovereign clouds and Managed HSM work, and the host allowlist still blocks an arbitrary URL |
| Duplicate or unknown parameters in an app reference | Last value won, unknown keys ignored | Rejected as an invalid reference |
| Error type | `fmt.Errorf` string, with the vault URL deliberately stripped | `*azdext.KeyVaultResolveError`, carrying `Reason` (`InvalidReference`, `ClientCreation`, `NotFound`, `AccessDenied`, `ServiceError`), the vault, and the secret name. Callers can branch on the reason with `errors.As` instead of matching substrings |

The redaction is deliberately reversed. The old code stripped the vault URL to
avoid disclosing it, but the error already quoted the reference the caller
passed in, and that reference contains the vault name. Redacting one copy while
printing the other only cost debuggability.

#### `akvs://` with a version

`azdext.ParseSecretReference` accepts exactly three segments. azd-core has
documented a fourth version segment since v0.1, so `normalizeReference` rewrites
`akvs://<subscription>/<vault>/<secret>/<version>` into
`@Microsoft.KeyVault(VaultName=<vault>;SecretName=<secret>;SecretVersion=<version>)`
before handing it to the SDK. Both forms keep working, and the subscription id
is still ignored, exactly as before.

#### New constructor

`NewKeyVaultResolverWithCredential(cred, opts)` accepts an
`*azdext.KeyVaultResolverOptions`, which exposes a `VaultSuffix` for sovereign
clouds and a `ClientFactory` for injecting a fake secret client in tests.
`NewKeyVaultResolver()` is unchanged and still builds a
`DefaultAzureCredential`.
### httpclient

The client keeps its own retry loop and its own pagination, because
`azdext.ResilientClient` and `azdext.Pager` cannot express what `azd rest`
exposes. See the migration notes below for what changed anyway.

| Behavior | Before | Now |
|---|---|---|
| Retry on 429 Too Many Requests | Not retried. A throttled Azure Resource Manager or Microsoft Graph call failed on the first response | Retried |
| Retry on 408 Request Timeout | Not retried | Retried |
| Retry on 501 and 505 | Retried, since every 5xx was retried | Not retried. Both describe a permanent property of the server, so retrying only spent the caller's time |
| `Retry-After` | Ignored | Honored, reading `retry-after-ms`, `x-ms-retry-after-ms`, and `retry-after` in either seconds or HTTP date form, capped at `MaxRetryAfterDuration` (120s) so a hostile value cannot stall the client |
| Backoff | Exactly 1s, 2s, 4s | The same base, capped at `MaxRetryBackoff` (30s) and jittered into [80%, 120%). Without jitter, every client throttled by the same server retries in lockstep |
| Discarded response body before a retry | Closed | Drained up to `MaxDrainBytes` (1 MiB) and then closed, so the connection can be reused |
| Redirect target | Only counted against `--max-redirects` | Also passed through `azdext.SSRFSafeRedirect`, which blocks HTTPS to HTTP downgrades, cloud metadata endpoints, and loopback targets |

#### Redirects and localhost

A caller who aimed the original request at localhost keeps full redirect
freedom. `azdext.SSRFSafeRedirect` blocks loopback unconditionally, and running
an extension against a local API server is a first-class azd workflow, so
applying it there would break local development for no security gain: the
caller already chose localhost. Every other origin gets the full policy.

`azdext.SSRFSafeRedirect` resolves the redirect host so it can test the address
against the private ranges, and it fails closed. A redirect to a host that
cannot be resolved is now refused rather than attempted.
### `auth.DetectScope` now resolves three more services

`DetectScope` delegates its static host lookups to `azdext.ScopeDetector`. Every
mapping that resolved before still resolves to the same scope. Three hosts that
previously returned an empty scope, meaning the request went out
unauthenticated, now resolve:

| Host suffix | Scope |
| --- | --- |
| `.openai.azure.com` | `https://cognitiveservices.azure.com/.default` |
| `.cognitiveservices.azure.com` | `https://cognitiveservices.azure.com/.default` |
| `.services.ai.azure.com` | `https://cognitiveservices.azure.com/.default` |

If you were passing an explicit `--scope` to reach Azure OpenAI, you can drop it.
An explicit scope still wins, so nothing breaks if you keep it.

Two rules stayed in `azd-core` because `azdext.ScopeDetector` cannot express
them. Azure Data Explorer needs a scope derived from the cluster host rather
than a fixed string. Service Bus and Event Hubs share the
`servicebus.windows.net` suffix and are told apart by the request path;
`azdext` resolves that suffix to Event Hubs unconditionally, which would hand a
queue operation a token for the wrong audience.

`azd-core` also keeps a rule for `.azurecr.io` that deliberately disagrees with
`azdext`. The SDK returns the Resource Manager scope, which is what you exchange
for an ACR refresh token. `azd-core` returns
`https://containerregistry.azure.net/.default` so a direct `/v2/` call works.

`DetectScope` still returns `("", nil)` for a host it does not recognize.
`azdext.ScopeDetector` returns a `*ScopeDetectorError` in that case; `azd-core`
translates it back so that an unrecognized host is still sent unauthenticated
rather than failing.

### `auth` gained two constructors for host-supplied credentials

`NewAzureTokenProviderWithCredential(cred azcore.TokenCredential)` wraps any
credential in the existing per-scope cache, request timeout, and error
classification.

`NewAzureTokenProviderForHost(ctx, client, opts)` uses `azdext.TokenProvider`
when `client` is non-nil and the resilient `azidentity` chain when it is nil.
Under a host this is what makes token acquisition tenant correct: the SDK
provider reads the tenant from the deployment context, while the credential
chain has no way to know it and acquires against whatever the local login points
at.

`azdext.TokenProvider` is not a replacement for `AzureTokenProvider`. It has no
token cache, so every call shells out to `azd`; it uses only the azd CLI
credential, so managed identity and workload identity are unavailable; and it
returns the raw `azidentity` error instead of an `AuthPermissionError` or an
`AuthCredentialUnavailableError`. Wrapping it preserves all three.

`NewAzureTokenProvider` is unchanged. Existing callers need no edit.

### `logutil` now honors every documented `AZD_DEBUG` value

`IsDebugEnabled` compared `AZD_DEBUG` against the literal string `true`, so
`AZD_DEBUG=1` and `AZD_DEBUG=yes` did nothing even though the azd extension
framework honors both. It now uses the same parsing as the SDK: anything
`strconv.ParseBool` accepts, plus `yes`, case-insensitively.

If you were relying on `AZD_DEBUG=1` being ignored, you will now get debug
output. Set `AZD_DEBUG=false` or unset it.

### `logutil.ComponentLogger` is backed by `azdext.Logger`

`NewLogger`, `WithService`, `WithOperation`, `WithFields`, `Component`, and the
four level methods are unchanged. Three additions:

| Method | Purpose |
| --- | --- |
| `WithComponent(name)` | Reparent under a new component, recording the old one as `parent_component`. Matches the SDK. |
| `AzdextLogger()` | The wrapped `*azdext.Logger`, for APIs that accept one. |
| `Slogger()` | The underlying `*slog.Logger`. |

Both accessors bypass `SetLevel`, since the level filter lives in the wrapper.

Level filtering and the output writer stayed in `logutil` on purpose.
`azdext.LoggerOptions` carries a single `Debug` boolean, so it can express only
debug and info; delegating the filter would have made `SetLevel(LevelWarn)` and
`SetLevel(LevelError)` silently ineffective. Separately,
`azdext.NewLogger` constructs a fresh handler and defaults to stderr rather than
inheriting `slog.Default`, contrary to its documentation, so `logutil` passes the
package writer explicitly. Without that, `SetOutput` and
`SetupLoggerWithWriter` would have become no-ops for component loggers and every
captured log line would have leaked to stderr.

One consequence worth knowing: a `ComponentLogger` captures the writer and
format in effect when it is constructed. Call `SetOutput` or
`SetupLoggerWithWriter` before creating loggers you intend to capture. This was
already true and is now documented.

### `cliout` color, table, and prompt behavior

`PrintJSON` and `Table` now render through `azdext.Output`. Three
long-standing defects are fixed as a result. All 43 exported functions and every
signature are unchanged.

**Color is now detected instead of assumed.** `noColor` used to be initialized
to a hardcoded `false`, and the `getNoColor()` accessor was never called by
anything, so `cliout.NoColor()` had no effect and ANSI escape sequences were
written even when stdout was redirected to a file or a pipe. Color state is now
seeded from `azdext.DetectInteractive().CanColorize()` (which honors
`FORCE_COLOR=1`, then any non-empty `NO_COLOR`, then whether stdout is a
terminal) and is applied at the single point where the package writes.
`ForceColor()` and `NoColor()` still override the detection and now actually
work.

If you were relying on escape codes appearing in captured output, set
`FORCE_COLOR=1` or call `cliout.ForceColor()`.

**`Table` honors JSON mode.** It previously had no format check, so
`--output json` printed a human readable table into the JSON stream. It now
emits a JSON array of objects in JSON mode. In text mode the rendering comes
from the SDK: the leading three space indent is gone and columns are separated
by two spaces. `Table` still returns without printing anything when the row
slice is empty. Rows remain `TableRow` (`map[string]string`); a header with
no matching key renders as an empty cell rather than a Go map dump.

**`Confirm` no longer blocks where nobody can answer.** It previously checked
only `IsJSON()` and otherwise called `fmt.Scanln` unconditionally, which
hung or silently failed under CI, `AZD_NO_PROMPT`, and AI agent hosts. It now
also consults `azdext.DetectInteractive().CanPrompt()` and returns `false`
(declines) when prompting is impossible. JSON mode still returns `true`, so
existing scripted flows are unaffected.

If you need an unattended yes, pass an explicit skip-confirmation flag rather
than relying on the prompt failing open.

### `env` loader stays local (no user-visible change)

No behavior changed. This entry exists so the decision is discoverable.

`env.LoadAzdEnvironment(ctx, envName)` and `env.ParseKeyValueFormat` were
evaluated for delegation to `azdext.LoadAzdEnvironment` and
`azdext.ParseEnvironmentVariables` and both were rejected.

The SDK loader runs `azd env get-values` with no `-e` flag, so it cannot
target a named environment. Delegating would have made `-e <name>` silently
read the default environment. The SDK loader also has no JSON output path, no
environment name validation, and no injectable command runner.

The SDK parser trims the value and strips only double quotes, so it changes a
value with significant leading whitespace and one wrapped in single quotes.

`env/azdext_divergence_test.go` pins both findings.

## Changed signatures in retained packages

| Symbol | Change | Notes |
|---|---|---|
| `version.NewCommand(info *Info, outputFormat *string)` | gains `opts ...Option` | Source compatible. Existing calls keep the previous behavior: `-q` shorthand on `--quiet`, and an `--output` declaration accepting `default` and `json`. |

`version.NewCommand` now calls `azdext.RegisterFlagOptions`, so azd gets shell
completion, help text, extension metadata, and parse-time validation for the
output flag from a single declaration.

Two things had to become options because hardcoding them breaks real
extensions:

| Option | Why it exists |
|---|---|
| `version.WithQuietShorthand(string)` | `NewCommand` hardcoded `-q` for `--quiet`. cobra panics during flag parsing when a subcommand shorthand collides with an inherited persistent flag, so any extension already binding `-q` crashed on every invocation. azd-rest binds `-q` to `--query` and passes `""`. |
| `version.WithOutputFlag(string, ...string)` | `NewCommand` hardcoded the flag name `output`. An extension that binds `outputFormat` to a flag of its own would have azd validating and completing a flag the command never reads. Pass `""` to skip the declaration. azd-rest reads its own `--format` and passes `""`. |

### azd-rest

`azd rest version` moves off `azdext.NewVersionCommand` and back onto
`version.NewCommand`. It regains `--quiet`, `buildDate`, and `gitCommit`, all
of which the SDK command drops. `--quiet` is registered without the `-q`
shorthand there, since `-q` remains `--query`.