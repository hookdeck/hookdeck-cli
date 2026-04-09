package mcp

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/project"
	"github.com/hookdeck/hookdeck-cli/pkg/validators"
)

const (
	loginPollInterval = 2 * time.Second
	loginMaxAttempts  = 120 // ~4 minutes
)

// loginState tracks a background login poll so that repeated calls to
// hookdeck_login don't start duplicate auth flows.
//
// Synchronization: err is written by the goroutine before close(done).
// The handler only reads err after receiving from done, so the channel
// close provides the happens-before guarantee — no separate mutex needed.
type loginState struct {
	browserURL string        // URL the user must open
	done       chan struct{} // closed when polling finishes
	err        error         // non-nil if polling failed
}

func handleLogin(srv *Server) mcpsdk.ToolHandler {
	client := srv.client
	cfg := srv.cfg
	var stateMu sync.Mutex
	var state *loginState

	return func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		in, err := parseInput(req.Params.Arguments)
		if err != nil {
			return ErrorResult(err.Error()), nil
		}
		reauth := in.Bool("reauth")

		stateMu.Lock()
		defer stateMu.Unlock()

		if reauth && client.APIKey != "" {
			if state != nil {
				select {
				case <-state.done:
					state = nil
				default:
					return ErrorResult(
						"A login flow is already in progress. Call hookdeck_login again after it completes, then use reauth: true if you still need to sign in again.",
					), nil
				}
			}
			if err := cfg.ClearActiveProfileCredentials(); err != nil {
				return ErrorResult(fmt.Sprintf("reauth: could not clear stored credentials: %v", err)), nil
			}
			client.APIKey = ""
			client.ProjectID = ""
			client.ProjectOrg = ""
			client.ProjectName = ""
		}

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
		authClient := &hookdeck.Client{BaseURL: parsedBaseURL, TelemetryDisabled: cfg.TelemetryDisabled}
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
		// WaitForAPIKey blocks with time.Sleep internally, so we run it in an
		// inner goroutine and select on the session-level context (not the
		// per-request ctx, which is cancelled when this handler returns).
		sessionCtx := srv.sessionCtx
		go func(s *loginState) {
			defer close(s.done)

			type pollResult struct {
				resp *hookdeck.PollAPIKeyResponse
				err  error
			}
			ch := make(chan pollResult, 1)
			go func() {
				resp, err := session.WaitForAPIKey(loginPollInterval, loginMaxAttempts)
				ch <- pollResult{resp, err}
			}()

			var response *hookdeck.PollAPIKeyResponse
			select {
			case <-sessionCtx.Done():
				s.err = fmt.Errorf("login cancelled: MCP session closed")
				log.Debug("Login polling cancelled — MCP session closed")
				return
			case r := <-ch:
				if r.err != nil {
					s.err = r.err
					log.WithError(r.err).Debug("Login polling failed")
					return
				}
				response = r.resp
			}

			if err := validators.APIKey(response.APIKey); err != nil {
				s.err = fmt.Errorf("received invalid API key: %s", err)
				return
			}

			// Persist credentials so future MCP sessions start authenticated.
			cfg.Profile.ApplyPollAPIKeyResponse(response, "")

			cfg.SaveActiveProfileAfterLogin()

			// Update the server-held client (in production this is the same pointer as
			// config.GetAPIClient(); tests inject a separate *hookdeck.Client, so we must
			// mutate this handle — RefreshCachedAPIClient only touches the global singleton).
			client.APIKey = response.APIKey
			client.ProjectID = response.ProjectID
			org, proj, err := project.ParseProjectName(response.ProjectName)
			if err != nil {
				org, proj = "", response.ProjectName
			}
			if o := strings.TrimSpace(response.OrganizationName); o != "" {
				org = o
			}
			client.ProjectOrg = org
			client.ProjectName = proj

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
