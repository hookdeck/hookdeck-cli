package tui

import (
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hookdeck/hookdeck-cli/pkg/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCopyRequestCopiesCompleteRequestWithoutResponse(t *testing.T) {
	m := newDetailsTestModel(t)

	updated, cmd := m.handleKeyPress(keyMsg('d'))
	require.Nil(t, cmd)
	m = updated.(Model)
	require.True(t, m.showingDetails)
	assert.NotContains(t, m.detailsViewport.View(), "off_screen_marker", "test marker should be outside the visible viewport")
	assert.Contains(t, m.renderDetailsView(), "[C] Copy request • [H] Copy headers • [B] Copy body")

	var copied string
	m.clipboardWrite = func(content string) error {
		copied = content
		return nil
	}

	updated, cmd = m.handleKeyPress(keyMsg('C'))
	m = updated.(Model)
	require.NotNil(t, cmd)
	assert.Equal(t, detailsCopyPending, m.detailsCopyState)

	result, ok := cmd().(copyDetailsResultMsg)
	require.True(t, ok)
	require.NoError(t, result.err)

	updated, cmd = m.Update(result)
	require.Nil(t, cmd)
	m = updated.(Model)
	assert.Equal(t, detailsCopySucceeded, m.detailsCopyState)

	assert.Contains(t, copied, "POST http://localhost:3500/webhooks")
	assert.Contains(t, copied, "x-test: header-value")
	assert.Contains(t, copied, `"off_screen_marker": "copied"`)
	assert.NotContains(t, copied, "evt_test")
	assert.NotContains(t, copied, `"response": "complete"`)
	assert.NotContains(t, copied, "Response")
	assert.NotContains(t, copied, "Return to event list")
	assert.NotContains(t, copied, "\x1b")
	assert.Contains(t, m.renderDetailsView(), "Copied request")
}

func TestCopyRequestHeadersOrBody(t *testing.T) {
	tests := []struct {
		name        string
		key         rune
		contains    string
		notContains []string
		status      string
	}{
		{
			name:        "headers",
			key:         'H',
			contains:    "x-test: header-value",
			notContains: []string{"POST ", "off_screen_marker", "response"},
			status:      "Copied request headers",
		},
		{
			name:        "body",
			key:         'B',
			contains:    `"off_screen_marker": "copied"`,
			notContains: []string{"POST ", "x-test:", "response"},
			status:      "Copied request body",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newDetailsTestModel(t)
			updated, _ := m.handleKeyPress(keyMsg('d'))
			m = updated.(Model)

			var copied string
			m.clipboardWrite = func(content string) error {
				copied = content
				return nil
			}

			updated, cmd := m.handleKeyPress(keyMsg(tt.key))
			m = updated.(Model)
			require.NotNil(t, cmd)

			result := cmd().(copyDetailsResultMsg)
			updated, _ = m.Update(result)
			m = updated.(Model)

			assert.Contains(t, copied, tt.contains)
			for _, excluded := range tt.notContains {
				assert.NotContains(t, copied, excluded)
			}
			assert.Contains(t, m.renderDetailsView(), tt.status)
		})
	}
}

func TestCopyRequestReportsClipboardFailure(t *testing.T) {
	m := newDetailsTestModel(t)

	updated, _ := m.handleKeyPress(keyMsg('d'))
	m = updated.(Model)
	m.clipboardWrite = func(string) error {
		return errors.New("clipboard unavailable")
	}

	updated, cmd := m.handleKeyPress(keyMsg('C'))
	m = updated.(Model)
	require.NotNil(t, cmd)

	result := cmd().(copyDetailsResultMsg)
	updated, _ = m.Update(result)
	m = updated.(Model)

	assert.Equal(t, detailsCopyFailed, m.detailsCopyState)
	assert.Contains(t, m.renderDetailsView(), "Could not copy request")
}

func TestCopyShortcutIsIgnoredOutsideDetailsView(t *testing.T) {
	m := newDetailsTestModel(t)
	called := false
	m.clipboardWrite = func(string) error {
		called = true
		return nil
	}

	updated, cmd := m.handleKeyPress(keyMsg('C'))
	m = updated.(Model)

	assert.Nil(t, cmd)
	assert.False(t, called)
	assert.False(t, m.showingDetails)
}

func newDetailsTestModel(t *testing.T) Model {
	t.Helper()

	targetURL, err := url.Parse("http://localhost:3500")
	require.NoError(t, err)

	m := NewModel(&Config{TargetURL: targetURL})
	m.width = 100
	m.height = 12
	m.ready = true
	m.viewportReady = true
	m.isConnected = true
	m.hasReceivedEvent = true
	m.AddEvent(EventInfo{
		ID:               "evt_test",
		Time:             time.Date(2026, time.July, 21, 18, 18, 38, 0, time.UTC),
		ResponseStatus:   200,
		ResponseHeaders:  map[string][]string{"content-type": {"application/json"}},
		ResponseBody:     `{"response":"complete"}`,
		ResponseDuration: 1190800 * time.Nanosecond,
		Data: &websocket.Attempt{Body: websocket.AttemptBody{
			Path: "/webhooks",
			Request: websocket.AttemptRequest{
				Method:     "POST",
				Headers:    []byte(`{"x-test":"header-value"}`),
				DataString: `{"padding":"` + strings.Repeat("long content ", 20) + `","off_screen_marker":"copied"}`,
			},
		}},
	})

	return m
}

func keyMsg(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}
