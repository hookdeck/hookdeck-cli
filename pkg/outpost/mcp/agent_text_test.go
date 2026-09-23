package mcp

import (
	"encoding/json"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
)

// TestAgentFacingTextNamesOnlyRealTools mirrors the Event Gateway guard.
//
// Help output, tool descriptions and error hints told agents to call
// `hookdeck_projects`, `outpost_tenants` and `outpost_destinations` — all names
// that stopped existing when each tool was split into `_read` and `_write`.
// Nothing failed to compile, because the names were either built by a helper or
// written as prose, so this reads every tool name an agent is shown and
// requires it to be a tool that exists.
func TestAgentFacingTextNamesOnlyRealTools(t *testing.T) {
	api := mockAPI(t, nil)

	// Write mode with a publish key, because that advertises the superset.
	session := connect(t, ServerOptions{
		Client:        newTestClient(t, api.URL),
		Config:        &config.Config{},
		WriteEnabled:  true,
		PublishAPIKey: "pk",
	})
	tools := listTools(t, session)

	real := make(map[string]bool, len(tools))
	for name := range tools {
		real[name] = true
	}
	// Read-only names too: help legitimately mentions the write tools while
	// describing what --allow-write unlocks, and vice versa.
	readOnly := connect(t, ServerOptions{Client: newTestClient(t, api.URL), Config: &config.Config{}})
	for name := range listTools(t, readOnly) {
		real[name] = true
	}

	mention := regexp.MustCompile(`\b(?:gateway|hookdeck|outpost)_[a-z_]+\b`)

	check := func(where, text string) {
		for _, name := range mention.FindAllString(text, -1) {
			assert.True(t, real[name],
				"%s names %q, which is not a tool this server advertises; "+
					"an agent following that text gets `unknown tool`", where, name)
		}
	}

	for name, tool := range tools {
		check("the description of "+name, tool.Description)
		if tool.InputSchema != nil {
			schema, err := json.Marshal(tool.InputSchema)
			require.NoError(t, err)
			check("the input schema of "+name, string(schema))
		}
	}

	check("outpost_help overview", resultText(t, callTool(t, session, "outpost_help", map[string]any{})))
	for name := range tools {
		check("outpost_help topic="+name,
			resultText(t, callTool(t, session, "outpost_help", map[string]any{"topic": name})))
	}
}
