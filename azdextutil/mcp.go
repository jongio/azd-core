package azdextutil

import (
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
)

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

// MarshalToolResult marshals any value to JSON and returns it as an MCP tool result.
func MarshalToolResult(data interface{}) (*mcp.CallToolResult, error) {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return mcp.NewToolResultError("failed to marshal result: " + err.Error()), nil
	}
	return mcp.NewToolResultText(string(jsonData)), nil
}
