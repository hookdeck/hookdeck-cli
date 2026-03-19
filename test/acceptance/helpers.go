package acceptance

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
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

// defaultAPIUpstream is the real Hookdeck API base URL used by the recording proxy.
const defaultAPIUpstream = "https://api.hookdeck.com"

// RecordedRequest holds a single HTTP request as captured by the recording proxy.
type RecordedRequest struct {
	Method    string
	Path      string
	Telemetry string
}

// RecordingProxy is an HTTP server that forwards requests to the real API and
// records each request (including X-Hookdeck-CLI-Telemetry). Use it to run the
// CLI with --api-base pointing at the proxy so all API traffic is captured
// while still hitting the real backend.
type RecordingProxy struct {
	t        *testing.T
	server   *httptest.Server
	upstream string
	mu       sync.Mutex
	recorded []RecordedRequest
}

// URL returns the proxy base URL (e.g. http://127.0.0.1:port). Pass this to
// the CLI as --api-base so requests go through the proxy.
func (p *RecordingProxy) URL() string {
	return p.server.URL
}

// Recorded returns a copy of the slice of recorded requests. Safe to call
// after the CLI command has finished.
func (p *RecordingProxy) Recorded() []RecordedRequest {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]RecordedRequest, len(p.recorded))
	copy(out, p.recorded)
	return out
}

// Close shuts down the proxy server.
func (p *RecordingProxy) Close() {
	p.server.Close()
}

// StartRecordingProxy starts an httptest.Server that acts as a reverse proxy to
// upstreamBase (e.g. https://api.hookdeck.com). Every request is recorded
// (method, path, X-Hookdeck-CLI-Telemetry) and then forwarded to the upstream;
// the upstream response is returned to the client. Use with CLIRunner.Run("--api-base", proxy.URL(), "gateway", ...).
func StartRecordingProxy(t *testing.T, upstreamBase string) *RecordingProxy {
	t.Helper()
	upstream, err := url.Parse(strings.TrimSuffix(upstreamBase, "/"))
	require.NoError(t, err, "parse upstream URL")

	p := &RecordingProxy{
		t:        t,
		upstream: upstream.String(),
		recorded: make([]RecordedRequest, 0),
	}

	p.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Record before forwarding
		p.mu.Lock()
		p.recorded = append(p.recorded, RecordedRequest{
			Method:    r.Method,
			Path:      r.URL.Path,
			Telemetry: r.Header.Get("X-Hookdeck-CLI-Telemetry"),
		})
		p.mu.Unlock()

		// Build upstream request: same method, path, query, body
		targetPath := r.URL.Path
		if r.URL.RawQuery != "" {
			targetPath += "?" + r.URL.RawQuery
		}
		dest, err := url.Parse(upstream.String() + targetPath)
		require.NoError(p.t, err)

		var bodyReader io.Reader
		if r.Body != nil {
			bodyReader = r.Body
		}

		req, err := http.NewRequest(r.Method, dest.String(), bodyReader)
		require.NoError(p.t, err)

		// Copy headers that the API cares about
		for _, k := range []string{
			"Authorization", "Content-Type", "X-Team-ID", "X-Project-ID",
			"X-Hookdeck-CLI-Telemetry", "X-Hookdeck-Client-User-Agent", "User-Agent",
		} {
			if v := r.Header.Get(k); v != "" {
				req.Header.Set(k, v)
			}
		}

		resp, err := http.DefaultClient.Do(req)
		require.NoError(p.t, err)
		defer resp.Body.Close()

		// Copy response back
		for k, v := range resp.Header {
			for _, vv := range v {
				w.Header().Add(k, vv)
			}
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	}))

	return p
}

// telemetryPayload is the structure of the X-Hookdeck-CLI-Telemetry header (JSON).
type telemetryPayload struct {
	CommandPath  string `json:"command_path"`
	InvocationID string `json:"invocation_id"`
}

// AssertTelemetryConsistent checks that every recorded request that has a
// telemetry header shares the same invocation_id and command_path, and that
// command_path equals expectedCommandPath.
func AssertTelemetryConsistent(t *testing.T, recorded []RecordedRequest, expectedCommandPath string) {
	t.Helper()
	var invocationID, commandPath string
	for i, r := range recorded {
		if r.Telemetry == "" {
			continue
		}
		var p telemetryPayload
		require.NoError(t, json.Unmarshal([]byte(r.Telemetry), &p), "request %d: invalid telemetry JSON: %s", i, r.Telemetry)
		if invocationID == "" {
			invocationID = p.InvocationID
			commandPath = p.CommandPath
		}
		require.Equal(t, invocationID, p.InvocationID, "request %d (%s %s): invocation_id should be consistent", i, r.Method, r.Path)
		require.Equal(t, commandPath, p.CommandPath, "request %d (%s %s): command_path should be consistent", i, r.Method, r.Path)
	}
	if invocationID == "" && len(recorded) > 0 {
		t.Fatalf("telemetry: %d HTTP request(s) recorded but X-Hookdeck-CLI-Telemetry was empty on every one (unset HOOKDECK_CLI_TELEMETRY_DISABLED / config telemetry_disabled for these tests)", len(recorded))
	}
	require.Equal(t, expectedCommandPath, commandPath, "command_path should match expected")
	require.NotEmpty(t, invocationID, "at least one request should have invocation_id")
}

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
	configPath  string // when set (ACCEPTANCE_SLICE), HOOKDECK_CONFIG_FILE is set so each slice uses its own config file
}

// NewCLIRunner creates a new CLI runner for tests
// It requires HOOKDECK_CLI_TESTING_API_KEY (and optionally HOOKDECK_CLI_TESTING_API_KEY_2 for slice 1, HOOKDECK_CLI_TESTING_API_KEY_3 for slice 2) to be set
func NewCLIRunner(t *testing.T) *CLIRunner {
	t.Helper()

	apiKey := getAcceptanceAPIKey(t)
	require.NotEmpty(t, apiKey, "HOOKDECK_CLI_TESTING_API_KEY (or HOOKDECK_CLI_TESTING_API_KEY_2 for slice 1, HOOKDECK_CLI_TESTING_API_KEY_3 for slice 2) must be set")

	// Get and store the absolute project root path before any directory changes
	projectRoot, err := filepath.Abs("../..")
	require.NoError(t, err, "Failed to get project root path")

	runner := &CLIRunner{
		t:           t,
		apiKey:      apiKey,
		projectRoot: projectRoot,
		configPath:  getAcceptanceConfigPath(),
	}

	// Authenticate in CI mode for tests
	stdout, stderr, err := runner.Run("ci", "--api-key", apiKey)
	require.NoError(t, err, "Failed to authenticate CLI: stdout=%s, stderr=%s", stdout, stderr)

	return runner
}

// NewCLIRunnerWithConfigPath creates a CLI runner that uses the given config file path.
// It runs "ci --api-key" so the config is populated (api_key, project_id, project_mode, etc.).
// Use this when a test needs a known config path (e.g. to write a minimal variant for another run).
func NewCLIRunnerWithConfigPath(t *testing.T, configPath string) *CLIRunner {
	t.Helper()
	apiKey := getAcceptanceAPIKey(t)
	require.NotEmpty(t, apiKey, "HOOKDECK_CLI_TESTING_API_KEY must be set")
	projectRoot, err := filepath.Abs("../..")
	require.NoError(t, err, "Failed to get project root path")
	runner := &CLIRunner{
		t:           t,
		apiKey:      apiKey,
		projectRoot: projectRoot,
		configPath:  configPath,
	}
	stdout, stderr, err := runner.Run("ci", "--api-key", apiKey)
	require.NoError(t, err, "Failed to authenticate CLI with config at %s: stdout=%s stderr=%s", configPath, stdout, stderr)
	return runner
}

// NewCLIRunnerWithConfigPathNoCI creates a CLI runner that uses the given config file path,
// without running "ci". Use when the config file is already populated (e.g. a minimal config
// written by the test). The runner will pass HOOKDECK_CONFIG_FILE to all Run() calls.
func NewCLIRunnerWithConfigPathNoCI(t *testing.T, configPath string) *CLIRunner {
	t.Helper()
	apiKey := getAcceptanceAPIKey(t)
	require.NotEmpty(t, apiKey, "HOOKDECK_CLI_TESTING_API_KEY must be set")
	projectRoot, err := filepath.Abs("../..")
	require.NoError(t, err, "Failed to get project root path")
	return &CLIRunner{
		t:           t,
		apiKey:      apiKey,
		projectRoot: projectRoot,
		configPath:  configPath,
	}
}

// getAcceptanceAPIKey returns the API key for the current acceptance slice.
// When ACCEPTANCE_SLICE=1 and HOOKDECK_CLI_TESTING_API_KEY_2 is set, use it; when ACCEPTANCE_SLICE=2 and HOOKDECK_CLI_TESTING_API_KEY_3 is set, use that; else HOOKDECK_CLI_TESTING_API_KEY.
func getAcceptanceAPIKey(t *testing.T) string {
	t.Helper()
	switch os.Getenv("ACCEPTANCE_SLICE") {
	case "1":
		if k := os.Getenv("HOOKDECK_CLI_TESTING_API_KEY_2"); k != "" {
			return k
		}
	case "2":
		if k := os.Getenv("HOOKDECK_CLI_TESTING_API_KEY_3"); k != "" {
			return k
		}
	}
	return os.Getenv("HOOKDECK_CLI_TESTING_API_KEY")
}

// NewCLIRunnerWithKey creates a new CLI runner authenticated with the given CLI key via
// hookdeck login --api-key. Used only for project list/use tests (HOOKDECK_CLI_TESTING_CLI_KEY);
// API and CI keys cannot list or switch projects, so those tests require a CLI key and login auth.
func NewCLIRunnerWithKey(t *testing.T, apiKey string) *CLIRunner {
	t.Helper()
	require.NotEmpty(t, apiKey, "api key must be non-empty for NewCLIRunnerWithKey")

	projectRoot, err := filepath.Abs("../..")
	require.NoError(t, err, "Failed to get project root path")

	runner := &CLIRunner{
		t:           t,
		apiKey:      apiKey,
		projectRoot: projectRoot,
		configPath:  getAcceptanceConfigPath(),
	}

	stdout, stderr, err := runner.Run("login", "--api-key", apiKey)
	require.NoError(t, err, "Failed to authenticate CLI (login --api-key): stdout=%s, stderr=%s", stdout, stderr)

	return runner
}

// getAcceptanceConfigPath returns a per-slice config path when ACCEPTANCE_SLICE is set,
// so parallel runs do not overwrite the same config file. Empty when not in sliced mode.
func getAcceptanceConfigPath() string {
	slice := os.Getenv("ACCEPTANCE_SLICE")
	if slice == "" {
		return ""
	}
	return filepath.Join(os.TempDir(), "hookdeck-acceptance-slice"+slice+"-config.toml")
}

// appendEnvOverride returns a copy of env with key=value set, replacing any existing key.
func appendEnvOverride(env []string, key, value string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env)+1)
	for _, e := range env {
		if !strings.HasPrefix(e, prefix) {
			out = append(out, e)
		}
	}
	out = append(out, prefix+value)
	return out
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
// The CLI is executed via `go run main.go` from the project root.
// When configPath is set (parallel slice mode), HOOKDECK_CONFIG_FILE env is set so each slice uses its own config file.
func (r *CLIRunner) Run(args ...string) (stdout, stderr string, err error) {
	r.t.Helper()

	// Use the stored project root path (set during NewCLIRunner)
	mainGoPath := filepath.Join(r.projectRoot, "main.go")

	cmdArgs := append([]string{"run", mainGoPath}, args...)
	cmd := exec.Command("go", cmdArgs...)

	// Set working directory to project root
	cmd.Dir = r.projectRoot

	if r.configPath != "" {
		cmd.Env = appendEnvOverride(os.Environ(), "HOOKDECK_CONFIG_FILE", r.configPath)
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err = cmd.Run()

	return stdoutBuf.String(), stderrBuf.String(), err
}

// RunWithEnv is like Run but merges extraEnv into the process environment (e.g. for HOOKDECK_CLI_USE_SYSTEM_BINARY).
// When extraEnv["HOOKDECK_CLI_USE_SYSTEM_BINARY"] == "1", runs the installed "hookdeck" binary on PATH instead of "go run main.go"
// (e.g. to run tests against 2.0.0 or another installed version).
func (r *CLIRunner) RunWithEnv(extraEnv map[string]string, args ...string) (stdout, stderr string, err error) {
	r.t.Helper()

	env := os.Environ()
	if r.configPath != "" {
		env = appendEnvOverride(env, "HOOKDECK_CONFIG_FILE", r.configPath)
	}
	for k, v := range extraEnv {
		env = appendEnvOverride(env, k, v)
	}

	var cmd *exec.Cmd
	if extraEnv != nil && extraEnv["HOOKDECK_CLI_USE_SYSTEM_BINARY"] == "1" {
		hookdeckPath, lookErr := exec.LookPath("hookdeck")
		if lookErr != nil {
			return "", "", fmt.Errorf("HOOKDECK_CLI_USE_SYSTEM_BINARY=1 but hookdeck not on PATH: %w", lookErr)
		}
		cmd = exec.Command(hookdeckPath, args...)
		cmd.Dir = r.projectRoot
	} else {
		mainGoPath := filepath.Join(r.projectRoot, "main.go")
		cmdArgs := append([]string{"run", mainGoPath}, args...)
		cmd = exec.Command("go", cmdArgs...)
		cmd.Dir = r.projectRoot
	}
	cmd.Env = env

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	err = cmd.Run()
	return stdoutBuf.String(), stderrBuf.String(), err
}

// RunListenWithTimeout starts the CLI with the given args (e.g. "--api-base", proxyURL,
// "listen", port, sourceName, "--output", "compact"), lets it run for runDuration, then
// kills the process. Uses a pre-built binary so we terminate the actual listen process
// (not a "go run" parent). Returns stdout, stderr, and the error from Wait (often
// non-nil because the process was killed). Uses the same project root and config env as Run().
func (r *CLIRunner) RunListenWithTimeout(args []string, runDuration time.Duration) (stdout, stderr string, err error) {
	r.t.Helper()
	tmpBinary := filepath.Join(r.projectRoot, "hookdeck-listen-test-"+generateTimestamp())
	defer os.Remove(tmpBinary)

	buildCmd := exec.Command("go", "build", "-o", tmpBinary, ".")
	buildCmd.Dir = r.projectRoot
	if err := buildCmd.Run(); err != nil {
		return "", "", fmt.Errorf("build CLI for listen test: %w", err)
	}

	cmd := exec.Command(tmpBinary, args...)
	cmd.Dir = r.projectRoot
	if r.configPath != "" {
		cmd.Env = appendEnvOverride(os.Environ(), "HOOKDECK_CONFIG_FILE", r.configPath)
	}
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	if err := cmd.Start(); err != nil {
		return "", "", err
	}
	timer := time.AfterFunc(runDuration, func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	})
	defer timer.Stop()
	waitErr := cmd.Wait()
	return stdoutBuf.String(), stderrBuf.String(), waitErr
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

	if r.configPath != "" {
		cmd.Env = appendEnvOverride(os.Environ(), "HOOKDECK_CONFIG_FILE", r.configPath)
	}

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

// assertFilterRuleFieldMatches asserts that a filter rule field (body or headers) matches the expected JSON.
// The API may return the field as either a string or a parsed map; both are accepted.
func assertFilterRuleFieldMatches(t *testing.T, actual interface{}, expectedJSON string, fieldName string) {
	t.Helper()
	var expectedMap map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(expectedJSON), &expectedMap), "expectedJSON should be valid JSON")
	var actualMap map[string]interface{}
	switch v := actual.(type) {
	case string:
		require.NoError(t, json.Unmarshal([]byte(v), &actualMap), "actual string should be valid JSON")
	case map[string]interface{}:
		actualMap = v
	default:
		t.Fatalf("%s should be string or map, got %T", fieldName, actual)
	}
	assert.Equal(t, expectedMap, actualMap, "%s should match expected JSON", fieldName)
}

// assertResponseStatusCodesMatch asserts that response_status_codes from the API match expected values.
// The API may return codes as strings or numbers; both are accepted.
func assertResponseStatusCodesMatch(t *testing.T, statusCodes interface{}, expected ...string) {
	t.Helper()
	slice, ok := statusCodes.([]interface{})
	require.True(t, ok, "response_status_codes should be an array, got %T", statusCodes)
	require.Len(t, slice, len(expected), "response_status_codes length")
	for i, exp := range expected {
		var actual string
		switch v := slice[i].(type) {
		case string:
			actual = v
		case float64:
			actual = fmt.Sprintf("%.0f", v)
		case int:
			actual = fmt.Sprintf("%d", v)
		default:
			actual = fmt.Sprintf("%v", slice[i])
		}
		assert.Equal(t, exp, actual, "response_status_codes[%d]", i)
	}
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
	URL  string `json:"url"`
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
	ID        string `json:"id"`
	Status    string `json:"status"`
	WebhookID string `json:"webhook_id"`
}

// Request represents a Hookdeck request for testing
type Request struct {
	ID string `json:"id"`
}

// Attempt represents a Hookdeck attempt for testing
type Attempt struct {
	ID           string `json:"id"`
	EventID      string `json:"event_id"`
	AttemptNumber int   `json:"attempt_number"`
	Status       string `json:"status"`
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

// createTestConnectionWithMockDestination creates a test connection with a MOCK_API destination.
// Events and attempts are generated by the backend without needing a live CLI. Use this for
// inspection tests (event/request/attempt) that need to trigger and then list/get events.
func createTestConnectionWithMockDestination(t *testing.T, cli *CLIRunner) string {
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
		"--destination-type", "MOCK_API",
	)
	require.NoError(t, err, "Failed to create test connection with mock destination")
	require.NotEmpty(t, conn.ID, "Connection ID should not be empty")

	t.Logf("Created test connection (mock dest): %s (ID: %s)", connName, conn.ID)

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
	code := `addHandler("transform", (request, context) => { return request; });`

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

// triggerTestEvent sends a POST request to the given source URL to create a request and event.
// Use after creating a connection; then list events with --connection-id to find the new event.
func triggerTestEvent(t *testing.T, sourceURL string) {
	t.Helper()

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(sourceURL, "application/json", strings.NewReader(`{"test":true}`))
	require.NoError(t, err, "POST to source URL failed")
	defer resp.Body.Close()
	require.True(t, resp.StatusCode >= 200 && resp.StatusCode < 300,
		"POST to source URL returned %d", resp.StatusCode)
}

// createConnectionAndTriggerEvent creates a test connection with a MOCK_API destination (so events
// are generated without a live CLI), triggers one request via the source URL, then polls for the
// event to appear. Returns connection ID and event ID. Caller should cleanup with deleteConnection(t, cli, connID).
func createConnectionAndTriggerEvent(t *testing.T, cli *CLIRunner) (connID, eventID string) {
	t.Helper()

	connID = createTestConnectionWithMockDestination(t, cli)
	var conn Connection
	require.NoError(t, cli.RunJSON(&conn, "gateway", "connection", "get", connID))
	require.NotEmpty(t, conn.Source.ID, "connection source ID")

	var src Source
	require.NoError(t, cli.RunJSON(&src, "gateway", "source", "get", conn.Source.ID))
	require.NotEmpty(t, src.URL, "source URL")

	triggerTestEvent(t, src.URL)

	// Poll for event to appear (API may take a few seconds)
	type EventListResponse struct {
		Models []Event `json:"models"`
	}
	for i := 0; i < 10; i++ {
		time.Sleep(2 * time.Second)
		var resp EventListResponse
		require.NoError(t, cli.RunJSON(&resp, "gateway", "event", "list", "--connection-id", connID, "--limit", "1"))
		if len(resp.Models) > 0 {
			return connID, resp.Models[0].ID
		}
	}
	require.Fail(t, "expected at least one event after trigger (waited ~20s)")
	return "", ""
}

// pollForRequestsBySourceID polls gateway request list by source ID until at least one request
// appears or the timeout (10 attempts × 2s) is reached. Use after triggering an event when the test
// requires at least one request; fails the test if none appear (no skip).
func pollForRequestsBySourceID(t *testing.T, cli *CLIRunner, sourceID string) []Request {
	t.Helper()
	type RequestListResponse struct {
		Models []Request `json:"models"`
	}
	for i := 0; i < 10; i++ {
		time.Sleep(2 * time.Second)
		var resp RequestListResponse
		require.NoError(t, cli.RunJSON(&resp, "gateway", "request", "list", "--source-id", sourceID, "--limit", "5"))
		if len(resp.Models) > 0 {
			return resp.Models
		}
	}
	require.Fail(t, "expected at least one request after trigger (waited ~20s)")
	return nil
}

// pollForAttemptsByEventID polls gateway attempt list by event ID until at least one attempt
// appears or the timeout (10 attempts × 2s) is reached. Use after createConnectionAndTriggerEvent
// when the test requires attempts; attempt creation may lag behind event creation.
func pollForAttemptsByEventID(t *testing.T, cli *CLIRunner, eventID string) []Attempt {
	t.Helper()
	type AttemptListResponse struct {
		Models []Attempt `json:"models"`
	}
	for i := 0; i < 10; i++ {
		time.Sleep(2 * time.Second)
		var resp AttemptListResponse
		require.NoError(t, cli.RunJSON(&resp, "gateway", "attempt", "list", "--event-id", eventID, "--limit", "5"))
		if len(resp.Models) > 0 {
			return resp.Models
		}
	}
	require.Fail(t, "expected at least one attempt after trigger (waited ~20s)")
	return nil
}

// Issue is a minimal issue model for acceptance tests.
type Issue struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Type   string `json:"type"`
}

// createConnectionWithFailingTransformationAndIssue creates a connection with a
// transformation that throws, triggers an event, and polls until a transformation
// issue appears. Returns connID and issueID. Caller must cleanup with
// deleteConnection(t, cli, connID). Fails the test if no issue appears within ~40s.
func createConnectionWithFailingTransformationAndIssue(t *testing.T, cli *CLIRunner) (connID, issueID string) {
	t.Helper()

	timestamp := generateTimestamp()
	connName := fmt.Sprintf("test-issue-conn-%s", timestamp)
	sourceName := fmt.Sprintf("test-issue-src-%s", timestamp)
	destName := fmt.Sprintf("test-issue-dst-%s", timestamp)
	// Transformation that throws with a unique message so each run produces a distinct issue
	// (avoids backend deduplication when multiple tests run in sequence).
	transformCode := fmt.Sprintf(`addHandler("transform", (request, context) => { throw new Error("acceptance test %s"); });`, timestamp)

	var conn Connection
	err := cli.RunJSON(&conn,
		"gateway", "connection", "create",
		"--name", connName,
		"--source-name", sourceName,
		"--source-type", "WEBHOOK",
		"--destination-name", destName,
		"--destination-type", "MOCK_API",
		"--rule-transform-name", "fail-transform",
		"--rule-transform-code", transformCode,
	)
	require.NoError(t, err, "Failed to create connection with failing transformation")
	require.NotEmpty(t, conn.ID, "Connection ID should not be empty")

	var getConn Connection
	require.NoError(t, cli.RunJSON(&getConn, "gateway", "connection", "get", conn.ID))
	require.NotEmpty(t, getConn.Source.ID, "connection source ID")

	var src Source
	require.NoError(t, cli.RunJSON(&src, "gateway", "source", "get", getConn.Source.ID))
	require.NotEmpty(t, src.URL, "source URL")

	triggerTestEvent(t, src.URL)

	type issueListResp struct {
		Models []Issue `json:"models"`
	}
	// After a previous issue is dismissed/resolved, the backend creates a new issue for
	// a new occurrence; allow enough time for that when running as second test in suite.
	for i := 0; i < 45; i++ {
		time.Sleep(2 * time.Second)
		var resp issueListResp
		require.NoError(t, cli.RunJSON(&resp, "gateway", "issue", "list", "--type", "transformation", "--status", "OPENED", "--limit", "5", "--order-by", "last_seen_at", "--dir", "desc"))
		if len(resp.Models) > 0 {
			return conn.ID, resp.Models[0].ID
		}
	}
	require.Fail(t, "expected at least one transformation issue after trigger (waited ~90s)")
	return "", ""
}

// dismissIssue dismisses (deletes) an issue so the slot is freed for the next test.
// Use in test cleanup after every test that creates an issue.
func dismissIssue(t *testing.T, cli *CLIRunner, issueID string) {
	t.Helper()
	stdout, stderr, err := cli.Run("gateway", "issue", "dismiss", issueID, "--force")
	if err != nil {
		t.Logf("Warning: Failed to dismiss issue %s: %v\nstdout: %s\nstderr: %s", issueID, err, stdout, stderr)
		return
	}
	t.Logf("Dismissed issue: %s", issueID)
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
