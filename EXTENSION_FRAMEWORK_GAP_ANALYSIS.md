# Azure Developer CLI Extension Framework - Comprehensive Gap Analysis

**Review Date:** 2025-06-01  
**Branch:** azdextimprove  
**Repositories Reviewed:**
- azd-core (shared library)
- azd-exec (script execution)
- azd-rest (REST API)
- azd-copilot (AI copilot)
- azd-app (app orchestrator - reference)

---

## Executive Summary

This analysis identifies **43 gaps** across 5 repositories covering framework utilization, code quality, security, and missing capabilities. Key findings:

### Critical Gaps
1. **azd-core**: Missing MCP scaffolding helpers, listen command factory, error handling patterns
2. **Security**: Inconsistent path validation, no input sanitization helpers, missing MCP security best practices
3. **MCP Tools**: Missing output schemas (6/4 extensions), inadequate tool annotations, no testing framework
4. **Lifecycle Events**: Minimal handlers (3/4 extensions have stubs), no error handling patterns, artifacts underutilized
5. **Testing**: Zero MCP tool tests, minimal lifecycle handler coverage, no integration tests

### Moderate Gaps
- Distributed tracing incomplete in 3/4 extensions
- Rate limiting inconsistent (2 custom implementations vs azdextutil)
- Error handling ad-hoc across extensions
- Documentation sparse for MCP tools and lifecycle handlers

---

## 1. azd-core Analysis (Shared Library)

### 1.1 azdextutil Package - Missing Core Helpers

#### 1.1.1 Listen Command Factory ❌ MISSING
**Priority: HIGH**

No helper to scaffold the boilerplate `listen` command. Every extension duplicates this pattern:

```go
// Current: Every extension reimplements
func NewListenCommand() *cobra.Command {
    return &cobra.Command{
        Use: "listen", Short: "...", Hidden: true,
        RunE: func(cmd *cobra.Command, args []string) error {
            ctx := azdext.WithAccessToken(cmd.Context())
            azdClient, err := azdext.NewAzdClient()
            if err != nil { return err }
            defer azdClient.Close()
            host := azdext.NewExtensionHost(azdClient)
            // ... register capabilities
            return host.Run(ctx)
        },
    }
}
```

**Recommendation:**
```go
// azdextutil/listen.go - NEW FILE
package azdextutil

// ListenCommandConfig configures the listen command
type ListenCommandConfig struct {
    ExtensionName string
    Short         string
    ConfigureHost func(*azdext.ExtensionHost) *azdext.ExtensionHost
}

// NewListenCommand creates a standard listen command with boilerplate handled
func NewListenCommand(cfg ListenCommandConfig) *cobra.Command {
    if cfg.Short == "" {
        cfg.Short = "Start extension listener (internal use only)"
    }
    return &cobra.Command{
        Use: "listen", Short: cfg.Short, Hidden: true,
        RunE: func(cmd *cobra.Command, args []string) error {
            ctx := SetupTracingFromEnv(cmd.Context())
            ctx = azdext.WithAccessToken(ctx)
            
            azdClient, err := azdext.NewAzdClient()
            if err != nil {
                return fmt.Errorf("failed to create azd client: %w", err)
            }
            defer azdClient.Close()
            
            host := azdext.NewExtensionHost(azdClient)
            if cfg.ConfigureHost != nil {
                host = cfg.ConfigureHost(host)
            }
            
            if err := host.Run(ctx); err != nil {
                return fmt.Errorf("[%s] extension failed: %w", cfg.ExtensionName, err)
            }
            return nil
        },
    }
}
```

**Impact:**
- Reduces 30-60 lines per extension
- Consistent error handling and tracing
- Easier to maintain/update across all extensions

**Files to Create:**
- `azd-core/azdextutil/listen.go`
- `azd-core/azdextutil/listen_test.go`

---

#### 1.1.2 MCP Server Scaffolding ❌ MISSING
**Priority: HIGH**

No helpers for common MCP server patterns. Extensions duplicate:
- Server creation with instructions
- Tool/resource registration
- Stdio transport setup
- Error handling

**Current State (duplicated 4x):**
```go
// Every extension reimplements
func runMCPServer(ctx context.Context) error {
    instructions := `...` // Different per extension
    s := server.NewMCPServer("name", version, 
        server.WithToolCapabilities(true),
        server.WithInstructions(instructions))
    s.AddTools(/* tools */)
    return server.ServeStdio(s)
}
```

**Recommendation:**
```go
// azdextutil/mcp.go - NEW FILE
package azdextutil

// MCPServerConfig configures an MCP server
type MCPServerConfig struct {
    Name         string
    Version      string
    Instructions string
    Tools        []server.ServerTool
    Resources    []server.ServerResource
    Prompts      []server.ServerPrompt
    RateLimiter  *RateLimiter
}

// NewMCPServer creates a preconfigured MCP server
func NewMCPServer(cfg MCPServerConfig) *server.MCPServer {
    s := server.NewMCPServer(cfg.Name, cfg.Version,
        server.WithToolCapabilities(len(cfg.Tools) > 0),
        server.WithResourceCapabilities(false, len(cfg.Resources) > 0),
        server.WithPromptCapabilities(len(cfg.Prompts) > 0),
        server.WithInstructions(cfg.Instructions))
    
    if cfg.RateLimiter != nil {
        // Wrap tools with rate limiting
        for _, tool := range cfg.Tools {
            tool.Handler = wrapWithRateLimit(tool.Handler, cfg.RateLimiter, tool.Tool.Name)
        }
    }
    
    if len(cfg.Tools) > 0 { s.AddTools(cfg.Tools...) }
    if len(cfg.Resources) > 0 { s.AddResources(cfg.Resources...) }
    if len(cfg.Prompts) > 0 { s.AddPrompts(cfg.Prompts...) }
    return s
}

// wrapWithRateLimit wraps a tool handler with rate limiting
func wrapWithRateLimit(handler server.ToolHandlerFunc, limiter *RateLimiter, toolName string) server.ToolHandlerFunc {
    return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
        if err := limiter.CheckRateLimit(toolName); err != nil {
            return mcp.NewToolResultError(err.Error()), nil
        }
        return handler(ctx, req)
    }
}

// ServeMCPStdio starts the MCP server via stdio (standard boilerplate)
func ServeMCPStdio(s *server.MCPServer) error {
    if err := server.ServeStdio(s); err != nil {
        fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
        return err
    }
    return nil
}
```

**Files to Create:**
- `azd-core/azdextutil/mcp.go`
- `azd-core/azdextutil/mcp_test.go`

---

#### 1.1.3 Error Handling Helpers ❌ MISSING
**Priority: MEDIUM**

No standardized error handling for:
- MCP tool errors
- Lifecycle event errors
- gRPC errors from azd client
- Validation errors

**Recommendation:**
```go
// azdextutil/errors.go - NEW FILE
package azdextutil

import "github.com/mark3labs/mcp-go/mcp"

// Common error patterns for MCP tools
func NewMCPValidationError(field, message string) *mcp.CallToolResult {
    return mcp.NewToolResultError(fmt.Sprintf("Validation error for '%s': %s", field, message))
}

func NewMCPRateLimitError() *mcp.CallToolResult {
    return mcp.NewToolResultError("Rate limit exceeded. Please wait before retrying.")
}

func NewMCPInternalError(err error) *mcp.CallToolResult {
    return mcp.NewToolResultError(fmt.Sprintf("Internal error: %v", err))
}

// WrapMCPError wraps an error and returns an MCP tool result
func WrapMCPError(err error, context string) *mcp.CallToolResult {
    if err == nil {
        return nil
    }
    return mcp.NewToolResultError(fmt.Sprintf("%s: %v", context, err))
}

// LifecycleEventError wraps errors from lifecycle handlers with context
type LifecycleEventError struct {
    Event   string
    Phase   string // "pre" or "post"
    Service string // empty for project-level events
    Err     error
}

func (e *LifecycleEventError) Error() string {
    if e.Service != "" {
        return fmt.Sprintf("[%s%s] service %s: %v", e.Phase, e.Event, e.Service, e.Err)
    }
    return fmt.Sprintf("[%s%s]: %v", e.Phase, e.Event, e.Err)
}

func (e *LifecycleEventError) Unwrap() error { return e.Err }

// NewLifecycleError creates a structured lifecycle event error
func NewLifecycleError(event, phase, service string, err error) error {
    return &LifecycleEventError{Event: event, Phase: phase, Service: service, Err: err}
}
```

**Files to Create:**
- `azd-core/azdextutil/errors.go`
- `azd-core/azdextutil/errors_test.go`

---

#### 1.1.4 Metadata Generation - Incomplete ⚠️ PARTIAL
**Priority: LOW**

Current `metadata.go` is functional but missing:
- Configuration schema support (JSON Schema generation)
- Environment variable metadata
- Service/project-level config distinction
- Examples from cobra command metadata

**Gaps:**
```go
// azdextutil/metadata.go - CURRENT
type ExtensionMetadata struct {
    SchemaVersion string
    ID            string
    Commands      []CommandMetadata
    Configuration *ConfigMetadata  // ❌ Not implemented
}

type ConfigMetadata struct {
    EnvironmentVariables []EnvVarMetadata
    // ❌ Missing: Global, Project, Service schemas
}
```

**Recommendation:** Extend metadata.go
```go
// Add to azdextutil/metadata.go
type ConfigMetadata struct {
    Global               *jsonschema.Schema   `json:"global,omitempty"`
    Project              *jsonschema.Schema   `json:"project,omitempty"`
    Service              *jsonschema.Schema   `json:"service,omitempty"`
    EnvironmentVariables []EnvVarMetadata     `json:"environmentVariables,omitempty"`
}

// Helper to generate config metadata from Go structs
func GenerateConfigMetadata(globalType, projectType, serviceType interface{}, envVars []EnvVarMetadata) *ConfigMetadata {
    cfg := &ConfigMetadata{EnvironmentVariables: envVars}
    if globalType != nil {
        cfg.Global = jsonschema.Reflect(globalType)
    }
    if projectType != nil {
        cfg.Project = jsonschema.Reflect(projectType)
    }
    if serviceType != nil {
        cfg.Service = jsonschema.Reflect(serviceType)
    }
    return cfg
}
```

**Files to Modify:**
- `azd-core/azdextutil/metadata.go` (add ConfigMetadata helpers)
- `azd-core/azdextutil/metadata_test.go` (add config tests)

---

#### 1.1.5 Testing Utilities ❌ MISSING
**Priority: MEDIUM**

No test helpers for:
- MCP tool testing
- Lifecycle event simulation
- Mock azd clients
- Rate limiter testing

**Recommendation:**
```go
// azdextutil/testing.go - NEW FILE
package azdextutil

// MCPToolTestCase represents a test case for an MCP tool
type MCPToolTestCase struct {
    Name           string
    Arguments      map[string]interface{}
    ExpectedError  bool
    ExpectedResult string // substring to check
}

// TestMCPTool executes test cases for an MCP tool handler
func TestMCPTool(t *testing.T, handler server.ToolHandlerFunc, cases []MCPToolTestCase) {
    for _, tc := range cases {
        t.Run(tc.Name, func(t *testing.T) {
            req := mcp.CallToolRequest{
                Params: mcp.CallToolRequestParams{
                    Arguments: tc.Arguments,
                },
            }
            result, err := handler(context.Background(), req)
            if tc.ExpectedError && !result.IsError {
                t.Errorf("expected error result, got success")
            }
            // ... more assertions
        })
    }
}

// NewMockRateLimiter creates a rate limiter for testing
func NewMockRateLimiter(allowCount int) *RateLimiter {
    return NewRateLimiter(float64(allowCount), 0) // No refill for predictable tests
}
```

**Files to Create:**
- `azd-core/azdextutil/testing.go`

---

### 1.2 Security Module - Path Validation Issues ⚠️

**File:** `azd-core/azdextutil/security.go`

#### Issue 1: ValidatePath doesn't enforce project root boundary
```go
// CURRENT - Lines 16-62
func ValidatePath(path string, allowedBases ...string) (string, error) {
    // ✅ Good: Resolves absolute, cleans, checks traversal
    // ❌ BAD: allowedBases is optional - if empty, allows ANY path
    if len(allowedBases) > 0 {
        // validation logic
    }
    return realPath, nil // ⚠️ Returns path even if no allowedBases
}
```

**Security Risk:** If caller forgets to pass `allowedBases`, any path is accepted.

**Fix:**
```go
func ValidatePath(path string, allowedBases ...string) (string, error) {
    if len(allowedBases) == 0 {
        return "", fmt.Errorf("at least one allowed base directory must be specified")
    }
    // ... rest of validation
}
```

#### Issue 2: GetProjectDir doesn't validate environment variable
```go
// CURRENT - Lines 78-94
func GetProjectDir(envVar string) (string, error) {
    dir := os.Getenv(envVar)
    // ❌ No validation - trusts env var implicitly
    if dir == "" {
        dir, err = os.Getwd()
    }
    return filepath.Clean(absDir), nil
}
```

**Fix:**
```go
func GetProjectDir(envVar string) (string, error) {
    dir := os.Getenv(envVar)
    if dir == "" {
        var err error
        dir, err = os.Getwd()
        if err != nil {
            return "", fmt.Errorf("failed to get working directory: %w", err)
        }
    }
    // Validate even environment-sourced paths
    return ValidatePath(dir, dir) // At minimum, validate against itself
}
```

---

### 1.3 Missing Utilities

#### 1.3.1 Input Sanitization ❌ MISSING
No helpers for:
- Service name validation (azd-app uses regex, not shared)
- Environment variable name validation
- File extension validation
- Command argument sanitization

**Recommendation:**
```go
// azdextutil/validation.go - NEW FILE
package azdextutil

var (
    // Safe identifier: alphanumeric, dash, underscore
    safeIdentifierPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)
    // Environment variable name: uppercase, underscore
    envVarPattern = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
)

func ValidateIdentifier(name, fieldName string) error {
    if !safeIdentifierPattern.MatchString(name) {
        return fmt.Errorf("%s must start with alphanumeric and contain only alphanumeric, dash, underscore", fieldName)
    }
    return nil
}

func ValidateEnvVarName(name string) error {
    if !envVarPattern.MatchString(name) {
        return fmt.Errorf("invalid environment variable name: must be uppercase with underscores")
    }
    return nil
}

func ValidateFileExtension(path string, allowedExts ...string) error {
    ext := strings.ToLower(filepath.Ext(path))
    for _, allowed := range allowedExts {
        if ext == strings.ToLower(allowed) {
            return nil
        }
    }
    return fmt.Errorf("file extension %q not allowed", ext)
}
```

**Files to Create:**
- `azd-core/azdextutil/validation.go`
- `azd-core/azdextutil/validation_test.go`

---

## 2. azd-exec Analysis

### 2.1 Listen Command - Good Implementation ✅

**File:** `azd-exec/cli/src/cmd/exec/commands/listen.go`

**Strengths:**
- Uses ExtensionHost builder pattern correctly
- Registers 3 event handlers (postprovision, postdeploy, service postdeploy)
- Clean error handling

**Minor Gap:** Handlers are stubs (only print statements)
```go
func handlePostProvision(ctx context.Context, args *azdext.ProjectEventArgs) error {
    fmt.Printf("Post-provision completed for project: %s\n", args.Project.Name)
    return nil // ❌ No actual logic
}
```

**Recommendation:** Add concrete examples or remove stubs. Document intent:
```go
// handlePostProvision demonstrates lifecycle event handling.
// Production extensions can use this to:
// - Validate provisioned resources
// - Populate environment variables
// - Trigger post-provision scripts
func handlePostProvision(ctx context.Context, args *azdext.ProjectEventArgs) error {
    // Example: Access provisioned resources from deployment context
    // if args.Project.GetDeploymentContext() != nil { ... }
    log.Printf("Post-provision completed for project: %s", args.Project.Name)
    return nil
}
```

---

### 2.2 MCP Tools - Moderate Implementation ⚠️

**File:** `azd-exec/cli/src/cmd/exec/commands/mcp.go`

#### 2.2.1 Tool Annotations - Good ✅
```go
mcp.WithReadOnlyHintAnnotation(true),
mcp.WithDestructiveHintAnnotation(true),
mcp.WithIdempotentHintAnnotation(true),
```
All tools have appropriate hint annotations.

#### 2.2.2 Security - Path Validation Good ✅
```go
validPath, err := azdextutil.ValidatePath(scriptPath, projectDir)
if err != nil {
    return mcp.NewToolResultError(fmt.Sprintf("Invalid script path: %v", err)), nil
}
```
Uses azdextutil.ValidatePath correctly with project dir as base.

#### 2.2.3 Shell Validation - Good ✅
```go
if shell != "" {
    if err := azdextutil.ValidateShellName(shell); err != nil {
        return mcp.NewToolResultError(fmt.Sprintf("Invalid shell: %v", err)), nil
    }
}
```

#### 2.2.4 Rate Limiting - Consistent ✅
```go
var globalRateLimiter = azdextutil.NewRateLimiter(10, 1.0)

if !globalRateLimiter.Allow() {
    return mcp.NewToolResultError("Rate limit exceeded..."), nil
}
```
Uses azdextutil correctly.

#### 2.2.5 Output Schemas ❌ MISSING
**Priority: MEDIUM**

No output schemas defined for tools:
```go
// CURRENT - No schema
func newExecScriptTool() server.ServerTool {
    return server.ServerTool{
        Tool: mcp.NewTool("exec_script", /* ... */),
        Handler: handleExecScript,
        // ❌ Missing: OutputSchema
    }
}
```

**Should be:**
```go
type execResult struct {
    Stdout   string `json:"stdout" jsonschema:"description=Standard output from command"`
    Stderr   string `json:"stderr" jsonschema:"description=Standard error from command"`
    ExitCode int    `json:"exitCode" jsonschema:"description=Process exit code"`
    Error    string `json:"error,omitempty" jsonschema:"description=Error message if execution failed"`
}

func newExecScriptTool() server.ServerTool {
    schema := jsonschema.Reflect(&execResult{})
    return server.ServerTool{
        Tool: mcp.NewTool("exec_script",
            // ... existing options
            mcp.WithOutputSchema(schema),
        ),
        Handler: handleExecScript,
    }
}
```

**Impact:** AI agents get typed responses, improving reliability.

**Files to Modify:**
- Add schemas to all 4 tools in `mcp.go`

---

#### 2.2.6 Error Handling - Ad-hoc ⚠️
Multiple error handling patterns:
```go
// Pattern 1: Inline formatting
return mcp.NewToolResultError(fmt.Sprintf("Invalid script path: %v", err)), nil

// Pattern 2: Direct message
return mcp.NewToolResultError("Rate limit exceeded..."), nil

// Pattern 3: marshalExecResult helper
return marshalExecResult(stdout.String(), stderr.String(), cmd.ProcessState, err)
```

**Recommendation:** Use azdextutil error helpers when created.

---

#### 2.2.7 Testing ❌ NONE
No tests for:
- MCP tool handlers
- Rate limiting
- Path validation integration
- Shell detection

**Recommendation:**
```go
// commands/mcp_test.go - NEW FILE
package commands

func TestExecScriptTool_ValidPath(t *testing.T) {
    // Test with valid script in project dir
}

func TestExecScriptTool_PathTraversal(t *testing.T) {
    // Test blocked path traversal
}

func TestExecScriptTool_RateLimit(t *testing.T) {
    // Test rate limiting behavior
}
```

**Files to Create:**
- `azd-exec/cli/src/cmd/exec/commands/mcp_test.go`

---

### 2.3 Metadata Command - Manual Implementation ⚠️

**File:** `azd-exec/cli/src/cmd/exec/commands/metadata.go`

Manually defines all metadata (92 lines):
```go
type metadataOutput struct {
    SchemaVersion string
    ID            string
    Commands      []commandMetadata
}
// ... manual construction
```

**Issue:** Could use azdextutil.GenerateMetadataFromCobra to reduce to ~20 lines.

**Recommendation:**
```go
func NewMetadataCommand() *cobra.Command {
    return azdextutil.NewMetadataCommand("jongio.azd.exec", func() *cobra.Command {
        return cmd.NewRootCommand() // Returns root with all subcommands
    })
}
```

**Impact:** Reduce from 92 to ~5 lines.

---

### 2.4 Distributed Tracing ❌ NOT IMPLEMENTED

No usage of azdextutil tracing:
```go
// main.go - Should have
ctx := azdextutil.SetupTracingFromEnv(context.Background())
rootCmd.ExecuteContext(ctx)
```

**Files to Modify:**
- `azd-exec/cli/src/cmd/exec/main.go`

---

## 3. azd-rest Analysis

### 3.1 Listen Command - Minimal ⚠️

**File:** `azd-rest/cli/src/internal/cmd/listen.go`

**Issue:** Event handlers are inline lambdas (harder to test):
```go
WithProjectEventHandler("postprovision", func(ctx context.Context, args *azdext.ProjectEventArgs) error {
    fmt.Printf("Post-provision completed for project: %s\n", args.Project.Name)
    return nil
}, nil)
```

**Recommendation:** Extract to named functions like azd-exec does.

---

### 3.2 MCP Tools - Good Core, Missing Features ⚠️

**File:** `azd-rest/cli/src/internal/cmd/mcp.go`

#### Strengths ✅
- Good rate limiting with azdextutil
- Proper OAuth scope detection
- MCP hint annotations on all tools
- Clean tool registration with helper funcs

#### Gaps:

**3.2.1 Output Schemas ❌ MISSING**
```go
type mcpResponse struct {
    StatusCode int               `json:"statusCode"`
    Headers    map[string]string `json:"headers,omitempty"`
    Body       string            `json:"body,omitempty"`
    // ❌ Missing jsonschema tags
}
```

**Should be:**
```go
type mcpResponse struct {
    StatusCode int               `json:"statusCode" jsonschema:"description=HTTP status code,minimum=100,maximum=599"`
    Headers    map[string]string `json:"headers,omitempty" jsonschema:"description=Response headers"`
    Body       string            `json:"body,omitempty" jsonschema:"description=Response body as string"`
}

// Add to each tool:
schema := jsonschema.Reflect(&mcpResponse{})
mcp.WithOutputSchema(schema),
```

**3.2.2 Tool Instructions ⚠️ INCOMPLETE**
Current instructions (lines 179-184) don't mention:
- Pagination support
- Retry behavior
- Timeout configuration
- Binary/streaming responses

**Recommendation:**
```go
const mcpInstructions = `You are an Azure REST API assistant powered by the azd-rest extension.

**Capabilities:**
- Execute authenticated HTTP requests (GET, POST, PUT, PATCH, DELETE, HEAD)
- Automatic OAuth scope detection for Azure services
- Custom header support
- Request body support (JSON)

**Authentication:**
- Bearer tokens automatically added for Azure APIs
- Scope auto-detected from URL:
  * management.azure.com -> https://management.azure.com/.default
  * graph.microsoft.com -> https://graph.microsoft.com/.default
  * vault.azure.net -> https://vault.azure.net/.default
- Use 'scope' parameter to override

**Best Practices:**
- Use rest_get for read operations
- Check status codes: 2xx = success, 4xx = client error, 5xx = server error
- Inspect 'headers' for pagination links (e.g., nextLink)
- Request retries happen automatically (3 attempts with exponential backoff)

**Rate Limiting:**
60 requests/minute with burst of 10
`
```

**3.2.3 Testing ❌ NONE**
No tests for:
- Tool handlers
- OAuth scope detection
- Rate limiting
- Error handling

**Files to Create:**
- `azd-rest/cli/src/internal/cmd/mcp_test.go`

---

### 3.3 Metadata - Manual but Complete ✅

**File:** `azd-rest/cli/src/internal/cmd/metadata.go`

133 lines of manual metadata generation. Well-structured but could be reduced to ~10 lines with azdextutil helper.

---

### 3.4 Distributed Tracing ❌ NOT IMPLEMENTED

No tracing setup in main.go or MCP handlers.

---

## 4. azd-copilot Analysis

### 4.1 MCP gRPC Tools - Excellent Pattern! ✅

**File:** `azd-copilot/cli/src/cmd/copilot/commands/mcp_grpc_tools.go`

**Strengths:**
- Clean separation of concerns (separate function per tool category)
- Uses azd gRPC services directly (Environment, Deployment, Account, Workflow, Compose)
- Proper context handling with `newAzdClient`
- Good error handling and JSON marshaling

**Example:**
```go
func registerEnvironmentTools(s *server.MCPServer) {
    s.AddTool(
        mcp.NewTool("list_environments",
            mcp.WithDescription("List all azd environments"),
            mcp.WithReadOnlyHintAnnotation(true),
        ),
        func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
            ctx, client, err := newAzdClient(ctx)
            if err != nil { return mcp.NewToolResultText(fmt.Sprintf("Error: %s", err)), nil }
            defer client.Close()
            
            resp, err := client.Environment().List(ctx, &azdext.EmptyRequest{})
            // ... clean JSON marshaling
        },
    )
}
```

### Gaps:

**4.1.1 Output Schemas ❌ MISSING**
All 10+ tools lack output schemas despite having well-defined response types.

**Example Fix:**
```go
type envInfo struct {
    Name    string `json:"name" jsonschema:"description=Environment name"`
    Local   bool   `json:"local" jsonschema:"description=Has local configuration"`
    Remote  bool   `json:"remote" jsonschema:"description=Has remote state"`
    Default bool   `json:"default" jsonschema:"description=Is default environment"`
}

s.AddTool(
    mcp.NewTool("list_environments",
        mcp.WithDescription("List all azd environments"),
        mcp.WithReadOnlyHintAnnotation(true),
        mcp.WithOutputSchema(jsonschema.Reflect(&[]envInfo{})), // ✅ Add this
    ),
    // ... handler
)
```

**4.1.2 Rate Limiting ❌ MISSING**
gRPC tools have no rate limiting.

**Recommendation:**
```go
var grpcRateLimiter = azdextutil.NewRateLimiter(30, 0.5) // 30 burst, 30/min sustained

func registerEnvironmentTools(s *server.MCPServer) {
    s.AddTool(..., wrapWithRateLimit(handler, grpcRateLimiter, "list_environments"))
}

func wrapWithRateLimit(handler server.ToolHandlerFunc, limiter *azdextutil.RateLimiter, toolName string) server.ToolHandlerFunc {
    return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
        if err := limiter.CheckRateLimit(toolName); err != nil {
            return mcp.NewToolResultError(err.Error()), nil
        }
        return handler(ctx, req)
    }
}
```

**4.1.3 Tool Annotations - Incomplete ⚠️**
Only `list_environments` has `WithReadOnlyHintAnnotation`. Other read tools (get_environment_values, get_deployment_info, etc.) are missing it.

**4.1.4 Testing ❌ NONE**
No tests for gRPC tool wrappers.

**4.1.5 Error Handling - Inconsistent ⚠️**
Some tools return formatted errors, others just wrap:
```go
// Pattern 1 (better):
if err != nil {
    return mcp.NewToolResultText(fmt.Sprintf("Error getting deployment: %s", err)), nil
}

// Pattern 2 (worse):
if name == "" {
    return mcp.NewToolResultText("Error: environment_name is required"), nil
}
```

**Recommendation:** Use azdextutil error helpers (when created).

---

### 4.2 Distributed Tracing ❌ NOT IMPLEMENTED

No tracing context passed to gRPC clients, despite `newAzdClient` being perfect place:
```go
func newAzdClient(ctx context.Context) (context.Context, *azdext.AzdClient, error) {
    // ❌ Missing: ctx = azdextutil.SetupTracingFromEnv(ctx)
    client, err := azdext.NewAzdClient()
    // ...
}
```

---

## 5. azd-app Analysis (Reference Implementation)

### 5.1 Listen Command - Excellent Reference ✅

**File:** `azd-app/cli/src/cmd/app/commands/listen.go`

**Strengths:**
- Clean ExtensionHost usage
- Service target provider registration
- Concrete postprovision handler (not a stub!)
- Good documentation

**Example of Real Handler:**
```go
func handlePostProvision(ctx context.Context, args *azdext.ServiceEventArgs) error {
    log.Printf("[azd-app] Post-provision event received for service: %s", args.Service.GetName())
    
    serviceinfo.RefreshEnvironmentCache()
    
    srv := dashboard.GetServer(projectDir)
    if srv != nil {
        if err := srv.BroadcastServiceUpdate(projectDir); err != nil {
            log.Printf("[azd-app] Warning: Failed to broadcast service update: %v", err)
        }
    }
    return nil
}
```

**Learning:** This shows how to use lifecycle events for real work (updating dashboard).

---

### 5.2 MCP Tools - Best-in-Class ✅ (with gaps)

**File:** `azd-app/cli/src/cmd/app/commands/mcp.go` (519 lines)

#### Strengths ✅
- Comprehensive tool set (12 tools)
- Excellent tool descriptions and instructions
- Output schemas defined for complex types (ServiceInfo, ProjectInfo, RequirementsResult)
- Robust error handling and validation
- Rate limiting with azd-app's TokenBucket
- Security: validateProjectDir with extensive system path checks
- Good helper functions (getArgsMap, getStringParam, extractProjectDirArg, etc.)

#### Gaps:

**5.2.1 Rate Limiting - Custom Implementation ⚠️**
Uses custom TokenBucket (mcp_ratelimit.go) instead of azdextutil.RateLimiter:
```go
// mcp_ratelimit.go
type TokenBucket struct {
    mu         sync.Mutex
    tokens     int           // ❌ int vs float64
    maxTokens  int
    refillRate time.Duration // ❌ duration vs tokens/sec
    lastRefill time.Time
}
```

**Issue:** Slightly different API than azdextutil. Should converge.

**Recommendation:** Migrate to azdextutil.RateLimiter or update azdextutil to match azd-app's API.

**5.2.2 Path Validation - Duplicated ⚠️**
Lines 338-442 in mcp.go implement comprehensive validateProjectDir:
- Symlink resolution
- System directory checks (Unix + Windows)
- CWD/home boundary enforcement

**This should be in azdextutil.security.go!**

**Recommendation:** Extract to azd-core:
```go
// azdextutil/security.go - ADD
func ValidateProjectDir(dir string) (string, error) {
    // Move azd-app's implementation here
}

// Then azd-app uses:
validatedPath, err := azdextutil.ValidateProjectDir(projectDir)
```

**5.2.3 Testing ❌ MINIMAL**
Only tests for TokenBucket rate limiter. No tests for:
- MCP tool handlers
- Path validation edge cases
- Error handling paths

**Files to Create:**
- `azd-app/cli/src/cmd/app/commands/mcp_test.go`

---

### 5.3 Metadata - Uses azdext ✅

**File:** `azd-app/cli/src/cmd/app/commands/metadata.go`

Uses `azdext.GenerateExtensionMetadata` - good! This is the recommended approach:
```go
metadata := azdext.GenerateExtensionMetadata(
    "1.0",
    "jongio.azd.app",
    rootCmd,
)
```

**Only gap:** Doesn't define Configuration metadata (environment variables, schemas). This is optional.

---

### 5.4 Distributed Tracing ✅ IMPLEMENTED

azd-app properly sets up tracing:
```go
// main.go
ctx := azdext.NewContext() // This includes trace context setup
rootCmd.ExecuteContext(ctx)
```

**This is the pattern other extensions should follow.**

---

## 6. Cross-Cutting Issues

### 6.1 Testing - Critical Gap ❌

**Coverage:**
- azd-core/azdextutil: ✅ 80%+ (good unit tests)
- azd-exec: ❌ 0% for MCP tools and lifecycle handlers
- azd-rest: ❌ 0% for MCP tools and lifecycle handlers  
- azd-copilot: ❌ 0% for MCP gRPC wrappers
- azd-app: ⚠️ 10% (only rate limiter)

**Missing Test Types:**
1. MCP tool handler tests (input validation, error cases, rate limiting)
2. Lifecycle event handler tests (with mock azd client)
3. Integration tests (end-to-end MCP server)
4. Security tests (path traversal, injection attacks)

**Recommendation:**
Create test framework in azdextutil:
```go
// azdextutil/testing.go
func TestMCPTool(t *testing.T, tool server.ServerTool, cases []MCPToolTestCase) { /* ... */ }
func NewMockAzdClient() *azdext.AzdClient { /* ... */ }
func NewMockProjectEventArgs(projectName string) *azdext.ProjectEventArgs { /* ... */ }
```

---

### 6.2 Documentation - Sparse ⚠️

**Missing:**
- README in each extension explaining MCP tools
- Examples of using gRPC services in lifecycle handlers
- Security best practices for MCP tool development
- Performance tuning guide (rate limiting, timeouts)

**Recommendation:**
```markdown
# Each extension should have:
README.md
  - Installation
  - MCP Tools Reference (with examples)
  - Lifecycle Events (with examples)
  - Security Considerations
  
CONTRIBUTING.md
  - Testing requirements
  - Code patterns
  - Common pitfalls
```

---

### 6.3 Error Handling - Inconsistent ⚠️

**Patterns observed:**
1. azd-exec: Direct mcp.NewToolResultError with inline formatting
2. azd-rest: formatResponse helper for structured responses
3. azd-copilot: Simple error text wrapping
4. azd-app: Comprehensive validation with detailed error messages

**Recommendation:**
- Use azdextutil error helpers (once created)
- Standardize error format: `{type: "validation|internal|rate_limit", message: "...", field: "..."}`

---

### 6.4 Distributed Tracing - Incomplete

**Status:**
- azd-core: ✅ azdextutil.SetupTracingFromEnv available
- azd-app: ✅ Uses azdext.NewContext (which includes tracing)
- azd-exec: ❌ Not implemented
- azd-rest: ❌ Not implemented
- azd-copilot: ❌ Not implemented

**Impact:** Lost correlation between azd and extension operations.

**Fix Effort:** 2 lines per extension main.go:
```go
ctx := azdextutil.SetupTracingFromEnv(context.Background())
rootCmd.ExecuteContext(ctx)
```

---

### 6.5 MCP Tool Quality Checklist

|Feature|azd-exec|azd-rest|azd-copilot|azd-app|
|---|---|---|---|---|
|Output Schemas|❌|❌|❌|✅|
|Hint Annotations|✅|✅|⚠️ Partial|✅|
|Rate Limiting|✅|✅|❌|✅|
|Input Validation|✅|✅|⚠️|✅|
|Error Handling|⚠️|⚠️|⚠️|✅|
|Security (Path)|✅|N/A|N/A|✅|
|Tool Instructions|✅|⚠️|❌|✅|
|Testing|❌|❌|❌|⚠️|

**Legend:**
- ✅ Fully implemented
- ⚠️ Partial or inconsistent
- ❌ Missing or not applicable

---

### 6.6 Lifecycle Event Handler Quality

|Feature|azd-exec|azd-rest|azd-copilot|azd-app|
|---|---|---|---|---|
|Event Registration|✅|✅|N/A|✅|
|Concrete Logic|❌ Stubs|❌ Stubs|N/A|✅|
|Error Handling|⚠️|⚠️|N/A|✅|
|Artifact Usage|❌|❌|N/A|✅|
|Testing|❌|❌|N/A|❌|

---

## 7. Prioritized Recommendations

### 7.1 Critical (Do First)

1. **azd-core: Add MCP/Listen Scaffolding** (2-3 days)
   - azdextutil/listen.go - Listen command factory
   - azdextutil/mcp.go - MCP server scaffolding
   - azdextutil/errors.go - Standardized error handling
   - Impact: Reduces extension code by 40%, improves consistency

2. **azd-core: Consolidate Security Helpers** (1 day)
   - Move azd-app's validateProjectDir to azdextutil
   - Fix ValidatePath to require allowedBases
   - Add validation.go for identifiers, env vars
   - Impact: Prevents security issues across all extensions

3. **Add Output Schemas to All MCP Tools** (2 days)
   - azd-exec: 4 tools
   - azd-rest: 6 tools
   - azd-copilot: 10+ tools
   - azd-app: 12 tools (already has types, just need schemas)
   - Impact: Better AI agent reliability

4. **Add MCP Tool Tests** (3-4 days)
   - Create azdextutil testing framework
   - Add tests for all extensions (30-40 test cases)
   - Impact: Catch regressions, validate security

### 7.2 High Priority (Next)

5. **Implement Distributed Tracing** (0.5 days)
   - azd-exec, azd-rest, azd-copilot: Add SetupTracingFromEnv
   - Impact: Full observability across azd + extensions

6. **Standardize Rate Limiting** (1 day)
   - Migrate azd-app to azdextutil.RateLimiter
   - Add rate limiting to azd-copilot gRPC tools
   - Impact: Consistent API abuse prevention

7. **Improve Lifecycle Handler Examples** (1 day)
   - azd-exec: Add real postprovision logic (e.g., run migration scripts)
   - azd-rest: Add postdeploy logic (e.g., health check API)
   - Document artifact usage patterns
   - Impact: Extensions learn from good examples

### 7.3 Medium Priority

8. **Extend Metadata Generation** (1 day)
   - Add config schema support to azdextutil
   - Add environment variable documentation
   - Impact: Better IDE support

9. **Add Documentation** (2 days)
   - READMEs for each extension
   - MCP tool reference
   - Security best practices
   - Impact: Easier onboarding

10. **Improve Tool Instructions** (1 day)
    - azd-rest: Expand with examples
    - azd-copilot: Add gRPC tool instructions
    - Impact: Better AI agent usage

### 7.4 Low Priority (Nice to Have)

11. **Add Integration Tests** (2-3 days)
    - End-to-end MCP server tests
    - Lifecycle event integration tests
    - Impact: Catch integration issues

12. **Performance Optimization** (1-2 days)
    - Profile MCP tool latency
    - Optimize JSON marshaling
    - Impact: Faster AI agent responses

---

## 8. Estimated Effort Summary

| Priority | Task Count | Days | Developer |
|----------|------------|------|-----------|
| Critical | 4 | 8-10 | Senior |
| High | 3 | 2.5 | Mid-Senior |
| Medium | 2 | 3 | Mid |
| Low | 2 | 3-5 | Junior-Mid |
| **Total** | **11** | **16.5-20.5** | **Mixed** |

**Assumes:**
- 1 developer full-time
- Senior for critical architectural changes
- Mid-level for implementation
- Junior for documentation/testing

---

## 9. Quick Wins (< 1 Day Each)

1. **Add distributed tracing** to azd-exec, azd-rest, azd-copilot (0.5 day)
2. **Add rate limiting** to azd-copilot gRPC tools (0.5 day)
3. **Fix azdextutil.ValidatePath** to require allowedBases (0.5 day)
4. **Add ReadOnlyHintAnnotation** to all read tools in azd-copilot (0.5 day)
5. **Standardize metadata commands** to use azdextutil helper (0.5 day)

**Total Quick Wins:** 2.5 days, eliminates 5 gaps

---

## 10. Files to Create

### azd-core
```
azdextutil/
  listen.go           - Listen command factory
  listen_test.go
  mcp.go             - MCP server scaffolding
  mcp_test.go
  errors.go          - Error handling helpers
  errors_test.go
  validation.go      - Input validation helpers
  validation_test.go
  testing.go         - Test utilities
```

### azd-exec
```
cli/src/cmd/exec/commands/
  mcp_test.go        - MCP tool tests
  listen_test.go     - Lifecycle handler tests
```

### azd-rest
```
cli/src/internal/cmd/
  mcp_test.go
  listen_test.go
```

### azd-copilot
```
cli/src/cmd/copilot/commands/
  mcp_grpc_tools_test.go
```

### azd-app
```
cli/src/cmd/app/commands/
  mcp_test.go        - Expand existing tests
  listen_test.go
```

---

## 11. Files to Modify

### azd-core
```
azdextutil/metadata.go      - Add ConfigMetadata helpers
azdextutil/security.go      - Fix ValidatePath, add validateProjectDir
```

### azd-exec
```
commands/mcp.go             - Add output schemas
commands/listen.go          - Improve handler examples
main.go                     - Add distributed tracing
```

### azd-rest
```
cmd/mcp.go                  - Add output schemas, improve instructions
cmd/listen.go               - Extract inline handlers
main.go                     - Add distributed tracing
```

### azd-copilot
```
commands/mcp_grpc_tools.go  - Add output schemas, rate limiting, annotations
main.go                     - Add distributed tracing
```

### azd-app
```
commands/mcp.go             - Migrate to azdextutil.RateLimiter
commands/mcp_ratelimit.go   - Deprecate or align with azdextutil
```

---

## 12. Comparison to Framework Documentation

### Framework Capabilities Utilization

|Capability|azd-exec|azd-rest|azd-copilot|azd-app|
|----------|--------|--------|-----------|-------|
|custom-commands|✅|✅|✅|✅|
|lifecycle-events|⚠️ Stubs|⚠️ Stubs|❌|✅|
|mcp-server|✅|✅|✅|✅|
|service-target-provider|❌|❌|❌|✅|
|framework-service-provider|❌|❌|❌|❌|
|metadata|✅|✅|❌|✅|

**Observations:**
- azd-copilot missing metadata capability (should add)
- Only azd-app uses service-target-provider (correctly)
- No extension uses framework-service-provider (expected - specialized)
- lifecycle-events underutilized (mostly stubs)

---

### Framework Best Practices Adherence

|Practice|azd-exec|azd-rest|azd-copilot|azd-app|
|--------|--------|--------|-----------|-------|
|ExtensionHost builder pattern|✅|✅|N/A|✅|
|Distributed tracing|❌|❌|❌|✅|
|Error reporting (ProgressReporter)|N/A|N/A|N/A|✅|
|MCP instructions|✅|⚠️|❌|✅|
|Tool annotations|✅|✅|⚠️|✅|
|Output schemas|❌|❌|❌|✅|
|Rate limiting|✅|✅|❌|✅|

**Overall Adherence:**
- azd-app: 85% (best practices followed)
- azd-exec: 65% (good foundation, missing advanced features)
- azd-rest: 60% (functional but incomplete)
- azd-copilot: 40% (basic implementation, many gaps)

---

## 13. Summary of Gaps by Category

### Critical (Must Fix)
- [ ] azd-core: Missing MCP/Listen scaffolding helpers
- [ ] azd-core: Path validation security issues
- [ ] All extensions: Missing output schemas for MCP tools
- [ ] All extensions: Zero MCP tool tests

### High Priority
- [ ] azd-exec, azd-rest, azd-copilot: No distributed tracing
- [ ] azd-copilot: No rate limiting on gRPC tools
- [ ] azd-exec, azd-rest: Lifecycle handlers are stubs

### Medium Priority
- [ ] azd-core: Incomplete metadata generation (config schemas)
- [ ] azd-app: Custom rate limiter instead of azdextutil
- [ ] azd-rest: Incomplete MCP instructions
- [ ] azd-copilot: Missing tool hint annotations

### Low Priority
- [ ] All extensions: Sparse documentation
- [ ] All extensions: No integration tests
- [ ] azd-exec, azd-rest: Manual metadata generation

---

## 14. Conclusion

**Overall Assessment:**
- **azd-core** foundation is solid but missing key helpers (MCP, Listen, testing)
- **azd-app** is the gold standard (85% framework adherence)
- **azd-exec** and **azd-rest** are functional but incomplete (60-65%)
- **azd-copilot** needs the most work (40% adherence)

**Biggest Wins:**
1. Add MCP/Listen scaffolding to azd-core → Reduce 40% of boilerplate
2. Add output schemas → Improve AI agent reliability
3. Add tests → Catch bugs before production
4. Standardize security → Prevent injection attacks

**Estimated Total Effort:** 16.5-20.5 days (one developer)

**Quick Wins (2.5 days):** Distributed tracing, rate limiting, hint annotations

**Recommendation:** Start with critical items (scaffolding + security + schemas) to establish foundation, then iterate on testing and documentation.
