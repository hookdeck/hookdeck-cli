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
	content.WriteString(ansi.Faint("| Press 'q' to return to events • Use arrow keys/Page Up/Down to scroll"))
	content.WriteString("\n")
	content.WriteString(ansi.Faint("───────────────────────────────────────────────────────────────────────────────"))
	content.WriteString("\n\n")

	// Event metadata
	timestampStr := selectedEvent.Time.Format(timeLayout)
	statusIcon := color.Green("✓")
	statusText := "succeeded"
	statusDisplay := color.Bold(fmt.Sprintf("%d", selectedEvent.Status))
	if !selectedEvent.Success {
		statusIcon = color.Red("x").Bold()
		statusText = "failed"
		if selectedEvent.Status == 0 {
			statusDisplay = color.Bold("error")
		}
	}

	content.WriteString(fmt.Sprintf("%s Event %s with status %s at %s\n", statusIcon, statusText, statusDisplay, ansi.Faint(timestampStr)))
	content.WriteString("\n")

	// Dashboard URL
	dashboardURL := ea.cfg.DashboardBaseURL
	if ea.cfg.ProjectID != "" {
		dashboardURL += "/cli/events/" + selectedEvent.ID
	}
	if ea.cfg.ProjectMode == "console" {
		dashboardURL = ea.cfg.ConsoleBaseURL
	}
	content.WriteString(fmt.Sprintf("%s %s\n", ansi.Faint("🔗"), ansi.Faint(dashboardURL)))
	content.WriteString("\n")
	content.WriteString(ansi.Faint("───────────────────────────────────────────────────────────────────────────────"))
	content.WriteString("\n\n")

	// Request section
	content.WriteString(ansi.Bold("Request"))
	content.WriteString("\n\n")
	// Construct the full URL with query params
	fullURL := ea.cfg.URL.Scheme + "://" + ea.cfg.URL.Host + ea.cfg.URL.Path + webhookEvent.Body.Path
	content.WriteString(fmt.Sprintf("%s %s\n", color.Bold(webhookEvent.Body.Request.Method), fullURL))
	content.WriteString("\n")
	content.WriteString(ansi.Faint("───────────────────────────────────────────────────────────────────────────────"))
	content.WriteString("\n\n")

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
		content.WriteString(ansi.Faint("───────────────────────────────────────────────────────────────────────────────"))
		content.WriteString("\n\n")
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

	// Footer
	content.WriteString("\n")
	content.WriteString(fmt.Sprintf("%s Use arrow keys/Page Up/Down to scroll • Press 'q' to return to events\n", ansi.Faint("⌨️")))

	// Use less with standard options
	cmd := exec.Command("sh", "-c", "less -R")

	// Create stdin pipe to send content
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		// Fallback: print directly
		fmt.Print(content.String())
		return false, nil
	}

	// Connect to terminal
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Start less
	if err := cmd.Start(); err != nil {
		// Fallback: print directly
		fmt.Print(content.String())
		return false, nil
	}

	// Write content to less
	stdinPipe.Write([]byte(content.String()))
	stdinPipe.Close()

	// Wait for less to exit
	cmd.Wait()

	return true, nil
}
