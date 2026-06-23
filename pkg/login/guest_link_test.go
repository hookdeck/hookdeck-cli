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
	"github.com/stretchr/testify/require"
)

func TestRefreshGuestSigninLink_updatesProfileOnSuccess(t *testing.T) {
	configpkg.ResetAPIClientForTesting()
	t.Cleanup(configpkg.ResetAPIClientForTesting)

	signin_link_hits := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/cli/guest/signin-link") {
			signin_link_hits++
			user, pass, ok := r.BasicAuth()
			require.True(t, ok)
			require.Equal(t, "hk_test_guest_refresh_key12", user)
			require.Empty(t, pass)
			body, err := json.Marshal(map[string]string{
				"id":         "usr_guest_refresh",
				"link":       "https://api.example.test/signin/guest?token=fresh_token",
				"expires_at": "2099-01-01T00:00:00.000Z",
			})
			require.NoError(t, err)
			_, _ = w.Write(body)
			return
		}
		t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
	}))
	t.Cleanup(ts.Close)

	configPath := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(configPath, []byte(`profile = "default"

[default]
api_key = "hk_test_guest_refresh_key12"
guest_url = "https://api.example.test/signin/guest?token=stale_token"
guest_user_id = "usr_guest_old"
`), 0o600))

	cfg, err := configpkg.LoadConfigFromFile(configPath)
	require.NoError(t, err)
	cfg.APIBaseURL = ts.URL
	cfg.LogLevel = "error"
	cfg.TelemetryDisabled = true

	got := RefreshGuestSigninLink(cfg)
	require.Equal(t, 1, signin_link_hits)
	require.Equal(t, "https://api.example.test/signin/guest?token=fresh_token", got)
	require.Equal(t, got, cfg.Profile.GuestURL)
	require.Equal(t, "usr_guest_refresh", cfg.Profile.GuestUserID)

	reloaded, err := os.ReadFile(configPath)
	require.NoError(t, err)
	require.Contains(t, string(reloaded), "fresh_token")
	require.Contains(t, string(reloaded), "usr_guest_refresh")
}

func TestRefreshGuestSigninLink_fallsBackToSavedURLOnAPIError(t *testing.T) {
	configpkg.ResetAPIClientForTesting()
	t.Cleanup(configpkg.ResetAPIClientForTesting)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/cli/guest/signin-link") {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"server boom"}`))
			return
		}
		t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
	}))
	t.Cleanup(ts.Close)

	saved_url := "https://api.example.test/signin/guest?token=stale_token"
	configPath := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(configPath, []byte(`profile = "default"

[default]
api_key = "hk_test_guest_refresh_key12"
guest_url = "`+saved_url+`"
guest_user_id = "usr_guest_old"
`), 0o600))

	cfg, err := configpkg.LoadConfigFromFile(configPath)
	require.NoError(t, err)
	cfg.APIBaseURL = ts.URL
	cfg.LogLevel = "error"
	cfg.TelemetryDisabled = true

	got := RefreshGuestSigninLink(cfg)
	require.Equal(t, saved_url, got)
	require.Equal(t, saved_url, cfg.Profile.GuestURL)
	require.Equal(t, "usr_guest_old", cfg.Profile.GuestUserID)
}

func TestRefreshGuestSigninLink_preservesSavedURLOnEmptyLink(t *testing.T) {
	configpkg.ResetAPIClientForTesting()
	t.Cleanup(configpkg.ResetAPIClientForTesting)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/cli/guest/signin-link") {
			body, err := json.Marshal(map[string]string{
				"id":         "usr_guest_refresh",
				"link":       "",
				"expires_at": "2099-01-01T00:00:00.000Z",
			})
			require.NoError(t, err)
			_, _ = w.Write(body)
			return
		}
		t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
	}))
	t.Cleanup(ts.Close)

	saved_url := "https://api.example.test/signin/guest?token=stale_token"
	configPath := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(configPath, []byte(`profile = "default"

[default]
api_key = "hk_test_guest_refresh_key12"
guest_url = "`+saved_url+`"
guest_user_id = "usr_guest_old"
`), 0o600))

	cfg, err := configpkg.LoadConfigFromFile(configPath)
	require.NoError(t, err)
	cfg.APIBaseURL = ts.URL
	cfg.LogLevel = "error"
	cfg.TelemetryDisabled = true

	got := RefreshGuestSigninLink(cfg)
	require.Equal(t, saved_url, got)
	require.Equal(t, saved_url, cfg.Profile.GuestURL)
	require.Equal(t, "usr_guest_old", cfg.Profile.GuestUserID)

	reloaded, err := os.ReadFile(configPath)
	require.NoError(t, err)
	require.Contains(t, string(reloaded), "stale_token")
	require.Contains(t, string(reloaded), "usr_guest_old")
}

func TestRefreshGuestSigninLink_returnsEmptyWithoutGuestProfile(t *testing.T) {
	cfg := &configpkg.Config{
		LogLevel:          "error",
		TelemetryDisabled: true,
	}
	cfg.Profile = configpkg.Profile{
		Name:   "default",
		APIKey: "hk_test_guest_refresh_key12",
		Config: cfg,
	}

	require.Empty(t, RefreshGuestSigninLink(cfg))

	cfg.Profile.GuestURL = "https://example.test/signin/guest?token=x"
	cfg.Profile.APIKey = ""
	require.Empty(t, RefreshGuestSigninLink(cfg))
}
