package tui

import (
	"regexp"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/hookdeck/hookdeck-cli/pkg/websocket"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// forceColorProfile makes lipgloss emit escape sequences under `go test`, which
// has no terminal and would otherwise render everything plain — hiding the very
// bytes #404 is about.
func forceColorProfile(t *testing.T) {
	t.Helper()
	previous := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(previous) })
}

// sgrPattern matches an SGR (colour/bold/faint) escape sequence.
var sgrPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// renderEveryTUISurface draws the frames a listen session actually produces:
// the connecting frame, the connected frame, an event list, and the details
// view. Counting SGR bytes across all of them is the measurement #404 reported.
func renderEveryTUISurface(t *testing.T) string {
	t.Helper()

	m := newConnectionTestModel(t)
	out := m.View() // connecting frame

	updated, _ := m.Update(ConnectedMsg{})
	m = updated.(Model)
	out += m.View() // connected, waiting for events

	m.AddEvent(EventInfo{
		ID:               "evt_1",
		Success:          true,
		Status:           200,
		ResponseStatus:   200,
		ResponseHeaders:  map[string][]string{"content-type": {"application/json"}},
		ResponseBody:     `{"ok":true}`,
		ResponseDuration: time.Millisecond,
		Time:             time.Date(2026, time.July, 21, 18, 18, 38, 0, time.UTC),
		LogLine:          "2026-07-21 18:18:38 [" + ColorizeStatus(200) + "] POST http://localhost:3030/",
		Data: &websocket.Attempt{Body: websocket.AttemptBody{
			Path:    "/webhooks",
			Request: websocket.AttemptRequest{Method: "POST", Headers: []byte(`{"x-test":"v"}`), DataString: `{"a":1}`},
		}},
	})
	m.headerCollapsed = false
	out += m.View() // header, event list and status bar

	m.serverHealthChecked = true
	m.serverHealthy = false
	out += m.renderConnectionInfo() // unhealthy-server warning

	m.setDetailsContent(m.GetSelectedEvent())
	m.showingDetails = true
	out += m.renderDetailsView()

	failed, _ := m.Update(ConnectionFailedMsg{Err: assertErr("refused")})
	out += failed.(Model).View()

	return out
}

type assertErr string

func (e assertErr) Error() string { return string(e) }

// TestSetColorEnabledFalseRemovesEveryEscapeSequence is the regression test for
// #404. --color off is applied in config.InitConfig, which only ever reached
// pkg/ansi; the TUI draws with lipgloss, so a controlling-pty run with the flag
// set still emitted 48 SGR sequences. Colour is decoration the user switched
// off, not something one output mode gets to opt out of.
func TestSetColorEnabledFalseRemovesEveryEscapeSequence(t *testing.T) {
	forceColorProfile(t)
	t.Cleanup(func() { SetColorEnabled(true) })

	SetColorEnabled(true)
	colored := renderEveryTUISurface(t)
	require.NotEmpty(t, sgrPattern.FindAllString(colored, -1),
		"with colour on the TUI must still be decorated, or this test proves nothing")

	SetColorEnabled(false)
	plain := renderEveryTUISurface(t)

	assert.Empty(t, sgrPattern.FindAllString(plain, -1),
		"--color off must reach the interactive renderer, not just the compact one")
}

// TestSetColorEnabledKeepsTheWords checks that switching colour off removes only
// decoration. Stripping the status text along with the escape codes would
// reintroduce #399 for anyone running with NO_COLOR set.
func TestSetColorEnabledKeepsTheWords(t *testing.T) {
	forceColorProfile(t)
	t.Cleanup(func() { SetColorEnabled(true) })

	SetColorEnabled(false)
	m := newConnectionTestModel(t)
	view := m.View()

	assert.Contains(t, view, "Listening on 1 source • 1 connection")
	// Anywhere in the frame, not specifically the status bar: where the
	// connection state is drawn belongs to TestStatusBarAlwaysReportsConnection-
	// State. Asserting on lastLine here meant a #399 status-bar regression
	// failed as a #404 colour bug and pointed at the wrong fix.
	assert.Contains(t, view, connectingLabel)
}
