package login

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	configpkg "github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/project"
	"github.com/stretchr/testify/require"
)

// TestLogin_validateNonUnauthorizedStillFails verifies that credential
// verification errors other than 401 are returned immediately (no browser flow).
func TestLogin_validateNonUnauthorizedStillFails(t *testing.T) {
	configpkg.ResetAPIClientForTesting()
	t.Cleanup(configpkg.ResetAPIClientForTesting)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/cli-auth/validate") {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"server boom"}`))
			return
		}
		t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
	}))
	t.Cleanup(ts.Close)

	cfg := &configpkg.Config{
		APIBaseURL:        ts.URL,
		DeviceName:        "test-device",
		LogLevel:          "error",
		TelemetryDisabled: true,
	}
	cfg.Profile = configpkg.Profile{
		Name:   "default",
		APIKey: "hk_test_123456789012",
		Config: cfg,
	}

	err := Login(cfg, strings.NewReader("\n"))
	require.Error(t, err)
}

// TestLogin_unauthorizedValidateStartsBrowserFlow checks that a 401 from
// validate is followed by POST /cli-auth (browser login), then a successful poll.
func TestLogin_unauthorizedValidateStartsBrowserFlow(t *testing.T) {
	configpkg.ResetAPIClientForTesting()
	t.Cleanup(configpkg.ResetAPIClientForTesting)

	// Browser sign-in is only reachable with a terminal, so say so explicitly.
	// This test used to pass without stubbing it, because a rejected key fell
	// into the browser flow whether or not anyone could complete it - see
	// TestLogin_rejectedKeyHeadlessFailsFast, which covers the other side.
	oldStdinIsTerminal := stdinIsTerminal
	stdinIsTerminal = func() bool { return true }
	t.Cleanup(func() { stdinIsTerminal = oldStdinIsTerminal })

	oldCan := canOpenBrowser
	oldOpen := openBrowser
	canOpenBrowser = func() bool { return false }
	openBrowser = func(string) error { return nil }
	t.Cleanup(func() {
		canOpenBrowser = oldCan
		openBrowser = oldOpen
	})

	pollHits := 0
	var serverURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/cli-auth/validate"):
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("Unauthorized"))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/cli-auth"):
			pollURL := serverURL + hookdeck.APIPathPrefix + "/cli-auth/poll?key=pollkey"
			body, err := json.Marshal(map[string]string{
				"browser_url": "https://example.test/auth",
				"poll_url":    pollURL,
			})
			require.NoError(t, err)
			_, _ = w.Write(body)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/cli-auth/poll"):
			pollHits++
			resp := map[string]interface{}{
				"claimed":           true,
				"key":               "hk_test_newkey_abcdefghij",
				"team_id":           "tm_1",
				"team_type":         "event_gateway",
				"team_name":         "Proj",
				"user_name":         "U",
				"user_email":        "u@example.com",
				"organization_name": "Org",
				"organization_id":   "org_1",
				"client_id":         "cl_1",
			}
			enc, err := json.Marshal(resp)
			require.NoError(t, err)
			_, _ = w.Write(enc)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	serverURL = ts.URL
	t.Cleanup(ts.Close)

	configPath := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(configPath, []byte(`profile = "default"

[default]
api_key = "hk_test_oldkey_abcdefghij"
`), 0o600))

	cfg, err := configpkg.LoadConfigFromFile(configPath)
	require.NoError(t, err)
	cfg.APIBaseURL = ts.URL
	cfg.DeviceName = "test-device"
	cfg.LogLevel = "error"
	cfg.TelemetryDisabled = true

	err = Login(cfg, strings.NewReader("\n"))
	require.NoError(t, err)
	require.Equal(t, 1, pollHits, "poll should run once with immediate claimed=true")
	require.Equal(t, "hk_test_newkey_abcdefghij", cfg.Profile.APIKey)
}

// TestLogin_guestProfileWithValidKeyStartsGuestUpgrade verifies that a guest Console
// profile with a still-valid API key opens a refreshed guest signup link and waits for
// the same key to validate as a permanent user.
func TestLogin_guestProfileWithValidKeyStartsGuestUpgrade(t *testing.T) {
	configpkg.ResetAPIClientForTesting()
	t.Cleanup(configpkg.ResetAPIClientForTesting)

	oldCan := canOpenBrowser
	oldOpen := openBrowser
	canOpenBrowser = func() bool { return false }
	openBrowser = func(string) error { return nil }
	t.Cleanup(func() {
		canOpenBrowser = oldCan
		openBrowser = oldOpen
	})

	validateHits := 0
	guestRefreshHits := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/cli-auth/validate"):
			validateHits++
			resp := map[string]interface{}{
				"user_id":           "usr_guest",
				"user_name":         "Guest",
				"user_email":        "guest@example.com",
				"user_is_guest":     validateHits == 1,
				"organization_name": "Org",
				"organization_id":   "org_1",
				"team_id":           "tm_console",
				"team_name_no_org":  "Sandbox",
				"team_type":         "console",
				"client_id":         "cl_guest",
			}
			enc, err := json.Marshal(resp)
			require.NoError(t, err)
			_, _ = w.Write(enc)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/cli/guest"):
			guestRefreshHits++
			user, pass, ok := r.BasicAuth()
			require.True(t, ok)
			require.Equal(t, "hk_test_guestkey_abcdefghij", user)
			require.Empty(t, pass)
			var body map[string]string
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			require.Equal(t, "signup", body["link_context"])
			enc, err := json.Marshal(map[string]string{
				"id":   "usr_guest",
				"key":  "hk_test_guestkey_abcdefghij",
				"link": "https://example.test/signin/guest?token=fresh&redirect=signup",
			})
			require.NoError(t, err)
			_, _ = w.Write(enc)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	t.Cleanup(ts.Close)

	configPath := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(configPath, []byte(`profile = "default"

[default]
api_key = "hk_test_guestkey_abcdefghij"
guest_url = "https://console.test/signin/guest?token=abc"
`), 0o600))

	cfg, err := configpkg.LoadConfigFromFile(configPath)
	require.NoError(t, err)
	cfg.APIBaseURL = ts.URL
	cfg.DeviceName = "test-device"
	cfg.LogLevel = "error"
	cfg.TelemetryDisabled = true

	err = Login(cfg, strings.NewReader("\n"))
	require.NoError(t, err)
	require.Equal(t, 2, validateHits)
	require.Equal(t, 1, guestRefreshHits)
	require.Equal(t, "hk_test_guestkey_abcdefghij", cfg.Profile.APIKey)
	require.Empty(t, cfg.Profile.GuestURL)
}

func TestLogin_ciKeyHeadlessFailsFast(t *testing.T) {
	configpkg.ResetAPIClientForTesting()
	t.Cleanup(configpkg.ResetAPIClientForTesting)

	oldStdinIsTerminal := stdinIsTerminal
	stdinIsTerminal = func() bool { return false }
	t.Cleanup(func() { stdinIsTerminal = oldStdinIsTerminal })

	var sawCLIAuthPost bool
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/cli-auth/validate") {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(hookdeck.ValidateAPIKeyResponse{
				ProjectID:        "tm_ci",
				ProjectType:      "event_gateway",
				OrganizationName: "Org",
				OrganizationID:   "org_1",
				ProjectName:      "CI",
			})
			return
		}
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/cli-auth") {
			sawCLIAuthPost = true
		}
		t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
	}))
	t.Cleanup(ts.Close)

	configPath := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(configPath, []byte(`profile = "default"

[default]
api_key = "hk_test_cikey_abcdefghij"
`), 0o600))

	cfg, err := configpkg.LoadConfigFromFile(configPath)
	require.NoError(t, err)
	cfg.APIBaseURL = ts.URL
	cfg.DeviceName = "test-device"
	cfg.LogLevel = "error"
	cfg.TelemetryDisabled = true

	err = Login(cfg, strings.NewReader("\n"))
	require.ErrorIs(t, err, project.ErrProjectScopedCredentials)
	require.False(t, sawCLIAuthPost)
}

func TestLogin_ciKeyStartsBrowserFlow(t *testing.T) {
	configpkg.ResetAPIClientForTesting()
	t.Cleanup(configpkg.ResetAPIClientForTesting)

	oldStdinIsTerminal := stdinIsTerminal
	stdinIsTerminal = func() bool { return true }
	t.Cleanup(func() { stdinIsTerminal = oldStdinIsTerminal })

	oldCan := canOpenBrowser
	oldOpen := openBrowser
	canOpenBrowser = func() bool { return false }
	openBrowser = func(string) error { return nil }
	t.Cleanup(func() {
		canOpenBrowser = oldCan
		openBrowser = oldOpen
	})

	var sawCLIAuthPost bool
	pollHits := 0
	var serverURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/cli-auth/validate"):
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(hookdeck.ValidateAPIKeyResponse{
				ProjectID:        "tm_ci",
				ProjectType:      "event_gateway",
				OrganizationName: "Org",
				OrganizationID:   "org_1",
				ProjectName:      "CI",
			})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/cli-auth"):
			sawCLIAuthPost = true
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
				"key":               "hk_test_userkey_abcdefghij",
				"user_id":           "usr_1",
				"team_id":           "tm_1",
				"team_type":         "event_gateway",
				"team_name":         "Proj",
				"user_name":         "U",
				"user_email":        "u@example.com",
				"organization_name": "Org",
				"organization_id":   "org_1",
				"client_id":         "cl_1",
			}
			enc, encErr := json.Marshal(resp)
			require.NoError(t, encErr)
			_, _ = w.Write(enc)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	serverURL = ts.URL
	t.Cleanup(ts.Close)

	configPath := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(configPath, []byte(`profile = "default"

[default]
api_key = "hk_test_cikey_abcdefghij"
`), 0o600))

	cfg, err := configpkg.LoadConfigFromFile(configPath)
	require.NoError(t, err)
	cfg.APIBaseURL = ts.URL
	cfg.DeviceName = "test-device"
	cfg.LogLevel = "error"
	cfg.TelemetryDisabled = true

	err = Login(cfg, strings.NewReader("\n"))
	require.NoError(t, err)
	require.True(t, sawCLIAuthPost)
	require.Equal(t, 1, pollHits)
	require.Equal(t, "hk_test_userkey_abcdefghij", cfg.Profile.APIKey)
}

// TestLogin_rejectedKeyHeadlessFailsFast covers the other way into browser
// sign-in. TestLogin_ciKeyHeadlessFailsFast covers a key that is valid but
// project-scoped; this covers one the API rejects outright.
//
// Without the guard, login announced "Starting browser sign-in...", walked past
// the Enter prompt because there is no terminal to read from, and then polled
// for a confirmation that could never arrive. CI spent 248 seconds on a mistyped
// key before giving up. The failure is instant and says what to check.
func TestLogin_rejectedKeyHeadlessFailsFast(t *testing.T) {
	configpkg.ResetAPIClientForTesting()
	t.Cleanup(configpkg.ResetAPIClientForTesting)

	oldStdinIsTerminal := stdinIsTerminal
	stdinIsTerminal = func() bool { return false }
	t.Cleanup(func() { stdinIsTerminal = oldStdinIsTerminal })

	var sawCLIAuthPost bool
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/cli-auth/validate") {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("Unauthorized"))
			return
		}
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/cli-auth") {
			// Reaching here means we started the browser flow anyway, which is
			// the bug: there is nobody to complete it.
			sawCLIAuthPost = true
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"browser_url": "https://example.test", "poll_url": "https://example.test/poll"})
			return
		}
		t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
	}))
	t.Cleanup(ts.Close)

	configPath := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(configPath, []byte(`profile = "default"

[default]
api_key = "hk_test_rejected_abcdefghij"
`), 0o600))

	cfg, err := configpkg.LoadConfigFromFile(configPath)
	require.NoError(t, err)
	cfg.APIBaseURL = ts.URL
	cfg.DeviceName = "test-device"
	cfg.LogLevel = "error"
	cfg.TelemetryDisabled = true

	err = Login(cfg, strings.NewReader("\n"))
	require.ErrorIs(t, err, ErrRejectedKeyNoTerminal)
	require.False(t, sawCLIAuthPost, "browser sign-in must not be started without a terminal")
	require.Contains(t, err.Error(), "CLI key", "the error should say what kind of key is expected")
}

// TestLogin_rejectedKeyNoBrowserStillSignsIn covers the third environment in the
// matrix: no terminal, no browser, stale key. waitForLoginSession prints the URL
// and polls in that case without reading stdin, so a human opening it elsewhere
// completes the flow - a Linux container invoked without a TTY, typically.
//
// Reported in review: the guard had equated "stdin is not interactive" with
// "nobody can authenticate", so this was refused while the same environment with
// no saved key at all succeeded.
func TestLogin_rejectedKeyNoBrowserStillSignsIn(t *testing.T) {
	configpkg.ResetAPIClientForTesting()
	t.Cleanup(configpkg.ResetAPIClientForTesting)

	oldStdinIsTerminal := stdinIsTerminal
	stdinIsTerminal = func() bool { return false }
	t.Cleanup(func() { stdinIsTerminal = oldStdinIsTerminal })

	oldCan := canOpenBrowser
	canOpenBrowser = func() bool { return false }
	t.Cleanup(func() { canOpenBrowser = oldCan })

	var sawCLIAuthPost bool
	var serverURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/cli-auth/validate"):
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("Unauthorized"))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/cli-auth"):
			sawCLIAuthPost = true
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{
				"browser_url": "https://example.test/auth",
				"poll_url":    serverURL + hookdeck.APIPathPrefix + "/cli-auth/poll?key=k",
			})
		case strings.Contains(r.URL.Path, "/cli-auth/poll"):
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"claimed": true, "key": "hk_test_newkey_abcdefghij",
				"team_id": "tm_x", "team_type": "event_gateway",
				"team_name": "P", "user_name": "U", "user_email": "u@example.test",
				"organization_name": "O", "organization_id": "org_x", "client_id": "cl_x",
			})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	serverURL = ts.URL
	t.Cleanup(ts.Close)

	configPath := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(configPath, []byte(`profile = "default"

[default]
api_key = "hk_test_stale_abcdefghij"
`), 0o600))

	cfg, err := configpkg.LoadConfigFromFile(configPath)
	require.NoError(t, err)
	cfg.APIBaseURL = ts.URL
	cfg.DeviceName = "test-device"
	cfg.LogLevel = "error"
	cfg.TelemetryDisabled = true

	require.NoError(t, Login(cfg, strings.NewReader("")),
		"a URL-only sign-in needs no terminal and must not be refused")
	require.True(t, sawCLIAuthPost, "should have started the device flow")
}
