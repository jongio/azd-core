// Package yamlutil provides utilities for manipulating YAML files while preserving
// formatting, comments, and structure. It uses text-based manipulation to guarantee
// zero data loss when updating YAML configuration files.
package yamlutil

import (
	"fmt"
	"os"
	"strings"

	"github.com/jongio/azd-core/security"
	"gopkg.in/yaml.v3"
)

// UpdateServiceLogsConfig updates the logs configuration for a specific service in azure.yaml.
// This preserves all comments, formatting, schema fields, and other content in the file.
// It only modifies the specific analytics configuration (tables or query) for the given service.
func UpdateServiceLogsConfig(azureYamlPath, serviceName string, tables []string, query string) error {
	if err := security.ValidatePath(azureYamlPath); err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// #nosec G304 -- Path validated by security.ValidatePath
	data, err := os.ReadFile(azureYamlPath)
	if err != nil {
		return fmt.Errorf("failed to read azure.yaml: %w", err)
	}

	content := string(data)

	// Parse YAML to verify service exists
	var azureYaml struct {
		Services map[string]any `yaml:"services"`
	}
	if parseErr := yaml.Unmarshal(data, &azureYaml); parseErr != nil {
		return fmt.Errorf("failed to parse azure.yaml: %w", parseErr)
	}

	if azureYaml.Services == nil {
		return fmt.Errorf("no services section found in azure.yaml")
	}

	if _, exists := azureYaml.Services[serviceName]; !exists {
		return fmt.Errorf("service '%s' not found in azure.yaml", serviceName)
	}

	updatedContent, err := updateLogsConfigInText(content, serviceName, tables, query)
	if err != nil {
		return err
	}

	// #nosec G703 -- Path validated by security.ValidatePath.
	if err := os.WriteFile(azureYamlPath, []byte(updatedContent), 0o600); err != nil {
		return fmt.Errorf("failed to write azure.yaml: %w", err)
	}

	return nil
}

// updateLogsConfigInText adds or updates the logs.analytics configuration in the service definition.
func updateLogsConfigInText(content, serviceName string, tables []string, query string) (string, error) {
	lines := strings.Split(content, "\n")

	servicesInfo, err := findSection(lines, "services")
	if err != nil {
		return "", fmt.Errorf("services section not found")
	}

	serviceInfo, err := FindServiceInSection(lines, servicesInfo, serviceName)
	if err != nil {
		return "", err
	}

	logs := findOrCreateLogsSection(&lines, serviceInfo)
	analytics := findOrCreateAnalyticsSection(&lines, logs)

	// Remove old tables/query and add new ones
	updateAnalyticsContent(&lines, analytics, tables, query)

	return strings.Join(lines, "\n"), nil
}

// logsSection holds information about the logs section and whether it was created.
type logsSection struct {
	idx     int
	indent  string
	created bool
}

// analyticsSection holds information about the analytics section and whether it was created.
type analyticsSection struct {
	idx     int
	indent  string
	created bool
}

// findOrCreateLogsSection finds or creates the logs section within a service.
func findOrCreateLogsSection(lines *[]string, serviceInfo *serviceInfo) logsSection {
	serviceIndent := serviceInfo.indent
	logsIndent := serviceIndent

	lastPropertyIdx := serviceInfo.lineIdx

	for i := serviceInfo.lineIdx + 1; i < len(*lines); i++ {
		line := (*lines)[i]
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		lineIndent := getIndentation(line)

		if len(lineIndent) < len(serviceIndent) {
			break
		}

		if len(lineIndent) == len(serviceIndent) {
			if strings.HasPrefix(trimmed, "logs:") {
				return logsSection{idx: i, indent: lineIndent, created: false}
			}
			lastPropertyIdx = i
		} else if len(lineIndent) > len(serviceIndent) {
			lastPropertyIdx = i
		}
	}

	// Not found, so insert after the last property
	insertIdx := lastPropertyIdx + 1
	logsLine := logsIndent + "logs:"

	result := make([]string, 0, len(*lines)+1)
	result = append(result, (*lines)[:insertIdx]...)
	result = append(result, logsLine)
	result = append(result, (*lines)[insertIdx:]...)
	*lines = result

	return logsSection{idx: insertIdx, indent: logsIndent, created: true}
}

// findOrCreateAnalyticsSection finds or creates the analytics section within logs.
func findOrCreateAnalyticsSection(lines *[]string, logs logsSection) analyticsSection {
	analyticsIndent := logs.indent + "    "

	for i := logs.idx + 1; i < len(*lines); i++ {
		line := (*lines)[i]
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		lineIndent := getIndentation(line)

		if len(lineIndent) <= len(logs.indent) {
			break
		}

		if strings.HasPrefix(trimmed, "analytics:") {
			return analyticsSection{idx: i, indent: lineIndent, created: false}
		}
	}

	// Not found, so insert after logs:
	insertIdx := logs.idx + 1
	analyticsLine := logs.indent + "    analytics:"

	result := make([]string, 0, len(*lines)+1)
	result = append(result, (*lines)[:insertIdx]...)
	result = append(result, analyticsLine)
	result = append(result, (*lines)[insertIdx:]...)
	*lines = result

	return analyticsSection{idx: insertIdx, indent: analyticsIndent, created: true}
}

// updateAnalyticsContent removes old tables/query and adds new ones.
func updateAnalyticsContent(lines *[]string, analytics analyticsSection, tables []string, query string) {
	startIdx := analytics.idx + 1
	endIdx := startIdx

	foundContent := false
	for i := startIdx; i < len(*lines); i++ {
		line := (*lines)[i]
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		lineIndent := getIndentation(line)

		if len(lineIndent) <= len(analytics.indent) {
			endIdx = i
			break
		}

		foundContent = true
		endIdx = i + 1
	}

	if !foundContent && endIdx != startIdx {
		endIdx = startIdx
	}

	var newContent []string
	contentIndent := analytics.indent + "    "

	if len(tables) > 0 {
		newContent = append(newContent, contentIndent+"tables:")
		for _, table := range tables {
			newContent = append(newContent, contentIndent+fmt.Sprintf("    - %s", table))
		}
	} else if query != "" {
		newContent = append(newContent, contentIndent+"query: |")
		// Split query into lines and indent each
		queryLines := strings.Split(strings.TrimSpace(query), "\n")
		for _, qLine := range queryLines {
			newContent = append(newContent, contentIndent+"    "+qLine)
		}
	}

	// Replace old content with new
	result := make([]string, 0, len(*lines)-endIdx+startIdx+len(newContent))
	result = append(result, (*lines)[:startIdx]...)
	result = append(result, newContent...)
	result = append(result, (*lines)[endIdx:]...)
	*lines = result
}
