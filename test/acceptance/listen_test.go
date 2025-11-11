package acceptance

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

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
