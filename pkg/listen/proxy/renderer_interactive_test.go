package proxy

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/listen/tui"
)

// recordingRenderer aside, InteractiveRenderer had no unit tests: its messages
// only ever went to a real tea.Program, which needs a terminal. sendMsg exists
// so they can be recorded instead.
func recordingInteractiveRenderer() (*InteractiveRenderer, *[]tea.Msg) {
	var sent []tea.Msg
	r := &InteractiveRenderer{doneCh: make(chan struct{})}
	r.sendMsg = func(msg tea.Msg) { sent = append(sent, msg) }
	return r, &sent
}

// TestInteractiveRendererReportsASessionErrorAsAConnectionFailure covers the
// other half of #399's failure path.
//
// A session-level OnError means there is no connection at all - a rejected
// session, a dead API - and the TUI has to say so rather than sit on
// "Connecting…" until it is torn down. Per-event errors go to OnEventError and
// are unaffected.
func TestInteractiveRendererReportsASessionErrorAsAConnectionFailure(t *testing.T) {
	r, sent := recordingInteractiveRenderer()

	sessionErr := errors.New("error while authenticating with Hookdeck: 401 Unauthorized")
	r.OnError(sessionErr)

	require.Len(t, *sent, 1, "a session error must reach the TUI")
	failed, ok := (*sent)[0].(tui.ConnectionFailedMsg)
	require.True(t, ok, "a session error must be shown as a connection failure, got %T", (*sent)[0])
	assert.Equal(t, sessionErr, failed.Err, "the TUI needs the reason to display")
}

// TestInteractiveRendererConnectionFailedReachesTheModel checks the message the
// renderer sends is one the model acts on, so the two halves cannot drift: a
// renamed or unhandled message would leave the status bar claiming the CLI is
// still connecting while it exits.
func TestInteractiveRendererConnectionFailedReachesTheModel(t *testing.T) {
	r, sent := recordingInteractiveRenderer()
	r.OnConnectionFailed(errors.New("dial tcp 127.0.0.1:9: connect: connection refused"))
	require.Len(t, *sent, 1)

	model := tui.NewModel(&tui.Config{DeviceName: "test-device"})
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m, ok := updated.(tui.Model)
	require.True(t, ok, "unexpected model type %T", updated)

	assert.NotContains(t, m.View(), "connection refused",
		"the failure must come from the message, not from the initial frame")

	updated, _ = m.Update((*sent)[0])
	m, ok = updated.(tui.Model)
	require.True(t, ok, "unexpected model type %T", updated)

	assert.Contains(t, m.View(), "connection refused",
		"the model must act on the message the renderer sends")
}

// TestInteractiveRendererWithoutATUISendsNothing keeps the nil-program guard:
// the renderer is constructed before Bubble Tea is known to be usable, and
// these are called from Proxy.Run regardless.
func TestInteractiveRendererWithoutATUISendsNothing(t *testing.T) {
	r := &InteractiveRenderer{doneCh: make(chan struct{})}
	assert.NotPanics(t, func() {
		r.OnConnecting()
		r.OnConnected()
		r.OnDisconnected()
		r.OnError(errors.New("boom"))
		r.OnConnectionFailed(errors.New("boom"))
		r.OnServerHealthChanged(false, errors.New("boom"))
	})
}
