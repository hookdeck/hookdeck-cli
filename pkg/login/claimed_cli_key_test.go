package login

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	configpkg "github.com/hookdeck/hookdeck-cli/pkg/config"
)

func TestConfigureFromClaimedCliKey_guestProfileReplacesCredentials(t *testing.T) {
	configpkg.ResetAPIClientForTesting()
	t.Cleanup(configpkg.ResetAPIClientForTesting)

	onboardingKey := "hk_test_onboard_abcdefghij"
	var serverURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.True(t, strings.HasSuffix(r.URL.Path, "/cli-auth/validate"))

		body, err := json.Marshal(map[string]string{
			"user_id":           "usr_platform",
			"user_name":         "Platform User",
			"user_email":        "platform@example.com",
			"organization_name": "Acme",
			"organization_id":   "org_1",
			"team_id":           "tm_gateway",
			"team_name_no_org":  "Production",
			"team_product":      "event_gateway",
			"client_id":         "cl_onboard",
		})
		require.NoError(t, err)
		_, _ = w.Write(body)
	}))
	serverURL = ts.URL
	t.Cleanup(ts.Close)

	configPath := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(configPath, []byte(`profile = "default"

[default]
api_key = "hk_test_guestkey_abcdefghij"
guest_url = "https://console.test/signin/guest?token=abc"
`), 0o600))

	cfg, err := configpkg.LoadConfigFromFile(configPath)
	require.NoError(t, err)
	cfg.APIBaseURL = serverURL
	cfg.LogLevel = "error"
	cfg.TelemetryDisabled = true

	err = ConfigureFromClaimedCliKey(cfg, onboardingKey)
	require.NoError(t, err)
	require.Equal(t, onboardingKey, cfg.Profile.APIKey)
	require.Equal(t, "tm_gateway", cfg.Profile.ProjectId)
	require.Empty(t, cfg.Profile.GuestURL)

	rewritten, err := os.ReadFile(configPath)
	require.NoError(t, err)
	require.Contains(t, string(rewritten), onboardingKey)
	require.Contains(t, string(rewritten), "tm_gateway")
	require.NotContains(t, string(rewritten), "usr_guest")
}

func TestConfigureFromClaimedCliKey_validateFailureDoesNotStartDeviceLogin(t *testing.T) {
	configpkg.ResetAPIClientForTesting()
	t.Cleanup(configpkg.ResetAPIClientForTesting)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		require.True(t, strings.HasSuffix(r.URL.Path, "/cli-auth/validate"))
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(ts.Close)

	cfg := &configpkg.Config{
		APIBaseURL:        ts.URL,
		LogLevel:          "error",
		TelemetryDisabled: true,
	}
	cfg.Profile = configpkg.Profile{
		Name:     "default",
		GuestURL: "https://console.test/guest",
	}

	err := ConfigureFromClaimedCliKey(cfg, "hk_test_invalid_abcdefghij")
	require.Error(t, err)
}
