<!-- NEXT: 11 -->
# Tasks - azd-core

## TODO

12. Add coverage quality gates: add codecov.yml with project/patch thresholds (e.g., 75% project, 70% patch) and required status in CI, ensure go test emits a single atomic coverprofile across packages.
13. Surface coverage to developers: add a Codecov badge to README and a short "local coverage" section (commands for coverprofile and HTML), update docs/coverage-report.md if commands or thresholds change.

## IN PROGRESS

11. Diagnose and harden CI coverage upload: review run 20862431838/job/59945223747 logs, confirm coverage.txt is produced on ubuntu matrix, ensure Codecov upload succeeds without requiring an explicit token (private repo? set CODECOV_TOKEN secret if needed), and fix any path/matrix issues. (BLOCKED: Codecov rejects tokenless upload; need CODECOV_TOKEN secret configured)

See docs/archive/azd-core-archive-001.md for history of completed tasks.

## DONE

3. Add environment helper that applies the resolver to env maps/slices; include example and docs for consumers (azd-app, azd-exec).
8. Add SECURITY.md describing how to report vulnerabilities (private reporting channel) and supported versions policy.
9. Add GitHub templates under .github/ (ISSUE_TEMPLATE/bug_report.md, ISSUE_TEMPLATE/feature_request.md, pull_request_template.md) to standardize contributions.
10. Add GitHub Actions workflows: ci.yml (test/lint/build on Go 1.22-1.23, Linux/Windows/macOS), codeql.yml (security scanning), dependabot.yml (dependency updates), govulncheck.yml (Go vulnerability scanning).

## DONE
8. Add SECURITY.md describing how to report vulnerabilities (private reporting channel) and supported versions policy.
9. Add GitHub templates under .github/ (ISSUE_TEMPLATE/bug_report.md, ISSUE_TEMPLATE/feature_request.md, pull_request_template.md) to standardize contributions.
