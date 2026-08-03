# azd SDK upgrade policy

This policy covers how every repo in the `azd-*` family depends on the Azure Developer
CLI extension SDK, `github.com/azure/azure-dev/cli/azd` (the module that provides
`pkg/azdext`).

It exists because the SDK is the one dependency all of our extensions share. If the
repos drift onto different versions of it, extensions that are supposed to speak the
same protocol to the same host stop agreeing on what that protocol is, and the failure
shows up at runtime against a real `azd` install rather than in CI.

## The rules

### 1. Depend on a published stable tag, never a pseudo-version

The `require` line must name a real released version:

```
github.com/azure/azure-dev/cli/azd v1.29.0
```

Never a pseudo-version:

```
github.com/azure/azure-dev/cli/azd v1.29.1-0.20260214153000-abcdef123456
```

A pseudo-version pins an arbitrary commit on someone else's branch. It can be
force-pushed away, it has not been through upstream's release validation, and it makes
"which SDK are we on" unanswerable without decoding a timestamp and a commit hash.

Pseudo-versions creep in silently. `go get <module>@main` and `go get <module>@master`
both produce one. So does `go get <module>@<sha>`. If you find one in a `go.mod`, treat
it as a bug and pin the nearest stable tag.

### 2. Never track a branch

`@main`, `@master`, `@latest` against an unreleased ref, and `replace` directives
pointing at a local checkout of azure-dev are all forbidden in a merged commit.

A local `replace` is legitimate while you are developing against an unreleased SDK
change, but it must not survive the PR. A `replace` pointing at a path that only exists
on your disk makes the module unbuildable for everyone else and for CI.

We enforce this mechanically in the three extension repos: `mage verifyNoLocalReplace`
fails if a `replace` with a filesystem path reaches a build, and CI runs it. Run it
locally before you push. `azd-core` has no `replace` directives and so has no such
target; if it ever gains one, it gains the guard at the same time.

### 3. All four repos move together

`azd-core`, `azd-app`, `azd-copilot`, and `azd-rest` pin the same SDK version at any
given time. An upgrade is a single coordinated change across all repos that depend on
the SDK, not a per-repo decision.

Current pinned version: **v1.29.0** (azd-app, azd-copilot, azd-rest). `azd-core` does
not currently depend on the SDK directly; when it starts to, it adopts the same pin.

If you need a newer SDK for one extension, upgrade all of them. If one repo cannot move
because the new SDK breaks it, that is a blocker to be fixed, not a reason to let that
repo lag.

### 4. Take the latest stable tag, and take it deliberately

Prefer the newest published `cli/azd/vX.Y.Z`. Do not sit on an old SDK to avoid churn.
The framework is still evolving quickly, and the cost of a large multi-version jump is
much higher than the cost of several small ones.

"Deliberately" means an upgrade is its own change, reviewed on its own merits. Do not
fold an SDK bump into an unrelated feature PR, because when the bump breaks something
you want to be able to revert it without reverting the feature.

## How to upgrade

Per repo, from the module directory (`cli/` in the extensions, the repo root in
`azd-core`):

```
go get github.com/azure/azure-dev/cli/azd@vX.Y.Z
go mod tidy
go build ./...
mage test
mage coverageGate
mage verifyNoLocalReplace   # extensions only; azd-core has no such target
```

An SDK bump should require **no source changes**. If it does, that is a signal worth
paying attention to:

- If upstream made a breaking change, the source edits belong in the same commit as the
  bump, and the commit message should say what broke and why.
- If the change is large enough to need design work, land the bump behind the design
  work rather than rushing an adaptation.

Coverage must not regress. `mage coverageGate` enforces this against
`coverage-baseline.json`. If the new SDK genuinely changes what is reachable, re-record
the baseline as an explicit, reviewable step. Never silently.

## Verifying the pin across the family

```
go list -m github.com/azure/azure-dev/cli/azd
```

Run it in each module. All results must match. A mismatch means an upgrade landed
partially, which is the exact situation this policy exists to prevent.

## Where the SDK version is recorded

| Repo | Module file |
| --- | --- |
| azd-core | `go.mod` |
| azd-app | `cli/go.mod` |
| azd-copilot | `cli/go.mod` |
| azd-rest | `cli/go.mod` |

Nothing else should hardcode an SDK version. If a workflow, script, or doc mentions a
specific SDK version, it will rot. Point at the module file instead.
