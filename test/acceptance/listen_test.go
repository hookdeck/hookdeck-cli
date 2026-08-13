//go:build listen

package acceptance

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// startListenCapturingOutput starts `hookdeck listen` with the given extra args,
// stdout and stderr wired to pipes (i.e. not a terminal — the condition that made
// #333 fail), and returns the process plus buffers. The caller kills the process.
func startListenCapturingOutput(t *testing.T, cli *CLIRunner, extraArgs ...string) (*exec.Cmd, *bytes.Buffer, *bytes.Buffer, chan error) {
	t.Helper()

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

	var stdout, stderr bytes.Buffer
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

	var stdout, stderr bytes.Buffer
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

	// Clean up the source listen auto-created, and its CLI destination.
	t.Cleanup(func() {
		var sources SourceListResponseForCleanup
		if err := cleanupCLI.RunJSON(&sources, "gateway", "source", "list"); err != nil {
			t.Logf("cleanup: could not list sources: %v", err)
			return
		}
		for _, s := range sources.Models {
			if s.Name == sourceName {
				_, _, _ = cleanupCLI.Run("gateway", "source", "delete", s.ID, "--force")
			}
		}
	})

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

// SourceListResponseForCleanup is a minimal shape for the cleanup step above.
type SourceListResponseForCleanup struct {
	Models []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"models"`
}

// TestListenCommandBasic tests that the listen command starts without errors
// and can be terminated gracefully
func TestListenCommandBasic(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	// Ensure we're authenticated (NewCLIRunner handles this)
	_ = NewCLIRunner(t)

	// Generate unique source name
	timestamp := generateTimestamp()
	sourceName := "test-" + timestamp

	// Get the absolute path to the project root
	projectRoot, err := filepath.Abs("../..")
	require.NoError(t, err, "Failed to get project root")

	mainGoPath := filepath.Join(projectRoot, "main.go")

	// Build the listen command
	// We use exec.Command directly here instead of CLIRunner.Run because we need
	// to start the process in the background and then kill it
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "run", mainGoPath,
		"listen", "8080", sourceName, "--output", "compact")
	cmd.Dir = projectRoot

	// Start the command in the background
	err = cmd.Start()
	require.NoError(t, err, "listen command should start without error")

	t.Logf("Started listen command with PID %d", cmd.Process.Pid)

	// Register cleanup to ensure process is killed even if test fails
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	})

	// Wait for the listen command to initialize
	t.Log("Waiting 5 seconds for listen command to initialize...")
	time.Sleep(5 * time.Second)

	// Check if the command has exited early (which would be an error)
	// We'll use a non-blocking channel to check if Wait() returns immediately
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		// Process exited early - this is a failure
		t.Fatalf("listen command exited early with error: %v", err)
	case <-time.After(100 * time.Millisecond):
		// Process is still running - this is what we want
		t.Logf("Listen command successfully initialized and is running")
	}

	// Terminate the process
	err = cmd.Process.Kill()
	require.NoError(t, err, "should be able to kill the listen process")

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

	// Ensure we're authenticated (NewCLIRunner handles this)
	_ = NewCLIRunner(t)

	// Generate unique source name
	timestamp := generateTimestamp()
	sourceName := "test-ctx-" + timestamp

	// Get the absolute path to the project root
	projectRoot, err := filepath.Abs("../..")
	require.NoError(t, err, "Failed to get project root")

	mainGoPath := filepath.Join(projectRoot, "main.go")

	// Create a context with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	// Build the listen command with context
	cmd := exec.CommandContext(ctx, "go", "run", mainGoPath,
		"listen", "8080", sourceName, "--output", "compact")
	cmd.Dir = projectRoot

	// Start the command
	err = cmd.Start()
	require.NoError(t, err, "listen command should start without error")

	t.Logf("Started listen command with PID %d (will auto-cancel after 8s)", cmd.Process.Pid)

	// Register cleanup
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	})

	// Wait for initialization
	time.Sleep(5 * time.Second)

	// Check if the command has exited early
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		t.Fatalf("listen command exited early with error: %v", err)
	case <-time.After(100 * time.Millisecond):
		t.Logf("Listen command is running, now canceling context...")
	}

	// Cancel the context (this will kill the process)
	cancel()

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
