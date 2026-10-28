package proxy

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/websocket"
)

func TestProcessAttemptJoinsForwardingURLPathAndEventPath(t *testing.T) {
	tests := []struct {
		name       string
		forwardTo  string
		eventPath  string
		expectPath string
	}{
		{"forwarding URL with a trailing slash", "/base/", "/webhook", "/base/webhook"},
		{"forwarding URL without a trailing slash", "/base", "/webhook", "/base/webhook"},
		{"forwarding URL is the root", "/", "/webhook", "/webhook"},
		{"forwarding URL has no path", "", "/webhook", "/webhook"},
		{"event path keeps its own trailing slash", "/base/", "/webhook/", "/base/webhook/"},
		{"event path is empty", "/base/", "", "/base/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			received := make(chan string, 1)
			local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				received <- r.URL.Path
				w.WriteHeader(http.StatusOK)
			}))
			t.Cleanup(local.Close)

			target, err := url.Parse(local.URL + tt.forwardTo)
			require.NoError(t, err)
			p := New(&Config{URL: target, NoHealthcheck: true}, nil, newRecordingRenderer())

			p.processAttempt(websocket.IncomingMessage{Attempt: &websocket.Attempt{
				Body: websocket.AttemptBody{
					Path:      tt.eventPath,
					EventID:   "evt_test",
					AttemptId: "evt_test",
					Request:   websocket.AttemptRequest{Method: http.MethodPost, DataString: "{}", Headers: json.RawMessage(`{}`)},
				},
			}})

			select {
			case got := <-received:
				assert.Equal(t, tt.expectPath, got)
			case <-time.After(5 * time.Second):
				t.Fatal("local server never received the request")
			}
		})
	}
}
