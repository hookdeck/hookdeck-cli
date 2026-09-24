package mcp

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// Every tool split from one resource shares that resource's handler, and the
// handler fills in an omitted action with a read -- list or get. So on a write,
// pause or use tool, an omitted action ran a sibling tool's read and reported
// success: hookdeck_projects_use {} returned the project list and switched
// nothing, and gateway_connections_pause {} listed connections. An agent that
// forgot the argument was told it had worked.
//
// The schema marks action as required on every tool. Read tools keep their
// default; every other tool now enforces it.
//
// This walks every registered tool rather than a list, so a tool split in
// future is covered without anyone remembering to add it.
func TestNonReadToolsRequireAnAction(t *testing.T) {
	var apiCalls int
	session := mockAPIWithClientWriteEnabled(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/": func(w http.ResponseWriter, r *http.Request) {
			apiCalls++
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"models":[],"pagination":{}}`))
		},
	})

	tools, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)

	checked := 0
	for _, tool := range tools.Tools {
		props, _ := tool.InputSchema.(map[string]any)["properties"].(map[string]any)
		if _, takesAction := props["action"]; !takesAction {
			continue // login, help: no action to omit
		}
		if strings.HasSuffix(tool.Name, "_read") {
			continue // read tools may default to list or get
		}
		checked++

		t.Run(tool.Name, func(t *testing.T) {
			before := apiCalls
			result := callTool(t, session, tool.Name, map[string]any{})

			require.True(t, result.IsError, "%s with no action must be refused, not run a sibling's read", tool.Name)
			body := textContent(t, result)
			assert.Contains(t, body, "action is required")
			assert.Contains(t, body, "This tool offers:")
			assert.Equal(t, before, apiCalls, "nothing may reach the API")
		})
	}
	require.GreaterOrEqual(t, checked, 9, "expected at least the nine split tools that share a list/get default")
}

// Read tools keep defaulting, so the change is confined to the tools where the
// default ran something the caller could not have meant.
func TestReadToolsStillDefaultWhenActionIsOmitted(t *testing.T) {
	session := mockAPIWithClientWriteEnabled(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/": func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"models":[],"pagination":{}}`))
		},
	})
	result := callTool(t, session, "gateway_connections_read", map[string]any{})
	assert.False(t, result.IsError, textContent(t, result))
}
