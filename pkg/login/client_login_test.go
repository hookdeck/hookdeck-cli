package login

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

	// Stated explicitly: this test used to pass without stubbing it, because a
	// rejected key fell into the browser flow regardless of who could finish it.
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

// TestLogin_rejectedKeyHeadlessFailsFast: a key the API rejects, with no
// terminal and no other way to complete sign-in. Without the guard this walked
// past the Enter prompt and polled for a confirmation nobody could give - 248
// seconds in CI.
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

// TestLogin_rejectedKeyNoBrowserStillSignsIn: no terminal, no browser, stale key.
// waitForLoginSession prints the URL and polls without reading stdin, so this
// completes. The guard once refused it while the same environment with no saved
// key succeeded.
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

// TestLogin_noCredentialsHeadlessFailsFast is the bug this guard exists for: an
// empty config skipped the saved-key block entirely, so nothing checked for a
// terminal before waitForLoginSession printed "Press Enter", read EOF from
// /dev/null, opened a real browser window and polled forever.
func TestLogin_noCredentialsHeadlessFailsFast(t *testing.T) {
	configpkg.ResetAPIClientForTesting()
	t.Cleanup(configpkg.ResetAPIClientForTesting)

	oldStdinIsTerminal := stdinIsTerminal
	stdinIsTerminal = func() bool { return false }
	t.Cleanup(func() { stdinIsTerminal = oldStdinIsTerminal })

	// A machine that could open a browser, which is exactly what makes this
	// dangerous: the window opens on somebody's desktop unasked.
	oldCan := canOpenBrowser
	canOpenBrowser = func() bool { return true }
	t.Cleanup(func() { canOpenBrowser = oldCan })

	for _, key := range []string{"SSH_TTY", "SSH_CONNECTION", "SSH_CLIENT"} {
		t.Setenv(key, "")
		require.NoError(t, os.Unsetenv(key))
	}

	browserOpens := 0
	oldOpen := openBrowser
	openBrowser = func(string) error {
		browserOpens++
		return nil
	}
	t.Cleanup(func() { openBrowser = oldOpen })

	var sawCLIAuthPost bool
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/cli-auth") {
			sawCLIAuthPost = true
		}
		t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
	}))
	t.Cleanup(ts.Close)

	configPath := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(configPath, []byte(""), 0o600))

	cfg, err := configpkg.LoadConfigFromFile(configPath)
	require.NoError(t, err)
	cfg.APIBaseURL = ts.URL
	cfg.DeviceName = "test-device"
	cfg.LogLevel = "error"
	cfg.TelemetryDisabled = true
	require.Empty(t, cfg.Profile.APIKey, "the repro starts from an empty config")

	done := make(chan error, 1)
	go func() { done <- Login(cfg, strings.NewReader("")) }()

	select {
	case err = <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Login did not return; it is waiting for a confirmation nobody can give")
	}

	require.ErrorIs(t, err, ErrNoCredentialsNoTerminal)
	require.Equal(t, 0, browserOpens, "no browser window without a terminal to have asked for one")
	require.False(t, sawCLIAuthPost, "browser sign-in must not be started without a terminal")
	require.Contains(t, err.Error(), "hookdeck ci --api-key")
	require.Contains(t, err.Error(), "--cli-key")
	require.Contains(t, err.Error(), "HOOKDECK_API_KEY")
}

// TestLogin_noCredentialsNoBrowserStillSignsIn: no key, no terminal, no browser.
// waitForLoginSession prints the URL and polls without reading stdin, so a human
// elsewhere can finish it. The guard must not take this branch out.
func TestLogin_noCredentialsNoBrowserStillSignsIn(t *testing.T) {
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
	require.NoError(t, os.WriteFile(configPath, []byte(""), 0o600))

	cfg, err := configpkg.LoadConfigFromFile(configPath)
	require.NoError(t, err)
	cfg.APIBaseURL = ts.URL
	cfg.DeviceName = "test-device"
	cfg.LogLevel = "error"
	cfg.TelemetryDisabled = true

	require.NoError(t, Login(cfg, strings.NewReader("")),
		"a URL-only sign-in needs no terminal and must not be refused")
	require.True(t, sawCLIAuthPost, "should have started the device flow")
	require.Equal(t, "hk_test_newkey_abcdefghij", cfg.Profile.APIKey)
}

// TestLogin_noCredentialsWithTerminalOpensBrowser pins the unchanged path: with
// a terminal on stdin, Enter still opens the browser.
func TestLogin_noCredentialsWithTerminalOpensBrowser(t *testing.T) {
	configpkg.ResetAPIClientForTesting()
	t.Cleanup(configpkg.ResetAPIClientForTesting)

	oldStdinIsTerminal := stdinIsTerminal
	stdinIsTerminal = func() bool { return true }
	t.Cleanup(func() { stdinIsTerminal = oldStdinIsTerminal })

	oldCan := canOpenBrowser
	canOpenBrowser = func() bool { return true }
	t.Cleanup(func() { canOpenBrowser = oldCan })

	for _, key := range []string{"SSH_TTY", "SSH_CONNECTION", "SSH_CLIENT"} {
		t.Setenv(key, "")
		require.NoError(t, os.Unsetenv(key))
	}

	var openedURL string
	oldOpen := openBrowser
	openBrowser = func(u string) error {
		openedURL = u
		return nil
	}
	t.Cleanup(func() { openBrowser = oldOpen })

	var serverURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/cli-auth"):
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
	require.NoError(t, os.WriteFile(configPath, []byte(""), 0o600))

	cfg, err := configpkg.LoadConfigFromFile(configPath)
	require.NoError(t, err)
	cfg.APIBaseURL = ts.URL
	cfg.DeviceName = "test-device"
	cfg.LogLevel = "error"
	cfg.TelemetryDisabled = true

	// Capture stdout: the prompt had no trailing newline, so it ran straight
	// into the "Waiting for confirmation..." spinner on the same line.
	stdoutFile, err := os.CreateTemp(t.TempDir(), "stdout")
	require.NoError(t, err)
	oldStdout := os.Stdout
	os.Stdout = stdoutFile
	t.Cleanup(func() { os.Stdout = oldStdout })

	require.NoError(t, Login(cfg, strings.NewReader("\n")))

	os.Stdout = oldStdout
	require.NoError(t, stdoutFile.Close())
	out, err := os.ReadFile(stdoutFile.Name())
	require.NoError(t, err)
	require.Contains(t, string(out), "Press Enter to open the browser (^C to quit)\n",
		"the prompt must end its own line, not run into the spinner")

	require.Equal(t, "https://example.test/auth", openedURL,
		"with a terminal the Enter-then-browser branch is unchanged")
	require.Equal(t, "hk_test_newkey_abcdefghij", cfg.Profile.APIKey)
}
