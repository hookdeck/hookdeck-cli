package mcp

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

const (
	loginPollInterval = 2 * time.Second
	loginMaxAttempts  = 120 // ~4 minutes
)

// loginState tracks a background login poll so that repeated calls to
// hookdeck_login don't start duplicate auth flows.
type loginState struct {
	mu         sync.Mutex
	browserURL string        // URL the user must open
	done       chan struct{} // closed when polling finishes
	err        error         // non-nil if polling failed
}

func handleLogin(client *hookdeck.Client, cfg *config.Config, mcpServer *mcpsdk.Server) mcpsdk.ToolHandler {
	var state *loginState

	return func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		// Already authenticated — nothing to do.
		if client.APIKey != "" {
			return TextResult("Already authenticated. All Hookdeck tools are available."), nil
		}

		// If a login flow is already in progress, check its status.
		if state != nil {
			select {
			case <-state.done:
				// Polling finished — check result.
				if state.err != nil {
					errMsg := state.err.Error()
					browserURL := state.browserURL
					state = nil // allow a fresh retry
					return ErrorResult(fmt.Sprintf(
						"Authentication failed: %s\n\nPlease call hookdeck_login again to retry.\nThe user needs to open this URL in their browser:\n\n%s",
						errMsg, browserURL,
					)), nil
				}
				// Success was already handled by the goroutine (client.APIKey set).
				return TextResult("Already authenticated. All Hookdeck tools are available."), nil
			default:
				// Still polling — remind the agent about the URL.
				return TextResult(fmt.Sprintf(
					"Login is already in progress. Waiting for the user to complete authentication.\n\nThe user needs to open this URL in their browser:\n\n%s\n\nCall hookdeck_login again to check status.",
					state.browserURL,
				)), nil
			}
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

		// Set up background polling state.
		state = &loginState{
			browserURL: session.BrowserURL,
			done:       make(chan struct{}),
		}

		// Poll in the background so we return the URL to the agent immediately.
		go func(s *loginState) {
			defer close(s.done)

			response, err := session.WaitForAPIKey(loginPollInterval, loginMaxAttempts)
			if err != nil {
				s.mu.Lock()
				s.err = err
				s.mu.Unlock()
				log.WithError(err).Debug("Login polling failed")
				return
			}

			if err := validators.APIKey(response.APIKey); err != nil {
				s.mu.Lock()
				s.err = fmt.Errorf("received invalid API key: %s", err)
				s.mu.Unlock()
				return
			}

			// Persist credentials so future MCP sessions start authenticated.
			cfg.Profile.APIKey = response.APIKey
			cfg.Profile.ProjectId = response.ProjectID
			cfg.Profile.ProjectMode = response.ProjectMode
			cfg.Profile.GuestURL = ""

			if err := cfg.Profile.SaveProfile(); err != nil {
				log.WithError(err).Error("Login succeeded but failed to save profile")
			}
			if err := cfg.Profile.UseProfile(); err != nil {
				log.WithError(err).Error("Login succeeded but failed to activate profile")
			}

			// Update the shared client so all resource tools start working.
			client.APIKey = response.APIKey
			client.ProjectID = response.ProjectID

			// Remove the login tool now that auth is complete.
			mcpServer.RemoveTools("hookdeck_login")

			log.WithFields(log.Fields{
				"user":    response.UserName,
				"project": response.ProjectName,
			}).Info("MCP login completed successfully")
		}(state)

		// Return the URL immediately so the agent can show it to the user.
		return TextResult(fmt.Sprintf(
			"Login initiated. The user must open the following URL in their browser to authenticate:\n\n%s\n\nOnce the user completes authentication in the browser, all Hookdeck tools will become available.\nCall hookdeck_login again to check if authentication has completed.",
			session.BrowserURL,
		)), nil
	}
}
