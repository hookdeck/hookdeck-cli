# Manual Test Plan - Hookdeck CLI Listen Command

This document outlines all the scenarios that should be manually tested for the `listen` command with the new UI improvements.

---

## 1. Connection & Setup Scenarios

### 1.1 Normal Connection
- **Test:** Start CLI with valid credentials and existing source
- **Expected:** 
  - Shows "Listening on" section with source details
  - Shows animated green dot "● Connected. Waiting for events..."
  - Websocket connects successfully

### 1.2 First-Time Guest User (No API Key)
- **Test:** Run CLI without authentication
- **Expected:**
  - Guest login flow triggers
  - Shows: "💡 Sign up to make your webhook URL permanent: [URL]"
  - Creates temporary source

### 1.3 WebSocket Connection Failure
- **Test:** Simulate websocket connection error (network issue, server down)
- **Expected:**
  - Error message displayed
  - Retry logic kicks in (up to 3 attempts)
  - Graceful failure message if all attempts fail

### 1.4 Invalid API Key
- **Test:** Use invalid/expired API key
- **Expected:**
  - Authentication error message
  - CLI exits with appropriate error

---

## 2. Source Configuration Scenarios

### 2.1 Single Source, Single Connection
- **Test:** Standard case with one source and one connection
- **Expected:**
```
test
├─ Request sent to → [webhook URL]
└─ Forwards to     → [local URL] (connection_name)
```

### 2.2 Single Source, Multiple Connections
- **Test:** One source with 2-3 connections
- **Expected:**
```
shopify
├─ Request sent to → [webhook URL]
├─ Forwards to     → [local URL 1] (connection_1)
└─ Forwards to     → [local URL 2] (connection_2)
```

### 2.3 Multiple Sources, Multiple Connections
- **Test:** 2-3 sources, each with 1-2 connections
- **Expected:**
  - Each source shown with tree structure
  - Blank line between sources
  - All properly aligned

### 2.4 Source Without Connections
- **Test:** Source exists but has no CLI connections configured
- **Expected:**
  - Should show error or warning
  - CLI should handle gracefully

### 2.5 Non-Existent Source
- **Test:** Specify source name that doesn't exist
- **Expected:**
  - Source gets created automatically
  - Shows new source in "Listening on" section

---

## 3. Event Handling Scenarios

### 3.1 First Event Received
- **Test:** Send first webhook to source
- **Expected:**
  - Animated green dot stops
  - Event displays with proper formatting
  - Status line shows: "> ✓ Last event succeeded..." or "> x Last event failed..."
  - Event auto-selected (has `>` indicator)
  - Proper blank line before first event

### 3.2 Successful Event (2xx Status)
- **Test:** Send webhook that returns 200-299 status
- **Expected:**
  - Green ✓ icon in status
  - Event shows `[200]` or appropriate status code
  - Status: "> ✓ Last event succeeded with status 200"

### 3.3 Failed Event (4xx/5xx Status)
- **Test:** Send webhook that returns 400/500 status
- **Expected:**
  - Red x (bold) in status
  - Event shows `[400]` or `[500]`
  - Status: "> x Last event failed with status 500"
  - Same red color for "x" and status code

### 3.4 Multiple Events (Auto-selection)
- **Test:** Send 3-4 events without navigating
- **Expected:**
  - Each new event auto-selected (gets `>`)
  - Previous events show `  ` (no selection)
  - Only ONE blank line between last event and status
  - No duplicate events
  - No duplicate status lines

### 3.5 Connection Error (Local Server Down)
- **Test:** Stop local server, send webhook
- **Expected:**
  - ERROR displayed with connection error message
  - Status shows error with status 0 or similar
  - Error tracked in event history

---

## 4. Keyboard Navigation Scenarios

### 4.1 Arrow Up Navigation
- **Test:** Press ↑ after receiving 3+ events
- **Expected:**
  - `>` moves to previous event
  - Status updates to reflect selected event
  - `userNavigated` flag set
  - No screen clearing (initial content preserved)
  - No duplicate rows

### 4.2 Arrow Down Navigation
- **Test:** Navigate up, then press ↓
- **Expected:**
  - `>` moves to next event
  - Status updates correctly
  - Can navigate back to latest event

### 4.3 Navigate to First Event
- **Test:** Navigate all the way to first event (↑ multiple times)
- **Expected:**
  - Stops at first event (index 0)
  - `>` on first event
  - Status reflects first event details

### 4.4 Navigate to Last Event
- **Test:** Navigate down to last event
- **Expected:**
  - Stops at last event
  - `>` on last event
  - Auto-selection resumes (userNavigated = false)

### 4.5 New Event While Navigated
- **Test:** Navigate to old event, then new webhook arrives
- **Expected:**
  - New event appears with `  ` (not selected)
  - User stays on previously selected event
  - Extra spacing handled correctly
  - Selection doesn't jump to new event

### 4.6 Navigate Back to Latest
- **Test:** Navigate away, then navigate back to latest event
- **Expected:**
  - Auto-selection resumes for future events
  - `userNavigated` flag reset

---

## 5. Keyboard Actions

### 5.1 Retry Event (r/R)
- **Test:** Select failed event, press 'r'
- **Expected:**
  - API call to retry event
  - Success/error message displayed
  - Original status line restored

### 5.2 Open in Dashboard (o/O)
- **Test:** Select any event, press 'o'
- **Expected:**
  - Browser opens to event details page
  - Correct URL with event ID

### 5.3 Show Event Details (d/D)
- **Test:** Select any event, press 'd'
- **Expected:**
  - Request details displayed (headers, body)
  - Formatted nicely
  - Status line restored after viewing

### 5.4 Quit (q/Q)
- **Test:** Press 'q'
- **Expected:**
  - Terminal restored to normal mode
  - Clean exit
  - No leftover artifacts

### 5.5 Ctrl+C
- **Test:** Press Ctrl+C
- **Expected:**
  - Same as quit
  - Clean shutdown
  - WebSocket connection closed properly

---

## 6. Terminal Display Scenarios

### 6.1 Waiting State Animation
- **Test:** Start CLI, don't send events for 5+ seconds
- **Expected:**
  - Green dot alternates between ● and ○ every 500ms
  - "Connected. Waiting for events..." message
  - Animation smooth and visible

### 6.2 Status Line Updates
- **Test:** Send multiple events, observe status line
- **Expected:**
  - Status line always shows selected event info
  - Proper clearing (no duplicate status lines)
  - Keyboard shortcuts shown: [↑↓] Navigate • [r] Retry • [o] Open • [d] Details • [q] Quit

### 6.3 Screen Clearing Behavior
- **Test:** Navigate between events
- **Expected:**
  - Initial content (Listening on, hint, Events) preserved
  - Only event area and status redrawn
  - No flickering
  - Clean transitions

### 6.4 Long URLs
- **Test:** Use very long webhook URLs or local URLs
- **Expected:**
  - URLs don't break formatting
  - Tree structure maintained
  - Still readable

### 6.5 Many Events (10+)
- **Test:** Send 10-20 events rapidly
- **Expected:**
  - All events displayed
  - Scrolling works naturally
  - Navigation works through all events
  - Performance acceptable

---

## 7. Edge Cases

### 7.1 Empty Connection Name
- **Test:** Connection with empty or null name
- **Expected:**
  - Handles gracefully (shows ID or default name)

### 7.2 Special Characters in Names
- **Test:** Source/connection names with emoji, unicode, special chars
- **Expected:**
  - Displays correctly
  - No formatting issues
  - Tree structure preserved

### 7.3 Very Long Source Names
- **Test:** Source name with 50+ characters
- **Expected:**
  - Displays without breaking layout
  - Tree structure maintained

### 7.4 Rapid Event Bursts
- **Test:** Send 5 events within 1 second
- **Expected:**
  - All events captured
  - Display updates correctly
  - No race conditions
  - No missing events

### 7.5 Terminal Resize
- **Test:** Resize terminal window while running
- **Expected:**
  - Layout adjusts reasonably
  - No crashes
  - Content still readable

### 7.6 No Events for Extended Period
- **Test:** Let CLI run for 5+ minutes without events
- **Expected:**
  - Animation continues smoothly
  - No memory leaks
  - Still responsive to input

---

## 8. Multi-Source Scenarios

### 8.1 Listen to All Sources (*)
- **Test:** `hookdeck listen 3000 *`
- **Expected:**
  - All sources with CLI connections shown
  - Multi-source message displayed
  - Each source in tree format

### 8.2 Switch Between Sources
- **Test:** Receive events from different sources
- **Expected:**
  - Events grouped by source or shown chronologically
  - Can identify which source each event came from

---

## 9. Path Configuration

### 9.1 Custom Path Flag
- **Test:** `hookdeck listen 3000 source --path /webhooks`
- **Expected:**
  - Path shown in "Forwards to" URL
  - Requests forwarded to correct path

### 9.2 Path Update
- **Test:** Change path for existing destination
- **Expected:**
  - Path update message shown
  - New path reflected in display

---

## 10. Console Mode

### 10.1 Console Mode User
- **Test:** User in console mode (not dashboard mode)
- **Expected:**
  - Console URL shown instead of dashboard URL
  - 💡 hint shows console link

---

## Testing Checklist

Use this checklist to track testing progress:

- [ ] 1.1 Normal Connection
- [ ] 1.2 Guest User
- [ ] 1.3 WebSocket Failure
- [ ] 1.4 Invalid API Key
- [ ] 2.1 Single Source/Connection
- [ ] 2.2 Multiple Connections
- [ ] 2.3 Multiple Sources
- [ ] 2.4 Source Without Connections
- [ ] 2.5 Non-Existent Source
- [ ] 3.1 First Event
- [ ] 3.2 Successful Event
- [ ] 3.3 Failed Event
- [ ] 3.4 Multiple Events
- [ ] 3.5 Connection Error
- [ ] 4.1-4.6 All Navigation Tests
- [ ] 5.1-5.5 All Keyboard Actions
- [ ] 6.1-6.5 All Display Tests
- [ ] 7.1-7.6 All Edge Cases
- [ ] 8.1-8.2 Multi-Source Tests
- [ ] 9.1-9.2 Path Tests
- [ ] 10.1 Console Mode

---

## Known Issues to Watch For

Based on previous fixes, pay special attention to:
1. Duplicate events appearing
2. Duplicate status lines
3. Missing newlines or extra newlines
4. Selection indicator (`>`) showing on multiple events
5. Screen clearing issues during navigation
6. First row duplication after navigation
7. Color consistency (red for errors)
8. Animation continuing after first event

---

## Testing Environment

- **Terminal:** iTerm2, Terminal.app, VSCode Terminal, etc.
- **OS:** macOS, Linux, Windows
- **Network:** Test with/without stable connection
- **Local Server:** Test with running/stopped local server

