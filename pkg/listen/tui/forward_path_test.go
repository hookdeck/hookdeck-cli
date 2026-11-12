package tui

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/websocket"
)

// The TUI prints the forwarded URL in two places, and both used to leave the
// forwarding URL's path out or join it with a doubled "/".

// The request line in the event details (and the copied request) is the URL the
// event was forwarded to.
func TestEventDetailsRequestLineJoinsForwardingURLAndEventPath(t *testing.T) {
	for _, tc := range []struct {
		name      string
		forwardTo string
		eventPath string
		want      string
	}{
		{"trailing slash on the forwarding URL", "http://localhost:3500/base/", "/webhook", "POST http://localhost:3500/base/webhook"},
		{"no trailing slash on the forwarding URL", "http://localhost:3500/base", "/webhook", "POST http://localhost:3500/base/webhook"},
		{"forwarding URL is the root", "http://localhost:3500/", "/webhook", "POST http://localhost:3500/webhook"},
		{"event path keeps its own trailing slash", "http://localhost:3500/base/", "/webhook/", "POST http://localhost:3500/base/webhook/"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newDetailsTestModel(t)
			target, err := url.Parse(tc.forwardTo)
			require.NoError(t, err)
			m.cfg.TargetURL = target

			event := &EventInfo{
				ID: "evt_test",
				Data: &websocket.Attempt{Body: websocket.AttemptBody{
					Path:    tc.eventPath,
					Request: websocket.AttemptRequest{Method: "POST", DataString: "{}"},
				}},
			}

			content, copyContent := m.buildEventDetailsContent(event)

			assert.Equal(t, tc.want, copyContent.requestLine)
			assert.Contains(t, content, tc.want)
		})
	}
}

// The header lists each connection under its source as "Forwards to → <URL>".
func TestConnectionInfoForwardsToJoinsForwardingURLAndConnectionPath(t *testing.T) {
	for _, tc := range []struct {
		name      string
		forwardTo string
		cliPath   string
		want      string
	}{
		{"trailing slash on the forwarding URL", "http://localhost:3030/base/", "/webhook", "Forwards to → http://localhost:3030/base/webhook"},
		{"no trailing slash on the forwarding URL", "http://localhost:3030/base", "/webhook", "Forwards to → http://localhost:3030/base/webhook"},
		{"forwarding URL has no path", "http://localhost:3030", "/webhook", "Forwards to → http://localhost:3030/webhook"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newConnectionTestModel(t)
			target, err := url.Parse(tc.forwardTo)
			require.NoError(t, err)
			m.cfg.TargetURL = target
			m.cfg.Connections[0].Destination = &hookdeck.Destination{
				ID: "des_1", Type: "CLI", Config: map[string]interface{}{"path": tc.cliPath},
			}

			assert.Contains(t, m.renderConnectionInfo(), tc.want)
		})
	}
}
