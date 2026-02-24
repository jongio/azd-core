package azdextutil

import (
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// Deprecated: Use azdext.ParseToolArgs() from github.com/azure/azure-dev/cli/azd/pkg/azdext instead.
// azdext.ToolArgs provides typed access via RequireString, RequireInt, OptionalBool, etc.
//
// GetArgsMap extracts the arguments map from an MCP tool call request.
// Returns an empty map if arguments are nil or not a map.
func GetArgsMap(request mcp.CallToolRequest) map[string]interface{} {
	if request.Params.Arguments != nil {
		if m, ok := request.Params.Arguments.(map[string]interface{}); ok {
			return m
		}
	}
	return map[string]interface{}{}
}

// Deprecated: Use azdext.ToolArgs.RequireString() or azdext.ToolArgs.OptionalString()
// from github.com/azure/azure-dev/cli/azd/pkg/azdext instead.
//
// GetStringParam extracts a string parameter from the arguments map.
// Returns the value and whether it was found and is a string.
func GetStringParam(args map[string]interface{}, key string) (string, bool) {
	val, ok := args[key]
	if !ok {
		return "", false
	}
	s, ok := val.(string)
	return s, ok
}

// Deprecated: Use azdext.MCPJSONResult() from github.com/azure/azure-dev/cli/azd/pkg/azdext instead.
// MCPJSONResult returns an error result (not a Go error) on marshal failure, simplifying callers.
// Also see MCPTextResult and MCPErrorResult for other result types.
//
// MarshalToolResult marshals any value to JSON and returns it as an MCP tool result.
func MarshalToolResult(data interface{}) (*mcp.CallToolResult, error) {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}
	return mcp.NewToolResultText(string(jsonData)), nil
}
