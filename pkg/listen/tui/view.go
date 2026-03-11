package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// View renders the TUI with fixed header and scrollable event list
func (m Model) View() string {
	if !m.ready || !m.viewportReady {
		return ""
	}

	// If showing details, render full-screen details view with action bar
	if m.showingDetails {
		return m.renderDetailsView()
	}

	// Build fixed header (connection info + events title + divider)
	var header strings.Builder
	header.WriteString(m.renderConnectionInfo())
	header.WriteString("\n")

	// Add events title with divider
	eventsTitle := "Events • [↑↓] Navigate "
	titleLen := len(eventsTitle)
	remainingWidth := m.width - titleLen
	if remainingWidth < 0 {
		remainingWidth = 0
	}
	dividerLine := strings.Repeat("─", remainingWidth)
	header.WriteString(faintStyle.Render(eventsTitle + dividerLine))
	header.WriteString("\n")

	headerStr := header.String()
	headerHeight := m.calculateHeaderHeight(headerStr)

	// Build scrollable content for viewport
	var content strings.Builder

	// If not connected yet, show connecting status
	if !m.isConnected {
		content.WriteString("\n")
		content.WriteString(m.renderConnectingStatus())
		content.WriteString("\n")
	} else if !m.hasReceivedEvent {
		// If no events received yet, show waiting animation
		content.WriteString("\n")
		content.WriteString(m.renderWaitingStatus())
		content.WriteString("\n")
	} else {
		// Add newline before event history (part of scrollable content)
		content.WriteString("\n")
		// Render event history
		content.WriteString(m.renderEventHistory())
	}

	// Update viewport content
	m.viewport.SetContent(content.String())

	// Calculate exact viewport height
	// m.height is total LINES on screen
	// We need: header lines + viewport lines + divider (1) + status (1) = m.height

	var viewportHeight int
	if m.isConnected {
		// When connected, always show status bar (for server health indicator)
		// Total lines: header + viewport + divider + status
		viewportHeight = m.height - headerHeight - 2
	} else {
		// When not connected, no status bar
		// Total lines: header + viewport
		viewportHeight = m.height - headerHeight
	}

	if viewportHeight < 1 {
		viewportHeight = 1
	}
	m.viewport.Height = viewportHeight

	// Auto-scroll to bottom if tracking latest event
	if !m.userNavigated && len(m.events) > 0 {
		m.viewport.GotoBottom()
	}

	// Build output with exact line control
	output := headerStr // Header with its newlines

	// Viewport renders exactly viewportHeight lines
	viewportOutput := m.viewport.View()
	output += viewportOutput

	if m.isConnected {
		// When connected, always show status bar (includes server health indicator)
		// Ensure we have a newline before divider if viewport doesn't end with one
		if !strings.HasSuffix(viewportOutput, "\n") {
			output += "\n"
		}

		// Divider line
		divider := strings.Repeat("─", m.width)
		output += dividerStyle.Render(divider) + "\n"

		// Status bar - LAST line, no trailing newline
		output += m.renderStatusBar()
	} else {
		// Remove any trailing newline if no status bar
		output = strings.TrimSuffix(output, "\n")
	}

	return output
}

// renderConnectingStatus shows the connecting animation
func (m Model) renderConnectingStatus() string {
	dot := "●"
	if m.waitingFrameToggle {
		dot = "○"
	}

	return connectingDotStyle.Render(dot) + " Connecting..."
}

// renderWaitingStatus shows the waiting animation before first event
func (m Model) renderWaitingStatus() string {
	dot := "●"
	if m.waitingFrameToggle {
		dot = "○"
	}

	return waitingDotStyle.Render(dot) + " Connected. Waiting for events..."
}

// renderEventHistory renders all events with selection indicator on selected
func (m Model) renderEventHistory() string {
	if len(m.events) == 0 {
		return ""
	}

	var s strings.Builder

	// Render all events with selection indicator
	for i, event := range m.events {
		if i == m.selectedIndex {
			// Selected event - show with ">" prefix
			s.WriteString(selectionIndicatorStyle.Render("> "))
			s.WriteString(event.LogLine)
		} else {
			// Non-selected event - no prefix
			s.WriteString(event.LogLine)
		}
		s.WriteString("\n")
	}

	return s.String()
}

// renderDetailsView renders the details view with action bar at bottom
func (m Model) renderDetailsView() string {
	// Calculate space for action bar (divider + action bar = 2 lines)
	viewportHeight := m.height - 2
	if viewportHeight < 1 {
		viewportHeight = 1
	}
	m.detailsViewport.Height = viewportHeight

	var output strings.Builder

	// Viewport content (scrollable)
	output.WriteString(m.detailsViewport.View())
	output.WriteString("\n")

	// Divider line
	divider := strings.Repeat("─", m.width)
	output.WriteString(dividerStyle.Render(divider))
	output.WriteString("\n")

	// Action bar - LAST line, no trailing newline
	actionBar := "[d] Return to event list • [↑↓] Scroll • [PgUp/PgDn] Page"
	output.WriteString(statusBarStyle.Render(actionBar))

	return output.String()
}

// renderStatusBar renders the bottom status bar with keyboard shortcuts
func (m Model) renderStatusBar() string {
	// If no events yet, just show quit instruction
	selectedEvent := m.GetSelectedEvent()
	if selectedEvent == nil {
		return statusBarStyle.Width(m.width).Render("[q] Quit")
	}

	// Determine width-based verbosity
	// Threshold chosen to show full text only when it fits without wrapping
	// Full text requires ~105 chars with some padding
	isNarrow := m.width < 108
	isVeryNarrow := m.width < 60

	// Build event status message
	var eventStatusMsg string
	eventType := "Last event"
	if m.userNavigated {
		eventType = "Selected event"
	}

	if selectedEvent.Success {
		// Success status
		checkmark := greenStyle.Render("✓")
		if isVeryNarrow {
			eventStatusMsg = fmt.Sprintf("> %s %s [%d]", checkmark, eventType, selectedEvent.Status)
		} else if isNarrow {
			eventStatusMsg = fmt.Sprintf("> %s %s succeeded [%d] | [r] [o] [d] [q]",
				checkmark, eventType, selectedEvent.Status)
		} else {
			eventStatusMsg = fmt.Sprintf("> %s %s succeeded with status %d | [r] Retry • [o] Open in dashboard • [d] Show data • [q] Quit",
				checkmark, eventType, selectedEvent.Status)
		}
	} else {
		// Error status
		xmark := redStyle.Render("x")
		statusText := "failed"
		if selectedEvent.Status == 0 {
			statusText = "failed with error"
		} else {
			statusText = fmt.Sprintf("failed with status %d", selectedEvent.Status)
		}

		if isVeryNarrow {
			if selectedEvent.Status == 0 {
				eventStatusMsg = fmt.Sprintf("> %s %s [ERR]", xmark, eventType)
			} else {
				eventStatusMsg = fmt.Sprintf("> %s %s [%d]", xmark, eventType, selectedEvent.Status)
			}
		} else if isNarrow {
			if selectedEvent.Status == 0 {
				eventStatusMsg = fmt.Sprintf("> %s %s failed | [r] [o] [d] [q]",
					xmark, eventType)
			} else {
				eventStatusMsg = fmt.Sprintf("> %s %s failed [%d] | [r] [o] [d] [q]",
					xmark, eventType, selectedEvent.Status)
			}
		} else {
			eventStatusMsg = fmt.Sprintf("> %s %s %s | [r] Retry • [o] Open in dashboard • [d] Show event data • [q] Quit",
				xmark, eventType, statusText)
		}
	}

	return statusBarStyle.Width(m.width).Render(eventStatusMsg)
}

// FormatEventLog formats an event into a log line matching the current style
func FormatEventLog(event EventInfo, dashboardURL, consoleURL, projectMode string) string {
	localTime := event.Time.Format(timeLayout)

	// Build event URL
	var url string
	if projectMode == "console" {
		url = consoleURL + "/?event_id=" + event.ID
	} else {
		url = dashboardURL + "/events/" + event.ID
	}

	// Format based on whether request failed or succeeded
	if event.ResponseStatus == 0 && !event.Success {
		// Request failed completely (no response)
		return fmt.Sprintf("%s [%s] Failed to %s: network error",
			faintStyle.Render(localTime),
			redStyle.Render("ERROR"),
			event.Data.Body.Request.Method,
		)
	}

	// Format normal response
	durationMs := event.ResponseDuration.Milliseconds()
	requestURL := fmt.Sprintf("http://localhost%s", event.Data.Body.Path) // Simplified for now

	return fmt.Sprintf("%s [%s] %s %s %s %s %s",
		faintStyle.Render(localTime),
		ColorizeStatus(event.ResponseStatus),
		event.Data.Body.Request.Method,
		requestURL,
		faintStyle.Render(fmt.Sprintf("(%dms)", durationMs)),
		faintStyle.Render("→"),
		faintStyle.Render(url),
	)
}

// renderConnectionInfo renders the sources and connections header
func (m Model) renderConnectionInfo() string {
	// If header is collapsed, show compact view
	if m.headerCollapsed {
		return m.renderCompactHeader()
	}

	var s strings.Builder

	// Brand header
	s.WriteString(m.renderBrandHeader())
	s.WriteString("\n\n")

	// Title with source/connection count and collapse hint
	numSources := 0
	numConnections := 0
	if m.cfg.Sources != nil {
		numSources = len(m.cfg.Sources)
	}
	if m.cfg.Connections != nil {
		numConnections = len(m.cfg.Connections)
	}

	sourcesText := fmt.Sprintf("%d source", numSources)
	if numSources != 1 {
		sourcesText += "s"
	}
	connectionsText := fmt.Sprintf("%d connection", numConnections)
	if numConnections != 1 {
		connectionsText += "s"
	}

	listeningTitle := fmt.Sprintf("Listening on %s • %s • [i] Collapse", sourcesText, connectionsText)
	s.WriteString(faintStyle.Render(listeningTitle))
	s.WriteString("\n\n")

	// Group connections by source
	sourceConnections := make(map[string][]*struct {
		connection *interface{}
		destName   string
		cliPath    string
	})

	if m.cfg.Sources != nil && m.cfg.Connections != nil {
		for _, conn := range m.cfg.Connections {
			sourceID := conn.Source.ID
			destName := ""
			cliPath := ""

			if conn.FullName != nil {
				parts := strings.Split(*conn.FullName, "->")
				if len(parts) == 2 {
					destName = strings.TrimSpace(parts[1])
				}
			}

			if p := conn.Destination.GetCLIPath(); p != nil {
				cliPath = *p
			}

			if sourceConnections[sourceID] == nil {
				sourceConnections[sourceID] = make([]*struct {
					connection *interface{}
					destName   string
					cliPath    string
				}, 0)
			}

			sourceConnections[sourceID] = append(sourceConnections[sourceID], &struct {
				connection *interface{}
				destName   string
				cliPath    string
			}{nil, destName, cliPath})
		}

		// Render each source
		for i, source := range m.cfg.Sources {
			s.WriteString(boldStyle.Render(source.Name))
			s.WriteString("\n")

			// Show webhook URL
			s.WriteString("│  Requests to → ")
			s.WriteString(source.URL)
			s.WriteString("\n")

			// Show connections
			if conns, exists := sourceConnections[source.ID]; exists {
				numConns := len(conns)
				for j, conn := range conns {
					fullPath := m.cfg.TargetURL.Scheme + "://" + m.cfg.TargetURL.Host + conn.cliPath

					connDisplay := ""
					if conn.destName != "" {
						connDisplay = " " + faintStyle.Render(fmt.Sprintf("(%s)", conn.destName))
					}

					if j == numConns-1 {
						s.WriteString("└─ Forwards to → ")
					} else {
						s.WriteString("├─ Forwards to → ")
					}
					s.WriteString(fullPath)
					s.WriteString(connDisplay)
					s.WriteString("\n")
				}
			}

			// Add spacing between sources
			if i < len(m.cfg.Sources)-1 {
				s.WriteString("\n")
			}
		}
	}

	// Show server health warning if unhealthy
	if m.serverHealthChecked && !m.serverHealthy {
		s.WriteString("\n")
		targetURL := m.cfg.TargetURL.Scheme + "://" + m.cfg.TargetURL.Host
		warningMsg := fmt.Sprintf("%s %s is unreachable. Check the server is running", yellowStyle.Render("● Warning:"), targetURL)
		s.WriteString(warningMsg)
		s.WriteString("\n")
	}

	// Show filters if any are active
	if m.cfg.Filters != nil {
		// Type assert to SessionFilters and display each filter
		if filters, ok := m.cfg.Filters.(*hookdeck.SessionFilters); ok && filters != nil {
			s.WriteString("\n")
			s.WriteString(yellowStyle.Render("⏺"))
			s.WriteString(" Filters provided, only events matching the filter will be forwarded for this session\n")

			if filters.Body != nil {
				s.WriteString("  • Body: ")
				s.WriteString(faintStyle.Render(string(*filters.Body)))
				s.WriteString("\n")
			}
			if filters.Headers != nil {
				s.WriteString("  • Headers: ")
				s.WriteString(faintStyle.Render(string(*filters.Headers)))
				s.WriteString("\n")
			}
			if filters.Query != nil {
				s.WriteString("  • Query: ")
				s.WriteString(faintStyle.Render(string(*filters.Query)))
				s.WriteString("\n")
			}
			if filters.Path != nil {
				s.WriteString("  • Path: ")
				s.WriteString(faintStyle.Render(string(*filters.Path)))
				s.WriteString("\n")
			}
		}
	}

	// Dashboard/guest URL hint
	s.WriteString("\n")
	if m.cfg.GuestURL != "" {
		s.WriteString("💡 Sign up to make your webhook URL permanent: ")
		s.WriteString(m.cfg.GuestURL)
	} else {
		// Build URL with team_id query parameter
		var displayURL string
		if m.cfg.ProjectMode == "console" {
			displayURL = m.cfg.ConsoleBaseURL + "?team_id=" + m.cfg.ProjectID
		} else {
			displayURL = m.cfg.DashboardBaseURL + "/events/cli?team_id=" + m.cfg.ProjectID
		}
		s.WriteString("💡 View dashboard to inspect, retry & bookmark events: ")
		s.WriteString(displayURL)
	}
	s.WriteString("\n")

	return s.String()
}

// renderBrandHeader renders the Hookdeck CLI brand header
func (m Model) renderBrandHeader() string {
	// Connection visual with brand name
	leftLine := brandAccentStyle.Render("●──")
	rightLine := brandAccentStyle.Render("──●")
	brandName := brandStyle.Render(" HOOKDECK CLI ")
	return leftLine + brandName + rightLine
}

// renderCompactHeader renders a collapsed/compact version of the connection header
func (m Model) renderCompactHeader() string {
	var s strings.Builder

	// Brand header
	s.WriteString(m.renderBrandHeader())
	s.WriteString("\n\n")

	// Count sources and connections
	numSources := 0
	numConnections := 0
	if m.cfg.Sources != nil {
		numSources = len(m.cfg.Sources)
	}
	if m.cfg.Connections != nil {
		numConnections = len(m.cfg.Connections)
	}

	// Compact summary with toggle hint
	sourcesText := fmt.Sprintf("%d source", numSources)
	if numSources != 1 {
		sourcesText += "s"
	}
	connectionsText := fmt.Sprintf("%d connection", numConnections)
	if numConnections != 1 {
		connectionsText += "s"
	}

	summary := fmt.Sprintf("Listening on %s • %s • [i] Expand",
		sourcesText,
		connectionsText)
	s.WriteString(faintStyle.Render(summary))
	s.WriteString("\n")

	// Show server health warning if unhealthy (ensure it's always visible even when collapsed)
	if m.serverHealthChecked && !m.serverHealthy {
		s.WriteString("\n")
		targetURL := m.cfg.TargetURL.Scheme + "://" + m.cfg.TargetURL.Host
		warningMsg := fmt.Sprintf("%s %s is unreachable. Check the server is running", yellowStyle.Render("● Warning:"), targetURL)
		s.WriteString(warningMsg)
		s.WriteString("\n")
	}

	return s.String()
}

// Utility function to strip ANSI codes for length calculation (if needed)
func stripANSI(s string) string {
	// Lipgloss handles this internally, but we can provide a simple implementation
	// For now, we'll use the string as-is since Lipgloss manages rendering
	return lipgloss.NewStyle().Render(s)
}
