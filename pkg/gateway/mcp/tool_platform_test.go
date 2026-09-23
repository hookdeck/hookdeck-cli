package mcp

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// use is available without --allow-write: a read-only session stuck in one
// project cannot investigate another.
func TestProjectsUseIsReachableInReadOnlyMode(t *testing.T) {
	tools := listTools(t, connectInMemory(t, newTestClient("https://api.hookdeck.com", "k")))
	require.Contains(t, tools, "hookdeck_projects_use")
	assert.Equal(t, []string{"use"}, actionEnum(t, tools["hookdeck_projects_use"]))
}

// API key management is excluded from MCP by decision. This asserts it at the
// registration level, not just on the spec: no tool, in either mode, on either
// server, may offer one.
func TestNoAPIKeyToolIsRegistered(t *testing.T) {
	for _, writeEnabled := range []bool{false, true} {
		tools := listTools(t, connectInMemoryWithMode(t, newTestClient("https://api.hookdeck.com", "k"), writeEnabled))
		for name, tool := range tools {
			assert.NotContains(t, strings.ToLower(name), "api_key",
				"API keys must not be reachable from MCP; use the CLI or the dashboard")
			assert.NotContains(t, strings.ToLower(name), "apikey")
			for _, action := range actionEnum(t, tool) {
				assert.NotContains(t, strings.ToLower(action), "key",
					"%s offers a key-shaped action (%s); an agent able to mint a credential "+
						"can grant itself access this server would refuse", name, action)
			}
		}
	}
}
