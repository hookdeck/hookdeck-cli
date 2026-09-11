//go:build listen

package acceptance

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// syncBuffer is a bytes.Buffer safe for concurrent use.
//
// os/exec writes subprocess output from its own goroutine for as long as the
// process is running, while these tests read the captured output mid-run to
// check what has been printed so far. A bare bytes.Buffer races there and can
// return partial output, so every read and write goes through the mutex.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// startListenCapturingOutput starts `hookdeck listen` with the given extra args,
// stdout and stderr wired to pipes (i.e. not a terminal — the condition that made
// #333 fail), and returns the process plus buffers. The caller kills the process.
func startListenCapturingOutput(t *testing.T, cli *CLIRunner, extraArgs ...string) (*exec.Cmd, *syncBuffer, *syncBuffer, chan error) {
	t.Helper()

	// Registered before the cleanup that kills the process, so it runs after it:
	// listen creates the source it is pointed at, plus a cli-<source> connection
	// and destination, and nothing else in the test knows their ids.
	registerListenCleanup(t, cli, extraArgs)

	projectRoot, err := filepath.Abs("../..")
	require.NoError(t, err, "Failed to get project root")

	binary := filepath.Join(projectRoot, "hookdeck-listen-test-"+generateTimestamp())
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
	cmd.Env = env

	var stdout, stderr syncBuffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	require.NoError(t, cmd.Start(), "listen should start")
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	})

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	return cmd, &stdout, &stderr, done
}

// TestListenDefaultsToCompactWithoutTTY is the regression test for #333. With no
// terminal attached, `hookdeck listen` used to build the interactive renderer,
// which opens /dev/tty itself, fail with "could not open a new TTY", and exit 0
// having forwarded nothing. Crucially this test passes no --output, so it
// exercises the default path an agent or CI job actually hits.
func TestListenDefaultsToCompactWithoutTTY(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	sourceName := "test-notty-" + generateTimestamp()

	_, stdout, stderr, done := startListenCapturingOutput(t, cli, "listen", "8080", sourceName)

	t.Log("Waiting 6 seconds for listen to initialize...")
	time.Sleep(6 * time.Second)

	combined := stdout.String() + stderr.String()

	select {
	case err := <-done:
		t.Logf("STDOUT: %s", stdout.String())
		t.Logf("STDERR: %s", stderr.String())
		t.Fatalf("listen exited early without a TTY (#333): %v", err)
	default:
		t.Log("listen is still running without a terminal")
	}

	assert.NotContains(t, combined, "could not open a new TTY",
		"listen must not attempt the interactive renderer without a terminal (#333)")
	assert.NotContains(t, combined, "Bubble Tea error",
		"the TUI must not be started without a terminal (#333)")

	// The default path must stay quiet about a flag the caller never passed.
	assert.NotContains(t, combined, "falling back to --output compact",
		"no warning should be printed when --output was not set explicitly")

	t.Logf("Verified listen falls back to compact output without a TTY")
}

// TestListenExplicitInteractiveWithoutTTYWarnsAndFallsBack covers the other half
// of the #333 behaviour: someone who explicitly asked for interactive output on a
// non-terminal should still get a working tunnel, plus a warning naming the mode
// they actually got.
func TestListenExplicitInteractiveWithoutTTYWarnsAndFallsBack(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	sourceName := "test-explicit-" + generateTimestamp()

	_, stdout, stderr, done := startListenCapturingOutput(t, cli,
		"listen", "8080", sourceName, "--output", "interactive")

	t.Log("Waiting 6 seconds for listen to initialize...")
	time.Sleep(6 * time.Second)

	combined := stdout.String() + stderr.String()

	select {
	case err := <-done:
		t.Logf("STDOUT: %s", stdout.String())
		t.Logf("STDERR: %s", stderr.String())
		t.Fatalf("listen --output interactive exited early without a TTY: %v", err)
	default:
	}

	assert.NotContains(t, combined, "could not open a new TTY",
		"explicit interactive must still fall back rather than fail (#333)")
	assert.True(t,
		strings.Contains(combined, "falling back to --output compact"),
		"an explicit --output interactive on a non-terminal should warn; got:\n%s", combined)

	t.Logf("Verified explicit --output interactive warns and falls back")
}

// TestListenForwardsEventsWithoutTTY is the end-to-end test for #333, and the
// scenario that found it: start a local service, start a tunnel, send an event,
// check it arrives. Previously the tunnel died at startup with a /dev/tty error
// and `listen` exited 0, so from the outside it looked like nothing had been set
// up at all — no tunnel, no delivery, and no error to attribute it to.
//
// Asserting on the absence of an error message is not enough here: the point of
// the command is forwarding, so this asserts a real event reaches a real local
// server with no --output flag and no terminal.
func TestListenForwardsEventsWithoutTTY(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	timestamp := generateTimestamp()

	// A local service standing in for the user's app.
	received := make(chan string, 8)
	localServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		select {
		case received <- string(body):
		default:
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer localServer.Close()

	localURL, err := url.Parse(localServer.URL)
	require.NoError(t, err)
	port := localURL.Port()
	require.NotEmpty(t, port, "local test server should expose a port")

	// A source and a CLI connection for listen to attach to.
	sourceName := "test-fwd-" + timestamp
	var conn Connection
	require.NoError(t, cli.RunJSON(&conn,
		"gateway", "connection", "create",
		"--name", "test-fwd-conn-"+timestamp,
		"--source-name", sourceName,
		"--source-type", "WEBHOOK",
		"--destination-name", "test-fwd-dst-"+timestamp,
		"--destination-type", "CLI",
		"--destination-cli-path", "/",
	))
	require.NotEmpty(t, conn.ID)
	t.Cleanup(func() { deleteConnection(t, cli, conn.ID) })

	var src Source
	require.NoError(t, cli.RunJSON(&src, "gateway", "source", "get", conn.Source.ID))
	require.NotEmpty(t, src.URL, "source URL")

	// No --output: this is the default path an agent or CI job hits, and the one
	// that used to fail.
	_, stdout, stderr, done := startListenCapturingOutput(t, cli, "listen", port, sourceName)

	t.Log("Waiting 12 seconds for the tunnel to connect...")
	time.Sleep(12 * time.Second)

	select {
	case err := <-done:
		t.Logf("STDOUT: %s", stdout.String())
		t.Logf("STDERR: %s", stderr.String())
		t.Fatalf("listen exited before forwarding anything (#333): %v", err)
	default:
	}

	triggerTestEvent(t, src.URL)

	select {
	case body := <-received:
		t.Logf("Local server received: %s", body)
		assert.Contains(t, body, "test",
			"the forwarded body should be the payload that was sent")
	case <-time.After(45 * time.Second):
		t.Logf("STDOUT: %s", stdout.String())
		t.Logf("STDERR: %s", stderr.String())
		t.Fatal("no event reached the local server within 45s — the tunnel is not forwarding (#333)")
	}

	combined := stdout.String() + stderr.String()
	assert.NotContains(t, combined, "could not open a new TTY")
	assert.NotContains(t, combined, "Bubble Tea error")
}

// TestListenUsesHookdeckAPIKeyInsteadOfGuestAccount is the regression test for
// #334. With HOOKDECK_API_KEY set but no stored login, listen used to fall
// through to GuestLogin and create a throwaway account: events really did arrive
// locally, so it looked like it worked, but none of it was in the caller's
// project. HOOKDECK_API_KEY is a Project API key, which the CLI-auth endpoints
// reject, so the fix exchanges it for a CLI client key exactly as `hookdeck ci`
// does.
func TestListenUsesHookdeckAPIKeyInsteadOfGuestAccount(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	// An authenticated runner, used only to clean up what listen auto-creates.
	cleanupCLI := NewCLIRunner(t)

	projectRoot, err := filepath.Abs("../..")
	require.NoError(t, err, "Failed to get project root")

	binary := filepath.Join(projectRoot, "hookdeck-envkey-test-"+generateTimestamp())
	buildCmd := exec.Command("go", "build", "-o", binary, ".")
	buildCmd.Dir = projectRoot
	require.NoError(t, buildCmd.Run(), "failed to build CLI binary")
	t.Cleanup(func() { _ = os.Remove(binary) })

	// A config file with no credentials at all: the state that used to trigger
	// the guest fallback.
	emptyConfig := filepath.Join(t.TempDir(), "no-credentials.toml")
	require.NoError(t, os.WriteFile(emptyConfig, []byte(""), 0600))

	sourceName := "test-envkey-" + generateTimestamp()

	cmd := exec.Command(binary, "listen", "8080", sourceName)
	cmd.Dir = projectRoot
	cmd.Env = appendEnvOverride(
		appendEnvOverride(os.Environ(), "HOOKDECK_CONFIG_FILE", emptyConfig),
		"HOOKDECK_API_KEY", cleanupCLI.apiKey,
	)

	var stdout, stderr syncBuffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	require.NoError(t, cmd.Start(), "listen should start")
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	})

	t.Log("Waiting 10 seconds for listen to authenticate and connect...")
	time.Sleep(10 * time.Second)

	_ = cmd.Process.Kill()
	combined := stdout.String() + stderr.String()
	t.Logf("Output:\n%s", combined)

	// Clean up the source listen auto-created, and its CLI connection and destination.
	t.Cleanup(func() { cleanupListenResources(t, cleanupCLI, sourceName) })

	assert.NotContains(t, combined, "without a permanent account",
		"listen must not create a guest account when HOOKDECK_API_KEY is set (#334)")
	assert.NotContains(t, combined, "Creating a guest account",
		"listen must not create a guest account when HOOKDECK_API_KEY is set (#334)")

	assert.Contains(t, combined, "configured on project",
		"listen should report the project it authenticated to via HOOKDECK_API_KEY")

	// The exchanged CLI client key must be persisted, so later runs need no env var.
	configBytes, err := os.ReadFile(emptyConfig)
	require.NoError(t, err)
	assert.Contains(t, string(configBytes), "api_key",
		"the exchanged CLI client key should be saved, as `hookdeck ci` does")
	assert.Contains(t, string(configBytes), "project_id",
		"the project from HOOKDECK_API_KEY should be saved")
	assert.NotContains(t, string(configBytes), "console.hookdeck.com/e/",
		"a guest URL must not be present when authenticating via HOOKDECK_API_KEY")
}

// TestListenCommandBasic tests that the listen command starts without errors
// and can be terminated gracefully
func TestListenCommandBasic(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)

	// Generate unique source name
	timestamp := generateTimestamp()
	sourceName := "test-" + timestamp

	// Run a built binary rather than `go run`. `go run` starts the compiled
	// program as a child process, so killing it leaves that child alive holding
	// the inherited stdout and stderr — cmd.Wait then blocks until the orphan
	// exits, and the captured output is never readable. The #333 tests below
	// already take this route; these two predate it.
	cmd, stdout, stderr, done := startListenCapturingOutput(t, cli,
		"listen", "8080", sourceName, "--output", "compact")

	// Wait for the listen command to initialize
	t.Log("Waiting 5 seconds for listen command to initialize...")
	time.Sleep(5 * time.Second)

	select {
	case err := <-done:
		// Process exited early - this is a failure
		t.Fatalf("listen command exited early with error: %v\n--- stdout ---\n%s\n--- stderr ---\n%s",
			err, stdout.String(), stderr.String())
	case <-time.After(100 * time.Millisecond):
		// Process is still running - this is what we want
		t.Logf("Listen command successfully initialized and is running")
	}

	// Terminate the process
	require.NoError(t, cmd.Process.Kill(), "should be able to kill the listen process")

	// Wait for the process to exit (with timeout)
	select {
	case <-done:
		t.Logf("Listen command terminated successfully")
	case <-time.After(2 * time.Second):
		t.Fatal("Timed out waiting for listen command to terminate")
	}

	t.Logf("Successfully terminated listen command")
}

// TestListenCommandWithContext tests listen command with context cancellation
// This is a more Go-idiomatic approach
func TestListenCommandWithContext(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)

	// Generate unique source name
	timestamp := generateTimestamp()
	sourceName := "test-ctx-" + timestamp

	// A built binary rather than `go run` — see the note in
	// TestListenCommandBasic. Killing `go run` orphans the child holding the
	// output pipes, so the captured output is never readable.
	cmd, stdout, stderr, done := startListenCapturingOutput(t, cli,
		"listen", "8080", sourceName, "--output", "compact")

	// Wait for initialization
	time.Sleep(5 * time.Second)

	select {
	case err := <-done:
		t.Fatalf("listen command exited early with error: %v\n--- stdout ---\n%s\n--- stderr ---\n%s",
			err, stdout.String(), stderr.String())
	case <-time.After(100 * time.Millisecond):
		t.Logf("Listen command is running, now stopping it...")
	}

	// Stop the process. This test previously relied on an exec context
	// deadline; with a directly-executed binary a kill is the same signal
	// without the orphaned-child problem.
	require.NoError(t, cmd.Process.Kill(), "should be able to stop the listen process")

	// Wait for the command to finish
	select {
	case err := <-done:
		// We expect an error since we're canceling the context
		require.Error(t, err, "command should error when context is canceled")
		t.Logf("Listen command terminated via context cancellation")
	case <-time.After(2 * time.Second):
		t.Fatal("Timed out waiting for listen command to terminate after context cancellation")
	}

	t.Logf("Listen command terminated via context cancellation")
}

// TestListenPrefersEnvAPIKeyOverGuestProfile covers the follow-up to #334 raised
// in review: a persisted guest profile has a non-empty api_key, so a naive "no
// credentials stored" test treats the throwaway account as a real login. After a
// single guest run, later runs would then stay on the guest project even with a
// Project API key exported — the original bug, one run later.
//
// The guest profile here is synthetic (api_key plus guest_url, the shape
// GuestLogin persists) so the test is deterministic and does not create a real
// guest account on every CI run.
func TestListenPrefersEnvAPIKeyOverGuestProfile(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cleanupCLI := NewCLIRunner(t)

	projectRoot, err := filepath.Abs("../..")
	require.NoError(t, err)

	binary := filepath.Join(projectRoot, "hookdeck-guest-test-"+generateTimestamp())
	buildCmd := exec.Command("go", "build", "-o", binary, ".")
	buildCmd.Dir = projectRoot
	require.NoError(t, buildCmd.Run(), "failed to build CLI binary")
	t.Cleanup(func() { _ = os.Remove(binary) })

	guestConfig := filepath.Join(t.TempDir(), "guest-profile.toml")
	guestConfigBody := "profile = 'default'\n\n[default]\n" +
		"api_key = 'cli_guest_key_placeholder_000000'\n" +
		"guest_url = 'https://console.hookdeck.com/e/2v20grezcbj6yw'\n" +
		"project_id = 'tm_guest_placeholder'\n" +
		"project_mode = 'console'\n"
	require.NoError(t, os.WriteFile(guestConfig, []byte(guestConfigBody), 0600))

	sourceName := "test-guest-env-" + generateTimestamp()

	cmd := exec.Command(binary, "listen", "8080", sourceName)
	cmd.Dir = projectRoot
	cmd.Env = appendEnvOverride(
		appendEnvOverride(os.Environ(), "HOOKDECK_CONFIG_FILE", guestConfig),
		"HOOKDECK_API_KEY", cleanupCLI.apiKey,
	)

	var stdout, stderr syncBuffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	require.NoError(t, cmd.Start())
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	})

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	t.Log("Waiting 10 seconds for listen to authenticate...")
	time.Sleep(10 * time.Second)
	_ = cmd.Process.Kill()
	<-done

	combined := stdout.String() + stderr.String()
	t.Logf("Output:\n%s", combined)

	t.Cleanup(func() { cleanupListenResources(t, cleanupCLI, sourceName) })

	assert.Contains(t, combined, "configured on project",
		"HOOKDECK_API_KEY must take precedence over a stored guest profile (#334)")
	assert.Contains(t, combined, "replacing the temporary guest profile",
		"replacing a guest profile is a side effect and must be announced, not silent")

	// The guest credentials must be gone, replaced by the real project's.
	configBytes, err := os.ReadFile(guestConfig)
	require.NoError(t, err)
	assert.NotContains(t, string(configBytes), "cli_guest_key_placeholder_000000",
		"the guest key should have been replaced by the exchanged CLI client key")
	assert.NotContains(t, string(configBytes), "console.hookdeck.com/e/",
		"guest_url should be cleared once a real project is configured")
}
