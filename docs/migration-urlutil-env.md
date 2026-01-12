# Migration Guide: URL Validation and Environment Patterns

This guide helps you migrate from custom URL validation and environment filtering to azd-core's standardized `urlutil` and `env` packages.

## Table of Contents

- [URL Validation Migration](#url-validation-migration)
- [Environment Variable Filtering Migration](#environment-variable-filtering-migration)
- [Breaking Changes](#breaking-changes)
- [Benefits](#benefits)
- [Examples](#examples)

---

## URL Validation Migration

### What Changed

The `urlutil` package provides RFC-compliant URL validation using Go's standard `net/url.Parse`, replacing custom string-based validation.

### Before: Custom URL Validation

```go
// Old custom validation approach (azd-app/azd-exec)
func validateURL(rawURL string) error {
    if rawURL == "" {
        return errors.New("URL cannot be empty")
    }
    
    // Custom string checking (prone to bypasses)
    if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
        return errors.New("URL must start with http:// or https://")
    }
    
    // Simple validation, can miss edge cases
    if rawURL == "http://" || rawURL == "https://" {
        return errors.New("URL must have a host")
    }
    
    return nil
}
```

**Problems with this approach:**
- Not RFC-compliant
- Misses edge cases (e.g., `http://` with spaces)
- No length validation (DoS risk)
- No protection against protocol injection
- Doesn't validate host properly

### After: Using urlutil

```go
// New standardized approach using azd-core/urlutil
import "github.com/jongio/azd-core/urlutil"

// Simple validation
if err := urlutil.Validate(customURL); err != nil {
    return fmt.Errorf("invalid custom URL: %w", err)
}

// HTTPS-only for production (allows localhost HTTP for dev)
if err := urlutil.ValidateHTTPSOnly(apiEndpoint); err != nil {
    return fmt.Errorf("API endpoint must use HTTPS: %w", err)
}

// Parse and use
parsed, err := urlutil.Parse(userProvidedURL)
if err != nil {
    return err
}
fmt.Printf("Connecting to: %s://%s\n", parsed.Scheme, parsed.Host)
```

**Improvements:**
- ✅ RFC-compliant via `net/url.Parse`
- ✅ Prevents protocol injection (`javascript:`, `file:`, `data:`)
- ✅ Validates host presence
- ✅ 2048 character length limit (DoS prevention)
- ✅ Better error messages
- ✅ 97.1% test coverage (60+ test cases)

### Migration Checklist

1. **Add dependency:**
   ```bash
   go get github.com/jongio/azd-core/urlutil@latest
   ```

2. **Replace validation calls:**
   ```go
   // Replace this:
   if err := validateCustomURL(url); err != nil { ... }
   
   // With this:
   import "github.com/jongio/azd-core/urlutil"
   if err := urlutil.Validate(url); err != nil { ... }
   ```

3. **Use HTTPS enforcement for production:**
   ```go
   // For API endpoints that should be HTTPS-only
   if err := urlutil.ValidateHTTPSOnly(azureEndpoint); err != nil {
       return fmt.Errorf("Azure endpoint must use HTTPS: %w", err)
   }
   // Note: localhost HTTP is still allowed for development
   ```

4. **Remove custom validation code:**
   - Delete internal validation functions
   - Remove duplicate validation logic

5. **Run tests:**
   ```bash
   go test ./...
   ```

---

## Environment Variable Filtering Migration

### What Changed

The `env` package now provides pattern-based extraction helpers for common environment variable filtering scenarios.

### Before: Custom Environment Filtering

```go
// Old approach: Manual filtering (azd-app serviceinfo)
func extractAzureVars(envVars map[string]string) map[string]string {
    result := make(map[string]string)
    
    // Pattern: SERVICE_<NAME>_URL
    for key, value := range envVars {
        if strings.HasPrefix(strings.ToUpper(key), "SERVICE_") && 
           strings.HasSuffix(strings.ToUpper(key), "_URL") {
            
            // Extract service name
            serviceName := key[8:] // Remove "SERVICE_"
            serviceName = serviceName[:len(serviceName)-4] // Remove "_URL"
            serviceName = strings.ToLower(serviceName)
            serviceName = strings.ReplaceAll(serviceName, "_", "-")
            
            result[serviceName] = value
        }
    }
    
    return result
}
```

**Problems:**
- Verbose and error-prone
- Manual case handling
- Hard to reuse
- Easy to make off-by-one errors

### After: Using env Patterns

```go
// New approach using azd-core/env
import "github.com/jongio/azd-core/env"

// Extract SERVICE_*_URL with automatic service name normalization
serviceURLs, err := env.ExtractPattern(envVars, env.PatternOptions{
    Prefix:          "SERVICE_",
    Suffix:          "_URL",
    TrimPrefix:      true,
    TrimSuffix:      true,
    KeyTransform:    env.NormalizeServiceName, // MY_API → my-api
    ValueValidation: urlutil.Validate,         // Optional: validate URLs
})
if err != nil {
    return fmt.Errorf("invalid service URL: %w", err)
}
// Returns: {"my-api": "https://...", "my-worker": "https://..."}
```

**Improvements:**
- ✅ Declarative pattern matching
- ✅ Built-in key transformation
- ✅ Optional value validation
- ✅ Case-insensitive matching
- ✅ Reusable and composable
- ✅ 100% test coverage

### Common Patterns

#### Pattern 1: Filter by Prefix

```go
// Before
azureVars := make(map[string]string)
for k, v := range envVars {
    if strings.HasPrefix(strings.ToUpper(k), "AZURE_") {
        azureVars[k] = v
    }
}

// After
import "github.com/jongio/azd-core/env"
azureVars := env.FilterByPrefix(envVars, "AZURE_")
```

#### Pattern 2: Extract with Prefix/Suffix

```go
// Before: Manual extraction
customDomains := make(map[string]string)
for k, v := range envVars {
    upper := strings.ToUpper(k)
    if strings.HasPrefix(upper, "SERVICE_") && strings.HasSuffix(upper, "_CUSTOM_DOMAIN") {
        name := k[8:len(k)-14] // Extract middle part
        customDomains[normalizeServiceName(name)] = v
    }
}

// After: Pattern-based extraction
import "github.com/jongio/azd-core/env"
customDomains, _ := env.ExtractPattern(envVars, env.PatternOptions{
    Prefix:       "SERVICE_",
    Suffix:       "_CUSTOM_DOMAIN",
    TrimPrefix:   true,
    TrimSuffix:   true,
    KeyTransform: env.NormalizeServiceName,
})
```

#### Pattern 3: Service Name Normalization

```go
// Before: Manual normalization
func normalizeServiceName(envVarName string) string {
    s := strings.ReplaceAll(envVarName, "_", "")
    s = strings.ReplaceAll(s, "-", "")
    return strings.ToLower(s)
}

// After: Built-in normalization
import "github.com/jongio/azd-core/env"
serviceName := env.NormalizeServiceName("MY_API_SERVICE")
// Returns: "my-api-service"
```

### Migration Checklist

1. **Add dependency:**
   ```bash
   go get github.com/jongio/azd-core/env@latest
   ```

2. **Replace simple filtering:**
   ```go
   // Replace manual loops with FilterByPrefix
   azureVars := env.FilterByPrefix(envVars, "AZURE_")
   ```

3. **Replace pattern extraction:**
   ```go
   // Replace complex loops with ExtractPattern
   serviceURLs, err := env.ExtractPattern(envVars, env.PatternOptions{
       Prefix:       "SERVICE_",
       Suffix:       "_URL",
       TrimPrefix:   true,
       TrimSuffix:   true,
       KeyTransform: env.NormalizeServiceName,
   })
   ```

4. **Use service name normalization:**
   ```go
   // Replace custom normalization with env.NormalizeServiceName
   serviceName := env.NormalizeServiceName(envVarName)
   ```

5. **Remove custom helper functions:**
   - Delete manual filtering loops
   - Delete custom normalization functions

6. **Run tests:**
   ```bash
   go test ./...
   ```

---

## Breaking Changes

**None.** This release is 100% backward compatible and additive only.

- No changes to existing APIs
- No changes to function signatures
- No changes to behavior of existing packages
- Only adds new packages (`urlutil`, `env` extensions)

You can adopt these packages incrementally:
1. Start using them in new code
2. Gradually migrate existing code
3. Remove custom implementations when ready

---

## Benefits

### Code Reduction

Migrating to azd-core shared utilities removes duplicate code:

| Project   | Lines Removed | Details |
|-----------|---------------|---------|
| azd-app   | ~200 lines    | URL validation, env filtering in service/serviceinfo |
| azd-exec  | ~110 lines    | URL validation in executor package |
| **Total** | **~310 lines** | Replaced with well-tested shared utilities |

### Quality Improvements

- **Better Validation**: RFC-compliant URL parsing vs custom string checking
- **Security**: Protocol injection prevention, host validation, length limits
- **Error Messages**: Clearer error messages for users
- **Test Coverage**: 97.1% urlutil, 100% env (vs sporadic coverage in custom code)
- **Edge Cases**: 100+ test cases covering corner cases

### Standardization

- **Consistency**: Same URL validation across all projects
- **Maintainability**: Fix bugs once in azd-core, all consumers benefit
- **Documentation**: Comprehensive godoc and examples
- **Best Practices**: Built-in security and validation patterns

---

## Examples

### Complete Migration Example: azd-app serviceinfo

**Before (azd-app/cli/src/internal/serviceinfo/serviceinfo.go):**

```go
// Custom URL validation (90 lines)
func validateURL(url string) error {
    if url == "" {
        return nil
    }
    
    trimmed := strings.TrimSpace(url)
    if trimmed == "" {
        return errors.New("URL cannot be only whitespace")
    }
    
    if !strings.HasPrefix(trimmed, "http://") && !strings.HasPrefix(trimmed, "https://") {
        return errors.New("URL must start with http:// or https://")
    }
    
    if trimmed == "http://" || trimmed == "https://" {
        return errors.New("URL must have a domain/host")
    }
    
    return nil
}

// Custom environment extraction (110 lines)
func extractAzureServiceInfo(envVars map[string]string) map[string]*ServiceInfo {
    serviceMap := make(map[string]*ServiceInfo)
    
    // Pattern: SERVICE_<NAME>_URL (highest priority)
    for key, value := range envVars {
        if strings.HasPrefix(strings.ToUpper(key), "SERVICE_") && 
           strings.HasSuffix(strings.ToUpper(key), "_URL") {
            serviceName := extractServiceName(key, "SERVICE_", "_URL")
            if serviceMap[serviceName] == nil {
                serviceMap[serviceName] = &ServiceInfo{Name: serviceName}
            }
            serviceMap[serviceName].Azure = &AzureInfo{URL: value}
        }
    }
    
    // Pattern: <NAME>_URL (simple pattern)
    // ... more complex filtering logic ...
    
    return serviceMap
}

func extractServiceName(envVarName, prefix, suffix string) string {
    name := envVarName
    if prefix != "" {
        name = name[len(prefix):]
    }
    if suffix != "" {
        name = name[:len(name)-len(suffix)]
    }
    return normalizeServiceName(name)
}

func normalizeServiceName(name string) string {
    s := strings.ReplaceAll(name, "_", "")
    s = strings.ReplaceAll(s, "-", "")
    return strings.ToLower(s)
}
```

**After (using azd-core):**

```go
import (
    "github.com/jongio/azd-core/urlutil"
    "github.com/jongio/azd-core/env"
)

// URL validation (1 line!)
func validateURL(url string) error {
    return urlutil.Validate(url)
}

// Environment extraction (15 lines!)
func extractAzureServiceInfo(envVars map[string]string) (map[string]*ServiceInfo, error) {
    // Extract SERVICE_*_URL pattern with validation
    serviceURLs, err := env.ExtractPattern(envVars, env.PatternOptions{
        Prefix:          "SERVICE_",
        Suffix:          "_URL",
        TrimPrefix:      true,
        TrimSuffix:      true,
        KeyTransform:    env.NormalizeServiceName,
        ValueValidation: urlutil.Validate,
    })
    if err != nil {
        return nil, fmt.Errorf("invalid service URL: %w", err)
    }
    
    // Convert to ServiceInfo map
    serviceMap := make(map[string]*ServiceInfo)
    for serviceName, url := range serviceURLs {
        serviceMap[serviceName] = &ServiceInfo{
            Name:  serviceName,
            Azure: &AzureInfo{URL: url},
        }
    }
    
    return serviceMap, nil
}
```

**Result:**
- **200 lines reduced to 15 lines**
- **Better validation** (RFC-compliant)
- **Better error handling** (validation errors surface immediately)
- **Easier to understand** (declarative vs imperative)

---

## Testing

After migration, ensure all tests pass:

```bash
# Test azd-core packages
cd azd-core
go test ./urlutil -v      # 97.1% coverage
go test ./env -v          # 100% coverage

# Test your project after migration
cd your-project
go test ./...             # All tests should pass
```

---

## Support

- **Repository**: https://github.com/jongio/azd-core
- **Documentation**: https://pkg.go.dev/github.com/jongio/azd-core
- **Issues**: https://github.com/jongio/azd-core/issues

For questions or migration help, please open an issue on GitHub.
