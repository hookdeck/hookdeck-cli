package acceptance

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func init() {
	// Attempt to load .env file from test/acceptance/.env for local development
	// In CI, the environment variable will be set directly
	loadEnvFile()
}

// loadEnvFile loads environment variables from test/acceptance/.env if it exists
func loadEnvFile() {
	envPath := filepath.Join(".", ".env")
	file, err := os.Open(envPath)
	if err != nil {
		// .env file doesn't exist, which is fine (env var might be set directly)
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			// Only set if not already set
			if os.Getenv(key) == "" {
				os.Setenv(key, value)
			}
		}
	}
}

// CLIRunner provides utilities for running CLI commands in tests
type CLIRunner struct {
	t           *testing.T
	apiKey      string
	projectRoot string
}

// NewCLIRunner creates a new CLI runner for tests
// It requires HOOKDECK_CLI_TESTING_API_KEY environment variable to be set
func NewCLIRunner(t *testing.T) *CLIRunner {
	t.Helper()

	apiKey := os.Getenv("HOOKDECK_CLI_TESTING_API_KEY")
	require.NotEmpty(t, apiKey, "HOOKDECK_CLI_TESTING_API_KEY environment variable must be set")

	// Get and store the absolute project root path before any directory changes
	projectRoot, err := filepath.Abs("../..")
	require.NoError(t, err, "Failed to get project root path")

	runner := &CLIRunner{
		t:           t,
		apiKey:      apiKey,
		projectRoot: projectRoot,
	}

	// Authenticate in CI mode for tests
	stdout, stderr, err := runner.Run("ci", "--api-key", apiKey)
	require.NoError(t, err, "Failed to authenticate CLI: stdout=%s, stderr=%s", stdout, stderr)

	return runner
}

// Run executes the CLI with the given arguments and returns stdout, stderr, and error
// The CLI is executed via `go run main.go` from the project root
func (r *CLIRunner) Run(args ...string) (stdout, stderr string, err error) {
	r.t.Helper()

	// Use the stored project root path (set during NewCLIRunner)
	mainGoPath := filepath.Join(r.projectRoot, "main.go")

	// Build command: go run main.go [args...]
	cmdArgs := append([]string{"run", mainGoPath}, args...)
	cmd := exec.Command("go", cmdArgs...)

	// Set working directory to project root
	cmd.Dir = r.projectRoot

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err = cmd.Run()

	return stdoutBuf.String(), stderrBuf.String(), err
}

// RunExpectSuccess runs the CLI command and fails the test if it returns an error
// Returns only stdout for convenience
func (r *CLIRunner) RunExpectSuccess(args ...string) string {
	r.t.Helper()

	stdout, stderr, err := r.Run(args...)
	require.NoError(r.t, err, "CLI command failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)

	return stdout
}

// RunJSON runs the CLI command with --output json flag and unmarshals the result
// Automatically adds --output json to the arguments
func (r *CLIRunner) RunJSON(result interface{}, args ...string) error {
	r.t.Helper()

	// Append --output json to arguments
	argsWithJSON := append(args, "--output", "json")

	stdout, stderr, err := r.Run(argsWithJSON...)
	if err != nil {
		return fmt.Errorf("CLI command failed: %w\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}

	// Unmarshal JSON output
	if err := json.Unmarshal([]byte(stdout), result); err != nil {
		return fmt.Errorf("failed to unmarshal JSON output: %w\noutput: %s", err, stdout)
	}

	return nil
}

// generateTimestamp returns a timestamp string in the format YYYYMMDDHHMMSS plus microseconds
// This is used for creating unique test resource names
func generateTimestamp() string {
	now := time.Now()
	// Format: YYYYMMDDHHMMSS plus last 6 digits of Unix nano for uniqueness
	return fmt.Sprintf("%s%d", now.Format("20060102150405"), now.UnixNano()%1000000)
}

// Connection represents a Hookdeck connection for testing
type Connection struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Source      struct {
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"source"`
	Destination struct {
		Name   string      `json:"name"`
		Type   string      `json:"type"`
		Config interface{} `json:"config"`
	} `json:"destination"`
	Rules []map[string]interface{} `json:"rules"`
}

// createTestConnection creates a basic test connection and returns its ID
// The connection uses a WEBHOOK source and CLI destination
func createTestConnection(t *testing.T, cli *CLIRunner) string {
	t.Helper()

	timestamp := generateTimestamp()
	connName := fmt.Sprintf("test-conn-%s", timestamp)
	sourceName := fmt.Sprintf("test-src-%s", timestamp)
	destName := fmt.Sprintf("test-dst-%s", timestamp)

	var conn Connection
	err := cli.RunJSON(&conn,
		"connection", "create",
		"--name", connName,
		"--source-name", sourceName,
		"--source-type", "WEBHOOK",
		"--destination-name", destName,
		"--destination-type", "CLI",
		"--destination-cli-path", "/webhooks",
	)
	require.NoError(t, err, "Failed to create test connection")
	require.NotEmpty(t, conn.ID, "Connection ID should not be empty")

	t.Logf("Created test connection: %s (ID: %s)", connName, conn.ID)

	return conn.ID
}

// deleteConnection deletes a connection by ID using the --force flag
// This is safe to use in cleanup functions and won't prompt for confirmation
func deleteConnection(t *testing.T, cli *CLIRunner, id string) {
	t.Helper()

	stdout, stderr, err := cli.Run("connection", "delete", id, "--force")
	if err != nil {
		// Log but don't fail the test on cleanup errors
		t.Logf("Warning: Failed to delete connection %s: %v\nstdout: %s\nstderr: %s",
			id, err, stdout, stderr)
		return
	}

	t.Logf("Deleted connection: %s", id)
}

// cleanupConnections deletes multiple connections
// Useful for cleaning up test resources
func cleanupConnections(t *testing.T, cli *CLIRunner, ids []string) {
	t.Helper()

	for _, id := range ids {
		deleteConnection(t, cli, id)
	}
}

// assertContains checks if a string contains a substring
func assertContains(t *testing.T, s, substr, msgAndArgs string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Errorf("Expected string to contain %q but it didn't: %s\nActual: %s", substr, msgAndArgs, s)
	}
}
