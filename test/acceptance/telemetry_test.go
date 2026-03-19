//go:build connection

package acceptance

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/require"
)

// Telemetry proxy tests run commands through a recording proxy and assert that
// every API request from that run has the same invocation_id and command_path.
//
// Rule: one CLI command must produce exactly one command_path (and one
// invocation_id) on every API request. Commands that make multiple API calls
// (e.g. connection upsert does ValidateAPIKey, ListConnections, UpsertConnection)
// must send the same command_path on all of them. AssertTelemetryConsistent
// enforces this; if we had sent whoami / connection list / connection upsert
// for a single "connection upsert" run, that test would fail.
//
// Every test requires the command to succeed (require.NoError). Tests that need
// a real resource create it first without the proxy, then run the command under
// test with the proxy so the proxy only sees that one command.

// logRecordedTelemetry writes each recorded request's method, path, and telemetry
// (command_path, invocation_id) to the test log. Run with -v to see it.
func logRecordedTelemetry(t *testing.T, recorded []RecordedRequest) {
	t.Helper()
	t.Logf("DEBUG recorded telemetry: %d request(s)", len(recorded))
	for i, r := range recorded {
		t.Logf("  [%d] %s %s", i+1, r.Method, r.Path)
		if r.Telemetry == "" {
			t.Logf("       (no telemetry header)")
			continue
		}
		var p struct {
			CommandPath  string `json:"command_path"`
			InvocationID string `json:"invocation_id"`
		}
		if err := json.Unmarshal([]byte(r.Telemetry), &p); err != nil {
			t.Logf("       telemetry raw: %s", r.Telemetry)
			continue
		}
		t.Logf("       command_path=%q invocation_id=%q", p.CommandPath, p.InvocationID)
	}
}

// TestTelemetryLoginProxy verifies what we send when we run "hookdeck login --api-key":
// exactly one API call (GET /2025-07-01/cli-auth/validate) with one command_path and one
// invocation_id. Uses the same proxy approach as other telemetry tests (record then forward
// to the real API). Requires HOOKDECK_CLI_TESTING_CLI_KEY (the validate endpoint accepts
// CLI keys from interactive login; API/CI keys may return 401).
func TestTelemetryLoginProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cliKey := os.Getenv("HOOKDECK_CLI_TESTING_CLI_KEY")
	if cliKey == "" {
		t.Skip("Skipping login telemetry test: HOOKDECK_CLI_TESTING_CLI_KEY must be set (validate endpoint accepts CLI key)")
	}
	cli := NewCLIRunner(t)
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	stdout, stderr, err := cli.Run("--api-base", proxy.URL(), "login", "--api-key", cliKey)
	require.NoError(t, err, "login must succeed (proxy forwards to real API); stdout=%q stderr=%q", stdout, stderr)
	recorded := proxy.Recorded()
	require.Len(t, recorded, 1, "login with --api-key should make exactly one API call (ValidateAPIKey); got %d", len(recorded))
	AssertTelemetryConsistent(t, recorded, "hookdeck login")
}

func TestTelemetryGatewayConnectionListProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, _, err := cli.Run("--api-base", proxy.URL(), "gateway", "connection", "list")
	require.NoError(t, err)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway connection list")
}

func TestTelemetryGatewayConnectionGetProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	connID := createTestConnection(t, cli)
	defer deleteConnection(t, cli, connID)
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, _, err := cli.Run("--api-base", proxy.URL(), "gateway", "connection", "get", connID)
	require.NoError(t, err)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway connection get")
}

func TestTelemetryGatewayConnectionUpsertProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	ts := generateTimestamp()
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	var conn Connection
	require.NoError(t, cli.RunJSON(&conn,
		"--api-base", proxy.URL(),
		"gateway", "connection", "upsert", "telemetry-upsert-"+ts,
		"--source-name", "telemetry-us-"+ts, "--source-type", "WEBHOOK",
		"--destination-name", "telemetry-ud-"+ts, "--destination-type", "CLI", "--destination-cli-path", "/"))
	defer deleteConnection(t, cli, conn.ID)
	require.NotEmpty(t, conn.ID)
	recorded := proxy.Recorded()
	// Debug: log recorded telemetry so we can see how many API calls and what command_path each sends.
	logRecordedTelemetry(t, recorded)
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway connection upsert")
}

// TestTelemetryInvocationIDConsistentWhenValidateAPIKeyInPreRun reproduces the v2.0.0 bug:
// when gateway's PersistentPreRunE calls requireGatewayProject() it can perform ValidateAPIKey
// (first API request); then connection's PersistentPreRun runs and overwrites the telemetry
// singleton, so the next requests get a different invocation_id and command_path. This test runs
// "gateway connection upsert" through the recording proxy and asserts every request has the same
// invocation_id and command_path. When the bug occurs (3+ requests with ValidateAPIKey in
// PreRun), v2.0.0 sends inconsistent telemetry and AssertTelemetryConsistent fails; with the root
// fix (set invocation ID only when empty) all requests share one ID and the test passes.
func TestTelemetryInvocationIDConsistentWhenValidateAPIKeyInPreRun(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	tmpDir := t.TempDir()
	fullConfigPath := filepath.Join(tmpDir, "full.toml")
	minimalConfigPath := filepath.Join(tmpDir, "minimal.toml")

	runnerFull := NewCLIRunnerWithConfigPath(t, fullConfigPath)
	ts := generateTimestamp()
	connName := "telemetry-inv-id-" + ts
	var conn Connection
	require.NoError(t, runnerFull.RunJSON(&conn,
		"gateway", "connection", "create",
		"--source-name", "telemetry-inv-src-"+ts, "--source-type", "WEBHOOK",
		"--destination-name", "telemetry-inv-dst-"+ts, "--destination-type", "CLI", "--destination-cli-path", "/",
		"--name", connName))
	defer deleteConnection(t, runnerFull, conn.ID)
	require.NotEmpty(t, conn.ID)

	// Minimal config (api_key + project_id only) so requireGatewayProject() would call ValidateAPIKey
	// if project_type is not set. Running from tmpDir with this config aims to trigger 3 requests.
	var fullStruct struct {
		Default struct {
			APIKey    string `toml:"api_key"`
			ProjectID string `toml:"project_id"`
		} `toml:"default"`
	}
	data, err := os.ReadFile(fullConfigPath)
	require.NoError(t, err, "read full config")
	require.NoError(t, toml.Unmarshal(data, &fullStruct), "parse full config")
	require.NotEmpty(t, fullStruct.Default.APIKey)
	require.NotEmpty(t, fullStruct.Default.ProjectID)
	minimalStruct := struct {
		Default struct {
			APIKey    string `toml:"api_key"`
			ProjectID string `toml:"project_id"`
		} `toml:"default"`
	}{Default: fullStruct.Default}
	minimalData, err := toml.Marshal(minimalStruct)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(minimalConfigPath, minimalData, 0600))

	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()

	// Run CLI binary from tmpDir with minimal config so only our file is used.
	projectRoot, err := filepath.Abs("../..")
	require.NoError(t, err)
	cliBinary := filepath.Join(tmpDir, "hookdeck-test-binary")
	buildCmd := exec.Command("go", "build", "-o", cliBinary, ".")
	buildCmd.Dir = projectRoot
	require.NoError(t, buildCmd.Run(), "build CLI for test")
	runCmd := exec.Command(cliBinary,
		"--api-base", proxy.URL(),
		"--hookdeck-config", "minimal.toml",
		"gateway", "connection", "upsert", connName)
	runCmd.Dir = tmpDir
	runCmd.Env = appendEnvOverride(os.Environ(), "HOOKDECK_CONFIG_FILE", filepath.Join(tmpDir, "minimal.toml"))
	var stdoutBuf, stderrBuf bytes.Buffer
	runCmd.Stdout = &stdoutBuf
	runCmd.Stderr = &stderrBuf
	err = runCmd.Run()
	require.NoError(t, err, "connection upsert must succeed; stdout=%q stderr=%q", stdoutBuf.String(), stderrBuf.String())

	recorded := proxy.Recorded()
	logRecordedTelemetry(t, recorded)
	require.GreaterOrEqual(t, len(recorded), 2, "expected at least ListConnections + UpsertConnection; got %d", len(recorded))
	// When 3 requests occur (ValidateAPIKey in gateway PreRun + List + Upsert), v2.0.0 sends
	// different invocation_ids on request 1 vs 2–3; this assertion fails and demonstrates the bug.
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway connection upsert")
}

// TestTelemetryGatewayConnectionUpsertThreeRequestsProxy verifies that when connection upsert
// makes multiple API calls (ListConnections + UpsertConnection, and optionally ValidateAPIKey),
// every request has the same command_path and invocation_id. This reproduces the scenario that
// previously produced three different command_paths in logging.
//
// Combination that yields multiple requests: run upsert with only the connection name
// (no --source-* / --destination-*) so the CLI calls ListConnections then UpsertConnection.
// When the config has no project_type (e.g. minimal config with only api_key and project_id),
// the CLI also calls ValidateAPIKey first, yielding three requests total. With project_type
// already set (e.g. from "ci" or default config), only two requests are made.
func TestTelemetryGatewayConnectionUpsertThreeRequestsProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	tmpDir := t.TempDir()
	fullConfigPath := filepath.Join(tmpDir, "full.toml")
	minimalConfigPath := filepath.Join(tmpDir, "minimal.toml")

	// Populate full config via "ci" so we have api_key, project_id, project_mode, project_type.
	runnerFull := NewCLIRunnerWithConfigPath(t, fullConfigPath)
	ts := generateTimestamp()
	// Create a connection without proxy so it exists; we'll upsert it by name through the proxy.
	connName := "telemetry-three-" + ts
	var conn Connection
	require.NoError(t, runnerFull.RunJSON(&conn,
		"gateway", "connection", "create",
		"--source-name", "telemetry-three-src-"+ts, "--source-type", "WEBHOOK",
		"--destination-name", "telemetry-three-dst-"+ts, "--destination-type", "CLI", "--destination-cli-path", "/",
		"--name", connName))
	defer deleteConnection(t, runnerFull, conn.ID)
	require.NotEmpty(t, conn.ID)

	// Read full config and write minimal (api_key + project_id only) so the CLI will call ValidateAPIKey.
	var fullStruct struct {
		Default struct {
			APIKey    string `toml:"api_key"`
			ProjectID string `toml:"project_id"`
		} `toml:"default"`
	}
	data, err := os.ReadFile(fullConfigPath)
	require.NoError(t, err, "read full config")
	require.NoError(t, toml.Unmarshal(data, &fullStruct), "parse full config")
	require.NotEmpty(t, fullStruct.Default.APIKey, "full config must have api_key")
	require.NotEmpty(t, fullStruct.Default.ProjectID, "full config must have project_id")

	minimalStruct := struct {
		Default struct {
			APIKey    string `toml:"api_key"`
			ProjectID string `toml:"project_id"`
		} `toml:"default"`
	}{
		Default: fullStruct.Default,
	}
	minimalData, err := toml.Marshal(minimalStruct)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(minimalConfigPath, minimalData, 0600))

	// Run upsert through the proxy with minimal config and only the connection name (no source/dest flags).
	// Pass --hookdeck-config explicitly so the CLI uses only our minimal file (no merge with cwd .hookdeck).
	runnerMinimal := NewCLIRunnerWithConfigPathNoCI(t, minimalConfigPath)
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, _, err = runnerMinimal.Run("--api-base", proxy.URL(), "--hookdeck-config", minimalConfigPath, "gateway", "connection", "upsert", connName)
	require.NoError(t, err, "connection upsert with minimal config and name-only must succeed")

	recorded := proxy.Recorded()
	logRecordedTelemetry(t, recorded)
	require.GreaterOrEqual(t, len(recorded), 2,
		"expected at least two API calls (ListConnections, UpsertConnection); got %d", len(recorded))
	require.LessOrEqual(t, len(recorded), 3,
		"expected at most three API calls (ValidateAPIKey, ListConnections, UpsertConnection); got %d", len(recorded))
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway connection upsert")
}

// TestTelemetryGatewayConnectionUpsertSystemBinary runs "gateway connection upsert" through the
// recording proxy using the installed "hookdeck" binary on PATH (e.g. 2.0.0). With minimal config
// (no project_type) that run triggers ValidateAPIKey, ListConnections, and UpsertConnection — the
// bug in 2.0.0 was that those three requests were sent with three different command_paths the
// backend showed as "Who am I?", "Connection list", and "Connection upsert".
//
// Run with: HOOKDECK_CLI_USE_SYSTEM_BINARY=1 go test -v -run TestTelemetryGatewayConnectionUpsertSystemBinary ./test/acceptance/ -tags=connection
// Ensure the hookdeck on your PATH is 2.0.0 (or the version you want to test). The test skips if
// HOOKDECK_CLI_USE_SYSTEM_BINARY is not set.
func TestTelemetryGatewayConnectionUpsertSystemBinary(t *testing.T) {
	if os.Getenv("HOOKDECK_CLI_USE_SYSTEM_BINARY") != "1" {
		t.Skip("Skipping unless HOOKDECK_CLI_USE_SYSTEM_BINARY=1 (run against installed hookdeck binary, e.g. 2.0.0)")
	}
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	tmpDir := t.TempDir()
	fullConfigPath := filepath.Join(tmpDir, "full.toml")
	minimalConfigPath := filepath.Join(tmpDir, "minimal.toml")

	// Create connection using current code (go run) so it exists; upsert will be run with system binary.
	runnerFull := NewCLIRunnerWithConfigPath(t, fullConfigPath)
	ts := generateTimestamp()
	connName := "telemetry-sysbin-" + ts
	var conn Connection
	require.NoError(t, runnerFull.RunJSON(&conn,
		"gateway", "connection", "create",
		"--source-name", "telemetry-sysbin-src-"+ts, "--source-type", "WEBHOOK",
		"--destination-name", "telemetry-sysbin-dst-"+ts, "--destination-type", "CLI", "--destination-cli-path", "/",
		"--name", connName))
	defer deleteConnection(t, runnerFull, conn.ID)
	require.NotEmpty(t, conn.ID)

	var fullStruct struct {
		Default struct {
			APIKey    string `toml:"api_key"`
			ProjectID string `toml:"project_id"`
		} `toml:"default"`
	}
	data, err := os.ReadFile(fullConfigPath)
	require.NoError(t, err)
	require.NoError(t, toml.Unmarshal(data, &fullStruct))
	require.NotEmpty(t, fullStruct.Default.APIKey)
	require.NotEmpty(t, fullStruct.Default.ProjectID)

	minimalStruct := struct {
		Default struct {
			APIKey    string `toml:"api_key"`
			ProjectID string `toml:"project_id"`
		} `toml:"default"`
	}{Default: fullStruct.Default}
	minimalData, err := toml.Marshal(minimalStruct)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(minimalConfigPath, minimalData, 0600))

	// Cobra calls OnInitialize (InitConfig) before parsing flags, so Config.ConfigFileFlag is still ""
	// at config load time. We must set HOOKDECK_CONFIG_FILE so the minimal config is used; --hookdeck-config
	// alone would only apply after parsing, too late for InitConfig.
	systemBinary := map[string]string{"HOOKDECK_CLI_USE_SYSTEM_BINARY": "1"}
	runnerMinimal := NewCLIRunnerWithConfigPathNoCI(t, minimalConfigPath)
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, stderr, err := runnerMinimal.RunWithEnv(systemBinary,
		"--api-base", proxy.URL(), "--hookdeck-config", minimalConfigPath,
		"gateway", "connection", "upsert", connName)
	require.NoError(t, err, "system binary connection upsert failed; stderr=%s", stderr)

	recorded := proxy.Recorded()
	// With minimal config (no project_type) we expect 3 API calls: (1) whoami/ValidateAPIKey, (2) ListConnections, (3) UpsertConnection.
	// If the binary already has project_type from another source (or is a fixed build), we may see only 2 (list + upsert).
	require.GreaterOrEqual(t, len(recorded), 2, "expected at least 2 API calls (list + upsert); got %d", len(recorded))
	require.LessOrEqual(t, len(recorded), 3, "expected at most 3 API calls; got %d", len(recorded))

	logRecordedTelemetry(t, recorded)

	// Produce the problem: fail if the system binary sent multiple different command_paths in one run
	// (the bug the backend showed as "Who am I?", "Connection list", "Connection upsert").
	paths := make([]string, len(recorded))
	for i, r := range recorded {
		if r.Telemetry == "" {
			continue
		}
		var p struct {
			CommandPath string `json:"command_path"`
		}
		if err := json.Unmarshal([]byte(r.Telemetry), &p); err != nil {
			continue
		}
		paths[i] = p.CommandPath
	}
	uniquePaths := make(map[string]struct{})
	for _, cp := range paths {
		if cp != "" {
			uniquePaths[cp] = struct{}{}
		}
	}
	if len(uniquePaths) > 1 {
		t.Fatalf("BUG REPRODUCED: system binary sent %d different command_paths in one run (expected one). Paths: %v. "+
			"Backend would show these as separate commands (e.g. Who am I?, Connection list, Connection upsert). "+
			"Use the fixed CLI (go build) so all requests share the same command_path.", len(uniquePaths), paths)
	}
}

func TestTelemetryGatewaySourceListProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, _, err := cli.Run("--api-base", proxy.URL(), "gateway", "source", "list")
	require.NoError(t, err)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway source list")
}

func TestTelemetryGatewayDestinationListProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, _, err := cli.Run("--api-base", proxy.URL(), "gateway", "destination", "list")
	require.NoError(t, err)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway destination list")
}

func TestTelemetryWhoamiProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, _, err := cli.Run("--api-base", proxy.URL(), "whoami")
	require.NoError(t, err)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck whoami")
}

func TestTelemetryGatewayMetricsEventsProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	runTelemetryProxyTestSuccess(t,
		[]string{"gateway", "metrics", "events", "--start", "2025-01-01T00:00:00Z", "--end", "2025-01-02T00:00:00Z", "--measures", "count"},
		"hookdeck gateway metrics events")
}

func TestTelemetryGatewayIssueListProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, _, err := cli.Run("--api-base", proxy.URL(), "gateway", "issue", "list")
	require.NoError(t, err)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway issue list")
}

func TestTelemetryGatewayTransformationListProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, _, err := cli.Run("--api-base", proxy.URL(), "gateway", "transformation", "list")
	require.NoError(t, err)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway transformation list")
}

func TestTelemetryGatewayEventListProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, _, err := cli.Run("--api-base", proxy.URL(), "gateway", "event", "list")
	require.NoError(t, err)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway event list")
}

func TestTelemetryGatewayRequestListProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, _, err := cli.Run("--api-base", proxy.URL(), "gateway", "request", "list")
	require.NoError(t, err)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway request list")
}

// runTelemetryProxyTestSuccess runs the CLI through the proxy, requires success,
// and asserts all recorded requests have consistent telemetry for the expected command.
// Use when the command can succeed with the given args (e.g. list, create with valid payload).
func runTelemetryProxyTestSuccess(t *testing.T, args []string, expectedCommandPath string) {
	t.Helper()
	cli := NewCLIRunner(t)
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	fullArgs := append([]string{"--api-base", proxy.URL()}, args...)
	_, _, err := cli.Run(fullArgs...)
	require.NoError(t, err, "command must succeed so all recorded requests are from this single invocation")
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1, "expected at least one API request for %q", expectedCommandPath)
	AssertTelemetryConsistent(t, recorded, expectedCommandPath)
}

// --- Connection (remaining) ---
func TestTelemetryGatewayConnectionCreateProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	ts := generateTimestamp()
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	var conn Connection
	require.NoError(t, cli.RunJSON(&conn,
		"--api-base", proxy.URL(),
		"gateway", "connection", "create",
		"--name", "telemetry-conn-"+ts, "--source-name", "s-"+ts, "--source-type", "WEBHOOK",
		"--destination-name", "d-"+ts, "--destination-type", "CLI", "--destination-cli-path", "/"))
	defer deleteConnection(t, cli, conn.ID)
	require.NotEmpty(t, conn.ID)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway connection create")
}
func TestTelemetryGatewayConnectionUpdateProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	connID := createTestConnection(t, cli)
	defer deleteConnection(t, cli, connID)
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, _, err := cli.Run("--api-base", proxy.URL(), "gateway", "connection", "update", connID, "--description", "telemetry-desc")
	require.NoError(t, err)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway connection update")
}
func TestTelemetryGatewayConnectionDeleteProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	connID := createTestConnection(t, cli)
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, _, err := cli.Run("--api-base", proxy.URL(), "gateway", "connection", "delete", connID, "--force")
	require.NoError(t, err)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway connection delete")
}
func TestTelemetryGatewayConnectionEnableProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	connID := createTestConnection(t, cli)
	defer deleteConnection(t, cli, connID)
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, _, err := cli.Run("--api-base", proxy.URL(), "gateway", "connection", "enable", connID)
	require.NoError(t, err)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway connection enable")
}
func TestTelemetryGatewayConnectionDisableProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	connID := createTestConnection(t, cli)
	defer deleteConnection(t, cli, connID)
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, _, err := cli.Run("--api-base", proxy.URL(), "gateway", "connection", "disable", connID)
	require.NoError(t, err)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway connection disable")
}
func TestTelemetryGatewayConnectionPauseProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	connID := createTestConnection(t, cli)
	defer deleteConnection(t, cli, connID)
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, _, err := cli.Run("--api-base", proxy.URL(), "gateway", "connection", "pause", connID)
	require.NoError(t, err)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway connection pause")
}
func TestTelemetryGatewayConnectionUnpauseProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	connID := createTestConnection(t, cli)
	defer deleteConnection(t, cli, connID)
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, _, err := cli.Run("--api-base", proxy.URL(), "gateway", "connection", "unpause", connID)
	require.NoError(t, err)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway connection unpause")
}

// --- Source (remaining) ---
func TestTelemetryGatewaySourceGetProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	srcID := createTestSource(t, cli)
	defer deleteSource(t, cli, srcID)
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, _, err := cli.Run("--api-base", proxy.URL(), "gateway", "source", "get", srcID)
	require.NoError(t, err)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway source get")
}
func TestTelemetryGatewaySourceCreateProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	var src Source
	require.NoError(t, cli.RunJSON(&src,
		"--api-base", proxy.URL(),
		"gateway", "source", "create", "--name", "telemetry-src-"+generateTimestamp(), "--type", "WEBHOOK"))
	defer deleteSource(t, cli, src.ID)
	require.NotEmpty(t, src.ID)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway source create")
}
func TestTelemetryGatewaySourceUpdateProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	srcID := createTestSource(t, cli)
	defer deleteSource(t, cli, srcID)
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, _, err := cli.Run("--api-base", proxy.URL(), "gateway", "source", "update", srcID, "--name", "telemetry-src-updated")
	require.NoError(t, err)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway source update")
}
func TestTelemetryGatewaySourceDeleteProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	srcID := createTestSource(t, cli)
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, _, err := cli.Run("--api-base", proxy.URL(), "gateway", "source", "delete", srcID, "--force")
	require.NoError(t, err)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway source delete")
}
func TestTelemetryGatewaySourceEnableProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	srcID := createTestSource(t, cli)
	defer deleteSource(t, cli, srcID)
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, _, err := cli.Run("--api-base", proxy.URL(), "gateway", "source", "enable", srcID)
	require.NoError(t, err)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway source enable")
}
func TestTelemetryGatewaySourceDisableProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	srcID := createTestSource(t, cli)
	defer deleteSource(t, cli, srcID)
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, _, err := cli.Run("--api-base", proxy.URL(), "gateway", "source", "disable", srcID)
	require.NoError(t, err)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway source disable")
}
func TestTelemetryGatewaySourceUpsertProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	runTelemetryProxyTestSuccess(t,
		[]string{"gateway", "source", "upsert", "telemetry-src-upsert-"+generateTimestamp(), "--type", "WEBHOOK"},
		"hookdeck gateway source upsert")
}
func TestTelemetryGatewaySourceCountProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, _, err := cli.Run("--api-base", proxy.URL(), "gateway", "source", "count")
	require.NoError(t, err)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway source count")
}

// --- Destination (remaining) ---
func TestTelemetryGatewayDestinationGetProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	dstID := createTestDestination(t, cli)
	defer deleteDestination(t, cli, dstID)
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, _, err := cli.Run("--api-base", proxy.URL(), "gateway", "destination", "get", dstID)
	require.NoError(t, err)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway destination get")
}
func TestTelemetryGatewayDestinationCreateProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	var dst Destination
	require.NoError(t, cli.RunJSON(&dst,
		"--api-base", proxy.URL(),
		"gateway", "destination", "create", "--name", "telemetry-dst-"+generateTimestamp(), "--type", "HTTP", "--url", "https://example.com"))
	defer deleteDestination(t, cli, dst.ID)
	require.NotEmpty(t, dst.ID)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway destination create")
}
func TestTelemetryGatewayDestinationUpdateProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	dstID := createTestDestination(t, cli)
	defer deleteDestination(t, cli, dstID)
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, _, err := cli.Run("--api-base", proxy.URL(), "gateway", "destination", "update", dstID, "--name", "telemetry-dst-updated")
	require.NoError(t, err)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway destination update")
}
func TestTelemetryGatewayDestinationDeleteProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	dstID := createTestDestination(t, cli)
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, _, err := cli.Run("--api-base", proxy.URL(), "gateway", "destination", "delete", dstID, "--force")
	require.NoError(t, err)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway destination delete")
}
func TestTelemetryGatewayDestinationEnableProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	dstID := createTestDestination(t, cli)
	defer deleteDestination(t, cli, dstID)
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, _, err := cli.Run("--api-base", proxy.URL(), "gateway", "destination", "enable", dstID)
	require.NoError(t, err)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway destination enable")
}
func TestTelemetryGatewayDestinationDisableProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	dstID := createTestDestination(t, cli)
	defer deleteDestination(t, cli, dstID)
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, _, err := cli.Run("--api-base", proxy.URL(), "gateway", "destination", "disable", dstID)
	require.NoError(t, err)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway destination disable")
}
func TestTelemetryGatewayDestinationUpsertProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	runTelemetryProxyTestSuccess(t,
		[]string{"gateway", "destination", "upsert", "telemetry-dst-upsert-"+generateTimestamp(), "--type", "HTTP", "--url", "https://example.com"},
		"hookdeck gateway destination upsert")
}
func TestTelemetryGatewayDestinationCountProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, _, err := cli.Run("--api-base", proxy.URL(), "gateway", "destination", "count")
	require.NoError(t, err)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway destination count")
}

// --- Transformation (remaining) ---
func TestTelemetryGatewayTransformationGetProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	trnID := createTestTransformation(t, cli)
	defer deleteTransformation(t, cli, trnID)
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, _, err := cli.Run("--api-base", proxy.URL(), "gateway", "transformation", "get", trnID)
	require.NoError(t, err)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway transformation get")
}
func TestTelemetryGatewayTransformationCreateProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	minCode := `addHandler("transform", (request, context) => { return request; });`
	var trn Transformation
	require.NoError(t, cli.RunJSON(&trn,
		"--api-base", proxy.URL(),
		"gateway", "transformation", "create", "--name", "telemetry-trn-"+generateTimestamp(), "--code", minCode))
	defer deleteTransformation(t, cli, trn.ID)
	require.NotEmpty(t, trn.ID)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway transformation create")
}
func TestTelemetryGatewayTransformationUpdateProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	trnID := createTestTransformation(t, cli)
	defer deleteTransformation(t, cli, trnID)
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, _, err := cli.Run("--api-base", proxy.URL(), "gateway", "transformation", "update", trnID, "--name", "telemetry-trn-updated")
	require.NoError(t, err)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway transformation update")
}
func TestTelemetryGatewayTransformationDeleteProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	trnID := createTestTransformation(t, cli)
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, _, err := cli.Run("--api-base", proxy.URL(), "gateway", "transformation", "delete", trnID, "--force")
	require.NoError(t, err)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway transformation delete")
}
func TestTelemetryGatewayTransformationRunProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	runTelemetryProxyTestSuccess(t,
		[]string{"gateway", "transformation", "run", "--code", `addHandler("transform", (request, context) => { return request; });`, "--request", `{"headers":{}}`},
		"hookdeck gateway transformation run")
}
func TestTelemetryGatewayTransformationUpsertProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	runTelemetryProxyTestSuccess(t,
		[]string{"gateway", "transformation", "upsert", "telemetry-trn-upsert-"+generateTimestamp(), "--code", `addHandler("transform", (request, context) => { return request; });`},
		"hookdeck gateway transformation upsert")
}
func TestTelemetryGatewayTransformationCountProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, _, err := cli.Run("--api-base", proxy.URL(), "gateway", "transformation", "count")
	require.NoError(t, err)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway transformation count")
}
func TestTelemetryGatewayTransformationExecutionsListProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	trnID := createTestTransformation(t, cli)
	defer deleteTransformation(t, cli, trnID)
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, _, err := cli.Run("--api-base", proxy.URL(), "gateway", "transformation", "executions", "list", trnID)
	require.NoError(t, err)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway transformation executions list")
}
func TestTelemetryGatewayTransformationExecutionsGetProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	trnID := createTestTransformation(t, cli)
	defer deleteTransformation(t, cli, trnID)
	// Run transformation to create an execution
	_, _, err := cli.Run("gateway", "transformation", "run", "--id", trnID, "--request", `{"headers":{}}`)
	require.NoError(t, err)
	var listResp struct {
		Models []struct {
			ID string `json:"id"`
		} `json:"models"`
	}
	require.NoError(t, cli.RunJSON(&listResp, "gateway", "transformation", "executions", "list", trnID))
	if len(listResp.Models) == 0 {
		t.Skip("no executions from run; skipping executions get telemetry test")
		return
	}
	execID := listResp.Models[0].ID
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, _, err = cli.Run("--api-base", proxy.URL(), "gateway", "transformation", "executions", "get", trnID, execID)
	require.NoError(t, err)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway transformation executions get")
}

// --- Event (remaining) ---
func TestTelemetryGatewayEventGetProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	connID, eventID := createConnectionAndTriggerEvent(t, cli)
	defer deleteConnection(t, cli, connID)
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, _, err := cli.Run("--api-base", proxy.URL(), "gateway", "event", "get", eventID)
	require.NoError(t, err)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway event get")
}
func TestTelemetryGatewayEventRetryProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	connID, eventID := createConnectionAndTriggerEvent(t, cli)
	defer deleteConnection(t, cli, connID)
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, _, err := cli.Run("--api-base", proxy.URL(), "gateway", "event", "retry", eventID)
	require.NoError(t, err)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway event retry")
}
func TestTelemetryGatewayEventCancelProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	connID, eventID := createConnectionAndTriggerEvent(t, cli)
	defer deleteConnection(t, cli, connID)
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, _, err := cli.Run("--api-base", proxy.URL(), "gateway", "event", "cancel", eventID)
	require.NoError(t, err)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway event cancel")
}
func TestTelemetryGatewayEventMuteProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	connID, eventID := createConnectionAndTriggerEvent(t, cli)
	defer deleteConnection(t, cli, connID)
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, _, err := cli.Run("--api-base", proxy.URL(), "gateway", "event", "mute", eventID)
	require.NoError(t, err)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway event mute")
}
func TestTelemetryGatewayEventRawBodyProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	connID, eventID := createConnectionAndTriggerEvent(t, cli)
	defer deleteConnection(t, cli, connID)
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, _, err := cli.Run("--api-base", proxy.URL(), "gateway", "event", "raw-body", eventID)
	require.NoError(t, err)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway event raw-body")
}

// --- Request (remaining) ---
func TestTelemetryGatewayRequestGetProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	connID, _ := createConnectionAndTriggerEvent(t, cli)
	defer deleteConnection(t, cli, connID)
	var conn Connection
	require.NoError(t, cli.RunJSON(&conn, "gateway", "connection", "get", connID))
	requests := pollForRequestsBySourceID(t, cli, conn.Source.ID)
	require.NotEmpty(t, requests)
	requestID := requests[0].ID
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, _, err := cli.Run("--api-base", proxy.URL(), "gateway", "request", "get", requestID)
	require.NoError(t, err)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway request get")
}
func TestTelemetryGatewayRequestRetryProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	connID, _ := createConnectionAndTriggerEvent(t, cli)
	defer deleteConnection(t, cli, connID)
	var conn Connection
	require.NoError(t, cli.RunJSON(&conn, "gateway", "connection", "get", connID))
	requests := pollForRequestsBySourceID(t, cli, conn.Source.ID)
	require.NotEmpty(t, requests)
	requestID := requests[0].ID
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, stderr, err := cli.Run("--api-base", proxy.URL(), "gateway", "request", "retry", requestID)
	if err != nil {
		t.Skipf("Skipping request retry telemetry test: API rejected retry (exit 1). stderr=%q", stderr)
		return
	}
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway request retry")
}
func TestTelemetryGatewayRequestEventsProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	connID, _ := createConnectionAndTriggerEvent(t, cli)
	defer deleteConnection(t, cli, connID)
	var conn Connection
	require.NoError(t, cli.RunJSON(&conn, "gateway", "connection", "get", connID))
	requests := pollForRequestsBySourceID(t, cli, conn.Source.ID)
	require.NotEmpty(t, requests)
	requestID := requests[0].ID
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, _, err := cli.Run("--api-base", proxy.URL(), "gateway", "request", "events", requestID)
	require.NoError(t, err)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway request events")
}
func TestTelemetryGatewayRequestIgnoredEventsProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	connID, _ := createConnectionAndTriggerEvent(t, cli)
	defer deleteConnection(t, cli, connID)
	var conn Connection
	require.NoError(t, cli.RunJSON(&conn, "gateway", "connection", "get", connID))
	requests := pollForRequestsBySourceID(t, cli, conn.Source.ID)
	require.NotEmpty(t, requests)
	requestID := requests[0].ID
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, _, err := cli.Run("--api-base", proxy.URL(), "gateway", "request", "ignored-events", requestID)
	require.NoError(t, err)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway request ignored-events")
}
func TestTelemetryGatewayRequestRawBodyProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	connID, _ := createConnectionAndTriggerEvent(t, cli)
	defer deleteConnection(t, cli, connID)
	var conn Connection
	require.NoError(t, cli.RunJSON(&conn, "gateway", "connection", "get", connID))
	requests := pollForRequestsBySourceID(t, cli, conn.Source.ID)
	require.NotEmpty(t, requests)
	requestID := requests[0].ID
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, _, err := cli.Run("--api-base", proxy.URL(), "gateway", "request", "raw-body", requestID)
	require.NoError(t, err)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway request raw-body")
}

// --- Attempt ---
func TestTelemetryGatewayAttemptListProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	connID, eventID := createConnectionAndTriggerEvent(t, cli)
	defer deleteConnection(t, cli, connID)
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, _, err := cli.Run("--api-base", proxy.URL(), "gateway", "attempt", "list", "--event-id", eventID)
	require.NoError(t, err)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway attempt list")
}
func TestTelemetryGatewayAttemptGetProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	connID, eventID := createConnectionAndTriggerEvent(t, cli)
	defer deleteConnection(t, cli, connID)
	attempts := pollForAttemptsByEventID(t, cli, eventID)
	require.NotEmpty(t, attempts)
	attemptID := attempts[0].ID
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, _, err := cli.Run("--api-base", proxy.URL(), "gateway", "attempt", "get", attemptID)
	require.NoError(t, err)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway attempt get")
}
func TestTelemetryGatewayAttemptRetryProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	connID, eventID := createConnectionAndTriggerEvent(t, cli)
	defer deleteConnection(t, cli, connID)
	attempts := pollForAttemptsByEventID(t, cli, eventID)
	require.NotEmpty(t, attempts)
	attemptID := attempts[0].ID
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, stderr, err := cli.Run("--api-base", proxy.URL(), "gateway", "attempt", "retry", attemptID)
	if err != nil {
		t.Skipf("Skipping attempt retry telemetry test: API rejected retry (exit 1). stderr=%q", stderr)
		return
	}
	recorded := proxy.Recorded()
	if len(recorded) == 0 {
		t.Skip("Skipping attempt retry telemetry test: no API request was made (retry may be no-op in this state)")
		return
	}
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway attempt retry")
}

// --- Metrics (remaining) ---
func TestTelemetryGatewayMetricsRequestsProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	runTelemetryProxyTestSuccess(t,
		[]string{"gateway", "metrics", "requests", "--start", "2025-01-01T00:00:00Z", "--end", "2025-01-02T00:00:00Z", "--measures", "count"},
		"hookdeck gateway metrics requests")
}
func TestTelemetryGatewayMetricsAttemptsProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	runTelemetryProxyTestSuccess(t,
		[]string{"gateway", "metrics", "attempts", "--start", "2025-01-01T00:00:00Z", "--end", "2025-01-02T00:00:00Z", "--measures", "count"},
		"hookdeck gateway metrics attempts")
}
func TestTelemetryGatewayMetricsTransformationsProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	runTelemetryProxyTestSuccess(t,
		[]string{"gateway", "metrics", "transformations", "--start", "2025-01-01T00:00:00Z", "--end", "2025-01-02T00:00:00Z", "--measures", "count"},
		"hookdeck gateway metrics transformations")
}

// --- Issue (remaining) ---
func TestTelemetryGatewayIssueGetProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	var listResp struct {
		Models []Issue `json:"models"`
	}
	require.NoError(t, cli.RunJSON(&listResp, "gateway", "issue", "list", "--limit", "1"))
	if len(listResp.Models) == 0 {
		t.Skip("no issues in workspace; skipping issue get telemetry test")
		return
	}
	issueID := listResp.Models[0].ID
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, _, err := cli.Run("--api-base", proxy.URL(), "gateway", "issue", "get", issueID)
	require.NoError(t, err)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway issue get")
}
func TestTelemetryGatewayIssueUpdateProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	var listResp struct {
		Models []Issue `json:"models"`
	}
	require.NoError(t, cli.RunJSON(&listResp, "gateway", "issue", "list", "--status", "OPENED", "--limit", "1"))
	if len(listResp.Models) == 0 {
		t.Skip("no open issues in workspace; skipping issue update telemetry test")
		return
	}
	issueID := listResp.Models[0].ID
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, _, err := cli.Run("--api-base", proxy.URL(), "gateway", "issue", "update", issueID, "--status", "resolved")
	require.NoError(t, err)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway issue update")
}
func TestTelemetryGatewayIssueDismissProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	var listResp struct {
		Models []Issue `json:"models"`
	}
	require.NoError(t, cli.RunJSON(&listResp, "gateway", "issue", "list", "--status", "OPENED", "--limit", "1"))
	if len(listResp.Models) == 0 {
		t.Skip("no open issues in workspace; skipping issue dismiss telemetry test")
		return
	}
	issueID := listResp.Models[0].ID
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, _, err := cli.Run("--api-base", proxy.URL(), "gateway", "issue", "dismiss", issueID, "--force")
	require.NoError(t, err)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway issue dismiss")
}
func TestTelemetryGatewayIssueCountProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	proxy := StartRecordingProxy(t, defaultAPIUpstream)
	defer proxy.Close()
	_, _, err := cli.Run("--api-base", proxy.URL(), "gateway", "issue", "count")
	require.NoError(t, err)
	recorded := proxy.Recorded()
	require.GreaterOrEqual(t, len(recorded), 1)
	AssertTelemetryConsistent(t, recorded, "hookdeck gateway issue count")
}
