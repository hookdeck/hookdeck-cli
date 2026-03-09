package mcp

import (
	"encoding/json"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// JSONResult creates a CallToolResult containing the JSON-encoded value as
// text content. This is the standard way to return structured data from a
// tool handler.
func JSONResult(v any) (*mcpsdk.CallToolResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{Text: string(data)},
		},
	}, nil
}

// TextResult creates a CallToolResult containing a plain text message.
func TextResult(msg string) *mcpsdk.CallToolResult {
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{Text: msg},
		},
	}
}

// ErrorResult creates a CallToolResult with IsError set, containing the
// given error message.
func ErrorResult(msg string) *mcpsdk.CallToolResult {
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{Text: msg},
		},
		IsError: true,
	}
}
