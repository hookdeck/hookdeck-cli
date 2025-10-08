package proxy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// EventActions handles actions on selected events (retry, open, view details)
type EventActions struct {
	cfg          *Config
	eventHistory *EventHistory
	ui           *TerminalUI
}

// NewEventActions creates a new EventActions instance
func NewEventActions(cfg *Config, eventHistory *EventHistory, ui *TerminalUI) *EventActions {
	return &EventActions{
		cfg:          cfg,
		eventHistory: eventHistory,
		ui:           ui,
	}
}

// RetrySelectedEvent retries the currently selected event
func (ea *EventActions) RetrySelectedEvent() {
	selectedEvent := ea.eventHistory.GetSelectedEvent()
	if selectedEvent == nil {
		color := ansi.Color(os.Stdout)
		ea.ui.SafePrintf("[%s] No event selected to retry\n",
			color.Yellow("WARN"),
		)
		return
	}

	eventID := selectedEvent.ID
	if eventID == "" {
		color := ansi.Color(os.Stdout)
		ea.ui.SafePrintf("[%s] Selected event has no ID to retry\n",
			color.Yellow("WARN"),
		)
		return
	}

	// Create HTTP client for retry request
	parsedBaseURL, err := url.Parse(ea.cfg.APIBaseURL)
	if err != nil {
		color := ansi.Color(os.Stdout)
		ea.ui.SafePrintf("[%s] Failed to parse API URL for retry: %v\n",
			color.Red("ERROR").Bold(),
			err,
		)
		return
	}

	client := &hookdeck.Client{
		BaseURL:   parsedBaseURL,
		APIKey:    ea.cfg.Key,
		ProjectID: ea.cfg.ProjectID,
	}

	// Make retry request to Hookdeck API
	retryURL := fmt.Sprintf("/events/%s/retry", eventID)
	resp, err := client.Post(context.Background(), retryURL, []byte("{}"), nil)
	if err != nil {
		color := ansi.Color(os.Stdout)
		ea.ui.SafePrintf("[%s] Failed to retry event %s: %v\n",
			color.Red("ERROR").Bold(),
			eventID,
			err,
		)
		return
	}
	defer resp.Body.Close()
}

// OpenSelectedEventURL opens the currently selected event in the browser
func (ea *EventActions) OpenSelectedEventURL() {
	selectedEvent := ea.eventHistory.GetSelectedEvent()
	if selectedEvent == nil {
		color := ansi.Color(os.Stdout)
		ea.ui.SafePrintf("[%s] No event selected to open\n",
			color.Yellow("WARN"),
		)
		return
	}

	eventID := selectedEvent.ID
	if eventID == "" {
		color := ansi.Color(os.Stdout)
		ea.ui.SafePrintf("[%s] Selected event has no ID to open\n",
			color.Yellow("WARN"),
		)
		return
	}

	// Build event URL based on project mode
	var eventURL string
	if ea.cfg.ProjectMode == "console" {
		eventURL = ea.cfg.ConsoleBaseURL + "/?event_id=" + eventID
	} else {
		eventURL = ea.cfg.DashboardBaseURL + "/events/" + eventID
	}

	// Open URL in browser
	err := ea.openBrowser(eventURL)
	if err != nil {
		color := ansi.Color(os.Stdout)
		ea.ui.SafePrintf("[%s] Failed to open browser: %v\n",
			color.Red("ERROR").Bold(),
			err,
		)
		return
	}
}

// openBrowser opens a URL in the default browser
func (ea *EventActions) openBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start", url}
	case "darwin":
		cmd = "open"
		args = []string{url}
	default: // "linux", "freebsd", "openbsd", "netbsd"
		cmd = "xdg-open"
		args = []string{url}
	}

	return exec.Command(cmd, args...).Start()
}

// ShowEventDetails displays detailed event information using less pager
func (ea *EventActions) ShowEventDetails() (bool, error) {
	selectedEvent := ea.eventHistory.GetSelectedEvent()
	if selectedEvent == nil || selectedEvent.Data == nil {
		return false, nil
	}

	// Build the details content
	webhookEvent := selectedEvent.Data
	color := ansi.Color(os.Stdout)
	var content strings.Builder

	// Header with navigation hints
	content.WriteString(ansi.Bold("Event Details"))
	content.WriteString("\n")
	content.WriteString(ansi.Faint("Press 'q' to return to events • Use arrow keys/Page Up/Down to scroll"))
	content.WriteString("\n")
	content.WriteString(ansi.Faint("───────────────────────────────────────────────────────────────────────────────"))
	content.WriteString("\n\n")

	// Request section
	content.WriteString(ansi.Bold("Request"))
	content.WriteString("\n\n")
	// Construct the full URL with query params
	fullURL := ea.cfg.URL.Scheme + "://" + ea.cfg.URL.Host + ea.cfg.URL.Path + webhookEvent.Body.Path
	content.WriteString(fmt.Sprintf("%s %s\n\n", color.Bold(webhookEvent.Body.Request.Method), fullURL))

	// Headers section
	if len(webhookEvent.Body.Request.Headers) > 0 {
		content.WriteString(ansi.Bold("Headers"))
		content.WriteString("\n\n")

		var headers map[string]json.RawMessage
		if err := json.Unmarshal(webhookEvent.Body.Request.Headers, &headers); err == nil {
			keys := make([]string, 0, len(headers))
			for key := range headers {
				keys = append(keys, key)
			}
			sort.Strings(keys)

			for _, key := range keys {
				unquoted, _ := strconv.Unquote(string(headers[key]))
				content.WriteString(fmt.Sprintf("%s: %s\n", ansi.Faint(strings.ToLower(key)), unquoted))
			}
		}
		content.WriteString("\n")
	}

	// Body section
	if len(webhookEvent.Body.Request.DataString) > 0 {
		content.WriteString(ansi.Bold("Body"))
		content.WriteString("\n\n")

		var bodyData interface{}
		if err := json.Unmarshal([]byte(webhookEvent.Body.Request.DataString), &bodyData); err == nil {
			prettyJSON, err := json.MarshalIndent(bodyData, "", "  ")
			if err == nil {
				content.WriteString(string(prettyJSON))
				content.WriteString("\n")
			}
		} else {
			content.WriteString(webhookEvent.Body.Request.DataString)
			content.WriteString("\n")
		}
	}

	// Response section
	content.WriteString("\n")
	content.WriteString(ansi.Faint("───────────────────────────────────────────────────────────────────────────────"))
	content.WriteString("\n\n")

	// Check if this was an error (no response received)
	if selectedEvent.ResponseStatus == 0 && selectedEvent.ResponseBody == "" {
		// Request failed - no response received
		content.WriteString(ansi.Bold("Response"))
		content.WriteString("\n\n")
		content.WriteString(color.Red("Request failed - no response received").String())
		content.WriteString("\n")
	} else {
		// Response header with status and duration
		responseStatusText := fmt.Sprintf("%d", selectedEvent.ResponseStatus)
		if selectedEvent.ResponseStatus >= 200 && selectedEvent.ResponseStatus < 300 {
			responseStatusText = color.Green(responseStatusText).String()
		} else if selectedEvent.ResponseStatus >= 400 {
			responseStatusText = color.Red(responseStatusText).String()
		} else if selectedEvent.ResponseStatus >= 300 {
			responseStatusText = color.Yellow(responseStatusText).String()
		}

		durationMs := selectedEvent.ResponseDuration.Milliseconds()
		content.WriteString(fmt.Sprintf("%s • %s • %dms\n\n",
			ansi.Bold("Response"),
			responseStatusText,
			durationMs,
		))

		// Response headers section
		if len(selectedEvent.ResponseHeaders) > 0 {
			content.WriteString(ansi.Bold("Headers"))
			content.WriteString("\n\n")

			// Sort header keys for consistent display
			keys := make([]string, 0, len(selectedEvent.ResponseHeaders))
			for key := range selectedEvent.ResponseHeaders {
				keys = append(keys, key)
			}
			sort.Strings(keys)

			for _, key := range keys {
				values := selectedEvent.ResponseHeaders[key]
				// Join multiple values with comma
				content.WriteString(fmt.Sprintf("%s: %s\n",
					ansi.Faint(strings.ToLower(key)),
					strings.Join(values, ", "),
				))
			}
			content.WriteString("\n")
		}

		// Response body section
		if len(selectedEvent.ResponseBody) > 0 {
			content.WriteString(ansi.Bold("Body"))
			content.WriteString("\n\n")

			var bodyData interface{}
			if err := json.Unmarshal([]byte(selectedEvent.ResponseBody), &bodyData); err == nil {
				prettyJSON, err := json.MarshalIndent(bodyData, "", "  ")
				if err == nil {
					content.WriteString(string(prettyJSON))
					content.WriteString("\n")
				}
			} else {
				content.WriteString(selectedEvent.ResponseBody)
				content.WriteString("\n")
			}
		} else {
			content.WriteString(ansi.Faint("(empty)"))
			content.WriteString("\n\n")
		}
	}

	// Footer
	content.WriteString("\n")
	content.WriteString(ansi.Faint("Press 'q' to return to events • Use arrow keys/Page Up/Down to scroll"))
	content.WriteString("\n")

	// Create a temporary file for the content
	tmpfile, err := os.CreateTemp("", "hookdeck-event-*.txt")
	if err != nil {
		// Fallback: print directly
		fmt.Print(content.String())
		return false, nil
	}
	defer os.Remove(tmpfile.Name())

	// Write content to temp file
	if _, err := tmpfile.Write([]byte(content.String())); err != nil {
		tmpfile.Close()
		fmt.Print(content.String())
		return false, nil
	}
	tmpfile.Close()

	// Use less with options:
	// -R: Allow ANSI color codes
	// -P: Custom prompt to hide filename (show blank or custom message)
	cmd := exec.Command("less", "-R", "-P?eEND:.", tmpfile.Name())

	// CRITICAL: Restore normal terminal mode BEFORE opening /dev/tty
	// Our keyboard handler has put stdin in raw mode, and /dev/tty shares the same terminal
	// We need to restore normal mode so less can properly initialize its terminal handling
	ea.ui.TemporarilyRestoreNormalMode()

	// CRITICAL: Open /dev/tty directly so less doesn't use stdin (which our keyboard handler is reading from)
	// This gives less exclusive terminal access and prevents our keyboard handler from seeing its input
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		// Fallback: use stdin (but this means keyboard handler might see input)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		defer tty.Close()
		// Give less exclusive access via /dev/tty
		cmd.Stdin = tty
		cmd.Stdout = tty
		cmd.Stderr = tty
	}

	// Run less and wait for it to exit (it takes over terminal control)
	err = cmd.Run()

	// Re-enable raw mode after less exits
	ea.ui.ReEnableRawMode()

	if err != nil {
		// Fallback: print directly
		fmt.Print(content.String())
		return false, nil
	}

	return true, nil
}
