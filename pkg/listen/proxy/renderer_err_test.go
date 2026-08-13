package proxy

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRendererErrContract covers the second half of #333. Proxy.Run returns
// renderer.Err() when the renderer stops, so a renderer that failed to start must
// report a non-nil error and a normal quit must report nil. Getting this backwards
// is what made the original bug invisible: `listen` exited 0 with no tunnel.
func TestRendererErrContract(t *testing.T) {
	t.Run("simple renderer never reports an error", func(t *testing.T) {
		r := &SimpleRenderer{}
		assert.NoError(t, r.Err())
	})

	t.Run("interactive renderer reports nil after a normal quit", func(t *testing.T) {
		r := &InteractiveRenderer{doneCh: make(chan struct{})}
		close(r.doneCh)

		<-r.Done()
		assert.NoError(t, r.Err(), "a user-initiated quit must not look like a failure")
	})

	t.Run("interactive renderer surfaces a startup failure", func(t *testing.T) {
		r := &InteractiveRenderer{doneCh: make(chan struct{})}

		// Mirror what the TUI goroutine does when tea.Program.Run fails.
		go func() {
			r.mu.Lock()
			r.runErr = errors.New("could not open a new TTY: open /dev/tty: no such device or address")
			r.mu.Unlock()
			close(r.doneCh)
		}()

		<-r.Done()

		err := r.Err()
		require.Error(t, err, "a renderer that never started must be reported as a failure")
		assert.Contains(t, err.Error(), "/dev/tty")
	})

	t.Run("both renderers satisfy the Renderer interface", func(t *testing.T) {
		var _ Renderer = (*SimpleRenderer)(nil)
		var _ Renderer = (*InteractiveRenderer)(nil)
	})
}
