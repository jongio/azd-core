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
| `shellutil.Shell*` constants | `azdext.ShellType*` | Similar names, different type. `shellutil` used untyped `string`; the SDK uses a typed `azdext.ShellType`. `shellutil.ShellPwsh` and `ShellPowerShell` both map to `azdext.ShellTypePowerShell`. |

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
