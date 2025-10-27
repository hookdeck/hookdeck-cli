package tui

import (
	"context"
	"fmt"
	"net/url"
	"os/exec"
	"runtime"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// Update handles all events in the Bubble Tea event loop
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tea.MouseMsg:
		// Ignore all mouse events (including scroll)
		// Navigation should only work with arrow keys
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		if !m.viewportReady {
			// Initialize viewport on first window size message
			// Reserve space for header (will be calculated dynamically) and status bar (3 lines)
			m.viewport = viewport.New(msg.Width, msg.Height-15) // Initial estimate
			m.viewportReady = true
			m.ready = true
		} else {
			// Update viewport dimensions
			m.viewport.Width = msg.Width
			// Height will be set properly in the View function
		}
		return m, nil

	case NewEventMsg:
		m.AddEvent(msg.Event)
		return m, nil

	case UpdateEventMsg:
		m.UpdateEvent(msg)
		return m, nil

	case ConnectingMsg:
		m.isConnected = false
		return m, nil

	case ConnectedMsg:
		m.isConnected = true
		return m, nil

	case DisconnectedMsg:
		m.isConnected = false
		return m, nil

	case TickWaitingMsg:
		// Toggle waiting animation
		if !m.hasReceivedEvent {
			m.waitingFrameToggle = !m.waitingFrameToggle
			return m, tickWaitingAnimation()
		}
		return m, nil

	case retryResultMsg:
		// Retry completed (new attempt will arrive via websocket as a new event)
		return m, nil

	case openBrowserResultMsg:
		// Browser opened, could show notification if needed
		return m, nil
	}

	return m, nil
}

// handleKeyPress processes keyboard input
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Always allow quit and header toggle
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "i", "I":
		// Toggle header collapsed/expanded
		m.headerCollapsed = !m.headerCollapsed
		return m, nil
	}

	// Disable other shortcuts until connected and first event received
	if !m.isConnected || !m.hasReceivedEvent {
		return m, nil
	}

	// Handle navigation and actions
	switch msg.String() {
	case "up", "k":
		if m.showingDetails {
			// Scroll details view up
			m.detailsViewport.LineUp(1)
			return m, nil
		}
		if m.Navigate(-1) {
			return m, nil
		}

	case "down", "j":
		if m.showingDetails {
			// Scroll details view down
			m.detailsViewport.LineDown(1)
			return m, nil
		}
		if m.Navigate(1) {
			return m, nil
		}

	case "pgup":
		if m.showingDetails {
			m.detailsViewport.ViewUp()
			return m, nil
		}

	case "pgdown":
		if m.showingDetails {
			m.detailsViewport.ViewDown()
			return m, nil
		}

	case "r", "R":
		// Retry selected event (new attempt will arrive via websocket)
		return m, m.retrySelectedEvent()

	case "o", "O":
		// Open event in browser
		return m, m.openSelectedEventInBrowser()

	case "d", "D":
		// Toggle event details view
		if m.showingDetails {
			// Close details view
			m.showingDetails = false
		} else {
			// Open details view
			selectedEvent := m.GetSelectedEvent()
			if selectedEvent != nil {
				m.detailsContent = m.buildDetailsContent(selectedEvent)
				m.showingDetails = true

				// Initialize details viewport if not already done
				m.detailsViewport = viewport.New(m.width, m.height)
				m.detailsViewport.SetContent(m.detailsContent)
				m.detailsViewport.GotoTop()
			}
		}
		return m, nil

	case "esc":
		// Close details view
		if m.showingDetails {
			m.showingDetails = false
			return m, nil
		}
	}

	return m, nil
}

// retrySelectedEvent retries the currently selected event
func (m Model) retrySelectedEvent() tea.Cmd {
	selectedEvent := m.GetSelectedEvent()
	if selectedEvent == nil || selectedEvent.ID == "" {
		return nil
	}

	eventID := selectedEvent.ID
	apiKey := m.cfg.APIKey
	apiBaseURL := m.cfg.APIBaseURL
	projectID := m.cfg.ProjectID

	return func() tea.Msg {
		// Create HTTP client
		parsedBaseURL, err := url.Parse(apiBaseURL)
		if err != nil {
			return retryResultMsg{err: err}
		}

		client := &hookdeck.Client{
			BaseURL:   parsedBaseURL,
			APIKey:    apiKey,
			ProjectID: projectID,
		}

		// Make retry request
		retryURL := fmt.Sprintf("/events/%s/retry", eventID)
		resp, err := client.Post(context.Background(), retryURL, []byte("{}"), nil)
		if err != nil {
			return retryResultMsg{err: err}
		}
		defer resp.Body.Close()

		return retryResultMsg{success: true}
	}
}

// openSelectedEventInBrowser opens the event in the dashboard
func (m Model) openSelectedEventInBrowser() tea.Cmd {
	selectedEvent := m.GetSelectedEvent()
	if selectedEvent == nil || selectedEvent.ID == "" {
		return nil
	}

	return func() tea.Msg {
		// Build event URL with team_id query parameter
		var eventURL string
		if m.cfg.ProjectMode == "console" {
			eventURL = m.cfg.ConsoleBaseURL + "/?event_id=" + selectedEvent.ID + "&team_id=" + m.cfg.ProjectID
		} else {
			eventURL = m.cfg.DashboardBaseURL + "/events/" + selectedEvent.ID + "?team_id=" + m.cfg.ProjectID
		}

		// Open in browser
		err := openBrowser(eventURL)
		return openBrowserResultMsg{err: err}
	}
}

// openBrowser opens a URL in the default browser (cross-platform)
func openBrowser(url string) error {
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

// Result messages

type retryResultMsg struct {
	success bool
	err     error
}

type openBrowserResultMsg struct {
	err error
}
