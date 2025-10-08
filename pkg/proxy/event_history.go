package proxy

import (
	"sync"
	"time"

	"github.com/hookdeck/hookdeck-cli/pkg/websocket"
)

// EventInfo represents a single event for navigation
type EventInfo struct {
	ID      string
	Status  int
	Success bool
	Time    time.Time
	Data    *websocket.Attempt
	LogLine string
	// Response data
	ResponseStatus   int
	ResponseHeaders  map[string][]string
	ResponseBody     string
	ResponseDuration time.Duration
}

// EventHistory manages the history of events and navigation state
type EventHistory struct {
	mu                   sync.RWMutex
	events               []EventInfo
	selectedIndex        int
	userNavigated        bool // Track if user has manually navigated away from latest event
	eventsTitleDisplayed bool // Track if "Events" title has been displayed
}

// NewEventHistory creates a new EventHistory instance
func NewEventHistory() *EventHistory {
	return &EventHistory{
		events:        make([]EventInfo, 0),
		selectedIndex: -1, // Initialize to invalid index
	}
}

// AddEvent adds a new event to the history
// Returns true if the event was added, false if it was a duplicate
func (eh *EventHistory) AddEvent(eventInfo EventInfo) bool {
	eh.mu.Lock()
	defer eh.mu.Unlock()

	// Check if this exact event (same ID AND timestamp) already exists
	// This prevents true duplicates but allows retries (same ID, different timestamp) as separate entries
	for i := len(eh.events) - 1; i >= 0; i-- {
		if eh.events[i].ID == eventInfo.ID && eh.events[i].Time.Equal(eventInfo.Time) {
			return false // Duplicate
		}
	}

	// Add to history (either new event or retry with different timestamp)
	eh.events = append(eh.events, eventInfo)

	// Limit history to last 50 events - trim old ones
	if len(eh.events) > maxHistorySize {
		// Remove oldest event
		removedCount := len(eh.events) - maxHistorySize
		eh.events = eh.events[removedCount:]

		// Adjust selected index if it was pointing to a removed event
		if eh.selectedIndex < removedCount {
			eh.selectedIndex = 0
			eh.userNavigated = false // Reset navigation since selected event was removed
		} else {
			eh.selectedIndex -= removedCount
		}
	}

	// Auto-select the latest event unless user has navigated away
	if !eh.userNavigated {
		eh.selectedIndex = len(eh.events) - 1
	}

	return true
}

// GetEvents returns a copy of all events in the history
func (eh *EventHistory) GetEvents() []EventInfo {
	eh.mu.RLock()
	defer eh.mu.RUnlock()

	// Return a copy to prevent external modifications
	eventsCopy := make([]EventInfo, len(eh.events))
	copy(eventsCopy, eh.events)
	return eventsCopy
}

// GetSelectedIndex returns the currently selected event index
func (eh *EventHistory) GetSelectedIndex() int {
	eh.mu.RLock()
	defer eh.mu.RUnlock()
	return eh.selectedIndex
}

// GetSelectedEvent returns a copy of the currently selected event, or nil if no event is selected
// Returns a copy to avoid issues with slice reallocation and concurrent modifications
func (eh *EventHistory) GetSelectedEvent() *EventInfo {
	eh.mu.RLock()
	defer eh.mu.RUnlock()

	if eh.selectedIndex < 0 || eh.selectedIndex >= len(eh.events) {
		return nil
	}
	// Return a copy of the event to avoid pointer issues when slice is modified
	eventCopy := eh.events[eh.selectedIndex]
	return &eventCopy
}

// IsUserNavigated returns true if the user has manually navigated away from the latest event
func (eh *EventHistory) IsUserNavigated() bool {
	eh.mu.RLock()
	defer eh.mu.RUnlock()
	return eh.userNavigated
}

// IsEventsTitleDisplayed returns true if the "Events" title has been displayed
func (eh *EventHistory) IsEventsTitleDisplayed() bool {
	eh.mu.RLock()
	defer eh.mu.RUnlock()
	return eh.eventsTitleDisplayed
}

// SetEventsTitleDisplayed sets whether the "Events" title has been displayed
func (eh *EventHistory) SetEventsTitleDisplayed(displayed bool) {
	eh.mu.Lock()
	defer eh.mu.Unlock()
	eh.eventsTitleDisplayed = displayed
}

// Count returns the number of events in the history
func (eh *EventHistory) Count() int {
	eh.mu.RLock()
	defer eh.mu.RUnlock()
	return len(eh.events)
}

// GetNavigableEvents returns the indices of events that should be shown in the "Latest events" section
// This includes the last (maxNavigableEvents-1) chronological events, plus the selected event if it's outside this range
func (eh *EventHistory) GetNavigableEvents() []int {
	eh.mu.RLock()
	defer eh.mu.RUnlock()

	historySize := len(eh.events)

	// Calculate the normal navigable range (last 10 events)
	normalStartIdx := historySize - maxNavigableEvents
	if normalStartIdx < 0 {
		normalStartIdx = 0
	}

	// If user hasn't navigated or selected event is within normal range, return normal range
	if !eh.userNavigated || eh.selectedIndex >= normalStartIdx {
		indices := make([]int, 0, historySize-normalStartIdx)
		for i := normalStartIdx; i < historySize; i++ {
			indices = append(indices, i)
		}
		return indices
	}

	// Selected event is outside normal range - include it as the first navigable event
	// Show: selected event + last 9 chronological events
	indices := make([]int, 0, maxNavigableEvents)
	indices = append(indices, eh.selectedIndex) // Add selected event first

	// Add the last 9 events (skip one to make room for the pinned event)
	startIdx := historySize - (maxNavigableEvents - 1)
	if startIdx < 0 {
		startIdx = 0
	}
	for i := startIdx; i < historySize; i++ {
		// Skip the selected event if it's also in the last 9 (edge case)
		if i != eh.selectedIndex {
			indices = append(indices, i)
		}
	}

	return indices
}

// Navigate moves the selection up or down in the event history (within navigable events)
// direction: -1 for up, +1 for down
// Returns true if the selection changed, false otherwise
func (eh *EventHistory) Navigate(direction int) bool {
	eh.mu.Lock()
	defer eh.mu.Unlock()

	if len(eh.events) == 0 {
		return false
	}

	// Calculate navigable indices (inline to avoid double-locking)
	historySize := len(eh.events)
	normalStartIdx := historySize - maxNavigableEvents
	if normalStartIdx < 0 {
		normalStartIdx = 0
	}

	var navigableIndices []int
	if !eh.userNavigated || eh.selectedIndex >= normalStartIdx {
		navigableIndices = make([]int, 0, historySize-normalStartIdx)
		for i := normalStartIdx; i < historySize; i++ {
			navigableIndices = append(navigableIndices, i)
		}
	} else {
		navigableIndices = make([]int, 0, maxNavigableEvents)
		navigableIndices = append(navigableIndices, eh.selectedIndex)
		startIdx := historySize - (maxNavigableEvents - 1)
		if startIdx < 0 {
			startIdx = 0
		}
		for i := startIdx; i < historySize; i++ {
			if i != eh.selectedIndex {
				navigableIndices = append(navigableIndices, i)
			}
		}
	}

	if len(navigableIndices) == 0 {
		return false
	}

	// Find current position in the navigable indices
	currentPos := -1
	for i, idx := range navigableIndices {
		if idx == eh.selectedIndex {
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
		eh.selectedIndex = navigableIndices[newPos]
		eh.userNavigated = true // Mark that user has manually navigated

		// Reset userNavigated if user navigates back to the latest event
		if eh.selectedIndex == len(eh.events)-1 {
			eh.userNavigated = false
		}

		return true
	}

	return false
}
