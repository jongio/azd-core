# Shared Utilities Extraction for azd-core

## Context
Analysis of azd-app PR #110 (Service URL Configuration) revealed significant code duplication and opportunities to extract common logic into azd-core for reuse across azd-app, azd-exec, and future extensions.

**Key Findings:**
- URL validation logic duplicated across 3+ locations in azd-app
- Environment variable parsing patterns repeated ~80 lines across multiple files
- Basic string operations (URL, env var, service name normalization) scattered
- No shared utilities for common Azure resource ID parsing patterns

**Current State:**
- azd-app: Basic URL validation (39 lines), env var filtering (120+ lines)
- azd-core: Has `env`, `security`, `fileutil` packages but missing URL and string utilities
- Code duplication increases maintenance burden and inconsistency risk

## Goals
- Create `urlutil` package in azd-core for comprehensive HTTP/HTTPS URL validation and parsing
- Extend `env` package with pattern-based environment variable extraction helpers
- Reduce code duplication by 200-310 lines across azd-app and azd-exec
- Provide battle-tested, >=80% covered utilities for entire azd ecosystem
- Enable consistent URL validation with `net/url` stdlib (addresses MQ ISSUE #1)

## Non-Goals
- Custom domain detection logic (Azure SDK integration, stays in azd-app)
- Service-specific business logic (service info extraction, stays in azd-app)
- Azure SDK wrappers (too heavy for azd-core's lightweight utility focus)
- Runtime configuration management (stays in azd-app)

## Requirements

### Package 1: `urlutil` - URL Validation and Parsing

**Location:** `azd-core/urlutil/`

**Functions:**
```go
// Validate performs comprehensive HTTP/HTTPS URL validation using net/url
func Validate(rawURL string) error

// ValidateHTTPSOnly enforces HTTPS-only URLs for production use
func ValidateHTTPSOnly(rawURL string) error

// Parse parses and normalizes URLs with trimming and validation
func Parse(rawURL string) (*url.URL, error)

// NormalizeScheme ensures URL has http:// or https:// prefix
func NormalizeScheme(rawURL string, defaultScheme string) string
```

**Validation Rules:**
- Must use `net/url.Parse` for robust validation (not string prefix checks)
- Must validate protocol (http:// or https:// only for security)
- Must validate host presence (reject "http://", "https://")
- Must validate URL length (max 2048 characters - RFC standard)
- Must trim whitespace before validation
- Must provide clear error messages with context

**Error Messages:**
```go
// Examples
"url cannot be empty"
"invalid URL format: <parse error>"
"url must use http:// or https://, got: ftp"
"url missing host/domain"
"url exceeds maximum length of 2048 characters"
```

**Test Coverage:**
- Valid HTTP URLs: `http://localhost:3000`, `http://example.com`
- Valid HTTPS URLs: `https://example.com`, `https://api.example.com/path`
- Invalid protocols: `ftp://`, `file://`, `javascript://`
- Missing host: `http://`, `https://`
- Malformed: `not-a-url`, `example.com` (missing protocol)
- Edge cases: URL with port, query params, fragments, Unicode domains
- Length limits: URLs at/exceeding 2048 characters
- Target: >=90% coverage (higher standard for shared utilities)

### Package 2: `env` Extensions - Environment Variable Patterns

**Location:** `azd-core/env/patterns.go`

**Functions:**
```go
// FilterByPrefix returns environment variables matching a prefix
func FilterByPrefix(envVars map[string]string, prefix string) map[string]string

// FilterByPrefixSlice returns KEY=VALUE pairs matching a prefix
func FilterByPrefixSlice(envSlice []string, prefix string) []string

// ExtractPattern extracts env vars matching prefix/suffix with key transformation
func ExtractPattern(envVars map[string]string, opts PatternOptions) map[string]string

// PatternOptions configures pattern extraction
type PatternOptions struct {
    Prefix      string                    // Required prefix (e.g., "SERVICE_")
    Suffix      string                    // Optional suffix (e.g., "_URL")
    TrimPrefix  bool                      // Remove prefix from result keys
    TrimSuffix  bool                      // Remove suffix from result keys
    Transform   func(string) string       // Optional key transformation
    Validator   func(string) bool         // Optional value validation
}

// NormalizeServiceName converts env var naming to service naming
// Converts: SERVICE_MY_API_NAME -> my-api-name
func NormalizeServiceName(envVarName string) string
```

**Use Cases:**
- Extract all `AZURE_*` environment variables
- Extract all `SERVICE_*_URL` variables for service discovery
- Extract `SERVICE_*_CUSTOM_DOMAIN` with service name normalization
- Filter environment for specific contexts (local, Azure, build)

**Test Coverage:**
- Prefix filtering: single/multiple matches, no matches
- Suffix filtering: combined prefix+suffix patterns
- Key transformation: underscore to hyphen, case conversion
- Value validation: skip invalid values, preserve valid
- Edge cases: empty maps, malformed KEY=VALUE, Unicode keys
- Target: >=85% coverage

### Package 3: String Utilities (DEFERRED to Phase 2)

**Note:** Based on current analysis, `NormalizeServiceName` fits better in `env` package. Standalone `strutil` package deferred until more string utilities emerge.

## Implementation Considerations

### `urlutil` Package Design

**Prefer stdlib over custom logic:**
```go
// ✅ GOOD - Uses stdlib
func Validate(rawURL string) error {
    rawURL = strings.TrimSpace(rawURL)
    if rawURL == "" {
        return fmt.Errorf("url cannot be empty")
    }
    
    parsed, err := neturl.Parse(rawURL)
    if err != nil {
        return fmt.Errorf("invalid URL format: %w", err)
    }
    
    if parsed.Scheme != "http" && parsed.Scheme != "https" {
        return fmt.Errorf("url must use http:// or https://, got: %s", parsed.Scheme)
    }
    
    if parsed.Host == "" {
        return fmt.Errorf("url missing host/domain")
    }
    
    if len(rawURL) > MaxURLLength {
        return fmt.Errorf("url exceeds maximum length of %d characters", MaxURLLength)
    }
    
    return nil
}

// ❌ BAD - Custom string prefix checks (azd-app current state)
func Validate(url string) error {
    if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
        return fmt.Errorf("url must start with http:// or https://")
    }
    // Missing: parse validation, host validation, length limits
}
```

**Constants:**
```go
const (
    MaxURLLength = 2048  // RFC 2616 practical limit
    DefaultHTTPPort = 80
    DefaultHTTPSPort = 443
)
```

### `env` Package Extensions

**Integration with existing:**
- Builds on existing `MapToSlice`, `SliceToMap` helpers
- Complements Key Vault resolution (separate concern)
- No breaking changes to existing API

**Performance:**
```go
// Efficient filtering with pre-allocation
func FilterByPrefix(envVars map[string]string, prefix string) map[string]string {
    result := make(map[string]string, len(envVars)/4) // Estimate
    prefixUpper := strings.ToUpper(prefix)
    for k, v := range envVars {
        if strings.HasPrefix(strings.ToUpper(k), prefixUpper) {
            result[k] = v
        }
    }
    return result
}
```

### Migration Strategy

**Phase 1: Create packages in azd-core**
1. Implement `urlutil` package with comprehensive tests (>=90% coverage)
2. Extend `env` package with pattern helpers (>=85% coverage)
3. Update azd-core README with examples and usage
4. Release azd-core v0.x.x with new utilities

**Phase 2: Migrate azd-app**
1. Update azd-app go.mod to latest azd-core version
2. Replace URL validation in `service/config.go` with `urlutil.Validate`
3. Replace env filtering in `serviceinfo/serviceinfo.go` with `env.FilterByPrefix`
4. Remove duplicated validation logic
5. Verify all azd-app tests pass (must maintain 100% pass rate)

**Phase 3: Adopt in azd-exec**
1. Update azd-exec for any URL validation needs
2. Use `env.FilterByPrefix` for extension environment isolation
3. Document patterns in azd-exec extension development guide

**Backward Compatibility:**
- azd-app can continue using current logic during migration
- No breaking changes to azd-core existing APIs
- Incremental adoption (can migrate one function at a time)

## Success Metrics

**Code Quality:**
- `urlutil` test coverage: >=90%
- `env` extensions test coverage: >=85%
- azd-core overall coverage: >=80% (maintain/improve current 72.5%)
- All tests pass: 100% pass rate in all repos

**Code Reduction:**
- azd-app: Remove 50-80 lines of URL validation duplication
- azd-app: Remove 80-120 lines of env filtering duplication
- Total: 130-200 lines of duplicated code eliminated

**Reusability:**
- azd-app: 3+ uses of `urlutil.Validate`
- azd-app: 5+ uses of `env.FilterByPrefix` or `env.ExtractPattern`
- azd-exec: 1+ uses of shared utilities
- Future extensions: Available for all new azd ecosystem tools

**Quality Improvement:**
- MQ ISSUE #1 resolved: URL validation uses `net/url.Parse`
- MQ ISSUE #8 resolved: Environment parsing duplication eliminated
- Consistent URL validation across entire azd ecosystem
- Single source of truth for common patterns

## Open Questions
- Should `ValidateHTTPSOnly` be strict (reject all HTTP) or allow localhost HTTP? **Resolved: Allow localhost HTTP, reject others**
- Should URL length limit be configurable or fixed at 2048? **Resolved: Fixed at 2048 (RFC standard)**
- Should `env.ExtractPattern` support regex patterns or just prefix/suffix? **Resolved: Start with prefix/suffix, add regex if needed**
- Should we add `urlutil.IsLocalhost(url)` helper? **Deferred to Phase 2 based on actual need**

## Security Considerations

**URL Validation Security:**
- Reject non-HTTP(S) protocols to prevent `javascript:`, `file:`, `data:` injection
- Validate host presence to prevent malformed URLs
- Length limits prevent DoS via extremely long URLs
- Use `net/url.Parse` for RFC-compliant parsing (prevents parsing bypasses)

**Environment Variable Security:**
- No exposure of sensitive values (utilities are read-only)
- Pattern matching is case-insensitive for keys (prevents bypass via case variation)
- Value validation callback allows filtering sensitive patterns

**Threat Model:**
- Malicious URLs in config files: Rejected by validation
- Path traversal via URL: Not a concern (URLs are display/navigation only)
- Env var injection: Not applicable (read-only utilities)

## Documentation

**README Updates:**
Add to `azd-core/README.md`:

```markdown
### `urlutil`
URL validation and parsing utilities with RFC-compliant validation.

**Key Functions:**
- `Validate` - Comprehensive HTTP/HTTPS URL validation
- `ValidateHTTPSOnly` - Enforce HTTPS-only for production
- `Parse` - Parse and normalize URLs

**Example:**
\`\`\`go
import "github.com/jongio/azd-core/urlutil"

// Validate custom URL from config
if err := urlutil.Validate(customURL); err != nil {
    return fmt.Errorf("invalid custom URL: %w", err)
}

// Parse and normalize
parsed, err := urlutil.Parse(userProvidedURL)
if err != nil {
    return err
}
fmt.Printf("Accessing: %s://%s\n", parsed.Scheme, parsed.Host)
\`\`\`
```

**Package Documentation:**
- Comprehensive package-level godoc with examples
- Function-level documentation with parameter details
- Example tests demonstrating common use cases
- Migration guide in `docs/migration-v0.x.md`

## Risks and Mitigations

| Risk | Impact | Likelihood | Mitigation |
|------|--------|------------|------------|
| Breaking changes in azd-core | High | Low | Additive-only changes, no modifications to existing APIs |
| azd-app migration introduces bugs | Medium | Medium | Comprehensive test coverage (100% pass rate), incremental migration |
| Performance regression from stdlib | Low | Low | `net/url.Parse` is highly optimized, benchmarks confirm |
| Incomplete migration leaves duplication | Low | Medium | Clear migration checklist, grep for old patterns, code review |

**Mitigation Plan:**
1. All new packages are additions (no breaking changes)
2. Incremental migration with test verification at each step
3. Performance benchmarks before/after migration
4. Code review checklist: grep for old validation patterns

## Timeline

**Week 1 (Phase 1):**
- Day 1-2: Implement `urlutil` package (functions + tests)
- Day 3: Extend `env` package (pattern helpers + tests)
- Day 4: Update documentation (README, package docs, examples)
- Day 5: Release azd-core v0.x.x

**Week 2 (Phase 2):**
- Day 1-2: Migrate azd-app URL validation
- Day 3: Migrate azd-app env filtering
- Day 4: Verify tests, coverage, performance
- Day 5: Code review and PR for azd-app

**Week 3 (Phase 3):**
- Optional: Adopt in azd-exec based on actual needs
- Documentation updates for extension patterns

**Total Effort:** 10-15 working days (2-3 weeks)
