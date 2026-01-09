# Contributing to azd-core

Thanks for your interest in contributing! This project aims to provide reusable helpers extracted from Azure Developer CLI (azd). We welcome issues and pull requests.

## Prerequisites
- Go 1.25+ installed and on your `PATH`.
- Git (and a GitHub account).

## Setup
1. Fork the repository, then clone your fork:
   ```bash
   git clone https://github.com/your-username/azd-core.git
   cd azd-core
   ```
2. Install dependencies:
   ```bash
   go mod download
   ```

## Development Workflow
- Issues → PRs: Open an issue to discuss significant changes. Small fixes can go straight to a PR.
- Branch naming: `feature/<short-desc>` or `fix/<short-desc>` (e.g., `feature/env-resolver`).
- Commit messages: Use imperative mood, keep them concise, and reference issues (e.g., `Fix resolver cache (#123)`).
- Pull Requests: Keep PRs focused and include a clear summary of changes.

## Coding Standards (Go)
- Formatting: Run `gofmt -s -w .` before committing.
- Vetting: Run `go vet ./...` and address findings.
- Keep functions small and focused; prefer clear names over cleverness.

## Testing
- Run all tests locally:
  ```bash
  go test ./...
  ```
- Tests should be deterministic and avoid reliance on live Azure resources.

## Opening Issues
- Use the templates under `.github/ISSUE_TEMPLATE/`.
- Provide reproduction steps, expected behavior, environment details, and logs where relevant.
- GitHub Issues: https://github.com/jongio/azd-core/issues

## Releases & Versioning
- Versioning follows semantic versioning (semver) for tagged releases.
- Changelogs are derived from merged PRs; keep PR descriptions informative.

Thank you for contributing!
