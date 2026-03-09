package mcp

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

const (
	loginPollInterval = 2 * time.Second
	loginMaxAttempts  = 120 // ~4 minutes
)

func handleLogin(client *hookdeck.Client, cfg *config.Config, mcpServer *mcpsdk.Server) mcpsdk.ToolHandler {
	return func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		// If already authenticated, let the caller know.
		if client.APIKey != "" {
			return TextResult("Already authenticated. All Hookdeck tools are available."), nil
		}

		parsedBaseURL, err := url.Parse(cfg.APIBaseURL)
		if err != nil {
			return ErrorResult(fmt.Sprintf("Invalid API base URL: %s", err)), nil
		}

		deviceName, _ := os.Hostname()

		// Initiate browser-based device auth flow.
		authClient := &hookdeck.Client{BaseURL: parsedBaseURL}
		session, err := authClient.StartLogin(deviceName)
		if err != nil {
			return ErrorResult(fmt.Sprintf("Failed to start login: %s", err)), nil
		}

		// Poll until the user completes login or we time out.
		response, err := session.WaitForAPIKey(loginPollInterval, loginMaxAttempts)
		if err != nil {
			return &mcpsdk.CallToolResult{
				Content: []mcpsdk.Content{
					&mcpsdk.TextContent{Text: fmt.Sprintf(
						"Authentication timed out or failed: %s\n\nPlease try again by calling hookdeck_login.\nTo authenticate, the user needs to open this URL in their browser:\n\n%s",
						err, session.BrowserURL,
					)},
				},
				IsError: true,
			}, nil
		}

		if err := validators.APIKey(response.APIKey); err != nil {
			return ErrorResult(fmt.Sprintf("Received invalid API key: %s", err)), nil
		}

		// Persist credentials so future MCP sessions start authenticated.
		cfg.Profile.APIKey = response.APIKey
		cfg.Profile.ProjectId = response.ProjectID
		cfg.Profile.ProjectMode = response.ProjectMode
		cfg.Profile.GuestURL = "" // Clear guest URL for permanent accounts.

		if err := cfg.Profile.SaveProfile(); err != nil {
			return ErrorResult(fmt.Sprintf("Login succeeded but failed to save profile: %s", err)), nil
		}
		if err := cfg.Profile.UseProfile(); err != nil {
			return ErrorResult(fmt.Sprintf("Login succeeded but failed to activate profile: %s", err)), nil
		}

		// Update the shared client so all resource tools start working.
		client.APIKey = response.APIKey
		client.ProjectID = response.ProjectID

		// Remove the login tool now that auth is complete. This sends
		// notifications/tools/list_changed to clients that support it.
		mcpServer.RemoveTools("hookdeck_login")

		return TextResult(fmt.Sprintf(
			"Successfully authenticated as %s (%s).\nActive project: %s in organization %s.\nAll Hookdeck tools are now available.",
			response.UserName, response.UserEmail,
			response.ProjectName, response.OrganizationName,
		)), nil
	}
}
