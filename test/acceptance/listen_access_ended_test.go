//go:build listen

package acceptance

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	ws "github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests run the built binary against the real API, with only the
// websocket pointed at a local server through the hidden --ws-base flag. The
// unit tests call proxy.Run() directly; these cover what a user sees: the
// message, printed once, and the exit code.

// listenExitTimeout covers listen's startup against the real API (creating the
// source, connection and session) before it makes its first websocket connect.
const listenExitTimeout = 60 * time.Second

// startFakeWebsocket starts a local server that answers every websocket
// connect with handle, and counts the connects it receives.
func startFakeWebsocket(t *testing.T, handle http.HandlerFunc) (string, *atomic.Int32) {
	t.Helper()
	var connects atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connects.Add(1)
		handle(w, r)
	}))
	t.Cleanup(server.Close)
	return "ws" + strings.TrimPrefix(server.URL, "http"), &connects
}

// refuseWith answers a websocket connect with status and body, without
// upgrading it.
func refuseWith(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if body != "" {
			w.Header().Set("Content-Type", "application/json")
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

// startListenAgainst starts `hookdeck listen` on a new source, with its
// websocket pointed at wsURL.
func startListenAgainst(t *testing.T, wsURL, sourcePrefix string) (*syncBuffer, *syncBuffer, chan error) {
	t.Helper()
	cli := NewCLIRunner(t)
	sourceName := sourcePrefix + "-" + generateTimestamp()
	_, stdout, stderr, done := startListenCapturingOutput(t, cli,
		"listen", "8080", sourceName, "--ws-base", wsURL)
	return stdout, stderr, done
}

// requireExitCode waits for listen to exit and checks its exit code.
func requireExitCode(t *testing.T, done chan error, stdout, stderr *syncBuffer, want int) {
	t.Helper()
	select {
	case err := <-done:
		var exitErr *exec.ExitError
		require.True(t, errors.As(err, &exitErr),
			"listen should exit with status %d, got %v\nSTDOUT:\n%s\nSTDERR:\n%s", want, err, stdout.String(), stderr.String())
		assert.Equal(t, want, exitErr.ExitCode())
	case <-time.After(listenExitTimeout):
		t.Fatalf("listen did not exit within %s\nSTDOUT:\n%s\nSTDERR:\n%s", listenExitTimeout, stdout.String(), stderr.String())
	}
}

func TestListenStopsWhenConnectIsRefused(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	wsURL, connects := startFakeWebsocket(t, refuseWith(http.StatusForbidden, ""))
	stdout, stderr, done := startListenAgainst(t, wsURL, "test-refused")

	requireExitCode(t, done, stdout, stderr, 1)

	output := stdout.String() + stderr.String()
	assert.Equal(t, 1, strings.Count(output, "The connection was refused (HTTP 403)."),
		"the refusal should be printed exactly once; got:\n%s", output)
	assert.NotContains(t, output, "reconnecting", "a refused connect must not be retried")
	assert.Equal(t, int32(1), connects.Load(), "listen should connect once and stop")
}

func TestListenShowsTheServersReasonWhenConnectIsRefused(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	reason := "This CLI key can't reach this project."
	wsURL, connects := startFakeWebsocket(t, refuseWith(http.StatusForbidden,
		`{"error":{"code":"NO_PROJECT_ACCESS","message":"`+reason+`"}}`))
	stdout, stderr, done := startListenAgainst(t, wsURL, "test-refused-reason")

	requireExitCode(t, done, stdout, stderr, 1)

	output := stdout.String() + stderr.String()
	assert.Equal(t, 1, strings.Count(output, reason),
		"the server's reason should be printed exactly once; got:\n%s", output)
	assert.NotContains(t, output, "HTTP 403", "the default message must not replace the server's reason")
	assert.Equal(t, int32(1), connects.Load(), "listen should connect once and stop")
}

func TestListenStopsWhenKeyIsRevokedWhileConnected(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	upgrader := ws.Upgrader{}
	wsURL, connects := startFakeWebsocket(t, func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		// Let listen settle into the connected state before the revoke.
		time.Sleep(500 * time.Millisecond)
		_ = conn.WriteControl(ws.CloseMessage,
			ws.FormatCloseMessage(4003, "CLI_KEY_REVOKED"), time.Now().Add(time.Second))
		// Wait for the client's close reply or for it to hang up.
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	stdout, stderr, done := startListenAgainst(t, wsURL, "test-revoked")

	requireExitCode(t, done, stdout, stderr, 1)

	output := stdout.String() + stderr.String()
	assert.Equal(t, 1, strings.Count(output, "Your CLI key was revoked."),
		"the revoke should be printed exactly once; got:\n%s", output)
	assert.NotContains(t, output, "file a GitHub issue", "a revoke is not a CLI bug")
	assert.Equal(t, int32(1), connects.Load(), "listen should not reconnect after a revoke")
}

// TestListenKeepsReconnectingWhenServiceUnavailable is the negative control: the
// same fake server answering 503 must not stop listen. Without it, a fake server
// that broke listen in some other way would pass the tests above.
func TestListenKeepsReconnectingWhenServiceUnavailable(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	wsURL, connects := startFakeWebsocket(t, refuseWith(http.StatusServiceUnavailable, ""))
	stdout, stderr, done := startListenAgainst(t, wsURL, "test-unavailable")

	deadline := time.Now().Add(listenExitTimeout)
	for connects.Load() < 3 && time.Now().Before(deadline) {
		select {
		case err := <-done:
			t.Fatalf("listen exited on a 503, which a retry can fix: %v\nSTDOUT:\n%s\nSTDERR:\n%s", err, stdout.String(), stderr.String())
		case <-time.After(250 * time.Millisecond):
		}
	}
	require.GreaterOrEqual(t, connects.Load(), int32(3), "listen should keep reconnecting on a 503")

	select {
	case err := <-done:
		t.Fatalf("listen exited on a 503: %v", err)
	default:
	}

	output := stdout.String() + stderr.String()
	assert.NotContains(t, output, "The connection was refused")
	assert.NotContains(t, output, "Your CLI key was revoked")
}
