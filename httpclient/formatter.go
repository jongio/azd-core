package httpclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
)

// OutputFormat represents the output format type
type OutputFormat string

// Output format values.
const (
	FormatAuto    OutputFormat = "auto"
	FormatJSON    OutputFormat = "json"
	FormatRaw     OutputFormat = "raw"
	redactedValue              = "***REDACTED***"
	bearerPrefix               = "Bearer "
)

// Formatter handles response formatting and output
type Formatter struct {
	verbose bool
	format  OutputFormat
}

// NewFormatter creates a new formatter
func NewFormatter(verbose bool, format string) *Formatter {
	outputFormat := FormatAuto
	if format != "" {
		outputFormat = OutputFormat(format)
	}

	return &Formatter{
		verbose: verbose,
		format:  outputFormat,
	}
}

// Format formats the response for output
func (f *Formatter) Format(resp *Response) (string, error) {
	var output strings.Builder

	// Verbose mode - show headers and timing
	if f.verbose {
		fmt.Fprintf(&output, "< %s\n", resp.Status)
		fmt.Fprintf(&output, "< Duration: %v\n", resp.Duration)
		output.WriteString("< \n")
		output.WriteString("< Response Headers:\n")
		for key, values := range resp.Headers {
			for _, value := range values {
				value = RedactSensitiveHeader(key, value)
				fmt.Fprintf(&output, "<   %s: %s\n", key, value)
			}
		}
		output.WriteString("< \n")
		output.WriteString("< \n")
	}

	// Format body
	body := resp.Body
	contentType := resp.Headers.Get("Content-Type")

	format := f.format
	if format == FormatAuto {
		if strings.Contains(contentType, "application/json") {
			format = FormatJSON
		} else {
			format = FormatRaw
		}
	}

	switch format {
	case FormatJSON:
		formatted, err := f.formatJSON(body)
		if err != nil {
			output.Write(body)
		} else {
			output.WriteString(formatted)
		}
	case FormatRaw:
		output.Write(body)
	default:
		output.Write(body)
	}

	return output.String(), nil
}

// formatJSON pretty-prints JSON
func (f *Formatter) formatJSON(data []byte) (string, error) {
	var parsed any
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", err
	}

	formatted, err := json.MarshalIndent(parsed, "", "  ")
	if err != nil {
		return "", err
	}

	return string(formatted), nil
}

// WriteOutput writes the formatted output to the appropriate destination
func (f *Formatter) WriteOutput(output string, outputFile string) error {
	if outputFile != "" {
		return os.WriteFile(outputFile, []byte(output), 0o600)
	}

	fmt.Print(output)
	return nil
}

// WriteRawOutput writes raw bytes to a file or stdout
func (f *Formatter) WriteRawOutput(data []byte, outputFile string) error {
	if outputFile != "" {
		return os.WriteFile(outputFile, data, 0o600)
	}

	_, err := io.Copy(os.Stdout, bytes.NewReader(data))
	return err
}

// RedactSensitiveHeader redacts sensitive header values
func RedactSensitiveHeader(key, value string) string {
	keyLower := strings.ToLower(key)

	if keyLower == "authorization" {
		if strings.HasPrefix(strings.ToLower(value), "bearer ") {
			token := strings.TrimPrefix(value, bearerPrefix)
			token = strings.TrimPrefix(token, "bearer ")
			if len(token) > 12 {
				return bearerPrefix + token[:6] + "..." + token[len(token)-6:]
			}
			return bearerPrefix + redactedValue
		}
		return redactedValue
	}

	sensitiveHeaders := []string{
		"x-api-key",
		"x-auth-token",
		"cookie",
		"set-cookie",
		"x-csrf-token",
	}

	for _, sensitive := range sensitiveHeaders {
		if keyLower == sensitive {
			if len(value) > 12 {
				return value[:6] + "..." + value[len(value)-6:]
			}
			return redactedValue
		}
	}

	return value
}

// sensitiveQueryKeys lists URL query parameter names that may contain secrets.
// Values for these keys are replaced with redactedValue in verbose logging.
var sensitiveQueryKeys = map[string]bool{
	"sig":          true,
	"token":        true,
	"access_token": true,
	"api_key":      true,
	"api-key":      true,
	"apikey":       true,
	"key":          true,
	"secret":       true,
	"password":     true,
	"sas":          true,
	"se":           true, // SAS expiry
	"sp":           true, // SAS permissions
	"st":           true, // SAS start
	"sv":           true, // SAS version
	"sr":           true, // SAS resource
	"spr":          true, // SAS protocol
	"skt":          true, // SAS key start
	"ske":          true, // SAS key expiry
	"skv":          true, // SAS key version
	"sdd":          true, // SAS directory depth
	"sip":          true, // SAS IP range
	"si":           true, // SAS identifier
}

// RedactURL redacts sensitive query parameters from a URL string.
// It replaces values of known secret-bearing query keys with [REDACTED].
// If the URL cannot be parsed, the original URL is returned unchanged.
func RedactURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.RawQuery == "" {
		return rawURL
	}

	// Rebuild query string preserving original order and encoding
	parts := strings.Split(parsed.RawQuery, "&")
	redacted := false
	for i, part := range parts {
		eqIdx := strings.IndexByte(part, '=')
		if eqIdx < 0 {
			continue
		}
		key := part[:eqIdx]
		// Decode the key for case-insensitive lookup
		decodedKey, decErr := url.QueryUnescape(key)
		if decErr != nil {
			decodedKey = key
		}
		if sensitiveQueryKeys[strings.ToLower(decodedKey)] {
			parts[i] = key + "=REDACTED"
			redacted = true
		}
	}

	if !redacted {
		return rawURL
	}

	parsed.RawQuery = strings.Join(parts, "&")
	return parsed.String()
}

// RedactToken redacts sensitive parts of an authorization token
func RedactToken(token string) string {
	if len(token) <= 8 {
		return redactedValue
	}
	if len(token) <= 12 {
		return redactedValue
	}
	return token[:6] + "..." + token[len(token)-6:]
}

// IsJSON checks if content appears to be JSON
func IsJSON(data []byte) bool {
	var js any
	return json.Unmarshal(data, &js) == nil
}
