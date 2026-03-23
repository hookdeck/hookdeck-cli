package mcp

import (
	"encoding/json"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// JSONResultWithProjectID creates a CallToolResult containing the JSON-encoded
// value with an additional "active_project_id" field merged into the top-level
// object. This allows agents to self-verify that results came from the intended
// project. If projectID is empty, the result is identical to JSONResult.
func JSONResultWithProjectID(v any, projectID string) (*mcpsdk.CallToolResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	if projectID == "" {
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{Text: string(data)},
			},
		}, nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		// v is not a JSON object; return as-is
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{Text: string(data)},
			},
		}, nil
	}
	pid, _ := json.Marshal(projectID)
	m["active_project_id"] = pid
	out, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{Text: string(out)},
		},
	}, nil
}

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
