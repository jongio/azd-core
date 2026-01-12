package urlutil

import (
	"fmt"
	neturl "net/url"
	"strings"
)

const (
	// MaxURLLength is the RFC 2616 practical limit for URL length
	MaxURLLength = 2048
)

// Validate performs comprehensive HTTP/HTTPS URL validation using net/url.Parse.
// It validates that the URL:
//   - Is not empty or only whitespace
//   - Uses http:// or https:// protocol
//   - Has a valid host/domain
//   - Does not exceed MaxURLLength (2048 characters)
//   - Can be parsed by net/url.Parse (RFC 3986 compliant)
//
// Returns an error with context if validation fails.
//
// Example:
//
//	if err := urlutil.Validate("https://example.com"); err != nil {
//		return fmt.Errorf("invalid URL: %w", err)
//	}
func Validate(rawURL string) error {
	// Trim whitespace
	rawURL = strings.TrimSpace(rawURL)
	
	// Check for empty URL
	if rawURL == "" {
		return fmt.Errorf("url cannot be empty")
	}
	
	// Check length limit
	if len(rawURL) > MaxURLLength {
		return fmt.Errorf("url exceeds maximum length of %d characters", MaxURLLength)
	}
	
	// Parse URL using stdlib
	parsed, err := neturl.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}
	
	// Validate protocol (http or https only)
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		if parsed.Scheme == "" {
			return fmt.Errorf("url must use http:// or https://")
		}
		return fmt.Errorf("url must use http:// or https://, got: %s", parsed.Scheme)
	}
	
	// Validate host presence
	if parsed.Host == "" {
		return fmt.Errorf("url missing host/domain")
	}
	
	return nil
}

// ValidateHTTPSOnly enforces HTTPS-only URLs for production use.
// It allows HTTP for localhost (127.0.0.1, ::1, localhost) for local development,
// but rejects all other HTTP URLs.
//
// This is useful for production environments where encrypted connections are required,
// while still allowing local development workflows.
//
// Example:
//
//	if err := urlutil.ValidateHTTPSOnly(apiEndpoint); err != nil {
//		return fmt.Errorf("production endpoint must use HTTPS: %w", err)
//	}
func ValidateHTTPSOnly(rawURL string) error {
	// First perform standard validation
	if err := Validate(rawURL); err != nil {
		return err
	}
	
	// Parse URL (we know it's valid from Validate)
	parsed, _ := neturl.Parse(strings.TrimSpace(rawURL))
	
	// Allow HTTPS
	if parsed.Scheme == "https" {
		return nil
	}
	
	// Allow HTTP for localhost
	if parsed.Scheme == "http" && isLocalhost(parsed.Hostname()) {
		return nil
	}
	
	// Reject all other HTTP URLs
	return fmt.Errorf("url must use https:// (http:// only allowed for localhost)")
}

// Parse parses and normalizes URLs with trimming and validation.
// It returns a *url.URL if the URL is valid, or an error if validation fails.
//
// This is a convenience wrapper around Validate and net/url.Parse.
//
// Example:
//
//	parsed, err := urlutil.Parse(userInput)
//	if err != nil {
//		return err
//	}
//	fmt.Printf("Host: %s\n", parsed.Host)
func Parse(rawURL string) (*neturl.URL, error) {
	// Validate first
	if err := Validate(rawURL); err != nil {
		return nil, err
	}
	
	// Parse (we know it's valid)
	return neturl.Parse(strings.TrimSpace(rawURL))
}

// NormalizeScheme ensures URL has http:// or https:// prefix.
// If the URL already has a valid scheme (http:// or https://), it is returned unchanged.
// If the URL has no scheme or an invalid scheme, the defaultScheme is prepended.
//
// The defaultScheme should be either "http" or "https" (without "://").
//
// Example:
//
//	normalized := urlutil.NormalizeScheme("example.com", "https")
//	// Returns: "https://example.com"
//
//	normalized = urlutil.NormalizeScheme("http://example.com", "https")
//	// Returns: "http://example.com" (already has valid scheme)
func NormalizeScheme(rawURL, defaultScheme string) string {
	rawURL = strings.TrimSpace(rawURL)
	
	// Try to parse the URL
	parsed, err := neturl.Parse(rawURL)
	if err != nil {
		// If parsing fails, prepend default scheme
		return defaultScheme + "://" + rawURL
	}
	
	// If it has a valid http/https scheme, return as-is
	if parsed.Scheme == "http" || parsed.Scheme == "https" {
		return rawURL
	}
	
	// Otherwise, prepend default scheme
	return defaultScheme + "://" + rawURL
}

// isLocalhost checks if the hostname is a localhost address
func isLocalhost(hostname string) bool {
	// Normalize to lowercase for comparison
	hostname = strings.ToLower(hostname)
	
	// Check common localhost names and IPs
	return hostname == "localhost" ||
		hostname == "127.0.0.1" ||
		hostname == "::1" ||
		hostname == "[::1]"
}
