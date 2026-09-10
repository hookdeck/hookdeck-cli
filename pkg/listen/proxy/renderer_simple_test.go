package proxy

import (
	"io"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/ansi"
)

// captureStdout runs fn with os.Stdout redirected and returns what it wrote.
// The renderer prints with fmt.Printf, so stdout is the only place to look —
// which is the point: these tests assert on the stream a caller actually reads.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	original := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	done := make(chan string, 1)
	go func() {
		out, _ := io.ReadAll(r)
		done <- string(out)
	}()

	fn()

	require.NoError(t, w.Close())
	os.Stdout = original

	return <-done
}

// TestSimpleRendererAnnouncesReadinessWithoutASpinner covers the regression where
// `listen` connected but never said so. The readiness line lived inside `if
// r.spinner != nil`, and ansi.StartNewSpinner returns nil whenever the log stream
// is not a TTY or colors are disabled — so every piped, redirected, CI, or
// --color=off run forwarded events in silence and callers timed out waiting for a
// state the CLI had already reached.
func TestSimpleRendererAnnouncesReadinessWithoutASpinner(t *testing.T) {
	target, err := url.Parse("http://localhost:3000")
	require.NoError(t, err)

	// go test never gives the log stream a TTY, so the spinner is always nil in
	// these subtests — exactly the configuration that used to swallow the output.

	t.Run("compact mode says it is ready", func(t *testing.T) {
		r := NewSimpleRenderer(&RendererConfig{TargetURL: target}, false)

		out := captureStdout(t, func() {
			r.OnConnecting()
			r.OnConnected()
		})

		assert.Nil(t, r.spinner, "no TTY means no spinner: the case that regressed")
		assert.Contains(t, out, "Connected. Waiting for events...")
	})

	t.Run("quiet mode says it is ready", func(t *testing.T) {
		r := NewSimpleRenderer(&RendererConfig{TargetURL: target}, true)

		out := captureStdout(t, func() {
			r.OnConnecting()
			r.OnConnected()
		})

		assert.Contains(t, out, "Connected. Quiet mode: only errors and warnings will be shown.")
	})

	t.Run("--color off does not remove the readiness line", func(t *testing.T) {
		ansi.DisableColors = true
		t.Cleanup(func() { ansi.DisableColors = false })

		r := NewSimpleRenderer(&RendererConfig{TargetURL: target}, false)

		out := captureStdout(t, func() {
			r.OnConnecting()
			r.OnConnected()
		})

		assert.Contains(t, out, "Connected. Waiting for events...",
			"--color controls decoration, not whether the CLI reports its state")
	})

	t.Run("a dropped connection is reported on stdout too", func(t *testing.T) {
		r := NewSimpleRenderer(&RendererConfig{TargetURL: target}, false)

		out := captureStdout(t, func() {
			r.OnConnecting()
			r.OnConnected()
			r.OnDisconnected()
		})

		assert.Contains(t, out, "Connection lost, reconnecting...")
	})

	t.Run("a drop before the first connect stays quiet", func(t *testing.T) {
		r := NewSimpleRenderer(&RendererConfig{TargetURL: target}, false)

		out := captureStdout(t, func() {
			r.OnConnecting()
			r.OnDisconnected()
		})

		assert.NotContains(t, out, "Connection lost",
			"a failed first attempt is retried, not announced as a lost connection")
	})

	t.Run("reconnecting is announced once, then readiness again", func(t *testing.T) {
		r := NewSimpleRenderer(&RendererConfig{TargetURL: target}, false)

		out := captureStdout(t, func() {
			r.OnConnecting()
			r.OnConnected()
			r.OnDisconnected()
			r.OnDisconnected()
			r.OnConnected()
		})

		assert.Equal(t, 1, strings.Count(out, "Connection lost, reconnecting..."),
			"repeated retries must not repeat the notice")
		assert.Equal(t, 2, strings.Count(out, "Connected. Waiting for events..."),
			"a recovered connection is a state change worth reporting")
	})
}
