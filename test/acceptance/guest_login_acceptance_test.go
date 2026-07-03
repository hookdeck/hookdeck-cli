//go:build guest

package acceptance

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func runGuestLoginCLI(t *testing.T, projectRoot, configPath, serverURL string, extraArgs ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	mainGo := filepath.Join(projectRoot, "main.go")
	args := append([]string{"run", mainGo,
		"--api-base", serverURL,
		"--hookdeck-config", configPath,
		"--log-level", "error",
		"login",
	}, extraArgs...)

	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = projectRoot
	env := appendEnvOverride(os.Environ(), "HOOKDECK_CONFIG_FILE", configPath)
	env = appendEnvOverride(env, "SSH_CONNECTION", "acceptance-guest-login-mock")
	env = appendEnvOverride(env, "HOOKDECK_CLI_TELEMETRY_DISABLED", "1")
	cmd.Stdin = strings.NewReader("\n")
	cmd.Env = env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	require.NoError(t, err, "stdout=%q stderr=%q", stdout.String(), stderr.String())
}

func newGuestLoginMock(t *testing.T, assertBody func(map[string]interface{}), browserURL string) (*httptest.Server, string) {
	t.Helper()
	var serverURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/cli-auth/validate"):
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("Unauthorized"))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/cli/guest"):
			body, encErr := json.Marshal(map[string]string{
				"id":   "usr_guest_accept",
				"key":  "hk_guest_accept_key",
				"link": "https://example.test/signin/guest?token=guest",
			})
			require.NoError(t, encErr)
			_, _ = w.Write(body)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/cli-auth"):
			raw, readErr := io.ReadAll(r.Body)
			require.NoError(t, readErr)
			var payload map[string]interface{}
			require.NoError(t, json.Unmarshal(raw, &payload))
			assertBody(payload)
			pollURL := serverURL + "/2025-07-01/cli-auth/poll?key=pollkey"
			respBody, encErr := json.Marshal(map[string]string{
				"browser_url": browserURL,
				"poll_url":    pollURL,
			})
			require.NoError(t, encErr)
			_, _ = w.Write(respBody)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/cli-auth/poll"):
			resp := map[string]interface{}{
				"claimed":           true,
				"key":               "hk_test_guest_claimed",
				"team_id":           "tm_guest",
				"team_mode":         "console",
				"team_name":         "Guest Sandbox",
				"user_name":         "Guest",
				"user_email":        "guest@example.com",
				"organization_name": "GuestOrg",
				"organization_id":   "org_guest",
				"client_id":         "cl_guest",
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
	return ts, serverURL
}

func guestProfileConfig() string {
	return `profile = "default"

[default]
api_key = "hk_test_stale_guest01"
guest_url = "https://example.test/signin/guest?token=guest"
`
}

func loggedOutProfileConfig() string {
	return `profile = "default"

[default]
`
}

func TestGuestLoginDefaultClaimGuestAcceptance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	projectRoot, err := filepath.Abs("../..")
	require.NoError(t, err)

	configPath := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(configPath, []byte(guestProfileConfig()), 0o600))

	validateHits := 0
	guestRefreshHits := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/cli-auth/validate"):
			validateHits++
			body, encErr := json.Marshal(map[string]interface{}{
				"user_id":           "usr_guest_accept",
				"user_name":         "Guest",
				"user_email":        "guest@example.com",
				"user_is_guest":     validateHits == 1,
				"organization_name": "GuestOrg",
				"organization_id":   "org_guest",
				"team_id":           "tm_guest",
				"team_name_no_org":  "Guest Sandbox",
				"team_mode":         "console",
				"client_id":         "cl_guest",
			})
			require.NoError(t, encErr)
			_, _ = w.Write(body)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/cli/guest"):
			guestRefreshHits++
			user, pass, ok := r.BasicAuth()
			require.True(t, ok)
			require.Equal(t, "hk_test_stale_guest01", user)
			require.Empty(t, pass)

			var payload map[string]interface{}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			require.Equal(t, "signup", payload["link_context"])

			body, encErr := json.Marshal(map[string]string{
				"id":   "usr_guest_accept",
				"key":  "hk_test_stale_guest01",
				"link": "https://example.test/signin/guest?token=fresh&redirect=signup",
			})
			require.NoError(t, encErr)
			_, _ = w.Write(body)
		default:
			t.Fatalf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	serverURL := ts.URL
	t.Cleanup(ts.Close)

	runGuestLoginCLI(t, projectRoot, configPath, serverURL)
	require.Equal(t, 2, validateHits)
	require.Equal(t, 1, guestRefreshHits)
}

func TestGuestLoginAfterLogoutOmitsGuestCredentialsAcceptance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	projectRoot, err := filepath.Abs("../..")
	require.NoError(t, err)

	configPath := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(configPath, []byte(loggedOutProfileConfig()), 0o600))

	ts, serverURL := newGuestLoginMock(t, func(body map[string]interface{}) {
		require.NotContains(t, body, "auth_intent")
		require.NotContains(t, body, "guest_user_id")
		require.NotContains(t, body, "guest_api_key")
	}, "https://example.test/signin?redirect=%2Fcli-auth%2Fkey")

	runGuestLoginCLI(t, projectRoot, configPath, serverURL)
	_ = ts
}
