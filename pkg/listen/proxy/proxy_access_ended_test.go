package proxy

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	ws "github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/websocket"
)

// runAgainst runs `listen` against a websocket server and returns what Run
// returned, how many times the CLI connected, and the renderer.
func runAgainst(t *testing.T, wsHandler http.HandlerFunc) (error, int32, *recordingRenderer) {
	t.Helper()

	// No backoff, so a CLI that keeps retrying shows up as many connects.
	restoreBackoff := fixedConnectBackoffMS
	fixedConnectBackoffMS = 1
	t.Cleanup(func() { fixedConnectBackoffMS = restoreBackoff })

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == hookdeck.APIPathPrefix+"/cli-sessions" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"cli_sess_1"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(api.Close)

	var connects atomic.Int32
	wsServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connects.Add(1)
		wsHandler(w, r)
	}))
	t.Cleanup(wsServer.Close)

	apiURL, err := url.Parse(api.URL)
	require.NoError(t, err)
	targetURL, err := url.Parse("http://127.0.0.1:1")
	require.NoError(t, err)

	renderer := newRecordingRenderer()
	p := New(&Config{
		DeviceName:    "test-device",
		Key:           "sk_test_123456789012",
		URL:           targetURL,
		APIBaseURL:    api.URL,
		WSBaseURL:     "ws://" + strings.TrimPrefix(wsServer.URL, "http://"),
		NoWSS:         true,
		NoHealthcheck: true,
		APIClient:     &hookdeck.Client{BaseURL: apiURL, APIKey: "sk_test_123456789012"},
	}, nil, renderer)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	runErr := p.Run(ctx)
	require.NoError(t, ctx.Err(), "listen must stop by itself instead of retrying until the test times out")
	return runErr, connects.Load(), renderer
}

func assertStoppedWith(t *testing.T, runErr error, renderer *recordingRenderer, message string) {
	t.Helper()

	var accessErr *websocket.AccessEndedError
	require.True(t, errors.As(runErr, &accessErr), "listen must fail with why access ended, got %v", runErr)
	assert.Equal(t, message, runErr.Error())

	calls := renderer.recorded()
	failedAt := indexOfCall(calls, "OnConnectionFailed")
	cleanupAt := indexOfCall(calls, "Cleanup")
	require.GreaterOrEqual(t, failedAt, 0, "the renderer must be told why listen stopped: %v", calls)
	assert.Less(t, failedAt, cleanupAt, "the reason has to be shown before the renderer is torn down: %v", calls)
	assert.Equal(t, -1, indexOfCall(calls, "OnDisconnected"),
		"listen must not say it is reconnecting when it won't: %v", calls)
	assert.Equal(t, message, renderer.connectionFailure().Error())
}

func TestRunStopsWhenTheConnectIsRefused(t *testing.T) {
	message := "This CLI key can't reach this project. Run `hookdeck project use` to pick another one."
	runErr, connects, renderer := runAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"code":"NO_PROJECT_ACCESS","message":"` + message + `"}}`))
	})

	assertStoppedWith(t, runErr, renderer, message)
	assert.Equal(t, int32(1), connects, "a refused key must not be retried")
}

// Once connected, listen otherwise retries forever, so a revoke must stop it.
func TestRunStopsWhenTheKeyIsRevokedWhileConnected(t *testing.T) {
	upgrader := ws.Upgrader{}
	runErr, connects, renderer := runAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		_ = conn.WriteControl(
			ws.CloseMessage,
			ws.FormatCloseMessage(4003, "CLI_KEY_REVOKED"),
			time.Now().Add(time.Second),
		)
	})

	assertStoppedWith(t, runErr, renderer, "Your CLI key was revoked. Run `hookdeck login` to log in again.")
	assert.Equal(t, int32(1), connects, "a revoked key must not reconnect")
}
