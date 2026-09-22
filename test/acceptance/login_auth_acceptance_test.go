//go:build basic

package acceptance

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoginAfterValidate401StartsBrowserFlowAcceptance runs the real CLI against a local
// mock API: GET validate returns 401, then POST /cli-auth and poll complete the device flow.
// SSH_CONNECTION avoids the "Press Enter to open the browser" branch (non-interactive).
func TestLoginAfterValidate401StartsBrowserFlowAcceptance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	projectRoot, err := filepath.Abs("../..")
	require.NoError(t, err)
	mainGo := filepath.Join(projectRoot, "main.go")

	configPath := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(configPath, []byte(`profile = "default"

[default]
api_key = "hk_test_stale_accept01"
`), 0o600))

	pollHits := 0
	var serverURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/cli-auth/validate"):
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("Unauthorized"))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/cli-auth"):
			pollURL := serverURL + hookdeck.APIPathPrefix + "/cli-auth/poll?key=pollkey"
			body, encErr := json.Marshal(map[string]string{
				"browser_url": "https://example.test/auth",
				"poll_url":    pollURL,
			})
			require.NoError(t, encErr)
			_, _ = w.Write(body)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/cli-auth/poll"):
			pollHits++
			resp := map[string]interface{}{
				"claimed":           true,
				"key":               "hk_test_newkey_accept01",
				"team_id":           "tm_accept",
				"team_type":         "event_gateway",
				"team_name":         "AcceptProj",
				"user_name":         "Accept",
				"user_email":        "accept@example.com",
				"organization_name": "AcceptOrg",
				"organization_id":   "org_accept",
				"client_id":         "cl_accept",
			}
			enc, encErr := json.Marshal(resp)
			require.NoError(t, encErr)
			_, _ = w.Write(enc)
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	serverURL = ts.URL
	t.Cleanup(ts.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", []string{"run", mainGo,
		"--api-base", ts.URL,
		"--hookdeck-config", configPath,
		"--log-level", "error",
		"login",
	}...)
	cmd.Dir = projectRoot
	env := appendEnvOverride(os.Environ(), "HOOKDECK_CONFIG_FILE", configPath)
	env = appendEnvOverride(env, "SSH_CONNECTION", "acceptance-login-mock")
	env = appendEnvOverride(env, "HOOKDECK_CLI_TELEMETRY_DISABLED", "1")
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()

	// SSH_CONNECTION is set above on purpose: waitForLoginSession prints the URL
	// and polls in that case, without ever reading stdin, so the flow completes
	// with no terminal and a human elsewhere. That is the path this test covers.
	//
	// It briefly did not. A guard added for the CI hang refused on "no terminal"
	// alone, which took this branch out too, and this test was then rewritten to
	// assert the refusal - encoding the regression rather than catching it. The
	// guard now only refuses where nobody can act; see
	// TestLogin_rejectedKeyHeadlessFailsFast for that case.
	require.NoError(t, err, "stdout=%q stderr=%q", stdout.String(), stderr.String())
	require.Contains(t, stdout.String(), "no longer valid", "user should see stale-key message")
	require.Equal(t, 1, pollHits, "mock should see exactly one poll after cli-auth")

}

// TestCIWritesTheProjectTypeAcceptance is the end-to-end check that the
// 2026-09-01 project type survives the whole round trip: API response, to
// profile, to config file. Everything else about the rename is covered by unit
// tests against mocks this repo also writes, so this is the only place the
// persisted value is verified against a real CLI run.
//
// It goes through `ci` rather than `login` because `ci` is the path that works
// without a terminal, and CI has none.
func TestCIWritesTheProjectTypeAcceptance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	configPath := filepath.Join(t.TempDir(), "config.toml")
	// The runner authenticates with `ci`, which is what writes the config.
	NewCLIRunnerWithConfigPath(t, configPath)

	written, err := os.ReadFile(configPath)
	require.NoError(t, err)

	assert.Contains(t, string(written), "project_type = 'Gateway'",
		"the config is shared with older CLIs, which only understand the label")
	assert.Contains(t, string(written), "project_mode = 'inbound'",
		"the legacy mode is still written for older CLIs")
	assert.NotContains(t, string(written), "project_product",
		"the short-lived product key must not be written")
}

// TestCIFailsFastWithInvalidAPIKeyAcceptance verifies hookdeck ci does not enter the
// interactive browser login path when the project API key is invalid — it exits with
// an error quickly (CI-safe: no stdin / device flow).
func TestCIFailsFastWithInvalidAPIKeyAcceptance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	projectRoot, err := filepath.Abs("../..")
	require.NoError(t, err)
	mainGo := filepath.Join(projectRoot, "main.go")

	// Isolated empty profile so we do not merge with a developer's global config.
	configPath := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(configPath, []byte(`profile = "default"

[default]
`), 0o600))

	// Generous enough for every retry below, including `go run` compiling each
	// time: a deadline sized for one attempt would cut the retries short and
	// reintroduce the flake it is there to absorb.
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()

	invalidKey := "hk_test_ci_invalid_accept01" // valid shape, not a real key
	args := []string{"run", mainGo,
		"--hookdeck-config", configPath,
		"--log-level", "error",
		"ci", "--api-key", invalidKey,
	}
	env := appendEnvOverride(os.Environ(), "HOOKDECK_CONFIG_FILE", configPath)
	env = appendEnvOverride(env, "HOOKDECK_CLI_TELEMETRY_DISABLED", "1")

	// This test asserts on the shape of an authentication failure, so a
	// transport failure is not a result it can read. POST /cli-auth/ci answers
	// 502 often enough to have reddened this build three times in one day, and
	// a gateway error is not an auth outcome at all. Retry it the way
	// CLIRunner.Run does - this test builds its own exec.Cmd, so it does not go
	// through that path and inherited no retry.
	var stdout, stderr bytes.Buffer
	var elapsed time.Duration
	for attempt := 1; attempt <= acceptance502MaxAttempts; attempt++ {
		stdout.Reset()
		stderr.Reset()

		run := exec.CommandContext(ctx, "go", args...)
		run.Dir = projectRoot
		run.Env = env
		run.Stdout = &stdout
		run.Stderr = &stderr

		start := time.Now()
		err = run.Run()
		elapsed = time.Since(start)

		if !combinedOutputLooksLikeHTTP502(stdout.String(), stderr.String()) {
			break
		}
		if attempt < acceptance502MaxAttempts {
			t.Logf("acceptance: Hookdeck API transient HTTP 502 on cli-auth/ci; retrying (attempt %d/%d)", attempt, acceptance502MaxAttempts)
			time.Sleep(acceptance502RetryDelay)
		}
	}

	require.Error(t, err, "ci with bogus API key must fail")
	require.Less(t, elapsed, 30*time.Second, "ci should fail quickly without waiting for interactive login; took %v", elapsed)

	combined := stdout.String() + "\n" + stderr.String()
	if combinedOutputLooksLikeHTTP502(stdout.String(), stderr.String()) {
		t.Skipf("Hookdeck API returned HTTP 502 on every attempt; this test cannot observe an auth failure through a gateway error")
	}
	require.Contains(t, combined, "Authentication failed",
		"expected friendly auth message; stdout=%q stderr=%q", stdout.String(), stderr.String())

	// hookdeck ci uses POST /cli-auth/ci only — it must never start the interactive
	// browser/device login flow used by hookdeck login (pkg/login/client_login.go).
	for _, phrase := range []string{
		"Press Enter to open the browser",
		"To authenticate with Hookdeck, please go to:",
		"Your saved API key is no longer valid",
		"Starting browser sign-in",
		"Waiting for confirmation",
	} {
		require.NotContains(t, combined, phrase,
			"ci with invalid key must not trigger browser login; saw disallowed phrase %q in stdout=%q stderr=%q",
			phrase, stdout.String(), stderr.String())
	}
}
