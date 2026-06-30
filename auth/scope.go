package auth

import (
	"fmt"
	"net/url"
	"strings"
)

const (
	managementScope = "https://management.azure.com/.default"
	graphScope      = "https://graph.microsoft.com/.default"
	devOpsScope     = "499b84ac-1321-427f-aa17-267ca6975798/.default"
	serviceBusScope = "https://servicebus.azure.net/.default"
	eventHubsScope  = "https://eventhubs.azure.net/.default"
	keyVaultScope   = "https://vault.azure.net/.default"
	storageScope    = "https://storage.azure.com/.default"
	ossRdbmsScope   = "https://ossrdbms-aad.database.windows.net/.default"
)

// DetectScope analyzes a URL and returns the appropriate Azure OAuth scope.
// Returns empty string when the hostname does not match a known Azure service.
func DetectScope(urlString string) (string, error) {
	parsedURL, err := url.Parse(urlString)
	if err != nil {
		return "", fmt.Errorf("failed to parse URL: %w", err)
	}

	host := strings.ToLower(parsedURL.Hostname())
	if host == "" {
		return "", nil
	}

	path := parsedURL.EscapedPath()

	exactMatches := map[string]string{
		"management.azure.com": managementScope,
		"graph.microsoft.com":  graphScope,
		"api.loganalytics.io":  "https://api.loganalytics.io/.default",
		"dev.azure.com":        devOpsScope,
	}

	if scope, ok := exactMatches[host]; ok {
		return scope, nil
	}

	if strings.HasSuffix(host, ".visualstudio.com") {
		return devOpsScope, nil
	}

	if strings.HasSuffix(host, ".kusto.windows.net") {
		return fmt.Sprintf("https://%s/.default", host), nil
	}

	if strings.HasSuffix(host, ".servicebus.windows.net") {
		if strings.Contains(path, "/queue") || strings.Contains(path, "/queues") {
			return serviceBusScope, nil
		}
		return eventHubsScope, nil
	}

	suffixMatches := map[string]string{
		".vault.azure.net":             keyVaultScope,
		".blob.core.windows.net":       storageScope,
		".queue.core.windows.net":      storageScope,
		".table.core.windows.net":      storageScope,
		".file.core.windows.net":       storageScope,
		".dfs.core.windows.net":        storageScope,
		".azurecr.io":                  "https://containerregistry.azure.net/.default",
		".documents.azure.com":         "https://cosmos.azure.com/.default",
		".azconfig.io":                 "https://azconfig.io/.default",
		".batch.azure.com":             "https://batch.core.windows.net/.default",
		".postgres.database.azure.com": ossRdbmsScope,
		".mysql.database.azure.com":    ossRdbmsScope,
		".mariadb.database.azure.com":  ossRdbmsScope,
		".database.windows.net":        "https://database.windows.net/.default",
		".dev.azuresynapse.net":        "https://dev.azuresynapse.net/.default",
		".azuredatalakestore.net":      "https://datalake.azure.net/.default",
		".media.azure.net":             "https://rest.media.azure.net/.default",
	}

	for suffix, scope := range suffixMatches {
		if strings.HasSuffix(host, suffix) {
			return scope, nil
		}
	}

	return "", nil
}

// IsAzureHost checks if a hostname appears to be an Azure service
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
