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
	"github.com/stretchr/testify/require"
)

// TestLogin_validateNonUnauthorizedStillFails verifies that credential
// verification errors other than 401 are returned immediately (no browser flow).
func TestLogin_validateNonUnauthorizedStillFails(t *testing.T) {
	configpkg.ResetAPIClient()
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
	configpkg.ResetAPIClient()
	t.Cleanup(configpkg.ResetAPIClientForTesting)

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
				"team_mode":         "gateway",
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
