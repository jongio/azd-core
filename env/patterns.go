package env

import (
	"strings"
)

// FilterByPrefix returns environment variables matching a prefix.
// The prefix matching is case-insensitive for keys.
// Returns a new map containing only the matching entries.
//
// Example:
//
//	envVars := map[string]string{
//		"AZURE_TENANT_ID": "xyz",
//		"AZURE_CLIENT_ID": "abc",
//		"DATABASE_URL": "postgres://...",
//	}
//	azureVars := env.FilterByPrefix(envVars, "AZURE_")
//	// Returns: {"AZURE_TENANT_ID": "xyz", "AZURE_CLIENT_ID": "abc"}
func FilterByPrefix(envVars map[string]string, prefix string) map[string]string {
	if envVars == nil {
		return map[string]string{}
	}

	result := make(map[string]string)
	prefixUpper := strings.ToUpper(prefix)

	for k, v := range envVars {
		if strings.HasPrefix(strings.ToUpper(k), prefixUpper) {
			result[k] = v
		}
	}

	return result
}

// FilterByPrefixSlice returns KEY=VALUE pairs matching a prefix.
// The prefix matching is case-insensitive for keys.
// Returns a new slice containing only the matching entries.
// Malformed entries (without "=") are skipped.
//
// Example:
//
//	envSlice := []string{
//		"AZURE_TENANT_ID=xyz",
//		"AZURE_CLIENT_ID=abc",
//		"DATABASE_URL=postgres://...",
//	}
//	azureVars := env.FilterByPrefixSlice(envSlice, "AZURE_")
//	// Returns: ["AZURE_TENANT_ID=xyz", "AZURE_CLIENT_ID=abc"]
func FilterByPrefixSlice(envSlice []string, prefix string) []string {
	if envSlice == nil {
		return []string{}
	}

	result := make([]string, 0)
	prefixUpper := strings.ToUpper(prefix)

	for _, envVar := range envSlice {
		parts := strings.SplitN(envVar, "=", 2)
		if len(parts) != 2 {
			continue
		}

		if strings.HasPrefix(strings.ToUpper(parts[0]), prefixUpper) {
			result = append(result, envVar)
		}
	}

	return result
}

// PatternOptions configures pattern-based environment variable extraction.
type PatternOptions struct {
	// Prefix is the required prefix for keys (e.g., "SERVICE_")
	Prefix string

	// Suffix is the optional suffix for keys (e.g., "_URL")
	Suffix string

	// TrimPrefix removes the prefix from result keys if true
	TrimPrefix bool

	// TrimSuffix removes the suffix from result keys if true
	TrimSuffix bool

	// Transform is an optional key transformation function applied after trimming
	// Example: func(s string) string { return strings.ToLower(s) }
	Transform func(string) string

	// Validator is an optional value validation function
	// If provided, only entries where Validator(value) returns true are included
	// Example: func(v string) bool { return v != "" }
	Validator func(string) bool
}

// ExtractPattern extracts environment variables matching prefix/suffix with key transformation.
// Pattern matching is case-insensitive. Returns a new map with transformed keys.
//
// Example:
//
//	envVars := map[string]string{
//		"SERVICE_API_URL": "https://api.example.com",
//		"SERVICE_WEB_URL": "https://web.example.com",
//		"SERVICE_DB_HOST": "db.example.com",
//	}
//
//	// Extract all SERVICE_*_URL variables and normalize service names
//	urls := env.ExtractPattern(envVars, env.PatternOptions{
//		Prefix:     "SERVICE_",
//		Suffix:     "_URL",
//		TrimPrefix: true,
//		TrimSuffix: true,
//		Transform:  func(s string) string { return strings.ToLower(strings.ReplaceAll(s, "_", "-")) },
//	})
//	// Returns: {"api": "https://api.example.com", "web": "https://web.example.com"}
func ExtractPattern(envVars map[string]string, opts PatternOptions) map[string]string {
	if envVars == nil {
		return map[string]string{}
	}

	result := make(map[string]string)
	prefixUpper := strings.ToUpper(opts.Prefix)
	suffixUpper := strings.ToUpper(opts.Suffix)

	for k, v := range envVars {
		keyUpper := strings.ToUpper(k)

		// Check prefix match
		if opts.Prefix != "" && !strings.HasPrefix(keyUpper, prefixUpper) {
			continue
		}

		// Check suffix match
		if opts.Suffix != "" && !strings.HasSuffix(keyUpper, suffixUpper) {
			continue
		}

		// Validate value if validator provided
		if opts.Validator != nil && !opts.Validator(v) {
			continue
		}

		// Transform key
		resultKey := k

		// Trim prefix (case-sensitive trim after case-insensitive match)
		if opts.TrimPrefix && opts.Prefix != "" {
			// Find actual prefix in original key
			if len(k) >= len(opts.Prefix) && strings.EqualFold(k[:len(opts.Prefix)], opts.Prefix) {
				resultKey = k[len(opts.Prefix):]
			}
		}

		// Trim suffix (case-sensitive trim after case-insensitive match)
		if opts.TrimSuffix && opts.Suffix != "" {
			// Find actual suffix in original key
			if len(resultKey) >= len(opts.Suffix) && strings.EqualFold(resultKey[len(resultKey)-len(opts.Suffix):], opts.Suffix) {
				resultKey = resultKey[:len(resultKey)-len(opts.Suffix)]
			}
		}

		// Apply transform function
		if opts.Transform != nil {
			resultKey = opts.Transform(resultKey)
		}

		result[resultKey] = v
	}

	return result
}

// NormalizeServiceName converts environment variable naming to service naming.
// Converts uppercase underscore-separated names to lowercase hyphen-separated names.
// Commonly used with ExtractPattern to normalize service names from environment variables.
//
// Example:
//
//	name := env.NormalizeServiceName("MY_API_SERVICE")
//	// Returns: "my-api-service"
//
//	name = env.NormalizeServiceName("WEB_APP")
//	// Returns: "web-app"
func NormalizeServiceName(envVarName string) string {
	// Convert to lowercase and replace underscores with hyphens
	normalized := strings.ToLower(envVarName)
	normalized = strings.ReplaceAll(normalized, "_", "-")
	return normalized
}
