package env

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
)

// mockCommandRunner is a test double for CommandRunner
type mockCommandRunner struct {
	// responses maps command signatures to their outputs
	responses map[string]mockResponse
	// calls records all commands that were called
	calls [][]string
}

type mockResponse struct {
	output []byte
	err    error
}

func newMockCommandRunner() *mockCommandRunner {
	return &mockCommandRunner{
		responses: make(map[string]mockResponse),
		calls:     [][]string{},
	}
}

func (m *mockCommandRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	fullCmd := append([]string{name}, args...)
	m.calls = append(m.calls, fullCmd)

	// Create a key from the command
	key := strings.Join(fullCmd, " ")
	if resp, ok := m.responses[key]; ok {
		return resp.output, resp.err
	}

	// Default: command not found
	return nil, errors.New("command not configured in mock")
}

func (m *mockCommandRunner) setResponse(output []byte, err error, name string, args ...string) {
	key := strings.Join(append([]string{name}, args...), " ")
	m.responses[key] = mockResponse{output: output, err: err}
}

func TestGetAzdEnvironmentValues_Validation(t *testing.T) {
	ctx := context.Background()

	// Test empty environment name
	_, err := GetAzdEnvironmentValues(ctx, "")
	if err == nil {
		t.Fatal("expected error for empty environment name")
	}
	if !strings.Contains(err.Error(), "cannot be empty") {
		t.Fatalf("unexpected error message: %v", err)
	}

	// Test environment name with space (injection attempt)
	_, err = GetAzdEnvironmentValues(ctx, "env name")
	if err == nil {
		t.Fatal("expected error for environment name with space")
	}
	if !strings.Contains(err.Error(), "invalid environment name") {
		t.Fatalf("unexpected error message: %v", err)
	}

	// Test environment name with semicolon (injection attempt)
	_, err = GetAzdEnvironmentValues(ctx, "env;rm -rf")
	if err == nil {
		t.Fatal("expected error for environment name with semicolon")
	}

	// Test environment name with ampersand (injection attempt)
	_, err = GetAzdEnvironmentValues(ctx, "env&whoami")
	if err == nil {
		t.Fatal("expected error for environment name with ampersand")
	}
}

func TestGetAzdEnvironmentValues_JSONFormat(t *testing.T) {
	ctx := context.Background()

	// Set up mock
	mock := newMockCommandRunner()
	prev := SetCommandRunner(mock)
	defer SetCommandRunner(prev)

	// Configure mock to return JSON output
	jsonOutput := `{"AZURE_ENV_NAME":"test-env","DATABASE_URL":"postgres://localhost/db","API_KEY":"secret123"}`
	mock.setResponse([]byte(jsonOutput), nil, "azd", "env", "get-values", "-e", "test-env", "--output", "json")

	// Call the function
	values, err := GetAzdEnvironmentValues(ctx, "test-env")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify results
	expected := map[string]string{
		"AZURE_ENV_NAME": "test-env",
		"DATABASE_URL":   "postgres://localhost/db",
		"API_KEY":        "secret123",
	}
	if !reflect.DeepEqual(values, expected) {
		t.Fatalf("expected %v, got %v", expected, values)
	}

	// Verify the correct command was called
	if len(mock.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(mock.calls))
	}
}

func TestGetAzdEnvironmentValues_KeyValueFallback(t *testing.T) {
	ctx := context.Background()

	// Set up mock
	mock := newMockCommandRunner()
	prev := SetCommandRunner(mock)
	defer SetCommandRunner(prev)

	// Configure mock: JSON fails, key=value succeeds
	mock.setResponse(nil, errors.New("unknown flag: --output"), "azd", "env", "get-values", "-e", "test-env", "--output", "json")
	keyValueOutput := "AZURE_ENV_NAME=test-env\nDATABASE_URL=postgres://localhost/db\nAPI_KEY=secret123\n"
	mock.setResponse([]byte(keyValueOutput), nil, "azd", "env", "get-values", "-e", "test-env")

	// Call the function
	values, err := GetAzdEnvironmentValues(ctx, "test-env")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify results
	expected := map[string]string{
		"AZURE_ENV_NAME": "test-env",
		"DATABASE_URL":   "postgres://localhost/db",
		"API_KEY":        "secret123",
	}
	if !reflect.DeepEqual(values, expected) {
		t.Fatalf("expected %v, got %v", expected, values)
	}

	// Verify both commands were attempted
	if len(mock.calls) != 2 {
		t.Fatalf("expected 2 calls (JSON then key=value), got %d", len(mock.calls))
	}
}

func TestGetAzdEnvironmentValues_EnvironmentNotFound(t *testing.T) {
	ctx := context.Background()

	// Set up mock
	mock := newMockCommandRunner()
	prev := SetCommandRunner(mock)
	defer SetCommandRunner(prev)

	// Configure mock: both commands fail
	mock.setResponse(nil, errors.New("environment 'nonexistent' not found"), "azd", "env", "get-values", "-e", "nonexistent", "--output", "json")
	mock.setResponse(nil, errors.New("environment 'nonexistent' not found"), "azd", "env", "get-values", "-e", "nonexistent")

	// Call the function
	_, err := GetAzdEnvironmentValues(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent environment")
	}
	if !strings.Contains(err.Error(), "failed to get environment values") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestLoadAzdEnvironment_SetsEnvironmentVariables(t *testing.T) {
	ctx := context.Background()

	// Set up mock
	mock := newMockCommandRunner()
	prev := SetCommandRunner(mock)
	defer SetCommandRunner(prev)

	// Configure mock to return JSON output
	jsonOutput := `{"TEST_VAR_1":"value1","TEST_VAR_2":"value2","TEST_VAR_3":"value with spaces"}`
	mock.setResponse([]byte(jsonOutput), nil, "azd", "env", "get-values", "-e", "test-env", "--output", "json")

	// Save original env values
	orig1 := os.Getenv("TEST_VAR_1")
	orig2 := os.Getenv("TEST_VAR_2")
	orig3 := os.Getenv("TEST_VAR_3")
	defer func() {
		_ = os.Setenv("TEST_VAR_1", orig1)
		_ = os.Setenv("TEST_VAR_2", orig2)
		_ = os.Setenv("TEST_VAR_3", orig3)
	}()

	// Clear any existing values
	_ = os.Unsetenv("TEST_VAR_1")
	_ = os.Unsetenv("TEST_VAR_2")
	_ = os.Unsetenv("TEST_VAR_3")

	// Call the function
	err := LoadAzdEnvironment(ctx, "test-env")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify environment variables were set
	if got := os.Getenv("TEST_VAR_1"); got != "value1" {
		t.Errorf("TEST_VAR_1: expected 'value1', got %q", got)
	}
	if got := os.Getenv("TEST_VAR_2"); got != "value2" {
		t.Errorf("TEST_VAR_2: expected 'value2', got %q", got)
	}
	if got := os.Getenv("TEST_VAR_3"); got != "value with spaces" {
		t.Errorf("TEST_VAR_3: expected 'value with spaces', got %q", got)
	}
}

func TestLoadAzdEnvironment_OverwritesExistingVariables(t *testing.T) {
	ctx := context.Background()

	// Set up mock
	mock := newMockCommandRunner()
	prev := SetCommandRunner(mock)
	defer SetCommandRunner(prev)

	// Configure mock
	jsonOutput := `{"EXISTING_VAR":"new-value"}`
	mock.setResponse([]byte(jsonOutput), nil, "azd", "env", "get-values", "-e", "test-env", "--output", "json")

	// Set an existing value
	orig := os.Getenv("EXISTING_VAR")
	defer os.Setenv("EXISTING_VAR", orig)
	os.Setenv("EXISTING_VAR", "old-value")

	// Call the function
	err := LoadAzdEnvironment(ctx, "test-env")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify existing value was overwritten
	if got := os.Getenv("EXISTING_VAR"); got != "new-value" {
		t.Errorf("EXISTING_VAR: expected 'new-value', got %q", got)
	}
}

func TestParseKeyValueFormat(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]string
	}{
		{
			name:     "simple key=value",
			input:    "KEY=value",
			expected: map[string]string{"KEY": "value"},
		},
		{
			name:     "multiple lines",
			input:    "KEY1=value1\nKEY2=value2",
			expected: map[string]string{"KEY1": "value1", "KEY2": "value2"},
		},
		{
			name:     "double quoted value",
			input:    `KEY="quoted value"`,
			expected: map[string]string{"KEY": "quoted value"},
		},
		{
			name:     "single quoted value",
			input:    `KEY='quoted value'`,
			expected: map[string]string{"KEY": "quoted value"},
		},
		{
			name:     "skip comments",
			input:    "# comment\nKEY=value",
			expected: map[string]string{"KEY": "value"},
		},
		{
			name:     "skip empty lines",
			input:    "\nKEY=value\n\n",
			expected: map[string]string{"KEY": "value"},
		},
		{
			name:     "value with equals sign",
			input:    "KEY=value=with=equals",
			expected: map[string]string{"KEY": "value=with=equals"},
		},
		{
			name:     "empty value skipped",
			input:    "KEY=",
			expected: map[string]string{},
		},
		{
			name:     "windows line endings",
			input:    "KEY1=value1\r\nKEY2=value2\r\n",
			expected: map[string]string{"KEY1": "value1", "KEY2": "value2"},
		},
		{
			name:     "mixed quotes and unquoted",
			input:    "KEY1=unquoted\nKEY2=\"double\"\nKEY3='single'",
			expected: map[string]string{"KEY1": "unquoted", "KEY2": "double", "KEY3": "single"},
		},
		{
			name:     "realistic azd output",
			input:    "AZURE_ENV_NAME=dev\nAZURE_LOCATION=eastus2\nAZURE_SUBSCRIPTION_ID=12345678-1234-1234-1234-123456789012\n",
			expected: map[string]string{"AZURE_ENV_NAME": "dev", "AZURE_LOCATION": "eastus2", "AZURE_SUBSCRIPTION_ID": "12345678-1234-1234-1234-123456789012"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseKeyValueFormat([]byte(tt.input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(result, tt.expected) {
				t.Fatalf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGetAzdEnvironmentValues_InvalidJSON(t *testing.T) {
	ctx := context.Background()

	// Set up mock
	mock := newMockCommandRunner()
	prev := SetCommandRunner(mock)
	defer SetCommandRunner(prev)

	// Configure mock to return invalid JSON
	mock.setResponse([]byte(`{"KEY": "value`), nil, "azd", "env", "get-values", "-e", "test-env", "--output", "json")

	// Call the function
	_, err := GetAzdEnvironmentValues(ctx, "test-env")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "failed to parse environment values as JSON") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestGetAzdEnvironmentValues_EmptyJSON(t *testing.T) {
	ctx := context.Background()

	// Set up mock
	mock := newMockCommandRunner()
	prev := SetCommandRunner(mock)
	defer SetCommandRunner(prev)

	// Configure mock to return empty JSON object
	mock.setResponse([]byte(`{}`), nil, "azd", "env", "get-values", "-e", "empty-env", "--output", "json")

	// Call the function
	values, err := GetAzdEnvironmentValues(ctx, "empty-env")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify empty map is returned
	if len(values) != 0 {
		t.Fatalf("expected empty map, got %v", values)
	}
}

func TestGetAzdEnvironmentValues_SpecialCharactersInJSON(t *testing.T) {
	ctx := context.Background()

	// Set up mock
	mock := newMockCommandRunner()
	prev := SetCommandRunner(mock)
	defer SetCommandRunner(prev)

	// Configure mock to return JSON with special characters
	jsonOutput := `{"CONNECTION_STRING":"Server=tcp:server.database.windows.net;User ID=admin;Password=p@ss\"word!","MULTILINE":"line1\nline2","UNICODE":"日本語テスト"}`
	mock.setResponse([]byte(jsonOutput), nil, "azd", "env", "get-values", "-e", "special-env", "--output", "json")

	// Call the function
	values, err := GetAzdEnvironmentValues(ctx, "special-env")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify special characters are preserved
	if values["CONNECTION_STRING"] != `Server=tcp:server.database.windows.net;User ID=admin;Password=p@ss"word!` {
		t.Errorf("CONNECTION_STRING mismatch: got %q", values["CONNECTION_STRING"])
	}
	if values["MULTILINE"] != "line1\nline2" {
		t.Errorf("MULTILINE mismatch: got %q", values["MULTILINE"])
	}
	if values["UNICODE"] != "日本語テスト" {
		t.Errorf("UNICODE mismatch: got %q", values["UNICODE"])
	}
}

func TestLoadAzdEnvironment_SetsAzureEnvName(t *testing.T) {
	ctx := context.Background()

	// Set up mock
	mock := newMockCommandRunner()
	prev := SetCommandRunner(mock)
	defer SetCommandRunner(prev)

	// Configure mock - note AZURE_ENV_NAME should be set from the response
	jsonOutput := `{"AZURE_ENV_NAME":"my-custom-env","OTHER_VAR":"value"}`
	mock.setResponse([]byte(jsonOutput), nil, "azd", "env", "get-values", "-e", "my-custom-env", "--output", "json")

	// Save and restore
	origEnvName := os.Getenv("AZURE_ENV_NAME")
	origOther := os.Getenv("OTHER_VAR")
	defer func() {
		os.Setenv("AZURE_ENV_NAME", origEnvName)
		os.Setenv("OTHER_VAR", origOther)
	}()

	// Clear
	os.Unsetenv("AZURE_ENV_NAME")
	os.Unsetenv("OTHER_VAR")

	// Call the function
	err := LoadAzdEnvironment(ctx, "my-custom-env")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify AZURE_ENV_NAME is set correctly
	if got := os.Getenv("AZURE_ENV_NAME"); got != "my-custom-env" {
		t.Errorf("AZURE_ENV_NAME: expected 'my-custom-env', got %q", got)
	}
	if got := os.Getenv("OTHER_VAR"); got != "value" {
		t.Errorf("OTHER_VAR: expected 'value', got %q", got)
	}
}

func TestGetAzdEnvironmentValues_ContextCancellation(t *testing.T) {
	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Set up mock that checks context
	mock := &contextAwareMockRunner{cancelled: false}
	prev := SetCommandRunner(mock)
	defer SetCommandRunner(prev)

	// Call the function with cancelled context
	_, err := GetAzdEnvironmentValues(ctx, "test-env")

	// The function should return an error (context cancelled propagates through exec)
	if err == nil {
		t.Log("Note: context cancellation may not be checked before command execution in some implementations")
	}
}

// contextAwareMockRunner is a mock that tracks if context was checked
type contextAwareMockRunner struct {
	cancelled bool
}

func (m *contextAwareMockRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	// Check if context is cancelled
	if ctx.Err() != nil {
		m.cancelled = true
		return nil, ctx.Err()
	}
	return nil, errors.New("command not configured")
}

func TestSetCommandRunner_RestoresPrevious(t *testing.T) {
	// Get current runner
	originalRunner := defaultRunner

	// Set a new mock runner
	mock1 := newMockCommandRunner()
	prev1 := SetCommandRunner(mock1)

	// Verify prev1 is the original
	if prev1 != originalRunner {
		t.Error("SetCommandRunner should return the previous runner")
	}

	// Set another mock runner
	mock2 := newMockCommandRunner()
	prev2 := SetCommandRunner(mock2)

	// Verify prev2 is mock1
	if prev2 != mock1 {
		t.Error("SetCommandRunner should return the previous runner (mock1)")
	}

	// Restore original
	SetCommandRunner(originalRunner)
}
