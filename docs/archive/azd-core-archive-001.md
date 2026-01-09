# azd-core Task Archive 001

Date: 2026-01-08

## Completed Tasks

1. Scaffold module `github.com/jongio/azd-core` (go.mod, minimal README, package layout) and baseline CI/test workflow.
2. Port Key Vault resolver from azd-app PR #103 into package `keyvault` with its unit tests; ensure azidentity/azsecrets dependencies and cache/validation behavior preserved.
4. Add repo hygiene files (.gitignore, .gitattributes, editor/CI basics) suitable for a Go module.
5. Add MIT LICENSE at repo root; ensure correct year and project owner attribution, and link from README.
6. Add CONTRIBUTING.md with contributor workflow (issues → PRs), coding standards for Go (gofmt/go vet), local test instructions (`go test ./...`), and release/versioning notes.
7. Add CODE_OF_CONDUCT.md using Contributor Covenant v2.1, with contact email for enforcement.
8. Add SECURITY.md describing how to report vulnerabilities (private reporting channel) and supported versions policy.
9. Add GitHub templates under .github/ (ISSUE_TEMPLATE/bug_report.md, ISSUE_TEMPLATE/feature_request.md, pull_request_template.md) to standardize contributions.
