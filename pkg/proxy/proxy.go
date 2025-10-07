package proxy

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	"golang.org/x/term"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/websocket"
	hookdecksdk "github.com/hookdeck/hookdeck-go-sdk"
)

const timeLayout = "2006-01-02 15:04:05"
const maxHistorySize = 50     // Maximum events to keep in memory
const maxNavigableEvents = 10 // Only last 10 events are navigable

//
// Public types
//

// Config provides the configuration of a Proxy
type Config struct {
	// DeviceName is the name of the device sent to Hookdeck to help identify the device
	DeviceName string
	// Key is the API key used to authenticate with Hookdeck
	Key              string
	ProjectID        string
	ProjectMode      string
	URL              *url.URL
	APIBaseURL       string
	DashboardBaseURL string
	ConsoleBaseURL   string
	WSBaseURL        string
	// Indicates whether to print full JSON objects to stdout
	PrintJSON bool
	Log       *log.Logger
	// Force use of unencrypted ws:// protocol instead of wss://
	NoWSS    bool
	Insecure bool
}

// EventInfo represents a single event for navigation
type EventInfo struct {
	ID      string
	Status  int
	Success bool
	Time    time.Time
	Data    *websocket.Attempt
	LogLine string
}

// A Proxy opens a websocket connection with Hookdeck, listens for incoming
// webhook events, forwards them to the local endpoint and sends the response
// back to Hookdeck.
type Proxy struct {
	cfg                *Config
	connections        []*hookdecksdk.Connection
	webSocketClient    *websocket.Client
	connectionTimer    *time.Timer
	latestEventID      string
	latestEventStatus  int
	latestEventSuccess bool
	latestEventTime    time.Time
	latestEventData    *websocket.Attempt
	hasReceivedEvent   bool
	statusLineShown    bool
	terminalMutex      sync.Mutex
	rawModeState       *term.State
	// Event navigation
	eventHistory         []EventInfo
	selectedEventIndex   int
	userNavigated        bool // Track if user has manually navigated away from latest event
	eventsTitleDisplayed bool // Track if "Events" title has been displayed
	// Waiting animation
	waitingAnimationFrame int
	stopWaitingAnimation  chan bool
	// Details view
	showingDetails bool // Track if we're in alternate screen showing details
	// Connection state
	isConnected bool // Track if we're currently connected (disable actions during reconnection)
}

func withSIGTERMCancel(ctx context.Context, onCancel func()) context.Context {
	// Create a context that will be canceled when Ctrl+C is pressed
	ctx, cancel := context.WithCancel(ctx)

	interruptCh := make(chan os.Signal, 1)
	signal.Notify(interruptCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-interruptCh
		onCancel()
		cancel()
	}()
	return ctx
}

// calculateEventLines calculates how many terminal lines an event log occupies
// accounting for line wrapping based on terminal width
func (p *Proxy) calculateEventLines(logLine string) int {
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

// safePrintf temporarily disables raw mode, prints the message, then re-enables raw mode
func (p *Proxy) safePrintf(format string, args ...interface{}) {
	p.terminalMutex.Lock()
	defer p.terminalMutex.Unlock()

	// Temporarily restore normal terminal mode for printing
	if p.rawModeState != nil {
		term.Restore(int(os.Stdin.Fd()), p.rawModeState)
	}

	// Print the message
	fmt.Printf(format, args...)

	// Re-enable raw mode
	if p.rawModeState != nil {
		term.MakeRaw(int(os.Stdin.Fd()))
	}
}

// printEventAndUpdateStatus prints the event log and updates the status line in one operation
func (p *Proxy) printEventAndUpdateStatus(eventLog string) {
	p.terminalMutex.Lock()
	defer p.terminalMutex.Unlock()

	// Check if this is the 11th event - need to add "Events" title before the first historical event
	isEleventhEvent := len(p.eventHistory) == maxNavigableEvents && !p.eventsTitleDisplayed

	// If this is the 11th event, print the "Events" title now (before adding the event)
	if isEleventhEvent {
		// Temporarily restore normal terminal mode for printing
		if p.rawModeState != nil {
			term.Restore(int(os.Stdin.Fd()), p.rawModeState)
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

		p.eventsTitleDisplayed = true

		// Re-enable raw mode
		if p.rawModeState != nil {
			term.MakeRaw(int(os.Stdin.Fd()))
		}
	}

	// Check if any event will exit the navigable window when we add this new event
	// We need to remove indentation from events becoming immutable
	needToRedrawForExitingEvents := false
	if len(p.eventHistory) >= maxNavigableEvents {
		needToRedrawForExitingEvents = true
	}

	// Check if we need to redraw due to selection changes
	needToClearOldSelection := false
	if p.userNavigated && len(p.eventHistory) > 0 {
		// Calculate what the navigable range will be after adding the new event
		futureHistorySize := len(p.eventHistory) + 1
		futureNavigableStartIdx := futureHistorySize - maxNavigableEvents
		if futureNavigableStartIdx < 0 {
			futureNavigableStartIdx = 0
		}

		// If current selection will be outside future navigable range, we need to redraw
		// (The selected event will be pinned in the display, breaking chronological order)
		if p.selectedEventIndex < futureNavigableStartIdx {
			needToClearOldSelection = true
		}
	}

	// Redraw navigable window if events are exiting or selection is being cleared
	// BUT skip if we just printed the Events title (11th event case)
	if (needToRedrawForExitingEvents || needToClearOldSelection) && !isEleventhEvent {
		// Temporarily restore normal terminal mode for printing
		if p.rawModeState != nil {
			term.Restore(int(os.Stdin.Fd()), p.rawModeState)
		}

		// Calculate current navigable window
		currentNavigableStartIdx := len(p.eventHistory) - maxNavigableEvents
		if currentNavigableStartIdx < 0 {
			currentNavigableStartIdx = 0
		}
		currentNumNavigableEvents := len(p.eventHistory) - currentNavigableStartIdx

		// Calculate future navigable window to determine which event will become immutable
		futureHistorySize := len(p.eventHistory) + 1
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
		for i := currentNavigableStartIdx; i < len(p.eventHistory); i++ {
			// Events that will become immutable (fall outside future navigable range) have no indentation
			if i < futureNavigableStartIdx {
				fmt.Printf("%s\n", p.eventHistory[i].LogLine) // No indentation
			} else {
				// Add "Latest events" separator before first navigable event
				if i == futureNavigableStartIdx {
					color := ansi.Color(os.Stdout)
					fmt.Printf("\n%s\n\n", color.Faint("Latest events (↑↓ to navigate)")) // Extra newline after separator
				}
				// Only indent selected event with ">", others have no indentation
				if i == p.selectedEventIndex {
					fmt.Printf("> %s\n", p.eventHistory[i].LogLine) // Selected
				} else {
					fmt.Printf("%s\n", p.eventHistory[i].LogLine) // No indentation
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
		if p.rawModeState != nil {
			term.MakeRaw(int(os.Stdin.Fd()))
		}
	}

	// Create event info
	eventInfo := EventInfo{
		ID:      p.latestEventID,
		Status:  p.latestEventStatus,
		Success: p.latestEventSuccess,
		Time:    p.latestEventTime,
		Data:    p.latestEventData,
		LogLine: eventLog,
	}

	// Check if this exact event (same ID AND timestamp) already exists
	// This prevents true duplicates but allows retries (same ID, different timestamp) as separate entries
	isDuplicate := false
	for i := len(p.eventHistory) - 1; i >= 0; i-- {
		if p.eventHistory[i].ID == p.latestEventID && p.eventHistory[i].Time.Equal(p.latestEventTime) {
			isDuplicate = true
			break
		}
	}

	if !isDuplicate {
		// Add to history (either new event or retry with different timestamp)
		p.eventHistory = append(p.eventHistory, eventInfo)
	}
	// If it's a duplicate (same ID and timestamp), just skip adding it

	// Limit history to last 50 events - trim old ones
	if len(p.eventHistory) > maxHistorySize {
		// Remove oldest event
		removedCount := len(p.eventHistory) - maxHistorySize
		p.eventHistory = p.eventHistory[removedCount:]

		// Adjust selected index if it was pointing to a removed event
		if p.selectedEventIndex < removedCount {
			p.selectedEventIndex = 0
			p.userNavigated = false // Reset navigation since selected event was removed
		} else {
			p.selectedEventIndex -= removedCount
		}
	}

	// Auto-select the latest event unless user has navigated away
	if !p.userNavigated {
		p.selectedEventIndex = len(p.eventHistory) - 1
	}
	// Note: If user has navigated, we DON'T change selectedEventIndex
	// The display logic will handle showing it even if it's outside the normal navigable range

	// Temporarily restore normal terminal mode for printing
	if p.rawModeState != nil {
		term.Restore(int(os.Stdin.Fd()), p.rawModeState)
	}

	// Calculate the navigable window (last 10 events)
	navigableStartIdx := len(p.eventHistory) - maxNavigableEvents
	if navigableStartIdx < 0 {
		navigableStartIdx = 0
	}
	numNavigableEvents := len(p.eventHistory) - navigableStartIdx

	// If we have multiple navigable events and auto-selecting, redraw navigable window
	// Also redraw if user has navigated (to show pinned selection)
	if numNavigableEvents > 1 && !p.userNavigated {
		// Auto-selecting mode: redraw to move selection to latest
		// Calculate total terminal lines occupied by previous navigable events
		totalEventLines := 0
		for i := navigableStartIdx; i < len(p.eventHistory)-1; i++ {
			totalEventLines += p.calculateEventLines(p.eventHistory[i].LogLine)
		}
		linesToMoveUp := totalEventLines + 2 // previous event lines + blank + status
		fmt.Printf("\033[%dA", linesToMoveUp)
		fmt.Print("\033[J")

		// Print navigable events with selection on the latest
		for i := navigableStartIdx; i < len(p.eventHistory); i++ {
			if i == p.selectedEventIndex {
				fmt.Printf("> %s\n", p.eventHistory[i].LogLine)
			} else {
				fmt.Printf("%s\n", p.eventHistory[i].LogLine) // No indentation
			}
		}
	} else if p.userNavigated && numNavigableEvents > 1 {
		// User has navigated: redraw to show pinned selected event
		// Get the navigable events (includes pinned selected event if applicable)
		navigableIndices := p.getNavigableEvents()

		// Calculate total terminal lines occupied by previous navigable events
		totalEventLines := 0
		for i := 0; i < len(navigableIndices)-1; i++ {
			totalEventLines += p.calculateEventLines(p.eventHistory[navigableIndices[i]].LogLine)
		}
		linesToMoveUp := totalEventLines + 2 // previous event lines + blank + status
		fmt.Printf("\033[%dA", linesToMoveUp)
		fmt.Print("\033[J")

		// Print navigable events (including pinned event) with selection indicator
		for _, idx := range navigableIndices {
			if idx == p.selectedEventIndex {
				fmt.Printf("> %s\n", p.eventHistory[idx].LogLine)
			} else {
				fmt.Printf("%s\n", p.eventHistory[idx].LogLine)
			}
		}
	} else {
		// First event - simple append
		if p.statusLineShown {
			if len(p.eventHistory) == 1 {
				// First event - only clear the "waiting" status line
				fmt.Print("\033[1A\033[2K\r")
			} else {
				// Clear status line and blank line
				fmt.Print("\033[2A\033[2K\r\033[1B\033[2K\r\033[1A")
			}
		}

		// Print the new event
		newEventIndex := len(p.eventHistory) - 1
		// Only indent if selected, otherwise no indentation
		if p.selectedEventIndex == newEventIndex {
			fmt.Printf("> %s\n", p.eventHistory[newEventIndex].LogLine)
		} else {
			fmt.Printf("%s\n", p.eventHistory[newEventIndex].LogLine) // No indentation
		}
	}

	// Blank line
	fmt.Println()

	// Generate status message
	var statusMsg string
	color := ansi.Color(os.Stdout)

	// If user has navigated, show selected event status; otherwise show latest event status
	if p.userNavigated && p.selectedEventIndex >= 0 && p.selectedEventIndex < len(p.eventHistory) {
		selectedEvent := p.eventHistory[p.selectedEventIndex]
		if selectedEvent.Success {
			statusMsg = fmt.Sprintf("> %s Selected event succeeded with status %d | [↑↓] Navigate • [r] Retry • [o] Open in dashboard • [d] Show data • [Ctrl+C] Quit",
				color.Green("✓"), selectedEvent.Status)
		} else {
			if selectedEvent.Status == 0 {
				statusMsg = fmt.Sprintf("> %s Selected event failed with error | [↑↓] Navigate • [r] Retry • [o] Open in dashboard • [d] Show data & • [Ctrl+C] Quit",
					color.Red("x").Bold())
			} else {
				statusMsg = fmt.Sprintf("> %s Selected event failed with status %d | [↑↓] Navigate • [r] Retry • [o] Open in dashboard • [d] Show data • [Ctrl+C] Quit",
					color.Red("x").Bold(), selectedEvent.Status)
			}
		}
	} else {
		// Auto-selecting latest event
		if p.latestEventSuccess {
			statusMsg = fmt.Sprintf("> %s Last event succeeded with status %d | [↑↓] Navigate • [r] Retry • [o] Open in dashboard • [d] Show data • [Ctrl+C] Quit",
				color.Green("✓"), p.latestEventStatus)
		} else {
			if p.latestEventStatus == 0 {
				statusMsg = fmt.Sprintf("> %s Last event failed with error | [↑↓] Navigate • [r] Retry • [o] Open in dashboard • [d] Show request details • [Ctrl+C] Quit",
					color.Red("x").Bold())
			} else {
				statusMsg = fmt.Sprintf("> %s Last event failed with status %d | [↑↓] Navigate • [r] Retry • [o] Open in dashboard • [d] Show request details • [Ctrl+C] Quit",
					color.Red("x").Bold(), p.latestEventStatus)
			}
		}
	}

	fmt.Printf("%s\n", statusMsg)
	p.statusLineShown = true

	// Re-enable raw mode
	if p.rawModeState != nil {
		term.MakeRaw(int(os.Stdin.Fd()))
	}
}

// startWaitingAnimation starts an animation for the waiting indicator
func (p *Proxy) startWaitingAnimation(ctx context.Context) {
	p.stopWaitingAnimation = make(chan bool, 1)

	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-p.stopWaitingAnimation:
				return
			case <-ticker.C:
				if !p.hasReceivedEvent && p.statusLineShown {
					p.updateStatusLine()
				}
			}
		}
	}()
}

// updateStatusLine updates the bottom status line with the latest event information
func (p *Proxy) updateStatusLine() {
	p.terminalMutex.Lock()
	defer p.terminalMutex.Unlock()

	// Only update if we haven't received any events yet (just the waiting animation)
	if p.hasReceivedEvent {
		return
	}

	// Temporarily restore normal terminal mode for printing
	if p.rawModeState != nil {
		term.Restore(int(os.Stdin.Fd()), p.rawModeState)
	}

	// Animated green dot (alternates between ● and ○)
	color := ansi.Color(os.Stdout)
	var dot string
	if p.waitingAnimationFrame%2 == 0 {
		dot = fmt.Sprintf("%s", color.Green("●"))
	} else {
		dot = fmt.Sprintf("%s", color.Green("○"))
	}
	p.waitingAnimationFrame++
	statusMsg := fmt.Sprintf("%s Connected. Waiting for events...", dot)

	if p.statusLineShown {
		// If we've shown a status before, move up one line and clear it
		fmt.Printf("\033[1A\033[2K\r%s\n", statusMsg)
	} else {
		// First time showing status
		fmt.Printf("%s\n", statusMsg)
		p.statusLineShown = true
	}

	// Re-enable raw mode
	if p.rawModeState != nil {
		term.MakeRaw(int(os.Stdin.Fd()))
	}
}

func (p *Proxy) startKeyboardListener(ctx context.Context) {
	// Check if we're in a terminal
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return
	}

	go func() {
		// Enter raw mode once and keep it
		oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
		if err != nil {
			return
		}

		// Store the raw mode state for use in safePrintf
		p.rawModeState = oldState

		// Ensure we restore terminal state when this goroutine exits
		defer func() {
			p.terminalMutex.Lock()
			defer p.terminalMutex.Unlock()
			term.Restore(int(os.Stdin.Fd()), oldState)
		}()

		// Create a buffered channel for reading stdin
		inputCh := make(chan []byte, 1)

		// Start a separate goroutine to read from stdin
		go func() {
			defer close(inputCh)
			buf := make([]byte, 3) // Buffer for escape sequences
			for {
				select {
				case <-ctx.Done():
					return
				default:
					n, err := os.Stdin.Read(buf)
					if err != nil {
						// Log the error but don't crash the application
						log.WithField("prefix", "proxy.startKeyboardListener").Debugf("Error reading stdin: %v", err)
						return
					}
					if n == 0 {
						continue
					}
					select {
					case inputCh <- buf[:n]:
					case <-ctx.Done():
						return
					}
				}
			}
		}()

		// Main loop to process keyboard input
		for {
			select {
			case <-ctx.Done():
				return
			case input, ok := <-inputCh:
				if !ok {
					return
				}

				// Process the input
				p.processKeyboardInput(input)
			}
		}
	}()
}

// processKeyboardInput handles keyboard input including arrow keys
func (p *Proxy) processKeyboardInput(input []byte) {
	if len(input) == 0 {
		return
	}

	// Handle single character keys
	if len(input) == 1 {
		switch input[0] {
		case 0x03: // Ctrl+C
			proc, _ := os.FindProcess(os.Getpid())
			proc.Signal(os.Interrupt)
			return
		}
	}

	// Disable all other shortcuts until first event is received or while not connected
	if !p.hasReceivedEvent || !p.isConnected {
		return
	}

	// Handle escape sequences (arrow keys)
	if len(input) == 3 && input[0] == 0x1B && input[1] == 0x5B {
		// Disable navigation while in details view
		if p.showingDetails {
			return
		}

		switch input[2] {
		case 0x41: // Up arrow
			p.navigateEvents(-1)
		case 0x42: // Down arrow
			p.navigateEvents(1)
		}
		return
	}

	// Handle single character keys (after quit/ctrl+c check)
	if len(input) == 1 {
		switch input[0] {
		case 0x72, 0x52: // 'r' or 'R'
			if !p.showingDetails {
				p.retrySelectedEvent()
			}
		case 0x6F, 0x4F: // 'o' or 'O'
			p.openSelectedEventURL()
		case 0x64, 0x44: // 'd' or 'D'
			// Toggle alternate screen details view
			if p.showingDetails {
				p.exitDetailsView()
			} else {
				p.enterDetailsView()
			}
		}
	}
}

// getNavigableEvents returns the indices of events that should be shown in the "Latest events" section
// This includes the last (maxNavigableEvents-1) chronological events, plus the selected event if it's outside this range
func (p *Proxy) getNavigableEvents() []int {
	historySize := len(p.eventHistory)

	// Calculate the normal navigable range (last 10 events)
	normalStartIdx := historySize - maxNavigableEvents
	if normalStartIdx < 0 {
		normalStartIdx = 0
	}

	// If user hasn't navigated or selected event is within normal range, return normal range
	if !p.userNavigated || p.selectedEventIndex >= normalStartIdx {
		indices := make([]int, 0, historySize-normalStartIdx)
		for i := normalStartIdx; i < historySize; i++ {
			indices = append(indices, i)
		}
		return indices
	}

	// Selected event is outside normal range - include it as the first navigable event
	// Show: selected event + last 9 chronological events
	indices := make([]int, 0, maxNavigableEvents)
	indices = append(indices, p.selectedEventIndex) // Add selected event first

	// Add the last 9 events (skip one to make room for the pinned event)
	startIdx := historySize - (maxNavigableEvents - 1)
	if startIdx < 0 {
		startIdx = 0
	}
	for i := startIdx; i < historySize; i++ {
		// Skip the selected event if it's also in the last 9 (edge case)
		if i != p.selectedEventIndex {
			indices = append(indices, i)
		}
	}

	return indices
}

// navigateEvents moves the selection up or down in the event history (within navigable events)
func (p *Proxy) navigateEvents(direction int) {
	if len(p.eventHistory) == 0 {
		return
	}

	// Get the navigable events (includes pinned selected event if applicable)
	navigableIndices := p.getNavigableEvents()
	if len(navigableIndices) == 0 {
		return
	}

	// Find current position in the navigable indices
	currentPos := -1
	for i, idx := range navigableIndices {
		if idx == p.selectedEventIndex {
			currentPos = i
			break
		}
	}

	if currentPos == -1 {
		// Selected event not in navigable list (shouldn't happen), default to first
		currentPos = 0
	}

	// Calculate new position
	newPos := currentPos + direction

	// Clamp to navigable range
	if newPos < 0 {
		newPos = 0
	} else if newPos >= len(navigableIndices) {
		newPos = len(navigableIndices) - 1
	}

	if newPos != currentPos {
		p.selectedEventIndex = navigableIndices[newPos]
		p.userNavigated = true // Mark that user has manually navigated

		// Reset userNavigated if user navigates back to the latest event
		if p.selectedEventIndex == len(p.eventHistory)-1 {
			p.userNavigated = false
		}

		p.redrawEventsWithSelection()
	}
}

// redrawEventsWithSelection updates the selection indicators without clearing the screen (only last 10 events)
func (p *Proxy) redrawEventsWithSelection() {
	if len(p.eventHistory) == 0 {
		return
	}

	p.terminalMutex.Lock()
	defer p.terminalMutex.Unlock()

	// Temporarily restore normal terminal mode for printing
	if p.rawModeState != nil {
		term.Restore(int(os.Stdin.Fd()), p.rawModeState)
	}

	// Get the navigable events (includes pinned selected event if applicable)
	navigableIndices := p.getNavigableEvents()

	// Calculate the normal navigable start for determining if we need separator
	normalNavigableStartIdx := len(p.eventHistory) - maxNavigableEvents
	if normalNavigableStartIdx < 0 {
		normalNavigableStartIdx = 0
	}

	// Calculate total terminal lines occupied by navigable events
	totalEventLines := 0
	for _, idx := range navigableIndices {
		totalEventLines += p.calculateEventLines(p.eventHistory[idx].LogLine)
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
		if idx == p.selectedEventIndex {
			fmt.Printf("> %s\n", p.eventHistory[idx].LogLine) // Selected event with >
		} else {
			fmt.Printf("%s\n", p.eventHistory[idx].LogLine) // No indentation
		}
	}

	// Add a newline before the status line
	fmt.Println()

	// Generate and print the status message for the selected event
	var statusMsg string
	color := ansi.Color(os.Stdout)
	selectedEvent := p.eventHistory[p.selectedEventIndex]
	if selectedEvent.Success {
		statusMsg = fmt.Sprintf("> %s Selected event succeeded with status %d | [↑↓] Navigate • [r] Retry • [o] Open in dashboard • [d] Show request details • [Ctrl+C] Quit",
			color.Green("✓"), selectedEvent.Status)
	} else {
		if selectedEvent.Status == 0 {
			statusMsg = fmt.Sprintf("> %s Selected event failed with error | [↑↓] Navigate • [r] Retry • [o] Open in dashboard • [d] Show request details • [Ctrl+C] Quit",
				color.Red("x").Bold())
		} else {
			statusMsg = fmt.Sprintf("> %s Selected event failed with status %d | [↑↓] Navigate • [r] Retry • [o] Open in dashboard • [d] Show request details • [Ctrl+C] Quit",
				color.Red("x").Bold(), selectedEvent.Status)
		}
	}

	fmt.Printf("%s\n", statusMsg)
	p.statusLineShown = true

	// Re-enable raw mode
	if p.rawModeState != nil {
		term.MakeRaw(int(os.Stdin.Fd()))
	}
}

// Run manages the connection to Hookdeck.
// The connection is established in phases:
//   - Create a new CLI session
//   - Create a new websocket connection
func (p *Proxy) Run(parentCtx context.Context) error {
	const maxConnectAttempts = 10
	const maxReconnectAttempts = 10 // Also limit reconnection attempts
	nAttempts := 0

	// Track whether or not we have connected successfully.
	hasConnectedOnce := false
	canConnect := func() bool {
		if hasConnectedOnce {
			// After first successful connection, allow limited reconnection attempts
			return nAttempts < maxReconnectAttempts
		} else {
			// Initial connection attempts
			return nAttempts < maxConnectAttempts
		}
	}

	signalCtx := withSIGTERMCancel(parentCtx, func() {
		log.WithFields(log.Fields{
			"prefix": "proxy.Proxy.Run",
		}).Debug("Ctrl+C received, cleaning up...")
	})

	// Start keyboard listener for Ctrl+R/Cmd+R shortcuts
	p.startKeyboardListener(signalCtx)

	// Start waiting animation
	p.startWaitingAnimation(signalCtx)

	s := ansi.StartNewSpinner("Getting ready...", p.cfg.Log.Out)

	session, err := p.createSession(signalCtx)
	if err != nil {
		// Stop spinner and restore terminal state before fatal error
		p.terminalMutex.Lock()
		ansi.StopSpinner(s, "", p.cfg.Log.Out)
		if p.rawModeState != nil {
			term.Restore(int(os.Stdin.Fd()), p.rawModeState)
		}
		fmt.Print("\033[2K\r")
		p.terminalMutex.Unlock()

		p.cfg.Log.Fatalf("Error while authenticating with Hookdeck: %v", err)
	}

	if session.Id == "" {
		// Stop spinner and restore terminal state before fatal error
		p.terminalMutex.Lock()
		ansi.StopSpinner(s, "", p.cfg.Log.Out)
		if p.rawModeState != nil {
			term.Restore(int(os.Stdin.Fd()), p.rawModeState)
		}
		fmt.Print("\033[2K\r")
		p.terminalMutex.Unlock()

		p.cfg.Log.Fatalf("Error while starting a new session")
	}

	// Main loop to keep attempting to connect to Hookdeck once
	// we have created a session.
	for canConnect() {
		// Apply backoff delay BEFORE creating new client (except for first attempt)
		if nAttempts > 0 {
			// Exponential backoff: 100ms * 2^(attempt-1), capped at 30 seconds
			// Attempt 1: 100ms, 2: 200ms, 3: 400ms, 4: 800ms, 5: 1.6s, 6: 3.2s, 7: 6.4s, 8: 12.8s, 9+: 30s
			backoffMS := math.Min(100*math.Pow(2, float64(nAttempts-1)), 30000)
			sleepDurationMS := int(backoffMS)

			log.WithField(
				"prefix", "proxy.Proxy.Run",
			).Debugf(
				"Connect backoff (%dms)", sleepDurationMS,
			)

			// Reset the timer to the next duration
			p.connectionTimer.Stop()
			p.connectionTimer.Reset(time.Duration(sleepDurationMS) * time.Millisecond)

			// Clear the status line before showing reconnection spinner
			p.terminalMutex.Lock()
			if p.statusLineShown {
				// Move up and clear the status line
				fmt.Print("\033[1A\033[2K\r")
				p.statusLineShown = false
			}
			p.terminalMutex.Unlock()

			// Block with a spinner while waiting
			ansi.StopSpinner(s, "", p.cfg.Log.Out)
			// Use different message based on whether we've connected before
			if hasConnectedOnce {
				s = ansi.StartNewSpinner("Connection lost, reconnecting...", p.cfg.Log.Out)
			} else {
				s = ansi.StartNewSpinner("Connecting...", p.cfg.Log.Out)
			}
			select {
			case <-p.connectionTimer.C:
				// Continue to retry
			case <-signalCtx.Done():
				p.connectionTimer.Stop()
				ansi.StopSpinner(s, "", p.cfg.Log.Out)
				return nil
			}
		}

		p.webSocketClient = websocket.NewClient(
			p.cfg.WSBaseURL,
			session.Id,
			p.cfg.Key,
			p.cfg.ProjectID,
			&websocket.Config{
				Log:          p.cfg.Log,
				NoWSS:        p.cfg.NoWSS,
				EventHandler: websocket.EventHandlerFunc(p.processAttempt),
			},
		)

		// Monitor the websocket for connection and update the spinner appropriately.
		go func() {
			<-p.webSocketClient.Connected()
			// Mark as connected and reset attempt counter
			p.isConnected = true
			nAttempts = 0

			// Stop the spinner and update status line
			p.terminalMutex.Lock()
			if p.rawModeState != nil {
				term.Restore(int(os.Stdin.Fd()), p.rawModeState)
			}
			ansi.StopSpinner(s, "", p.cfg.Log.Out)
			if p.rawModeState != nil {
				term.MakeRaw(int(os.Stdin.Fd()))
			}
			p.terminalMutex.Unlock()

			// Always update the status line to show current state
			if hasConnectedOnce || p.hasReceivedEvent {
				// If we've reconnected or have events, just update the status
				p.updateStatusLine()
			} else {
				// First connection, show initial status
				p.updateStatusLine()
			}
			hasConnectedOnce = true
		}()

		// Run the websocket in the background
		go p.webSocketClient.Run(signalCtx)
		nAttempts++

		// Block until ctrl+c or the websocket connection is interrupted
		select {
		case <-signalCtx.Done():
			ansi.StopSpinner(s, "", p.cfg.Log.Out)
			return nil
		case <-p.webSocketClient.NotifyExpired:
			// Mark as disconnected
			p.isConnected = false

			if !canConnect() {
				// Stop the spinner and restore terminal state before fatal error
				p.terminalMutex.Lock()
				ansi.StopSpinner(s, "", p.cfg.Log.Out)
				if p.rawModeState != nil {
					term.Restore(int(os.Stdin.Fd()), p.rawModeState)
				}
				// Clear the spinner line
				fmt.Print("\033[2K\r")
				p.terminalMutex.Unlock()

				// Print error without timestamp (use fmt instead of log to avoid formatter)
				color := ansi.Color(os.Stdout)
				fmt.Fprintf(os.Stderr, "%s Could not establish connection. Terminating after %d attempts to connect.\n",
					color.Red("FATAL"), nAttempts)
				os.Exit(1)
			}
			// Connection lost, loop will retry (backoff happens at start of next iteration)
		}
	}

	if p.webSocketClient != nil {
		p.webSocketClient.Stop()
	}

	log.WithFields(log.Fields{
		"prefix": "proxy.Proxy.Run",
	}).Debug("Bye!")

	return nil
}

func (p *Proxy) createSession(ctx context.Context) (hookdeck.Session, error) {
	var session hookdeck.Session

	parsedBaseURL, err := url.Parse(p.cfg.APIBaseURL)
	if err != nil {
		return session, err
	}

	client := &hookdeck.Client{
		BaseURL:   parsedBaseURL,
		APIKey:    p.cfg.Key,
		ProjectID: p.cfg.ProjectID,
	}

	var connectionIDs []string
	for _, connection := range p.connections {
		connectionIDs = append(connectionIDs, connection.Id)
	}

	for i := 0; i <= 5; i++ {
		session, err = client.CreateSession(hookdeck.CreateSessionInput{
			ConnectionIds: connectionIDs,
		})

		if err == nil {
			return session, nil
		}

		select {
		case <-ctx.Done():
			return session, errors.New("canceled by context")
		case <-time.After(1 * time.Second):
		}
	}

	return session, err
}

func (p *Proxy) processAttempt(msg websocket.IncomingMessage) {
	if msg.Attempt == nil {
		p.cfg.Log.Debug("WebSocket specified for Events received unexpected event")
		return
	}

	webhookEvent := msg.Attempt

	// Store the latest event ID and data for retry/open/details functionality
	p.latestEventID = webhookEvent.Body.EventID
	p.latestEventData = webhookEvent

	p.cfg.Log.WithFields(log.Fields{
		"prefix": "proxy.Proxy.processAttempt",
	}).Debugf("Processing webhook event")

	if p.cfg.PrintJSON {
		p.safePrintf("%s\n", webhookEvent.Body.Request.DataString)
	} else {
		url := p.cfg.URL.Scheme + "://" + p.cfg.URL.Host + p.cfg.URL.Path + webhookEvent.Body.Path
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: p.cfg.Insecure},
		}

		timeout := webhookEvent.Body.Request.Timeout
		if timeout == 0 {
			timeout = 1000 * 30
		}

		client := &http.Client{
			Timeout:   time.Duration(timeout) * time.Millisecond,
			Transport: tr,
		}

		req, err := http.NewRequest(webhookEvent.Body.Request.Method, url, nil)
		if err != nil {
			p.safePrintf("Error: %s\n", err)
			return
		}
		x := make(map[string]json.RawMessage)
		err = json.Unmarshal(webhookEvent.Body.Request.Headers, &x)
		if err != nil {
			p.safePrintf("Error: %s\n", err)
			return
		}

		for key, value := range x {
			unquoted_value, _ := strconv.Unquote(string(value))
			req.Header.Set(key, unquoted_value)
		}

		req.Body = ioutil.NopCloser(strings.NewReader(webhookEvent.Body.Request.DataString))
		req.ContentLength = int64(len(webhookEvent.Body.Request.DataString))

		res, err := client.Do(req)

		if err != nil {
			color := ansi.Color(os.Stdout)
			localTime := time.Now().Format(timeLayout)

			errStr := fmt.Sprintf("%s [%s] Failed to %s: %v",
				color.Faint(localTime),
				color.Red("ERROR").Bold(),
				webhookEvent.Body.Request.Method,
				err,
			)

			// Track the failed event first
			p.latestEventStatus = 0 // Use 0 for connection errors
			p.latestEventSuccess = false
			p.latestEventTime = time.Now()
			if !p.hasReceivedEvent {
				p.hasReceivedEvent = true
				// Stop the waiting animation
				if p.stopWaitingAnimation != nil {
					p.stopWaitingAnimation <- true
				}
			}

			// Print the error and update status line in one operation
			p.printEventAndUpdateStatus(errStr)

			p.webSocketClient.SendMessage(&websocket.OutgoingMessage{
				ErrorAttemptResponse: &websocket.ErrorAttemptResponse{
					Event: "attempt_response",
					Body: websocket.ErrorAttemptBody{
						AttemptId: webhookEvent.Body.AttemptId,
						Error:     true,
					},
				}})
		} else {
			p.processEndpointResponse(webhookEvent, res)
		}
	}
}

func (p *Proxy) processEndpointResponse(webhookEvent *websocket.Attempt, resp *http.Response) {
	localTime := time.Now().Format(timeLayout)
	color := ansi.Color(os.Stdout)
	var url = p.cfg.DashboardBaseURL + "/events/" + webhookEvent.Body.EventID
	if p.cfg.ProjectMode == "console" {
		url = p.cfg.ConsoleBaseURL + "/?event_id=" + webhookEvent.Body.EventID
	}
	outputStr := fmt.Sprintf("%s [%d] %s %s %s %s",
		color.Faint(localTime),
		ansi.ColorizeStatus(resp.StatusCode),
		resp.Request.Method,
		resp.Request.URL,
		color.Faint("→"),
		color.Faint(url),
	)
	// Track the event status first
	p.latestEventStatus = resp.StatusCode
	p.latestEventSuccess = resp.StatusCode >= 200 && resp.StatusCode < 300
	p.latestEventTime = time.Now()
	if !p.hasReceivedEvent {
		p.hasReceivedEvent = true
		// Stop the waiting animation
		if p.stopWaitingAnimation != nil {
			p.stopWaitingAnimation <- true
		}
	}

	// Print the event log and update status line in one operation
	p.printEventAndUpdateStatus(outputStr)

	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		errStr := fmt.Sprintf("%s [%s] Failed to read response from endpoint, error = %v\n",
			color.Faint(localTime),
			color.Red("ERROR").Bold(),
			err,
		)
		log.Errorf(errStr)

		return
	}

	if p.webSocketClient != nil {
		p.webSocketClient.SendMessage(&websocket.OutgoingMessage{
			AttemptResponse: &websocket.AttemptResponse{
				Event: "attempt_response",
				Body: websocket.AttemptResponseBody{
					AttemptId: webhookEvent.Body.AttemptId,
					CLIPath:   webhookEvent.Body.Path,
					Status:    resp.StatusCode,
					Data:      string(buf),
				},
			}})
	}
}

func (p *Proxy) retrySelectedEvent() {
	if len(p.eventHistory) == 0 || p.selectedEventIndex < 0 || p.selectedEventIndex >= len(p.eventHistory) {
		color := ansi.Color(os.Stdout)
		p.safePrintf("[%s] No event selected to retry\n",
			color.Yellow("WARN"),
		)
		return
	}

	eventID := p.eventHistory[p.selectedEventIndex].ID
	if eventID == "" {
		color := ansi.Color(os.Stdout)
		p.safePrintf("[%s] Selected event has no ID to retry\n",
			color.Yellow("WARN"),
		)
		return
	}

	// Create HTTP client for retry request
	parsedBaseURL, err := url.Parse(p.cfg.APIBaseURL)
	if err != nil {
		color := ansi.Color(os.Stdout)
		p.safePrintf("[%s] Failed to parse API URL for retry: %v\n",
			color.Red("ERROR").Bold(),
			err,
		)
		return
	}

	client := &hookdeck.Client{
		BaseURL:   parsedBaseURL,
		APIKey:    p.cfg.Key,
		ProjectID: p.cfg.ProjectID,
	}

	// Make retry request to Hookdeck API
	retryURL := fmt.Sprintf("/events/%s/retry", eventID)
	resp, err := client.Post(context.Background(), retryURL, []byte("{}"), nil)
	if err != nil {
		color := ansi.Color(os.Stdout)
		p.safePrintf("[%s] Failed to retry event %s: %v\n",
			color.Red("ERROR").Bold(),
			eventID,
			err,
		)
		return
	}
	defer resp.Body.Close()
}

func (p *Proxy) openSelectedEventURL() {
	if len(p.eventHistory) == 0 || p.selectedEventIndex < 0 || p.selectedEventIndex >= len(p.eventHistory) {
		color := ansi.Color(os.Stdout)
		p.safePrintf("[%s] No event selected to open\n",
			color.Yellow("WARN"),
		)
		return
	}

	eventID := p.eventHistory[p.selectedEventIndex].ID
	if eventID == "" {
		color := ansi.Color(os.Stdout)
		p.safePrintf("[%s] Selected event has no ID to open\n",
			color.Yellow("WARN"),
		)
		return
	}

	// Build event URL based on project mode
	var eventURL string
	if p.cfg.ProjectMode == "console" {
		eventURL = p.cfg.ConsoleBaseURL + "/?event_id=" + eventID
	} else {
		eventURL = p.cfg.DashboardBaseURL + "/events/" + eventID
	}

	// Open URL in browser
	err := p.openBrowser(eventURL)
	if err != nil {
		color := ansi.Color(os.Stdout)
		p.safePrintf("[%s] Failed to open browser: %v\n",
			color.Red("ERROR").Bold(),
			err,
		)
		return
	}
}

func (p *Proxy) openBrowser(url string) error {
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

// enterDetailsView shows event details using less pager for scrolling
func (p *Proxy) enterDetailsView() {
	if p.selectedEventIndex < 0 || p.selectedEventIndex >= len(p.eventHistory) {
		return
	}

	selectedEvent := p.eventHistory[p.selectedEventIndex]
	if selectedEvent.Data == nil {
		return
	}

	p.terminalMutex.Lock()

	// Temporarily restore normal terminal mode
	if p.rawModeState != nil {
		term.Restore(int(os.Stdin.Fd()), p.rawModeState)
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
	dashboardURL := p.cfg.DashboardBaseURL
	if p.cfg.ProjectID != "" {
		dashboardURL += "/cli/events/" + selectedEvent.ID
	}
	if p.cfg.ProjectMode == "console" {
		dashboardURL = p.cfg.ConsoleBaseURL
	}
	content.WriteString(fmt.Sprintf("%s %s\n", ansi.Faint("🔗"), ansi.Faint(dashboardURL)))
	content.WriteString("\n")
	content.WriteString(ansi.Faint("───────────────────────────────────────────────────────────────────────────────"))
	content.WriteString("\n\n")

	// Request section
	content.WriteString(ansi.Bold("Request"))
	content.WriteString("\n\n")
	// Construct the full URL with query params the same way as in processAttempt
	fullURL := p.cfg.URL.Scheme + "://" + p.cfg.URL.Host + p.cfg.URL.Path + webhookEvent.Body.Path
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

	// Set the flag before launching pager
	p.showingDetails = true

	p.terminalMutex.Unlock()

	// Use less with standard options
	// Note: Custom key bindings are unreliable, so we stick with 'q' to quit
	// We use echo to pipe content to less, which allows less to read keyboard from terminal

	cmd := exec.Command("sh", "-c", "less -R")

	// Create stdin pipe to send content
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		// Fallback: print directly
		p.terminalMutex.Lock()
		fmt.Print(content.String())
		p.showingDetails = false
		if p.rawModeState != nil {
			term.MakeRaw(int(os.Stdin.Fd()))
		}
		p.terminalMutex.Unlock()
		return
	}

	// Connect to terminal
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Start less
	if err := cmd.Start(); err != nil {
		// Fallback: print directly
		p.terminalMutex.Lock()
		fmt.Print(content.String())
		p.showingDetails = false
		if p.rawModeState != nil {
			term.MakeRaw(int(os.Stdin.Fd()))
		}
		p.terminalMutex.Unlock()
		return
	}

	// Write content to less
	stdinPipe.Write([]byte(content.String()))
	stdinPipe.Close()

	// Wait for less to exit
	cmd.Wait()

	// After pager exits, restore state
	p.terminalMutex.Lock()
	p.showingDetails = false

	// Re-enable raw mode
	if p.rawModeState != nil {
		term.MakeRaw(int(os.Stdin.Fd()))
	}
	p.terminalMutex.Unlock()
}

// exitDetailsView is called when user presses 'd' or 'q' while in details view
func (p *Proxy) exitDetailsView() {
	p.showingDetails = false
}

//
// Public functions
//

// New creates a new Proxy
func New(cfg *Config, connections []*hookdecksdk.Connection) *Proxy {
	if cfg.Log == nil {
		cfg.Log = &log.Logger{Out: ioutil.Discard}
	}

	p := &Proxy{
		cfg:                cfg,
		connections:        connections,
		connectionTimer:    time.NewTimer(0), // Defaults to no delay
		selectedEventIndex: -1,               // Initialize to invalid index
	}

	return p
}
