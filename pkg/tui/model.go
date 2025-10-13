package tui

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	hookdecksdk "github.com/hookdeck/hookdeck-go-sdk"

	"github.com/hookdeck/hookdeck-cli/pkg/websocket"
)

const (
	maxEvents  = 1000                  // Maximum events to keep in memory (all navigable)
	timeLayout = "2006-01-02 15:04:05" // Time format for display
)

// EventInfo represents a single event with all its data
type EventInfo struct {
	ID               string // Event ID from Hookdeck
	AttemptID        string // Attempt ID (unique per retry)
	Status           int
	Success          bool
	Time             time.Time
	Data             *websocket.Attempt
	LogLine          string
	ResponseStatus   int
	ResponseHeaders  map[string][]string
	ResponseBody     string
	ResponseDuration time.Duration
}

// Model is the Bubble Tea model for the interactive TUI
type Model struct {
	// Configuration
	cfg *Config

	// Event history
	events        []EventInfo
	selectedIndex int
	userNavigated bool // Track if user has manually navigated away from latest

	// UI state
	ready              bool
	hasReceivedEvent   bool
	isConnected        bool
	waitingFrameToggle bool
	width              int
	height             int
	viewport           viewport.Model
	viewportReady      bool
	headerHeight       int // Height of the fixed header

	// Details view state
	showingDetails   bool
	detailsViewport  viewport.Model
	detailsContent   string
	eventsTitleShown bool // Track if "Events" title has been displayed

	// Header state
	headerCollapsed bool // Track if connection header is collapsed
}

// Config holds configuration for the TUI
type Config struct {
	DeviceName       string
	APIKey           string
	APIBaseURL       string
	DashboardBaseURL string
	ConsoleBaseURL   string
	ProjectMode      string
	ProjectID        string
	GuestURL         string
	TargetURL        *url.URL
	Sources          []*hookdecksdk.Source
	Connections      []*hookdecksdk.Connection
}

// NewModel creates a new TUI model
func NewModel(cfg *Config) Model {
	return Model{
		cfg:           cfg,
		events:        make([]EventInfo, 0),
		selectedIndex: -1,
		ready:         false,
		isConnected:   false,
	}
}

// Init initializes the model (required by Bubble Tea)
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tickWaitingAnimation(),
	)
}

// AddEvent adds a new event to the history
func (m *Model) AddEvent(event EventInfo) {
	// Check for duplicates using Time + EventID
	// This allows the same event to appear multiple times if retried at different times
	// while preventing true duplicates from the same moment
	for i := len(m.events) - 1; i >= 0; i-- {
		if m.events[i].ID == event.ID && m.events[i].Time.Equal(event.Time) {
			return // Duplicate, skip
		}
	}

	// Record if user is on the current latest before adding new event
	wasOnLatest := m.selectedIndex == len(m.events)-1

	// Add event
	m.events = append(m.events, event)

	// Trim to maxEvents if exceeded - old events just disappear
	if len(m.events) > maxEvents {
		removeCount := len(m.events) - maxEvents
		m.events = m.events[removeCount:]

		// Adjust selected index
		if m.selectedIndex >= 0 {
			m.selectedIndex -= removeCount
			if m.selectedIndex < 0 {
				// Selected event was removed, select latest
				m.selectedIndex = len(m.events) - 1
				m.userNavigated = false
			}
		}
	}

	// If user was on the latest event when new event arrived, resume auto-tracking
	if m.userNavigated && wasOnLatest {
		m.userNavigated = false
	}

	// Auto-select latest unless user has manually navigated
	if !m.userNavigated {
		m.selectedIndex = len(m.events) - 1
		// Note: viewport will be scrolled in View() after content is updated
	}

	// Mark as having received first event and auto-collapse header
	if !m.hasReceivedEvent {
		m.hasReceivedEvent = true
		m.headerCollapsed = true // Auto-collapse on first event
	}
}

// UpdateEvent updates an existing event by EventID + Time, or creates a new one if not found
func (m *Model) UpdateEvent(update UpdateEventMsg) {
	// Find event by EventID + Time (same uniqueness criteria as AddEvent)
	for i := range m.events {
		if m.events[i].ID == update.EventID && m.events[i].Time.Equal(update.Time) {
			// Update event fields
			m.events[i].Status = update.Status
			m.events[i].Success = update.Success
			m.events[i].LogLine = update.LogLine
			m.events[i].ResponseStatus = update.ResponseStatus
			m.events[i].ResponseHeaders = update.ResponseHeaders
			m.events[i].ResponseBody = update.ResponseBody
			m.events[i].ResponseDuration = update.ResponseDuration
			return
		}
	}

	// Event not found (response came back in < 100ms, so pending event was never created)
	// Create a new event with the complete data
	newEvent := EventInfo{
		ID:               update.EventID,
		AttemptID:        update.AttemptID,
		Status:           update.Status,
		Success:          update.Success,
		Time:             update.Time,
		Data:             update.Data,
		LogLine:          update.LogLine,
		ResponseStatus:   update.ResponseStatus,
		ResponseHeaders:  update.ResponseHeaders,
		ResponseBody:     update.ResponseBody,
		ResponseDuration: update.ResponseDuration,
	}
	m.AddEvent(newEvent)
}

// Navigate moves selection up or down (all events are navigable)
func (m *Model) Navigate(direction int) bool {
	if len(m.events) == 0 {
		return false
	}

	// Ensure selected index is valid
	if m.selectedIndex < 0 || m.selectedIndex >= len(m.events) {
		m.selectedIndex = len(m.events) - 1
		m.userNavigated = false
		return false
	}

	// Calculate new position
	newIndex := m.selectedIndex + direction

	// Clamp to valid range
	if newIndex < 0 {
		newIndex = 0
	} else if newIndex >= len(m.events) {
		newIndex = len(m.events) - 1
	}

	if newIndex != m.selectedIndex {
		m.selectedIndex = newIndex
		m.userNavigated = true

		// Don't reset userNavigated here to avoid jump when navigating to latest
		// It will be reset in AddEvent() when a new event arrives while on latest

		// Auto-scroll viewport to keep selected event visible
		m.scrollToSelectedEvent()

		return true
	}

	return false
}

// scrollToSelectedEvent scrolls the viewport to keep the selected event visible
func (m *Model) scrollToSelectedEvent() {
	if !m.viewportReady || m.selectedIndex < 0 {
		return
	}

	// Each event is one line, selected event is at line m.selectedIndex
	// Add 1 to account for the leading newline in renderEventHistory
	lineNum := m.selectedIndex + 1

	// Scroll to make this line visible
	if lineNum < m.viewport.YOffset {
		// Selected is above visible area, scroll up
		m.viewport.YOffset = lineNum
	} else if lineNum >= m.viewport.YOffset+m.viewport.Height {
		// Selected is below visible area, scroll down
		m.viewport.YOffset = lineNum - m.viewport.Height + 1
	}

	// Clamp offset
	if m.viewport.YOffset < 0 {
		m.viewport.YOffset = 0
	}
}

// GetSelectedEvent returns the currently selected event
func (m *Model) GetSelectedEvent() *EventInfo {
	if len(m.events) == 0 {
		return nil
	}

	if m.selectedIndex < 0 || m.selectedIndex >= len(m.events) {
		m.selectedIndex = len(m.events) - 1
		m.userNavigated = false
	}

	return &m.events[m.selectedIndex]
}

// calculateHeaderHeight counts the number of lines in the header
func (m *Model) calculateHeaderHeight(header string) int {
	return strings.Count(header, "\n") + 1
}

// buildDetailsContent builds the formatted details view for an event
func (m *Model) buildDetailsContent(event *EventInfo) string {
	var content strings.Builder

	content.WriteString(faintStyle.Render("[d] Return to event list • [↑↓] Scroll • [PgUp/PgDn] Page"))
	content.WriteString("\n\n")

	// Event metadata - compact single line format
	var metadataLine strings.Builder
	metadataLine.WriteString(event.ID)
	metadataLine.WriteString(" • ")
	metadataLine.WriteString(event.Time.Format(timeLayout))
	if event.ResponseDuration > 0 {
		metadataLine.WriteString(" • ")
		metadataLine.WriteString(event.ResponseDuration.String())
	}
	content.WriteString(metadataLine.String())
	content.WriteString("\n")
	content.WriteString(faintStyle.Render(strings.Repeat("─", 63)))
	content.WriteString("\n\n")

	// Request section
	if event.Data != nil {
		content.WriteString(boldStyle.Render("Request"))
		content.WriteString("\n\n")

		// HTTP request line: METHOD URL
		requestURL := m.cfg.TargetURL.Scheme + "://" + m.cfg.TargetURL.Host + event.Data.Body.Path
		content.WriteString(event.Data.Body.Request.Method + " " + requestURL + "\n\n")

		// Request headers
		if len(event.Data.Body.Request.Headers) > 0 {
			// Parse headers JSON
			var headers map[string]string
			if err := json.Unmarshal(event.Data.Body.Request.Headers, &headers); err == nil {
				for key, value := range headers {
					content.WriteString(faintStyle.Render(key+": ") + value + "\n")
				}
			} else {
				content.WriteString(string(event.Data.Body.Request.Headers) + "\n")
			}
		}
		content.WriteString("\n")

		// Request body
		if event.Data.Body.Request.DataString != "" {
			// Try to pretty print JSON
			prettyBody := m.prettyPrintJSON(event.Data.Body.Request.DataString)
			content.WriteString(prettyBody + "\n")
		}
		content.WriteString("\n")
	}

	// Response section
	content.WriteString(boldStyle.Render("Response"))
	content.WriteString("\n\n")

	if event.ResponseStatus > 0 {
		// HTTP status line
		content.WriteString(fmt.Sprintf("%d", event.ResponseStatus) + "\n\n")

		// Response headers
		if len(event.ResponseHeaders) > 0 {
			for key, values := range event.ResponseHeaders {
				for _, value := range values {
					content.WriteString(faintStyle.Render(key+": ") + value + "\n")
				}
			}
		}
		content.WriteString("\n")

		// Response body
		if event.ResponseBody != "" {
			// Try to pretty print JSON
			prettyBody := m.prettyPrintJSON(event.ResponseBody)
			content.WriteString(prettyBody + "\n")
		}
	} else {
		content.WriteString(faintStyle.Render("(No response received yet)") + "\n")
	}

	return content.String()
}

// prettyPrintJSON attempts to pretty print JSON, returns original if not valid JSON
func (m *Model) prettyPrintJSON(input string) string {
	var obj interface{}
	if err := json.Unmarshal([]byte(input), &obj); err != nil {
		// Not valid JSON, return original
		return input
	}

	// Pretty print with 2-space indentation
	pretty, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		// Fallback to original
		return input
	}

	return string(pretty)
}

// Messages for Bubble Tea

// NewEventMsg is sent when a new webhook event arrives
type NewEventMsg struct {
	Event EventInfo
}

// UpdateEventMsg is sent when an existing event gets a response
type UpdateEventMsg struct {
	EventID          string             // Event ID from Hookdeck
	AttemptID        string             // Attempt ID (unique per connection)
	Time             time.Time          // Event time
	Data             *websocket.Attempt // Full attempt data
	Status           int
	Success          bool
	LogLine          string
	ResponseStatus   int
	ResponseHeaders  map[string][]string
	ResponseBody     string
	ResponseDuration time.Duration
}

// ConnectingMsg is sent when starting to connect
type ConnectingMsg struct{}

// ConnectedMsg is sent when websocket connects
type ConnectedMsg struct{}

// DisconnectedMsg is sent when websocket disconnects
type DisconnectedMsg struct{}

// TickWaitingMsg is sent to animate waiting indicator
type TickWaitingMsg struct{}

func tickWaitingAnimation() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return TickWaitingMsg{}
	})
}
