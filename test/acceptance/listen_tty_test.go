//go:build listen

package acceptance

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ttySGRPattern matches an SGR (colour/bold/faint) escape sequence.
var ttySGRPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// startListenOnAControllingTTY runs `hookdeck listen` with a real pty as its
// controlling terminal, so the interactive renderer starts for real, and returns
// everything it drew during the window.
//
// creack/pty sets Setsid and Setctty, which is what makes this a *controlling*
// terminal rather than just a pipe that happens to pass term.IsTerminal. Earlier
// attempts to capture the TUI with `script -q` produced inconsistent output and
// sent people chasing failures that were not there.
func startListenOnAControllingTTY(t *testing.T, cli *CLIRunner, window time.Duration, extraArgs ...string) string {
	t.Helper()

	projectRoot, err := filepath.Abs("../..")
	require.NoError(t, err, "Failed to get project root")

	binary := filepath.Join(projectRoot, "hookdeck-listen-tty-"+generateTimestamp())
	buildCmd := exec.Command("go", "build", "-o", binary, ".")
	buildCmd.Dir = projectRoot
	require.NoError(t, buildCmd.Run(), "failed to build CLI binary")
	t.Cleanup(func() { _ = os.Remove(binary) })

	cmd := exec.Command(binary, extraArgs...)
	cmd.Dir = projectRoot

	env := os.Environ()
	if cli.configPath != "" {
		env = appendEnvOverride(env, "HOOKDECK_CONFIG_FILE", cli.configPath)
	}
	// A TUI-sized window, so the whole frame has room to render.
	env = appendEnvOverride(env, "TERM", "xterm-256color")
	cmd.Env = env

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 45, Cols: 120})
	require.NoError(t, err, "listen should start on a pty")

	t.Cleanup(func() {
		_ = ptmx.Close()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		}
	})

	var out strings.Builder
	deadline := time.Now().Add(window)
	buf := make([]byte, 32*1024)
	for time.Now().Before(deadline) {
		_ = ptmx.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		n, readErr := ptmx.Read(buf)
		if n > 0 {
			out.Write(buf[:n])
		}
		if readErr != nil && !os.IsTimeout(readErr) {
			break
		}
	}

	_ = cmd.Process.Kill()
	_, _ = cmd.Process.Wait()

	return out.String()
}

// TestListenInteractiveShowsPendingStateBeforeItIsConnected is the acceptance
// test for #399, run the way the issue was reported: a real terminal and a
// websocket base URL nothing is listening on.
//
// The TUI used to render its complete layout immediately — brand header,
// "Listening on …", "Requests to →", "Forwards to →" — with an empty status bar,
// and stay that way for the whole 40-second attempt budget before tearing the
// alt-screen down and printing an error. That is #376 inverted: there, readiness
// was never announced; here, the absence of a line was the only thing separating
// "connected" from "not connected", and absence is not a signal a user reads.
func TestListenInteractiveShowsPendingStateBeforeItIsConnected(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()

	var conn Connection
	require.NoError(t, cli.RunJSON(&conn,
		"gateway", "connection", "create",
		"--name", "test-tty-pending-conn-"+timestamp,
		"--source-name", "test-tty-pending-"+timestamp,
		"--source-type", "WEBHOOK",
		"--destination-name", "test-tty-pending-dst-"+timestamp,
		"--destination-type", "CLI",
		"--destination-cli-path", "/",
	))
	require.NotEmpty(t, conn.ID)
	t.Cleanup(func() { deleteConnection(t, cli, conn.ID) })

	// Port 9 (discard) is closed for websockets, so the CLI connects to the API,
	// renders the TUI, and then never establishes the tunnel.
	out := startListenOnAControllingTTY(t, cli, 20*time.Second,
		"listen", "3030", "test-tty-pending-"+timestamp,
		"--ws-base", "ws://127.0.0.1:9")

	require.Contains(t, out, "Listening on",
		"the TUI should have rendered; without that this test proves nothing")

	assert.Contains(t, out, "Connecting…",
		"an unconnected session must say so, not render a live-looking layout (#399)")
	assert.NotContains(t, out, "Connected.",
		"the CLI never connected, so it must never claim it did")
}

// TestListenInteractiveAnnouncesConnectedOnATTY is the other half of #399: the
// pending state has to be *replaced*, not merely added. Readiness stays
// affirmative in interactive mode exactly as #376 made it affirmative everywhere
// else.
func TestListenInteractiveAnnouncesConnectedOnATTY(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()

	var conn Connection
	require.NoError(t, cli.RunJSON(&conn,
		"gateway", "connection", "create",
		"--name", "test-tty-ready-conn-"+timestamp,
		"--source-name", "test-tty-ready-"+timestamp,
		"--source-type", "WEBHOOK",
		"--destination-name", "test-tty-ready-dst-"+timestamp,
		"--destination-type", "CLI",
		"--destination-cli-path", "/",
	))
	require.NotEmpty(t, conn.ID)
	t.Cleanup(func() { deleteConnection(t, cli, conn.ID) })

	out := startListenOnAControllingTTY(t, cli, 25*time.Second,
		"listen", "3030", "test-tty-ready-"+timestamp)

	require.Contains(t, out, "Listening on",
		"the TUI should have rendered; without that this test proves nothing")
	assert.Contains(t, out, "Connected.",
		"a connected session must say so in the status bar (#399)")
}

// TestListenInteractiveHonoursColorOff is the acceptance test for #404. --color
// off is applied in config.InitConfig, which only ever reached pkg/ansi; the TUI
// draws with lipgloss, so a controlling-pty run with the flag set still emitted
// 48 SGR sequences. The control run asserts the default is still decorated, so a
// TUI that failed to start could not make this pass by drawing nothing.
func TestListenInteractiveHonoursColorOff(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()

	var conn Connection
	require.NoError(t, cli.RunJSON(&conn,
		"gateway", "connection", "create",
		"--name", "test-tty-color-conn-"+timestamp,
		"--source-name", "test-tty-color-"+timestamp,
		"--source-type", "WEBHOOK",
		"--destination-name", "test-tty-color-dst-"+timestamp,
		"--destination-type", "CLI",
		"--destination-cli-path", "/",
	))
	require.NotEmpty(t, conn.ID)
	t.Cleanup(func() { deleteConnection(t, cli, conn.ID) })

	sourceName := "test-tty-color-" + timestamp

	colored := startListenOnAControllingTTY(t, cli, 20*time.Second, "listen", "3030", sourceName)
	require.Contains(t, colored, "Listening on", "the control run should have rendered the TUI")
	require.NotEmpty(t, ttySGRPattern.FindAllString(colored, -1),
		"the default TUI is decorated; without that this test proves nothing")

	plain := startListenOnAControllingTTY(t, cli, 20*time.Second,
		"listen", "3030", sourceName, "--color", "off")

	require.Contains(t, plain, "Listening on",
		"--color off must still render the TUI, just without decoration")
	assert.Empty(t, ttySGRPattern.FindAllString(plain, -1),
		"--color off must reach the interactive renderer, not just the compact one (#404)")
}
