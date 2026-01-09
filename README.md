# azd-core

[![Go Reference](https://pkg.go.dev/badge/github.com/jongio/azd-core.svg)](https://pkg.go.dev/github.com/jongio/azd-core)
[![Go Report Card](https://goreportcard.com/badge/github.com/jongio/azd-core)](https://goreportcard.com/report/github.com/jongio/azd-core)
[![CI](https://github.com/jongio/azd-core/workflows/CI/badge.svg)](https://github.com/jongio/azd-core/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/jongio/azd-core/branch/main/graph/badge.svg)](https://codecov.io/gh/jongio/azd-core)

Common reusable Go modules for building Azure Developer CLI (azd) extensions and tooling.

## Overview

`azd-core` provides shared utilities extracted from the Azure Developer CLI to support building azd extensions, custom CLI tools, and automation scripts. The goal is to enable developers to create azd-compatible tools without duplicating common logic or pulling in the entire azd runtime.

## Installation

```bash
go get github.com/jongio/azd-core
```

Or add specific packages to your `go.mod`:

```bash
go get github.com/jongio/azd-core/env
go get github.com/jongio/azd-core/keyvault
```

## Documentation

Full API documentation is available at [pkg.go.dev/github.com/jongio/azd-core](https://pkg.go.dev/github.com/jongio/azd-core).

## Packages

### `env`
Environment variable utilities for converting between maps and slices, resolving references, and applying transformations.

**Key Functions:**
- `ResolveMap` - Resolve references in environment maps
- `ResolveSlice` - Resolve references in environment slices (`[]string`)
- `MapToSlice` / `SliceToMap` - Convert between formats
- `HasKeyVaultReferences` - Detect Key Vault references in environment data

### `keyvault`
Azure Key Vault reference detection and resolution for environment variables.

**Supported Formats:**
- `@Microsoft.KeyVault(SecretUri=https://...)`
- `@Microsoft.KeyVault(VaultName=...;SecretName=...;SecretVersion=...)`
- `akvs://<subscription-id>/<vault-name>/<secret-name>[/<version>]`

**Features:**
- Uses `azidentity.DefaultAzureCredential` for authentication
- Thread-safe client caching
- Configurable error handling (fail-fast or graceful degradation)
- SSRF protection and validation

## Usage Examples

### Resolve Key Vault References in Environment

```go
package main

import (
    "context"
    "os"

    "github.com/jongio/azd-core/env"
    "github.com/jongio/azd-core/keyvault"
)

func main() {
    // Create resolver
    resolver, err := keyvault.NewKeyVaultResolver()
    if err != nil {
        panic(err)
    }

    // Resolve from environment map
    envMap := map[string]string{
        "DATABASE_PASSWORD": "@Microsoft.KeyVault(VaultName=myvault;SecretName=db-pass)",
        "API_ENDPOINT":      "https://api.example.com",
    }

    resolved, warnings, err := env.ResolveMap(
        context.Background(),
        envMap,
        resolver,
        keyvault.ResolveEnvironmentOptions{},
    )
    if err != nil {
        panic(err)
    }

    // Handle warnings
    for _, w := range warnings {
        os.Stderr.WriteString("warning: " + w.Err.Error() + "\n")
    }

    // Use resolved environment
    os.Setenv("DATABASE_PASSWORD", resolved["DATABASE_PASSWORD"])
}
```

### Use with `exec.Cmd`

```go
import (
    "context"
    "os"
    "os/exec"

    "github.com/jongio/azd-core/env"
    "github.com/jongio/azd-core/keyvault"
)

func runWithResolvedEnv(ctx context.Context) error {
    resolver, err := keyvault.NewKeyVaultResolver()
    if err != nil {
        return err
    }

    // Resolve environment from os.Environ()
    envSlice, _, err := env.ResolveSlice(
        ctx,
        os.Environ(),
        resolver,
        keyvault.ResolveEnvironmentOptions{},
    )
    if err != nil {
        return err
    }

    // Use with exec.Cmd
    cmd := exec.Command("myapp")
    cmd.Env = envSlice
    return cmd.Run()
}
```

## Authentication

The `keyvault` package uses `azidentity.DefaultAzureCredential`, supporting:
- Environment variables (`AZURE_TENANT_ID`, `AZURE_CLIENT_ID`, `AZURE_CLIENT_SECRET`)
- Managed identity (Azure VM, App Service, Container Apps, etc.)
- Azure CLI (`az login`)
- Azure PowerShell
- Interactive browser authentication

No global state is maintained, and client caching is thread-safe.

## Testing

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
go tool cover -html=coverage.out
```

Tests are offline-only and use mocks for Azure SDK interactions.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines on contributing to this project.

## Security

See [SECURITY.md](SECURITY.md) for information on reporting security vulnerabilities.

## License

This project is licensed under the MIT License. See [LICENSE](LICENSE).
