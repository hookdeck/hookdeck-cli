package mcp

import (
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// requireAuth checks whether the API client has a valid API key. If not, it
// returns an error result directing the agent to call hookdeck_login. Callers
// should return immediately when the result is non-nil.
func requireAuth(client *hookdeck.Client) *mcpsdk.CallToolResult {
	if client.APIKey == "" {
		return ErrorResult("Not authenticated. Please call the hookdeck_login tool to authenticate with Hookdeck.")
	}
	return nil
}
