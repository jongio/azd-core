# Coverage ratchet

Every azd repository enforces a coverage ratchet. Coverage may rise freely. It
may never fall.

This exists because the azdext alignment work rewrites large parts of azd-core
and all three extensions. Without a floor, a refactor can quietly delete a
tested code path and nobody notices until something breaks in the field.

## Recorded floors

| Repository | Baseline | Scope |
| --- | --- | --- |
| azd-core | 85.8% | all packages |
| azd-rest | 86.3% | `./src/...` |
| azd-app | 65.0% | `./src/...`, excluding generated protobuf under `src/gen` |
| azd-copilot | 30.0% | `./src/...` |

The numbers differ a lot, and that is the point. The gate holds each repository
to its own history rather than to a number somebody picked. azd-copilot at 30%
is not acceptable long term, but locking it there stops it sliding to 25% while
the real work happens.

## Daily use

```sh
mage coverage        # run the tests, then check the result against the baseline
mage coverageGate    # check an existing profile without re-running the tests
mage coverageRecord  # write a new baseline
```

`mage preflight` runs the gate too, so a release check cannot pass on a
regression.

## When the gate fails

The failure names every scope that dropped and by how much:

```
coverage regressed in 1 place(s):
  github.com/jongio/azd-core/registry: 91.2% fell below the baseline of 98.5% (-7.3)
```

Two valid responses:

1. **Add the missing tests.** Almost always the right answer. The gate has found
   real work, not a false alarm.
2. **Re-record, and justify it.** If the drop is deliberate, for example you
   deleted a well-tested package, run `mage coverageRecord` and explain why in
   the commit message. A baseline change in a diff is a prompt for a reviewer to
   ask what happened.

Do not widen the exclude patterns to make a failure disappear. The gate detects
that and fails anyway, because it is the easiest way to fake a pass.

## What the gate checks

- **Total coverage** against the recorded total.
- **Every package** against its own recorded floor. A total-only gate lets a
  refactor gut one package and hide it in the average.
- **New packages** against the baseline total, so new code cannot dilute
  coverage.
- **Exclude patterns**, compared against the ones recorded in the baseline.

A 0.5 point tolerance absorbs rounding noise. A missing baseline is a hard
failure rather than a silent pass, so a repository that was never recorded
cannot appear to be protected.

## In CI

CI gates the profile it already produced, so the suite runs once:

```yaml
- name: Enforce coverage ratchet
  if: matrix.os == 'ubuntu-latest'
  run: go run github.com/jongio/azd-core/covergate/cmd/covergate -profile ../coverage/coverage.out
```

## Why generated code is excluded in azd-app

`src/gen` holds roughly 950 generated protobuf functions sitting at zero
coverage. Counting them puts azd-app's total near 59% and, worse, means a real
regression of several points in hand-written code barely moves the number. With
them excluded the total is 65% and it responds to changes that matter.

## Baseline files are committed

`coverage-baseline.json` is tracked in git. Each repository's `.gitignore` has
broad `coverage*` patterns, so there is an explicit negation to keep the
baseline visible. If you change those patterns, check that the baseline is still
tracked, otherwise the gate silently stops protecting anything.
