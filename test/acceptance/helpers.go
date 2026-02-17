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

// NewManualCLIRunner creates a CLI runner for manual tests that use human authentication.
// Unlike NewCLIRunner, this does NOT run `hookdeck ci` and relies on existing CLI credentials
// from `hookdeck login`.
func NewManualCLIRunner(t *testing.T) *CLIRunner {
	t.Helper()

	// Get and store the absolute project root path before any directory changes
	projectRoot, err := filepath.Abs("../..")
	require.NoError(t, err, "Failed to get project root path")

	runner := &CLIRunner{
		t:           t,
		apiKey:      "", // No API key - using CLI credentials from `hookdeck login`
		projectRoot: projectRoot,
	}

	// Do NOT run `hookdeck ci` - manual tests use credentials from `hookdeck login`

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

// RunFromCwd executes the CLI from the current working directory.
// This is useful for tests that need to test --local flag behavior,
// which creates config in the current directory.
//
// This builds a temporary binary in the project root, then runs it from
// the current working directory.
func (r *CLIRunner) RunFromCwd(args ...string) (stdout, stderr string, err error) {
	r.t.Helper()

	// Build a temporary binary
	tmpBinary := filepath.Join(r.projectRoot, "hookdeck-test-"+generateTimestamp())
	defer os.Remove(tmpBinary) // Clean up after

	// Build the binary in the project root
	buildCmd := exec.Command("go", "build", "-o", tmpBinary, ".")
	buildCmd.Dir = r.projectRoot
	if err := buildCmd.Run(); err != nil {
		return "", "", fmt.Errorf("failed to build CLI binary: %w", err)
	}

	// Run the binary from the current working directory
	cmd := exec.Command(tmpBinary, args...)
	// Don't set cmd.Dir - use current working directory

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	cmd.Stdin = os.Stdin // Allow interactive input

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
		ID   string `json:"id"`
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"source"`
	Destination struct {
		ID     string      `json:"id"`
		Name   string      `json:"name"`
		Type   string      `json:"type"`
		Config interface{} `json:"config"`
	} `json:"destination"`
	Rules []map[string]interface{} `json:"rules"`
}

// Source represents a Hookdeck source for testing
type Source struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// Destination represents a Hookdeck destination for testing
type Destination struct {
	ID     string      `json:"id"`
	Name   string      `json:"name"`
	Type   string      `json:"type"`
	Config interface{} `json:"config"`
}

// Transformation represents a Hookdeck transformation for testing
type Transformation struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Code string `json:"code"`
}

// Event represents a Hookdeck event for testing
type Event struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// Request represents a Hookdeck request for testing
type Request struct {
	ID string `json:"id"`
}

// Attempt represents a Hookdeck attempt for testing
type Attempt struct {
	ID string `json:"id"`
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
		"gateway", "connection", "create",
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

	stdout, stderr, err := cli.Run("gateway", "connection", "delete", id, "--force")
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

// createTestSource creates a WEBHOOK source and returns its ID
func createTestSource(t *testing.T, cli *CLIRunner) string {
	t.Helper()

	timestamp := generateTimestamp()
	name := fmt.Sprintf("test-src-%s", timestamp)

	var src Source
	err := cli.RunJSON(&src,
		"gateway", "source", "create",
		"--name", name,
		"--type", "WEBHOOK",
	)
	require.NoError(t, err, "Failed to create test source")
	require.NotEmpty(t, src.ID, "Source ID should not be empty")

	t.Logf("Created test source: %s (ID: %s)", name, src.ID)
	return src.ID
}

// deleteSource deletes a source by ID using the --force flag
func deleteSource(t *testing.T, cli *CLIRunner, id string) {
	t.Helper()

	stdout, stderr, err := cli.Run("gateway", "source", "delete", id, "--force")
	if err != nil {
		t.Logf("Warning: Failed to delete source %s: %v\nstdout: %s\nstderr: %s",
			id, err, stdout, stderr)
		return
	}
	t.Logf("Deleted source: %s", id)
}

// createTestDestination creates an HTTP destination with a test URL and returns its ID
func createTestDestination(t *testing.T, cli *CLIRunner) string {
	t.Helper()

	timestamp := generateTimestamp()
	name := fmt.Sprintf("test-dst-%s", timestamp)

	var dst Destination
	err := cli.RunJSON(&dst,
		"gateway", "destination", "create",
		"--name", name,
		"--type", "HTTP",
		"--url", "https://example.com/webhooks",
	)
	require.NoError(t, err, "Failed to create test destination")
	require.NotEmpty(t, dst.ID, "Destination ID should not be empty")

	t.Logf("Created test destination: %s (ID: %s)", name, dst.ID)
	return dst.ID
}

// deleteDestination deletes a destination by ID using the --force flag
func deleteDestination(t *testing.T, cli *CLIRunner, id string) {
	t.Helper()

	stdout, stderr, err := cli.Run("gateway", "destination", "delete", id, "--force")
	if err != nil {
		t.Logf("Warning: Failed to delete destination %s: %v\nstdout: %s\nstderr: %s",
			id, err, stdout, stderr)
		return
	}
	t.Logf("Deleted destination: %s", id)
}

// createTestTransformation creates a transformation with minimal code and returns its ID
func createTestTransformation(t *testing.T, cli *CLIRunner) string {
	t.Helper()

	timestamp := generateTimestamp()
	name := fmt.Sprintf("test-trn-%s", timestamp)
	code := "module.exports = async (req) => req;"

	var trn Transformation
	err := cli.RunJSON(&trn,
		"gateway", "transformation", "create",
		"--name", name,
		"--code", code,
	)
	require.NoError(t, err, "Failed to create test transformation")
	require.NotEmpty(t, trn.ID, "Transformation ID should not be empty")

	t.Logf("Created test transformation: %s (ID: %s)", name, trn.ID)
	return trn.ID
}

// deleteTransformation deletes a transformation by ID using the --force flag
func deleteTransformation(t *testing.T, cli *CLIRunner, id string) {
	t.Helper()

	stdout, stderr, err := cli.Run("gateway", "transformation", "delete", id, "--force")
	if err != nil {
		t.Logf("Warning: Failed to delete transformation %s: %v\nstdout: %s\nstderr: %s",
			id, err, stdout, stderr)
		return
	}
	t.Logf("Deleted transformation: %s", id)
}

// assertContains checks if a string contains a substring
func assertContains(t *testing.T, s, substr, msgAndArgs string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Errorf("Expected string to contain %q but it didn't: %s\nActual: %s", substr, msgAndArgs, s)
	}
}

// RequireCLIAuthentication forces fresh CLI authentication for tests that need human interaction.
// This helper:
// 1. Clears any existing authentication
// 2. Runs `hookdeck login` for the user
// 3. Prompts user to complete browser authentication
// 4. Waits for user confirmation (Enter key)
// 5. Verifies authentication succeeded via `hookdeck whoami`
// 6. Fails the test if authentication doesn't succeed
//
// This ensures tests always run with fresh human-interactive CLI login.
func RequireCLIAuthentication(t *testing.T) string {
	t.Helper()

	// Get project root for running commands
	projectRoot, err := filepath.Abs("../..")
	require.NoError(t, err, "Failed to get project root path")

	mainGoPath := filepath.Join(projectRoot, "main.go")

	fmt.Println("\n🔐 Fresh Authentication Required")
	fmt.Println("=================================")
	fmt.Println("These tests require fresh CLI authentication with project access.")
	fmt.Println()

	// Step 1: Clear existing authentication
	fmt.Println("Step 1: Clearing existing authentication...")

	// Run logout command to clear authentication
	logoutCmd := exec.Command("go", "run", mainGoPath, "logout")
	logoutCmd.Dir = projectRoot
	logoutCmd.Stdout = os.Stdout
	logoutCmd.Stderr = os.Stderr
	_ = logoutCmd.Run() // Ignore errors - logout might fail if not logged in

	// Also delete config file directly to ensure clean state
	homeDir, err := os.UserHomeDir()
	if err == nil {
		configPath := filepath.Join(homeDir, ".config", "hookdeck", "config.toml")
		_ = os.Remove(configPath) // Ignore errors - file might not exist
	}

	fmt.Println("✅ Authentication cleared")
	fmt.Println()

	// Step 2: Start login process
	fmt.Println("Step 2: Starting login process...")
	fmt.Println()
	fmt.Println("Running: hookdeck login")
	fmt.Println("(The login command will prompt you to press Enter before opening the browser)")
	fmt.Println()

	// Open /dev/tty directly to ensure we can read user input even when stdin is redirected by go test
	tty, err := os.Open("/dev/tty")
	require.NoError(t, err, "Failed to open /dev/tty - tests must be run in an interactive terminal")
	defer tty.Close()

	// Run login command interactively - user will see project selection
	// CRITICAL: Connect directly to /dev/tty for full interactivity
	loginCmd := exec.Command("go", "run", mainGoPath, "login")
	loginCmd.Dir = projectRoot
	loginCmd.Stdout = os.Stdout
	loginCmd.Stderr = os.Stderr
	loginCmd.Stdin = tty // Use the actual terminal device, not os.Stdin

	// Run the command and let it inherit the terminal completely
	err = loginCmd.Run()
	require.NoError(t, err, "Login command failed - please ensure you completed browser authentication and project selection")

	fmt.Println()

	// Step 3: Verify authentication
	fmt.Println("Verifying authentication...")

	whoamiCmd := exec.Command("go", "run", mainGoPath, "whoami")
	whoamiCmd.Dir = projectRoot
	var whoamiOut bytes.Buffer
	whoamiCmd.Stdout = &whoamiOut
	whoamiCmd.Stderr = &whoamiOut

	err = whoamiCmd.Run()
	require.NoError(t, err, "Authentication verification failed. Please ensure you completed the login process.\nOutput: %s", whoamiOut.String())

	whoamiOutput := whoamiOut.String()
	require.Contains(t, whoamiOutput, "Logged in as", "whoami output should contain 'Logged in as'")

	// Extract and display user info from whoami output
	lines := strings.Split(whoamiOutput, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Logged in as") {
			fmt.Printf("✅ Authenticated successfully: %s\n", strings.TrimSpace(line))
			break
		}
	}
	fmt.Println()

	// Step 4: Let user select project to use for testing (safety measure)
	fmt.Println("⚠️  Project Selection for Testing")
	fmt.Println("====================================")
	fmt.Println("To avoid accidentally running tests against a production project,")
	fmt.Println("please select which project to use for these tests.")
	fmt.Println()
	fmt.Println("Running: hookdeck project use")
	fmt.Println()

	// Run project use interactively to let user select test project
	projectUseCmd := exec.Command("go", "run", mainGoPath, "project", "use")
	projectUseCmd.Dir = projectRoot
	projectUseCmd.Stdout = os.Stdout
	projectUseCmd.Stderr = os.Stderr
	projectUseCmd.Stdin = tty

	err = projectUseCmd.Run()
	require.NoError(t, err, "Failed to select project")

	fmt.Println()

	// Get the selected project via whoami again
	whoamiCmd2 := exec.Command("go", "run", mainGoPath, "whoami")
	whoamiCmd2.Dir = projectRoot
	var whoamiOut2 bytes.Buffer
	whoamiCmd2.Stdout = &whoamiOut2
	whoamiCmd2.Stderr = &whoamiOut2

	err = whoamiCmd2.Run()
	require.NoError(t, err, "Failed to verify project selection")

	selectedWhoami := whoamiOut2.String()
	fmt.Println("✅ Tests will run against:")
	for _, line := range strings.Split(selectedWhoami, "\n") {
		if strings.Contains(line, "on project") {
			fmt.Printf("   %s\n", strings.TrimSpace(line))
			break
		}
	}
	fmt.Println()

	// Return the final whoami output so tests can parse org/project if needed
	return selectedWhoami
}

// ParseOrgAndProjectFromWhoami extracts organization and project names from whoami output.
// Expected format: "Logged in as ... on project PROJECT_NAME in organization ORG_NAME"
func ParseOrgAndProjectFromWhoami(t *testing.T, whoamiOutput string) (org, project string) {
	t.Helper()

	lines := strings.Split(whoamiOutput, "\n")
	for _, line := range lines {
		if strings.Contains(line, "on project") && strings.Contains(line, "in organization") {
			// Extract project name (between "on project " and " in organization")
			projectStart := strings.Index(line, "on project ") + len("on project ")
			projectEnd := strings.Index(line, " in organization")
			if projectStart > 0 && projectEnd > projectStart {
				project = strings.TrimSpace(line[projectStart:projectEnd])
			}

			// Extract org name (after "in organization ")
			orgStart := strings.Index(line, "in organization ") + len("in organization ")
			if orgStart > 0 && orgStart < len(line) {
				org = strings.TrimSpace(line[orgStart:])
			}

			break
		}
	}

	require.NotEmpty(t, org, "Failed to parse organization from whoami output: %s", whoamiOutput)
	require.NotEmpty(t, project, "Failed to parse project from whoami output: %s", whoamiOutput)

	return org, project
}

// GetCurrentOrgAndProject returns the current organization and project from whoami.
// This is useful for tests that need to know which project they're working with.
func GetCurrentOrgAndProject(t *testing.T) (org, project string) {
	t.Helper()

	// Get project root for running commands
	projectRoot, err := filepath.Abs("../..")
	require.NoError(t, err, "Failed to get project root path")

	mainGoPath := filepath.Join(projectRoot, "main.go")

	whoamiCmd := exec.Command("go", "run", mainGoPath, "whoami")
	whoamiCmd.Dir = projectRoot
	var whoamiOut bytes.Buffer
	whoamiCmd.Stdout = &whoamiOut
	whoamiCmd.Stderr = &whoamiOut

	err = whoamiCmd.Run()
	require.NoError(t, err, "Failed to run whoami: %s", whoamiOut.String())

	return ParseOrgAndProjectFromWhoami(t, whoamiOut.String())
}

// RequireCLIAuthenticationOnce calls RequireCLIAuthentication only once per test run.
// Use this when running multiple manual tests to avoid repeated authentication.
var authenticationDone = false
var cachedWhoamiOutput string

func RequireCLIAuthenticationOnce(t *testing.T) string {
	t.Helper()

	if !authenticationDone {
		cachedWhoamiOutput = RequireCLIAuthentication(t)
		authenticationDone = true
	} else {
		fmt.Println("✅ Already authenticated (from previous test)")
		fmt.Println()
	}

	return cachedWhoamiOutput
}
