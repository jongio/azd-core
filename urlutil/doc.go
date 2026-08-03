// Package urlutil provides URL validation and parsing utilities with RFC-compliant validation.
//
// This package makes it easy for consumers (like azd-app and azd-exec) to validate
// and parse HTTP/HTTPS URLs with comprehensive validation rules. It uses the standard
// library's net/url.Parse for robust parsing and adds additional security and validation
// checks such as protocol restrictions, host validation, and length limits.
//
// # Usage
//
// Use Validate for comprehensive HTTP/HTTPS URL validation:
//
//	import "github.com/jongio/azd-core/urlutil"
//
//	// Validate custom URL from config
//	if err := urlutil.Validate(customURL); err != nil {
//		return fmt.Errorf("invalid custom URL: %w", err)
//	}
//
// Use ValidateHTTPSOnly for production environments requiring HTTPS:
//
//	// Enforce HTTPS-only (allows localhost HTTP for development)
//	if err := urlutil.ValidateHTTPSOnly(apiEndpoint); err != nil {
//		return fmt.Errorf("API endpoint must use HTTPS: %w", err)
//	}
//
// Use Parse to parse and normalize URLs:
//
//	// Parse and normalize user-provided URL
//	parsed, err := urlutil.Parse(userProvidedURL)
//	if err != nil {
//		return err
//	}
//	fmt.Printf("Accessing: %s://%s\n", parsed.Scheme, parsed.Host)
//
// Use NormalizeScheme to ensure URLs have proper protocols:
//
//	// Add default https:// if missing
//	normalized := urlutil.NormalizeScheme("example.com", "https")
//	// Returns: "https://example.com"
//
// # Validation Rules
//
// The validation functions enforce the following rules:
//   - URL must not be empty or only whitespace
//   - URL must use http:// or https:// protocol (rejects ftp://, file://, javascript://, etc.)
//   - URL must have a valid host/domain (rejects "http://", "https://")
//   - URL must not exceed 2048 characters (RFC 2616 practical limit)
//   - URL must be parseable by net/url.Parse (RFC 3986 compliant)
//
// # Security Considerations
//
// This package helps prevent common security issues:
//   - Protocol validation prevents javascript:, file:, and data: URL injection
//   - Host validation prevents malformed URLs that could bypass security checks
//   - Length limits prevent DoS via extremely long URLs
//   - Uses net/url.Parse for RFC-compliant parsing (prevents parsing bypasses)
//
// For production environments, use ValidateHTTPSOnly to enforce encrypted connections,
// while still allowing localhost HTTP for local development.
//
// # Relationship to azdext.SSRFGuard
//
// This package is not an SSRF control and does not replace one. Everything
// here is syntactic and offline: it inspects the URL string and never resolves
// a name or touches the network. [github.com/azure/azure-dev/cli/azd/pkg/azdext.SSRFGuard]
// is the SSRF control. It resolves the host and rejects cloud metadata
// endpoints, RFC 1918 ranges, loopback, link-local, CGNAT, and the IPv6
// transition ranges that can embed a private IPv4 address.
//
// They answer different questions and are meant to be layered, not swapped:
//
//   - Validating a URL that a user typed into config, which routinely points
//     at localhost or an internal host, use this package. An SSRF guard would
//     reject those by design, and would perform DNS lookups while parsing
//     config.
//   - Before issuing an outbound request to a URL you did not author, use
//     azdext.SSRFGuard. Syntactic validation cannot tell you where a name
//     resolves.
package urlutil
