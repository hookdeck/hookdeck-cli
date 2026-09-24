package mcpcore

import (
	"fmt"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// RequireAuth checks whether the API client has a valid API key. If not, it
// returns an error result directing the agent to the server's login tool.
// Callers should return immediately when the result is non-nil.
func RequireAuth(client *hookdeck.Client, loginTool string) *mcpsdk.CallToolResult {
	if client.APIKey == "" {
		return ErrorResult(fmt.Sprintf("Not authenticated. Please call the %s tool to authenticate with Hookdeck.", loginTool))
	}
	return nil
}

// RequireWrite guards a write action on a server started in read-only mode.
//
// The primary gate is the tool schema: a read-only server does not advertise
// write actions at all. This is the second line of defence, for a client that
// calls an action it was never offered. Callers should return immediately when
// the result is non-nil.
func RequireWrite(enabled bool, action string) *mcpsdk.CallToolResult {
	if enabled {
		return nil
	}
	return ErrorResult(fmt.Sprintf(
		"The %q action modifies data or returns a credential, and this MCP server is running in read-only mode. Restart it with --allow-write (or set HOOKDECK_MCP_ALLOW_WRITE=true) to enable write actions.",
		action,
	))
}
