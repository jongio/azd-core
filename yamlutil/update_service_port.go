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

// UpdateServicePort adds or updates the ports field for a specific service in azure.yaml.
// This preserves all comments, formatting, and other content in the file.
// The port is added as a single-element ports array: ports: ["8080"]
func UpdateServicePort(azureYamlPath, serviceName string, port int) error {
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

	updatedContent, err := updateServicePortsInText(content, serviceName, port)
	if err != nil {
		return err
	}

	// #nosec G703 -- Path validated by security.ValidatePath.
	if err := os.WriteFile(azureYamlPath, []byte(updatedContent), 0600); err != nil {
		return fmt.Errorf("failed to write azure.yaml: %w", err)
	}

	return nil
}

// updateServicePortsInText adds or updates the ports field in the service definition.
func updateServicePortsInText(content, serviceName string, port int) (string, error) {
	lines := strings.Split(content, "\n")

	servicesInfo, err := findSection(lines, "services")
	if err != nil {
		return "", fmt.Errorf("services section not found")
	}

	serviceInfo, err := FindServiceInSection(lines, servicesInfo, serviceName)
	if err != nil {
		return "", err
	}

	portsLineIdx, portsIndent := findPortsLine(lines, serviceInfo)

	portsLine := fmt.Sprintf("%sports:", portsIndent)
	portValueLine := fmt.Sprintf("%s  - \"%d\"", portsIndent, port)

	if portsLineIdx >= 0 {
		// Inline array format (ports: ["3000"]) — replace entire line
		currentPortsLine := lines[portsLineIdx]
		if strings.Contains(currentPortsLine, "[") {
			lineIndent := getIndentation(currentPortsLine)
			lines[portsLineIdx] = fmt.Sprintf("%sports: [\"%d\"]", lineIndent, port)
			return strings.Join(lines, "\n"), nil
		}

		// Multi-line ports array — replace first port value
		for i := portsLineIdx + 1; i < len(lines); i++ {
			line := lines[i]
			trimmed := strings.TrimSpace(line)
			lineIndent := getIndentation(line)

			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}

			if len(lineIndent) <= len(portsIndent) {
				break
			}

			// If this is an array item, update it
			if strings.HasPrefix(trimmed, "-") {
				lines[i] = portValueLine
				return strings.Join(lines, "\n"), nil
			}
		}
		// Ports array exists but has no items - add one
		result := make([]string, 0, len(lines)+1)
		result = append(result, lines[:portsLineIdx+1]...)
		result = append(result, portValueLine)
		result = append(result, lines[portsLineIdx+1:]...)
		lines = result
	} else {
		// No existing ports field — insert after service name
		insertIdx := serviceInfo.lineIdx + 1

		result := make([]string, 0, len(lines)+2)
		result = append(result, lines[:insertIdx]...)
		result = append(result, portsLine)
		result = append(result, portValueLine)
		result = append(result, lines[insertIdx:]...)
		lines = result
	}

	return strings.Join(lines, "\n"), nil
}

// serviceInfo holds information about a service location in YAML.
type serviceInfo struct {
	lineIdx int    // Line index where the service name appears
	indent  string // Indentation of the service properties
}

// FindServiceInSection finds a specific service within the services section.
// Exported for use by other yamlutil functions.
func FindServiceInSection(lines []string, servicesInfo *sectionInfo, serviceName string) (*serviceInfo, error) {
	searchKey := serviceName + ":"

	// Detect actual service-level indentation from the first service entry
	var serviceIndent string
	for i := servicesInfo.lineIdx + 1; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		lineIndent := getIndentation(line)

		// Check indentation — if at or before services indent, we've left the section
		if len(lineIndent) <= len(servicesInfo.indent) {
			break
		}

		if len(lineIndent) > len(servicesInfo.indent) {
			serviceIndent = lineIndent
			break
		}
	}

	// If we couldn't detect service indent, fall back to default
	if serviceIndent == "" {
		serviceIndent = servicesInfo.indent + "  "
	}

	// Now find the specific service
	for i := servicesInfo.lineIdx + 1; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		lineIndent := getIndentation(line)
		if len(lineIndent) <= len(servicesInfo.indent) {
			break
		}

		if len(lineIndent) == len(serviceIndent) && (trimmed == searchKey || strings.HasPrefix(trimmed, searchKey+" ")) {
			// Calculate property indent (same delta as service indent from services indent)
			indentDelta := len(serviceIndent) - len(servicesInfo.indent)
			if indentDelta < 0 {
				return nil, fmt.Errorf("malformed YAML: service indent (%d) shorter than services section indent (%d)", len(serviceIndent), len(servicesInfo.indent))
			}
			propertyIndent := serviceIndent + strings.Repeat(" ", indentDelta)
			return &serviceInfo{
				lineIdx: i,
				indent:  propertyIndent,
			}, nil
		}
	}

	return nil, fmt.Errorf("service '%s' not found in services section", serviceName)
}

// findPortsLine looks for an existing ports field in the service definition.
func findPortsLine(lines []string, serviceInfo *serviceInfo) (int, string) {
	for i := serviceInfo.lineIdx + 1; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		lineIndent := getIndentation(line)

		if len(lineIndent) < len(serviceInfo.indent) {
			break
		}

		if len(lineIndent) == len(serviceInfo.indent) {
			// Check if this is the ports line
			if strings.HasPrefix(trimmed, "ports:") {
				return i, lineIndent
			}
		}
	}

	// Ports not found, return indent for insertion
	return -1, serviceInfo.indent
}
