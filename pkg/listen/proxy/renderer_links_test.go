package proxy

import (
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/listen/tui"
	"github.com/hookdeck/hookdeck-cli/pkg/websocket"
)

// Both renderers used to build the event link by hand, so guest links stayed
// on Console after pkg/listen/links moved. These tests pin the log line each
// renderer prints to the shared builder's output.

func linksRendererConfig(t *testing.T, projectType string) *RendererConfig {
	t.Helper()
	target, err := url.Parse("http://localhost:3000")
	require.NoError(t, err)

	return &RendererConfig{
		DashboardBaseURL: "https://dashboard.hookdeck.com",
		InspectBaseURL:   "https://inspect.hookdeck.com",
		ProjectType:      projectType,
		ProjectID:        "tm_123",
		TargetURL:        target,
		Sources:          []*hookdeck.Source{{ID: "src_abc123def456"}},
	}
}

func linksTestAttempt() *websocket.Attempt {
	return &websocket.Attempt{Body: websocket.AttemptBody{
		Path:         "/webhooks",
		EventID:      "evt_1",
		AttemptId:    "atm_1",
		ConnectionId: "web_1",
		Request:      websocket.AttemptRequest{Method: "POST"},
	}}
}

func TestSimpleRendererEventLink(t *testing.T) {
	response := &EventResponse{StatusCode: 200, Duration: 5 * time.Millisecond}

	for _, tc := range []struct {
		name        string
		projectType string
		want        string
	}{
		{"dashboard is unchanged", "gateway", "→ https://dashboard.hookdeck.com/events/evt_1\n"},
		{"guest links to the event in Inspect", "console", "→ https://inspect.hookdeck.com/?event_id=evt_1&team_id=tm_123\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := NewSimpleRenderer(linksRendererConfig(t, tc.projectType), false)
			out := captureStdout(t, func() {
				r.OnEventComplete("evt_1", linksTestAttempt(), response, time.Now())
			})
			assert.Contains(t, out, tc.want)
			assert.NotContains(t, out, "console.hookdeck.com")
		})
	}
}

func TestInteractiveRendererEventLink(t *testing.T) {
	response := &EventResponse{StatusCode: 200, Duration: 5 * time.Millisecond}

	for _, tc := range []struct {
		name        string
		projectType string
		want        string
	}{
		{"dashboard is unchanged", "gateway", "https://dashboard.hookdeck.com/events/evt_1"},
		{"guest links to the event in Inspect", "console", "https://inspect.hookdeck.com/?event_id=evt_1&team_id=tm_123"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, sent := recordingInteractiveRenderer()
			r.cfg = linksRendererConfig(t, tc.projectType)

			r.OnEventComplete("evt_1", linksTestAttempt(), response, time.Now())

			require.Len(t, *sent, 1)
			update, ok := (*sent)[0].(tui.UpdateEventMsg)
			require.True(t, ok, "unexpected message %T", (*sent)[0])
			assert.Contains(t, update.LogLine, tc.want)
		})
	}
}
