package mcp

import (
	"encoding/json"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// JSONResultEnvelope returns a CallToolResult whose text body is always:
//
//	{"data":<payload>,"meta":{...}}
//
// When projectID is non-empty, meta always includes active_project_id and
// active_project_name (short name; may be ""). active_project_org is included
// when projectOrg is non-empty. When projectID is empty, meta is {}.
func JSONResultEnvelope(data any, projectID, projectOrg, projectShortName string) (*mcpsdk.CallToolResult, error) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var metaBytes []byte
	if projectID == "" {
		metaBytes = []byte("{}")
	} else {
		m := map[string]string{
			"active_project_id":   projectID,
			"active_project_name": projectShortName,
		}
		if projectOrg != "" {
			m["active_project_org"] = projectOrg
		}
		metaBytes, err = json.Marshal(m)
		if err != nil {
			return nil, err
		}
	}
	env := struct {
		Data json.RawMessage `json:"data"`
		Meta json.RawMessage `json:"meta"`
	}{
		Data: dataBytes,
		Meta: metaBytes,
	}
	out, err := json.Marshal(env)
	if err != nil {
		return nil, err
	}
	return &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{Text: string(out)},
		},
	}, nil
}

// JSONResultEnvelopeForClient wraps data using the client's project id, org, and short name.
func JSONResultEnvelopeForClient(data any, c *hookdeck.Client) (*mcpsdk.CallToolResult, error) {
	if c == nil {
		return JSONResultEnvelope(data, "", "", "")
	}
	return JSONResultEnvelope(data, c.ProjectID, c.ProjectOrg, c.ProjectName)
}

// JSONResult creates a CallToolResult containing the JSON-encoded value as
// text content. Prefer JSONResultEnvelope for Hookdeck MCP tools so responses
// follow the standard data/meta shape.
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
