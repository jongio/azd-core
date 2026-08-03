# azdext Alignment: Release Train

Spec: [spec.md](./spec.md) | Tasks: [tasks.md](./tasks.md)

This document covers the three things that must be settled around the alignment work
itself: what happens to the 35 open pull requests, what happens to the pre-existing
local branches, and the order in which the four repos get released.

Repos: **C** azd-core, **A** azd-app, **P** azd-copilot, **R** azd-rest.

## Current state

Branch `feat/azdext-alignment` is pushed in all four repos and carries the coverage
ratchet described in [coverage-ratchet.md](../../coverage-ratchet.md). The three
extensions carry a temporary `replace` directive pointing at a local azd-core
worktree, so their CI is expected to be red until azd-core publishes `v0.6.0`.
`mage verifyNoLocalReplace` is the guard that stops those directives reaching a release.

## Wave A: dependency and CI pull requests

These are safe, mergeable, and should land before any alignment code is written.
Task 0.1 of the plan is "bump the azd SDK to v1.29.0", and azd-app PR #590 already
does exactly that, so merging this wave completes part of Phase 0 for free.

| Order | Repo | PR | Why it goes here |
|---|---|---|---|
| A1 | A | #590 | Bumps `cli/azd` 1.28.1 to 1.29.0. This *is* task 0.1 for azd-app. P and R are already on 1.29.0 |
| A2 | A | #591 | `modernc.org/sqlite` 1.54.0 to 1.55.0 |
| A3 | A | #592, #593, #595, #596, #597 | Dashboard npm bumps, all confined to `cli/dashboard` |
| A4 | A | #594, #598, #599 | CodeQL action bumps, workflow-only |
| A5 | P | #86, #87, #88, #89 | CodeQL action bumps, workflow-only |
| A6 | R | #340, #341, #342 | CodeQL action bumps, workflow-only |

### The two human dependency PRs need a decision

azd-app #589 and azd-rest #343 are both titled "deps: update all dependencies to
latest" and both overlap the dependabot set above.

- **#589** touches `.github/workflows/ci.yml`, `cli/go.mod`, `cli/go.sum`, both
  dashboard and web lockfiles, plus real source edits in
  `cli/src/internal/dashboard/server_core.go` and `server_port_mgmt.go`.
- **#343** touches `cli/go.mod`, `cli/go.sum`, `pnpm-lock.yaml`, and the codeql and
  website workflows.

Because #589 carries source changes it is not purely a dependency bump and should not
be closed as superseded. Recommended handling:

1. Merge the dependabot PRs in the table above first. They are small and independent.
2. Rebase #589 and #343 onto the new main. The lockfile hunks will drop out as already
   applied, leaving #589's dashboard source changes as the real payload.
3. Review and merge the rebased remainder.

Both #589 and #343 touch `cli/go.mod`, and #589 also touches `.github/workflows/ci.yml`,
which `feat/azdext-alignment` also modifies to add the coverage ratchet step. So after
Wave A lands, `feat/azdext-alignment` must be rebased on main in all four repos before
alignment work continues. This is expected and is cheaper to do now than later.

## Wave B: the azd-rest idea backlog, decide before Phase 1

azd-rest has 16 open `idea/*` feature PRs. **12 of the 16 already conflict**, up from
2 when this effort started, and they collide with each other far more than with main:

| Collisions | File |
|---|---|
| 16 of 16 | `cli/src/internal/cmd/root.go` |
| 15 of 16 | `cli/src/internal/service/service.go` |
| 13 of 16 | `cli/src/internal/config/config.go` |
| 9 | `web/src/pages/reference.astro` |
| 9 | `README.md` |
| 8 | `cli/src/internal/cmd/root_test.go` |

Phase 1 tasks 1.1 through 1.3 rewrite `cli/src/cmd/rest/main.go` and
`cli/src/internal/cmd/`, which is precisely this blast radius. **Every one of these 16
PRs becomes effectively unmergeable once Phase 1 lands.** They have to be resolved
before the rewrite, not after.

Three options, in order of recommendation:

1. **Land the wanted ones now, close the rest.** Pick the features actually wanted for
   the next release, merge them one at a time in ascending PR number (each merge will
   require resolving the previous one's conflicts in `root.go`), then close the
   remainder with a note pointing at this document. Least total conflict work.
2. **Close all 16, re-open post-alignment.** Cleanest for the alignment work, but
   discards the implementations, and each would need rewriting against the new
   structured-error and `azdext.Run` entry point anyway.
3. **Defer the alignment for azd-rest.** Not recommended. It breaks the point of a
   coordinated release train.

The four non-conflicting ones today are #321, #305, #304, and the already-listed
dependency PRs. Those are the cheapest to land if only a subset is wanted.

## Wave C: pre-existing local branches

All four repos had work that existed on one disk only, with no remote tracking branch.
That has been pushed as-is so it cannot be lost:

| Repo | Branch | Contents |
|---|---|---|
| C | `deps/update-2026-07-29` | 1 commit, Go dependency update |
| P | `deps/update-2026-07-29` | 1 commit, Go and npm dependency update |
| R | `deps/update-2026-07-29` | 1 commit, Go and npm dependency update |
| A | `chore/lint-consolidation-and-hardening` | 7 commits, see below |

The three `deps/update-2026-07-29` branches duplicate Wave A and can be deleted once
Wave A merges.

azd-app's branch is the one that matters. It carries real fixes, not just dependency
noise:

```
ee311970 docs: regenerate changelog and whats-new pages
92eb1739 ci: fail the build when generated docs drift
19e8a9b8 fix(dashboard): stop losing bind errors during startup
32cc603b fix(azure): serialize concurrent writes on the log stream
061b1ee8 fix(security): harden workspace trust, file writes, and redaction
679405f3 refactor: remove unused code and fix new lint findings
5b94e25b chore(lint): consolidate golangci config at repo root
```

The security hardening and the log-stream race fix should not wait for the alignment
release. Recommended: open this as a PR and merge it in Wave A, ahead of the alignment
work. It touches lint config and `cli/src/internal/`, so landing it after Phase 1 would
mean redoing the conflict resolution.

That checkout also has 56 uncommitted files which have not been inspected or touched.
They need a decision before that worktree is reused for anything.

## Wave D: the coordinated release

The dependency graph forces the order. The three extensions all consume azd-core, so
azd-core ships first and everything else re-pins to it.

| Step | Repo | Action | Gate |
|---|---|---|---|
| D1 | C | Merge `feat/azdext-alignment` to main | Coverage ratchet green, all Phase 2 tasks done |
| D2 | C | Tag `v0.6.0` | `covergate` and the aligned API are both in the tag |
| D3 | A P R | Remove the `replace` directive, `go get github.com/jongio/azd-core@v0.6.0`, `go mod tidy` | `mage verifyNoLocalReplace` passes |
| D4 | A P R | CI goes green for the first time since the branch was cut | Coverage ratchet green in all three |
| D5 | A P R | Merge to main and tag | Requires explicit approval |

### Why one azd-core tag and not an alpha now

An alpha cut today would contain only `covergate` and nothing from Phases 1 through 4.
It would go stale the moment alignment work lands, forcing alpha.2, alpha.3 and so on,
with three extensions needing a re-pin each time. The temporary `replace` directives
give the same unblocking with none of the tag churn, and
`mage verifyNoLocalReplace` guarantees they cannot silently survive to a release.

### Release gate

D2 and D5 publish real releases and require explicit per-release approval. Nothing in
this document authorises a tag or a merge on its own.

## Summary of what is blocked on a decision

1. Approval to merge the Wave A dependency and CI PRs.
2. Which of the 16 azd-rest idea PRs to keep, and whether to close the rest.
3. Whether to open azd-app `chore/lint-consolidation-and-hardening` as a PR now.
4. What to do with the 56 uncommitted files in the azd-app checkout.
5. Approval for the D2 and D5 tags when the alignment work reaches them.

Items 1 through 4 are all cheaper to resolve before Phase 1 than after.
