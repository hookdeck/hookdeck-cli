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
				"team_mode":         "gateway",
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

	cmd := exec.CommandContext(ctx, "go", append([]string{"run", mainGo,
		"--api-base", ts.URL,
		"--hookdeck-config", configPath,
		"--log-level", "error",
		"login",
	})...)
	cmd.Dir = projectRoot
	env := appendEnvOverride(os.Environ(), "HOOKDECK_CONFIG_FILE", configPath)
	env = appendEnvOverride(env, "SSH_CONNECTION", "acceptance-login-mock")
	env = appendEnvOverride(env, "HOOKDECK_CLI_TELEMETRY_DISABLED", "1")
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	require.NoError(t, err, "stdout=%q stderr=%q", stdout.String(), stderr.String())
	require.Contains(t, stdout.String(), "no longer valid", "user should see stale-key message")
	require.Equal(t, 1, pollHits, "mock should see exactly one poll after cli-auth")
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

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	invalidKey := "hk_test_ci_invalid_accept01" // valid shape, not a real key
	cmd := exec.CommandContext(ctx, "go", append([]string{"run", mainGo,
		"--hookdeck-config", configPath,
		"--log-level", "error",
		"ci", "--api-key", invalidKey,
	})...)
	cmd.Dir = projectRoot
	env := appendEnvOverride(os.Environ(), "HOOKDECK_CONFIG_FILE", configPath)
	env = appendEnvOverride(env, "HOOKDECK_CLI_TELEMETRY_DISABLED", "1")
	cmd.Env = env

	start := time.Now()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	require.Error(t, err, "ci with bogus API key must fail")
	elapsed := time.Since(start)
	require.Less(t, elapsed, 30*time.Second, "ci should fail quickly without waiting for interactive login; took %v", elapsed)

	combined := stdout.String() + "\n" + stderr.String()
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
