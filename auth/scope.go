package auth

import (
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/azure/azure-dev/cli/azd/pkg/azdext"
)

const (
	serviceBusScope = "https://servicebus.azure.net/.default"
	eventHubsScope  = "https://eventhubs.azure.net/.default"
	ossRdbmsScope   = "https://ossrdbms-aad.database.windows.net/.default"
	storageScope    = "https://storage.azure.com/.default"
)

// extraScopeRules are the host to scope mappings azdext.ScopeDetector does not
// carry, plus the two where azd-core has always resolved a different scope than
// the SDK would.
//
// Custom rules are evaluated before the SDK defaults, so an entry here wins.
// A leading dot means suffix match; without one the host must match exactly.
var extraScopeRules = map[string]string{
	// Absent from azdext.
	"api.loganalytics.io":         "https://api.loganalytics.io/.default",
	".batch.azure.com":            "https://batch.core.windows.net/.default",
	".mariadb.database.azure.com": ossRdbmsScope,
	".database.windows.net":       "https://database.windows.net/.default",
	".dev.azuresynapse.net":       "https://dev.azuresynapse.net/.default",
	".azuredatalakestore.net":     "https://datalake.azure.net/.default",
	".media.azure.net":            "https://rest.media.azure.net/.default",

	// azdext maps a container registry to the Resource Manager scope, which is
	// the scope for the ACR refresh token exchange rather than for the registry
	// data plane. azd-core has always issued the data plane scope directly.
	".azurecr.io": "https://containerregistry.azure.net/.default",

	// azdext has no rule for these two storage endpoints.
	".table.core.windows.net": storageScope,
	".file.core.windows.net":  storageScope,
}

var (
	scopeDetectorOnce sync.Once
	scopeDetector     *azdext.ScopeDetector
)

// sharedScopeDetector builds the detector once. Rule construction walks a map
// and allocates a closure per entry, and DetectScope is called per request.
func sharedScopeDetector() *azdext.ScopeDetector {
	scopeDetectorOnce.Do(func() {
		scopeDetector = azdext.NewScopeDetector(&azdext.ScopeDetectorOptions{
			CustomRules: extraScopeRules,
		})
	})

	return scopeDetector
}

// DetectScope analyzes a URL and returns the appropriate Azure OAuth scope.
// Returns empty string when the hostname does not match a known Azure service.
//
// Most of the mapping comes from azdext.ScopeDetector, extended with
// extraScopeRules. Two services cannot be expressed as a static host to scope
// pair and are resolved here instead:
//
// Azure Data Explorer issues a token for the cluster itself, so the scope is
// derived from the host rather than looked up.
//
// A Service Bus and an Event Hubs namespace share the servicebus.windows.net
// suffix and are told apart by the request path. azdext resolves the suffix to
// Event Hubs unconditionally, which would hand a Service Bus queue call a token
// for the wrong audience.
func DetectScope(urlString string) (string, error) {
	parsedURL, err := url.Parse(urlString)
	if err != nil {
		return "", fmt.Errorf("failed to parse URL: %w", err)
	}

	host := strings.ToLower(parsedURL.Hostname())
	if host == "" {
		return "", nil
	}

	if strings.HasSuffix(host, ".kusto.windows.net") {
		return fmt.Sprintf("https://%s/.default", host), nil
	}

	if strings.HasSuffix(host, ".servicebus.windows.net") {
		path := parsedURL.EscapedPath()
		if strings.Contains(path, "/queue") || strings.Contains(path, "/queues") {
			return serviceBusScope, nil
		}

		return eventHubsScope, nil
	}

	scopes, err := sharedScopeDetector().ScopesForURL(urlString)
	if err != nil {
		// An unmapped host is not an error here. Callers treat the empty scope
		// as "send this request unauthenticated", which is how a request to a
		// non-Azure host has always been handled.
		return "", nil //nolint:nilerr // an unmapped host is not an error, see above
	}

	return scopes[0], nil
}

// IsAzureHost checks if a hostname appears to be an Azure service.
//
// This is deliberately broader than DetectScope: it answers "should we try to
// authenticate at all", while DetectScope answers "with which audience". A host
// can be recognizably Azure without azd-core knowing its scope.
func IsAzureHost(urlString string) bool {
	parsedURL, err := url.Parse(urlString)
	if err != nil {
		return false
	}

	host := strings.ToLower(parsedURL.Hostname())

	azurePatterns := []string{
		".azure.com",
		".azure.net",
		".windows.net",
		".azurecr.io",
		".azconfig.io",
		"management.azure.com",
		"graph.microsoft.com",
		"dev.azure.com",
		".visualstudio.com",
		".azuredatalakestore.net",
	}

	for _, pattern := range azurePatterns {
		if strings.Contains(host, pattern) || host == strings.TrimPrefix(pattern, ".") {
			return true
		}
	}

	return false
}
