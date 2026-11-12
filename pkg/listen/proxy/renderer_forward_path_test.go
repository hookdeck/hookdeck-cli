package proxy

import (
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/listen/tui"
	"github.com/hookdeck/hookdeck-cli/pkg/websocket"
)

// The request is sent to a URL built in processAttempt, and each renderer
// prints the URL it believes was requested. These tests pin both renderers to
// the single-slash join, so a forwarding URL ending in "/" does not print
// "/base//webhook" for a request that went to "/base/webhook".

func forwardPathRendererConfig(t *testing.T, forwardTo string) *RendererConfig {
	t.Helper()
	cfg := linksRendererConfig(t, "gateway")
	target, err := url.Parse(forwardTo)
	require.NoError(t, err)
	cfg.TargetURL = target
	return cfg
}

func forwardPathAttempt(eventPath string) *websocket.Attempt {
	return &websocket.Attempt{Body: websocket.AttemptBody{
		Path:         eventPath,
		EventID:      "evt_1",
		AttemptId:    "atm_1",
		ConnectionId: "web_1",
		Request:      websocket.AttemptRequest{Method: "POST"},
	}}
}

var forwardPathRendererCases = []struct {
	name      string
	forwardTo string
	eventPath string
	want      string
}{
	{"trailing slash on the forwarding URL", "http://localhost:3000/base/", "/webhook", "POST http://localhost:3000/base/webhook "},
	{"no trailing slash on the forwarding URL", "http://localhost:3000/base", "/webhook", "POST http://localhost:3000/base/webhook "},
	{"forwarding URL is the root", "http://localhost:3000/", "/webhook", "POST http://localhost:3000/webhook "},
	{"event path keeps its own trailing slash", "http://localhost:3000/base/", "/webhook/", "POST http://localhost:3000/base/webhook/ "},
}

func TestSimpleRendererJoinsForwardingURLAndEventPath(t *testing.T) {
	response := &EventResponse{StatusCode: 200, Duration: 5 * time.Millisecond}

	for _, tc := range forwardPathRendererCases {
		t.Run(tc.name, func(t *testing.T) {
			r := NewSimpleRenderer(forwardPathRendererConfig(t, tc.forwardTo), false)
			out := captureStdout(t, func() {
				r.OnEventComplete("evt_1", forwardPathAttempt(tc.eventPath), response, time.Now())
			})
			assert.Contains(t, out, tc.want)
		})
	}
}

func TestInteractiveRendererJoinsForwardingURLAndEventPath(t *testing.T) {
	response := &EventResponse{StatusCode: 200, Duration: 5 * time.Millisecond}

	for _, tc := range forwardPathRendererCases {
		t.Run(tc.name, func(t *testing.T) {
			r, sent := recordingInteractiveRenderer()
			r.cfg = forwardPathRendererConfig(t, tc.forwardTo)

			r.OnEventComplete("evt_1", forwardPathAttempt(tc.eventPath), response, time.Now())

			require.Len(t, *sent, 1)
			update, ok := (*sent)[0].(tui.UpdateEventMsg)
			require.True(t, ok, "unexpected message %T", (*sent)[0])
			assert.Contains(t, update.LogLine, tc.want)
		})
	}
}
