package tui

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newConnectionTestModel builds a Model wired to one source and one connection,
// sized like a real terminal and past the first WindowSizeMsg.
func newConnectionTestModel(t *testing.T) Model {
	t.Helper()

	targetURL, err := url.Parse("http://localhost:3030")
	require.NoError(t, err)

	fullName := "src -> dest"
	source := &hookdeck.Source{ID: "src_1", Name: "my-source", URL: "https://hkdk.events/src_1"}
	connection := &hookdeck.Connection{
		ID:          "web_1",
		FullName:    &fullName,
		Source:      source,
		Destination: &hookdeck.Destination{ID: "des_1", Type: "CLI", Config: map[string]interface{}{"path": "/"}},
	}

	m := NewModel(&Config{
		TargetURL:        targetURL,
		Sources:          []*hookdeck.Source{source},
		Connections:      []*hookdeck.Connection{connection},
		DashboardBaseURL: "https://dashboard.hookdeck.com",
		ProjectID:        "tm_1",
	})
	m.width = 120
	m.height = 30
	m.ready = true
	m.viewportReady = true

	return m
}

// lastLine returns the status bar: View() writes it last with no trailing newline.
func lastLine(view string) string {
	lines := strings.Split(view, "\n")
	return lines[len(lines)-1]
}

// TestStatusBarAlwaysReportsConnectionState is the regression test for #399.
//
// The TUI used to render the complete layout — brand header, "Listening on …",
// "Requests to →", "Forwards to →" — with no status bar at all until the
// websocket connected. A run that never connected therefore looked exactly like
// a working one for the whole 40-second attempt budget, the only difference
// being a line that was absent. #376 fixed the same bug in the non-TTY
// renderers; this is the interactive half of it.
func TestStatusBarAlwaysReportsConnectionState(t *testing.T) {
	t.Run("the first frame says Connecting, before anything is connected", func(t *testing.T) {
		m := newConnectionTestModel(t)

		view := m.View()

		require.NotEmpty(t, view)
		assert.Contains(t, view, "Listening on 1 source • 1 connection",
			"the header still renders; that was never the problem")
		assert.Contains(t, lastLine(view), connectingLabel,
			"the status bar must state the pending state on the very first frame")
		assert.NotContains(t, lastLine(view), connectedLabel)
	})

	t.Run("a connect replaces it with Connected", func(t *testing.T) {
		m := newConnectionTestModel(t)

		updated, _ := m.Update(ConnectedMsg{})
		view := updated.(Model).View()

		assert.Contains(t, lastLine(view), connectedLabel)
		assert.NotContains(t, lastLine(view), connectingLabel)
	})

	t.Run("failed attempts before the first connect are counted, not hidden", func(t *testing.T) {
		m := newConnectionTestModel(t)

		updated, _ := m.Update(DisconnectedMsg{})
		updated, _ = updated.(Model).Update(DisconnectedMsg{})
		view := updated.(Model).View()

		assert.Contains(t, lastLine(view), "Connecting… (attempt 3)",
			"retrying is progress the user can see, not silence")
		assert.NotContains(t, lastLine(view), reconnectingLabel,
			"the CLI never connected, so it cannot claim to be reconnecting")
	})

	t.Run("a drop after connecting reports reconnecting", func(t *testing.T) {
		m := newConnectionTestModel(t)

		updated, _ := m.Update(ConnectedMsg{})
		updated, _ = updated.(Model).Update(DisconnectedMsg{})
		view := updated.(Model).View()

		assert.Contains(t, lastLine(view), reconnectingLabel)
	})

	t.Run("giving up shows a visible failure state with the reason", func(t *testing.T) {
		m := newConnectionTestModel(t)

		updated, _ := m.Update(ConnectionFailedMsg{Err: errors.New("dial tcp 127.0.0.1:9: connect: connection refused")})
		view := updated.(Model).View()

		assert.Contains(t, lastLine(view), failedLabel)
		assert.Contains(t, lastLine(view), "connection refused",
			"the failure state must carry the reason, not just the fact")
	})

	t.Run("the connection state survives events arriving", func(t *testing.T) {
		m := newConnectionTestModel(t)
		updated, _ := m.Update(ConnectedMsg{})
		m = updated.(Model)
		m.AddEvent(EventInfo{ID: "evt_1", Success: true, Status: 200, LogLine: "200 POST /"})

		view := m.View()

		assert.Contains(t, lastLine(view), connectedLabel,
			"readiness must stay affirmative once the event list takes over the bar")
	})
}

// TestConnectingStateIsNotInferredFromAbsentText pins the shape of the bug
// rather than the wording: whatever state the model is in, the last line of the
// view must carry words about the connection. An empty status bar is the failure
// mode #399 reported.
func TestConnectingStateIsNotInferredFromAbsentText(t *testing.T) {
	states := []struct {
		name  string
		apply func(Model) Model
	}{
		{"initial", func(m Model) Model { return m }},
		{"connected", func(m Model) Model { u, _ := m.Update(ConnectedMsg{}); return u.(Model) }},
		{"reconnecting", func(m Model) Model {
			u, _ := m.Update(ConnectedMsg{})
			u, _ = u.(Model).Update(DisconnectedMsg{})
			return u.(Model)
		}},
		{"failed", func(m Model) Model {
			u, _ := m.Update(ConnectionFailedMsg{Err: errors.New("boom")})
			return u.(Model)
		}},
	}

	for _, state := range states {
		t.Run(state.name, func(t *testing.T) {
			view := state.apply(newConnectionTestModel(t)).View()
			assert.NotEmpty(t, strings.TrimSpace(lastLine(view)),
				"the status bar must never be blank: absence is not a signal")
		})
	}
}

// TestHeaderSummaryMatchesCompactRenderer guards the other half of #402 — the
// interactive header and the compact banner are built from the same helper, so
// they cannot drift apart again.
func TestHeaderSummaryMatchesCompactRenderer(t *testing.T) {
	m := newConnectionTestModel(t)

	assert.Contains(t, m.renderConnectionInfo(), "Listening on 1 source • 1 connection • [i] Collapse")

	m.headerCollapsed = true
	assert.Contains(t, m.renderConnectionInfo(), "Listening on 1 source • 1 connection • [i] Expand")
}

// TestStatusBarStaysOneLine checks that a long failure reason is trimmed rather
// than wrapped. A wrapped status bar makes the frame taller than the terminal,
// which scrolls the header out of view exactly when the user needs to read it.
func TestStatusBarStaysOneLine(t *testing.T) {
	longReason := "dial tcp 127.0.0.1:9: connect: connection refused " + strings.Repeat("and more detail ", 20)

	for _, width := range []int{40, 80, 120} {
		t.Run(fmt.Sprintf("width %d", width), func(t *testing.T) {
			m := newConnectionTestModel(t)
			m.width = width
			updated, _ := m.Update(ConnectionFailedMsg{Err: errors.New(longReason)})
			m = updated.(Model)

			bar := m.renderStatusBar()

			assert.NotContains(t, bar, "\n", "the status bar is one line")
			assert.LessOrEqual(t, len([]rune(bar)), width,
				"the bar must fit the terminal so it does not wrap")
			assert.Contains(t, bar, "Connection failed",
				"trimming the reason must not trim away the state itself")
		})
	}
}
