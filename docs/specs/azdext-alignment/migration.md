# azd-core v0.6.0 migration table

Accumulated as Phase 2 lands, one row per deleted or changed symbol. Task 2.15
copies this into `CHANGELOG.md` at release time so the table is never
reconstructed from memory.

`azdext` below means `github.com/azure/azure-dev/cli/azd/pkg/azdext`.

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