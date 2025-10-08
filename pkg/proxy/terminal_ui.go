package proxy

import (
	"fmt"
	"os"
	"sync"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	"golang.org/x/term"
)

// TerminalUI handles all terminal rendering and display logic
type TerminalUI struct {
	terminalMutex         sync.Mutex
	rawModeState          *term.State
	statusLineShown       bool
	waitingAnimationFrame int
	eventHistory          *EventHistory
}

// NewTerminalUI creates a new TerminalUI instance
func NewTerminalUI(eventHistory *EventHistory) *TerminalUI {
	return &TerminalUI{
		eventHistory: eventHistory,
	}
}

// SetRawModeState stores the terminal's raw mode state for safe printing
func (ui *TerminalUI) SetRawModeState(state *term.State) {
	ui.rawModeState = state
}

// SafePrintf temporarily disables raw mode, prints the message, then re-enables raw mode
func (ui *TerminalUI) SafePrintf(format string, args ...interface{}) {
	ui.terminalMutex.Lock()
	defer ui.terminalMutex.Unlock()

	// Temporarily restore normal terminal mode for printing
	if ui.rawModeState != nil {
		term.Restore(int(os.Stdin.Fd()), ui.rawModeState)
	}

	// Print the message
	fmt.Printf(format, args...)

	// Re-enable raw mode
	if ui.rawModeState != nil {
		term.MakeRaw(int(os.Stdin.Fd()))
	}
}

// calculateEventLines calculates how many terminal lines an event log occupies
// accounting for line wrapping based on terminal width
func (ui *TerminalUI) calculateEventLines(logLine string) int {
	// Get terminal width
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width <= 0 {
		width = 80 // Default fallback
	}

	// Add 2 for the potential "> " prefix or "  " indentation
	lineLength := len(logLine) + 2

	// Calculate how many lines this will occupy
	lines := (lineLength + width - 1) / width // Ceiling division
	if lines < 1 {
		lines = 1
	}
	return lines
}

// BuildStatusMessage generates the status line message based on the current state
func (ui *TerminalUI) BuildStatusMessage(hasReceivedEvent bool) string {
	color := ansi.Color(os.Stdout)

	// If no events received yet, show waiting animation
	if !hasReceivedEvent {
		var dot string
		if ui.waitingAnimationFrame%2 == 0 {
			dot = fmt.Sprintf("%s", color.Green("●"))
		} else {
			dot = fmt.Sprintf("%s", color.Green("○"))
		}
		ui.waitingAnimationFrame++
		return fmt.Sprintf("%s Connected. Waiting for events...", dot)
	}

	// Get the selected event to show its status
	selectedEvent := ui.eventHistory.GetSelectedEvent()
	if selectedEvent == nil {
		return "" // No events available
	}

	// If user has navigated, show "Selected event"
	if ui.eventHistory.IsUserNavigated() {
		if selectedEvent.Success {
			return fmt.Sprintf("> %s Selected event succeeded with status %d | [↑↓] Navigate • [r] Retry • [o] Open in dashboard • [d] Show data • [Ctrl+C] Quit",
				color.Green("✓"), selectedEvent.Status)
		} else {
			if selectedEvent.Status == 0 {
				return fmt.Sprintf("> %s Selected event failed with error | [↑↓] Navigate • [r] Retry • [o] Open in dashboard • [d] Show data & • [Ctrl+C] Quit",
					color.Red("x").Bold())
			} else {
				return fmt.Sprintf("> %s Selected event failed with status %d | [↑↓] Navigate • [r] Retry • [o] Open in dashboard • [d] Show data • [Ctrl+C] Quit",
					color.Red("x").Bold(), selectedEvent.Status)
			}
		}
	}

	// Auto-selecting latest event - show "Last event"
	if selectedEvent.Success {
		return fmt.Sprintf("> %s Last event succeeded with status %d | [↑↓] Navigate • [r] Retry • [o] Open in dashboard • [d] Show data • [Ctrl+C] Quit",
			color.Green("✓"), selectedEvent.Status)
	} else {
		if selectedEvent.Status == 0 {
			return fmt.Sprintf("> %s Last event failed with error | [↑↓] Navigate • [r] Retry • [o] Open in dashboard • [d] Show request details • [Ctrl+C] Quit",
				color.Red("x").Bold())
		} else {
			return fmt.Sprintf("> %s Last event failed with status %d | [↑↓] Navigate • [r] Retry • [o] Open in dashboard • [d] Show request details • [Ctrl+C] Quit",
				color.Red("x").Bold(), selectedEvent.Status)
		}
	}
}

// UpdateStatusLine updates the bottom status line with the latest event information
func (ui *TerminalUI) UpdateStatusLine(hasReceivedEvent bool) {
	ui.terminalMutex.Lock()
	defer ui.terminalMutex.Unlock()

	// Only update if we haven't received any events yet (just the waiting animation)
	if hasReceivedEvent {
		return
	}

	// Temporarily restore normal terminal mode for printing
	if ui.rawModeState != nil {
		term.Restore(int(os.Stdin.Fd()), ui.rawModeState)
	}

	// Generate status message (waiting animation)
	statusMsg := ui.BuildStatusMessage(hasReceivedEvent)

	if ui.statusLineShown {
		// If we've shown a status before, move up one line and clear it
		fmt.Printf("\033[1A\033[2K\r%s\n", statusMsg)
	} else {
		// First time showing status
		fmt.Printf("%s\n", statusMsg)
		ui.statusLineShown = true
	}

	// Re-enable raw mode
	if ui.rawModeState != nil {
		term.MakeRaw(int(os.Stdin.Fd()))
	}
}

// PrintEventAndUpdateStatus prints the event log and updates the status line in one operation
func (ui *TerminalUI) PrintEventAndUpdateStatus(eventInfo EventInfo, hasReceivedEvent bool, showingDetails bool) {
	ui.terminalMutex.Lock()
	defer ui.terminalMutex.Unlock()

	// Always add event to history (so it's available when returning from details view)
	ui.eventHistory.AddEvent(eventInfo)

	// Skip all terminal rendering if details view is showing (less has control of the screen)
	if showingDetails {
		return
	}

	// Check if this is the 11th event - need to add "Events" title before the first historical event
	isEleventhEvent := ui.eventHistory.Count() == maxNavigableEvents && !ui.eventHistory.IsEventsTitleDisplayed()

	// If this is the 11th event, print the "Events" title now (before adding the event)
	if isEleventhEvent {
		// Temporarily restore normal terminal mode for printing
		if ui.rawModeState != nil {
			term.Restore(int(os.Stdin.Fd()), ui.rawModeState)
		}

		// Move up to clear status line and blank line
		fmt.Print("\033[2A\033[2K\r\033[1B\033[2K\r\033[1A")

		// Print "Events" title with newline above
		color := ansi.Color(os.Stdout)
		fmt.Printf("\n%s\n\n", color.Faint("Events"))

		// Print blank line and status that will be replaced
		fmt.Println()
		statusMsg := fmt.Sprintf("%s Adding...", color.Faint("●"))
		fmt.Printf("%s\n", statusMsg)

		ui.eventHistory.SetEventsTitleDisplayed(true)

		// Re-enable raw mode
		if ui.rawModeState != nil {
			term.MakeRaw(int(os.Stdin.Fd()))
		}
	}

	// Check if any event will exit the navigable window when we add this new event
	// We need to remove indentation from events becoming immutable
	needToRedrawForExitingEvents := false
	if ui.eventHistory.Count() >= maxNavigableEvents {
		needToRedrawForExitingEvents = true
	}

	// Check if we need to redraw due to selection changes
	needToClearOldSelection := false
	if ui.eventHistory.IsUserNavigated() && ui.eventHistory.Count() > 0 {
		// Calculate what the navigable range will be after adding the new event
		futureHistorySize := ui.eventHistory.Count() + 1
		futureNavigableStartIdx := futureHistorySize - maxNavigableEvents
		if futureNavigableStartIdx < 0 {
			futureNavigableStartIdx = 0
		}

		// If current selection will be outside future navigable range, we need to redraw
		// (The selected event will be pinned in the display, breaking chronological order)
		if ui.eventHistory.GetSelectedIndex() < futureNavigableStartIdx {
			needToClearOldSelection = true
		}
	}

	// Redraw navigable window if events are exiting or selection is being cleared
	// BUT skip if we just printed the Events title (11th event case)
	if (needToRedrawForExitingEvents || needToClearOldSelection) && !isEleventhEvent {
		// Temporarily restore normal terminal mode for printing
		if ui.rawModeState != nil {
			term.Restore(int(os.Stdin.Fd()), ui.rawModeState)
		}

		events := ui.eventHistory.GetEvents()
		selectedIndex := ui.eventHistory.GetSelectedIndex()

		// Calculate current navigable window
		currentNavigableStartIdx := len(events) - maxNavigableEvents
		if currentNavigableStartIdx < 0 {
			currentNavigableStartIdx = 0
		}
		currentNumNavigableEvents := len(events) - currentNavigableStartIdx

		// Calculate future navigable window to determine which event will become immutable
		futureHistorySize := len(events) + 1
		futureNavigableStartIdx := futureHistorySize - maxNavigableEvents
		if futureNavigableStartIdx < 0 {
			futureNavigableStartIdx = 0
		}

		// Move cursor up and clear
		// Account for: navigable events + separator (3 lines if present) + blank + status
		linesToMoveUp := currentNumNavigableEvents + 2 // events + blank + status
		// If we'll have a separator, add 3 more lines (blank line + "Recent events" + blank line)
		if futureNavigableStartIdx > 0 {
			linesToMoveUp += 3
		}
		fmt.Printf("\033[%dA", linesToMoveUp)
		fmt.Print("\033[J")

		// NOTE: We NEVER redraw the "Events" title - it was printed once and stays permanent

		// Redraw events
		for i := currentNavigableStartIdx; i < len(events); i++ {
			// Events that will become immutable (fall outside future navigable range) have no indentation
			if i < futureNavigableStartIdx {
				fmt.Printf("%s\n", events[i].LogLine) // No indentation
			} else {
				// Add "Latest events" separator before first navigable event
				if i == futureNavigableStartIdx {
					color := ansi.Color(os.Stdout)
					fmt.Printf("\n%s\n\n", color.Faint("Latest events (↑↓ to navigate)")) // Extra newline after separator
				}
				// Only indent selected event with ">", others have no indentation
				if i == selectedIndex {
					fmt.Printf("> %s\n", events[i].LogLine) // Selected
				} else {
					fmt.Printf("%s\n", events[i].LogLine) // No indentation
				}
			}
		}

		// Blank line
		fmt.Println()

		// Status message (will be replaced soon)
		color := ansi.Color(os.Stdout)
		statusMsg := fmt.Sprintf("%s Updating...", color.Faint("●"))
		fmt.Printf("%s\n", statusMsg)

		// Re-enable raw mode
		if ui.rawModeState != nil {
			term.MakeRaw(int(os.Stdin.Fd()))
		}
	}

	// Note: Event was already added to history at the start of this function
	// (before the showingDetails check, so events are still tracked while viewing details)

	// Temporarily restore normal terminal mode for printing
	if ui.rawModeState != nil {
		term.Restore(int(os.Stdin.Fd()), ui.rawModeState)
	}

	events := ui.eventHistory.GetEvents()
	selectedIndex := ui.eventHistory.GetSelectedIndex()

	// Calculate the navigable window (last 10 events)
	navigableStartIdx := len(events) - maxNavigableEvents
	if navigableStartIdx < 0 {
		navigableStartIdx = 0
	}
	numNavigableEvents := len(events) - navigableStartIdx

	// If we have multiple navigable events and auto-selecting, redraw navigable window
	// Also redraw if user has navigated (to show pinned selection)
	if numNavigableEvents > 1 && !ui.eventHistory.IsUserNavigated() {
		// Auto-selecting mode: redraw to move selection to latest
		// Calculate total terminal lines occupied by previous navigable events
		totalEventLines := 0
		for i := navigableStartIdx; i < len(events)-1; i++ {
			totalEventLines += ui.calculateEventLines(events[i].LogLine)
		}
		linesToMoveUp := totalEventLines + 2 // previous event lines + blank + status
		fmt.Printf("\033[%dA", linesToMoveUp)
		fmt.Print("\033[J")

		// Print navigable events with selection on the latest
		for i := navigableStartIdx; i < len(events); i++ {
			if i == selectedIndex {
				fmt.Printf("> %s\n", events[i].LogLine)
			} else {
				fmt.Printf("%s\n", events[i].LogLine) // No indentation
			}
		}
	} else if ui.eventHistory.IsUserNavigated() && numNavigableEvents > 1 {
		// User has navigated: redraw to show pinned selected event
		// Get the navigable events (includes pinned selected event if applicable)
		navigableIndices := ui.eventHistory.GetNavigableEvents()

		// Calculate total terminal lines occupied by previous navigable events
		totalEventLines := 0
		for i := 0; i < len(navigableIndices)-1; i++ {
			totalEventLines += ui.calculateEventLines(events[navigableIndices[i]].LogLine)
		}
		linesToMoveUp := totalEventLines + 2 // previous event lines + blank + status
		fmt.Printf("\033[%dA", linesToMoveUp)
		fmt.Print("\033[J")

		// Print navigable events (including pinned event) with selection indicator
		for _, idx := range navigableIndices {
			if idx == selectedIndex {
				fmt.Printf("> %s\n", events[idx].LogLine)
			} else {
				fmt.Printf("%s\n", events[idx].LogLine)
			}
		}
	} else {
		// First event - simple append
		if ui.statusLineShown {
			if len(events) == 1 {
				// First event - only clear the "waiting" status line
				fmt.Print("\033[1A\033[2K\r")
			} else {
				// Clear status line and blank line
				fmt.Print("\033[2A\033[2K\r\033[1B\033[2K\r\033[1A")
			}
		}

		// Print the new event
		newEventIndex := len(events) - 1
		// Only indent if selected, otherwise no indentation
		if selectedIndex == newEventIndex {
			fmt.Printf("> %s\n", events[newEventIndex].LogLine)
		} else {
			fmt.Printf("%s\n", events[newEventIndex].LogLine) // No indentation
		}
	}

	// Blank line
	fmt.Println()

	// Generate and print status message
	statusMsg := ui.BuildStatusMessage(hasReceivedEvent)
	fmt.Printf("%s\n", statusMsg)
	ui.statusLineShown = true

	// Re-enable raw mode
	if ui.rawModeState != nil {
		term.MakeRaw(int(os.Stdin.Fd()))
	}
}

// RedrawAfterDetailsView redraws the event list after returning from the details view
// less uses alternate screen, so the original screen content should be restored automatically
// We just need to redraw the events that may have arrived while viewing details
func (ui *TerminalUI) RedrawAfterDetailsView(hasReceivedEvent bool) {
	ui.terminalMutex.Lock()
	defer ui.terminalMutex.Unlock()

	// Temporarily restore normal terminal mode for printing
	if ui.rawModeState != nil {
		term.Restore(int(os.Stdin.Fd()), ui.rawModeState)
	}

	// After less exits, the terminal should have restored the original screen
	// We need to redraw the entire navigable events section since events may have arrived

	events := ui.eventHistory.GetEvents()
	if len(events) == 0 {
		// Re-enable raw mode
		if ui.rawModeState != nil {
			term.MakeRaw(int(os.Stdin.Fd()))
		}
		return
	}

	selectedIndex := ui.eventHistory.GetSelectedIndex()

	// Get the navigable events (includes pinned selected event if applicable)
	navigableIndices := ui.eventHistory.GetNavigableEvents()

	// Calculate the normal navigable start for determining if we need separator
	normalNavigableStartIdx := len(events) - maxNavigableEvents
	if normalNavigableStartIdx < 0 {
		normalNavigableStartIdx = 0
	}

	// Calculate how many lines to move up: navigable events + separator (if present) + blank + status
	totalEventLines := 0
	for _, idx := range navigableIndices {
		totalEventLines += ui.calculateEventLines(events[idx].LogLine)
	}
	linesToMoveUp := totalEventLines + 2 // event lines + blank + status
	if normalNavigableStartIdx > 0 {
		linesToMoveUp += 3 // blank + "Latest events" + blank
	}

	// Move cursor up and clear everything below
	fmt.Printf("\033[%dA", linesToMoveUp)
	fmt.Print("\033[J")

	// Add separator if there are historical events
	if normalNavigableStartIdx > 0 {
		color := ansi.Color(os.Stdout)
		fmt.Printf("\n%s\n\n", color.Faint("Latest events (↑↓ to navigate)"))
	}

	// Print the navigable events with selection indicator
	for _, idx := range navigableIndices {
		if idx == selectedIndex {
			fmt.Printf("> %s\n", events[idx].LogLine) // Selected event with >
		} else {
			fmt.Printf("%s\n", events[idx].LogLine) // No indentation
		}
	}

	// Add a newline before the status line
	fmt.Println()

	// Generate and print the status message for the selected event
	statusMsg := ui.BuildStatusMessage(hasReceivedEvent)
	fmt.Printf("%s\n", statusMsg)
	ui.statusLineShown = true

	// Re-enable raw mode
	if ui.rawModeState != nil {
		term.MakeRaw(int(os.Stdin.Fd()))
	}
}

// RedrawEventsWithSelection updates the selection indicators without clearing the screen (only last 10 events)
func (ui *TerminalUI) RedrawEventsWithSelection(hasReceivedEvent bool) {
	if ui.eventHistory.Count() == 0 {
		return
	}

	ui.terminalMutex.Lock()
	defer ui.terminalMutex.Unlock()

	// Temporarily restore normal terminal mode for printing
	if ui.rawModeState != nil {
		term.Restore(int(os.Stdin.Fd()), ui.rawModeState)
	}

	events := ui.eventHistory.GetEvents()
	selectedIndex := ui.eventHistory.GetSelectedIndex()

	// Get the navigable events (includes pinned selected event if applicable)
	navigableIndices := ui.eventHistory.GetNavigableEvents()

	// Calculate the normal navigable start for determining if we need separator
	normalNavigableStartIdx := len(events) - maxNavigableEvents
	if normalNavigableStartIdx < 0 {
		normalNavigableStartIdx = 0
	}

	// Calculate total terminal lines occupied by navigable events
	totalEventLines := 0
	for _, idx := range navigableIndices {
		totalEventLines += ui.calculateEventLines(events[idx].LogLine)
	}

	// Move cursor up to start of navigable events and clear everything below
	linesToMoveUp := totalEventLines + 2 // event lines + blank + status
	// If we have a separator, add 3 more lines (blank line + "Latest events" + blank line)
	if normalNavigableStartIdx > 0 {
		linesToMoveUp += 3
	}
	fmt.Printf("\033[%dA", linesToMoveUp)
	fmt.Print("\033[J")

	// NOTE: We NEVER redraw the "Events" title - it was printed once and stays permanent

	// Add separator if there are historical events
	if normalNavigableStartIdx > 0 {
		color := ansi.Color(os.Stdout)
		fmt.Printf("\n%s\n\n", color.Faint("Latest events (↑↓ to navigate)")) // Extra newline after separator
	}

	// Print the navigable events (including pinned event if applicable) with selection indicator
	for _, idx := range navigableIndices {
		if idx == selectedIndex {
			fmt.Printf("> %s\n", events[idx].LogLine) // Selected event with >
		} else {
			fmt.Printf("%s\n", events[idx].LogLine) // No indentation
		}
	}

	// Add a newline before the status line
	fmt.Println()

	// Generate and print the status message for the selected event
	statusMsg := ui.BuildStatusMessage(hasReceivedEvent)
	fmt.Printf("%s\n", statusMsg)
	ui.statusLineShown = true

	// Re-enable raw mode
	if ui.rawModeState != nil {
		term.MakeRaw(int(os.Stdin.Fd()))
	}
}
