package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validateStub serves the project-agnostic cli-auth/validate endpoint, which is
// what resolves the project a supplied key belongs to.
func validateStub(t *testing.T, projectID, projectName string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != hookdeck.APIPathPrefix+"/cli-auth/validate" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(hookdeck.ValidateAPIKeyResponse{
			ProjectID:   projectID,
			ProjectType: "console",
			ProjectName: projectName,
		})
	}))
	t.Cleanup(server.Close)
	return server
}

func listenCmdWithCliKeyFlag(t *testing.T, key string) *listenCmd {
	t.Helper()
	lc := newListenCmd()
	require.NoError(t, lc.cmd.Flags().Set("cli-key", key))
	return lc
}

func TestApplyCliKey(t *testing.T) {
	t.Run("no-op when --cli-key was not supplied", func(t *testing.T) {
		old := Config
		t.Cleanup(func() { Config = old })
		config.ResetAPIClientForTesting()
		t.Cleanup(config.ResetAPIClientForTesting)

		// Points at a port nothing is listening on: if the guard is wrong and a
		// validate call is attempted, this fails rather than passing quietly.
		Config = config.Config{}
		Config.APIBaseURL = "http://127.0.0.1:1"
		Config.Profile.ProjectId = "existing_project"

		lc := newListenCmd()
		require.NoError(t, lc.applyCliKey(lc.cmd))
		assert.Equal(t, "existing_project", Config.Profile.ProjectId,
			"context must be untouched when the flag is absent")
	})

	// The bug this guards against: a key given on the command line was sent
	// alongside a project id read from a previous login, which belongs to a
	// different project, so every call failed with "invalid or expired".
	t.Run("adopts the key's own project over a stale stored one", func(t *testing.T) {
		old := Config
		t.Cleanup(func() { Config = old })
		config.ResetAPIClientForTesting()
		t.Cleanup(config.ResetAPIClientForTesting)

		server := validateStub(t, "project_from_key", "Sandbox")

		dir := t.TempDir()
		path := filepath.Join(dir, "config.toml")
		require.NoError(t, os.WriteFile(path, []byte(`profile = "default"

[default]
api_key = "sk_test_stored_key_1234"
project_id = "stale_project_from_previous_login"
`), 0600))

		cfg, err := config.LoadConfigFromFile(path)
		require.NoError(t, err)
		cfg.APIBaseURL = server.URL
		Config = *cfg

		lc := listenCmdWithCliKeyFlag(t, "sk_test_flag_key_5678")
		Config.Profile.APIKey = "sk_test_flag_key_5678"

		require.NoError(t, lc.applyCliKey(lc.cmd))

		assert.Equal(t, "project_from_key", Config.Profile.ProjectId,
			"the key's project must replace the stale one for this run")
	})

	// A short debugging session with a Console key should not cost someone the
	// login they already had.
	t.Run("does not overwrite an existing stored login", func(t *testing.T) {
		old := Config
		t.Cleanup(func() { Config = old })
		config.ResetAPIClientForTesting()
		t.Cleanup(config.ResetAPIClientForTesting)

		server := validateStub(t, "project_from_key", "Sandbox")

		dir := t.TempDir()
		path := filepath.Join(dir, "config.toml")
		stored := `profile = "default"

[default]
api_key = "sk_test_stored_key_1234"
project_id = "stored_project"
`
		require.NoError(t, os.WriteFile(path, []byte(stored), 0600))

		cfg, err := config.LoadConfigFromFile(path)
		require.NoError(t, err)
		cfg.APIBaseURL = server.URL
		Config = *cfg
		require.True(t, Config.HasStoredAPIKey, "fixture must represent an existing login")

		lc := listenCmdWithCliKeyFlag(t, "sk_test_flag_key_5678")
		Config.Profile.APIKey = "sk_test_flag_key_5678"

		require.NoError(t, lc.applyCliKey(lc.cmd))

		after, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, stored, string(after), "an existing login must be left on disk untouched")
	})

	// `--cli-key=` satisfies Changed but carries nothing to authenticate with.
	// It must fail locally rather than as an opaque auth error from the API.
	t.Run("rejects an explicitly empty --cli-key before calling the API", func(t *testing.T) {
		old := Config
		t.Cleanup(func() { Config = old })
		config.ResetAPIClientForTesting()
		t.Cleanup(config.ResetAPIClientForTesting)

		// A dead port: reaching the network at all fails this test.
		Config = config.Config{}
		Config.APIBaseURL = "http://127.0.0.1:1"

		lc := listenCmdWithCliKeyFlag(t, "")

		err := lc.applyCliKey(lc.cmd)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "--cli-key needs a value",
			"the error must name the flag, not surface as an auth failure")
	})

	t.Run("propagates a validation failure without writing", func(t *testing.T) {
		old := Config
		t.Cleanup(func() { Config = old })
		config.ResetAPIClientForTesting()
		t.Cleanup(config.ResetAPIClientForTesting)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message":"invalid api key"}`))
		}))
		t.Cleanup(server.Close)

		dir := t.TempDir()
		path := filepath.Join(dir, "config.toml")
		require.NoError(t, os.WriteFile(path, []byte("profile = \"default\"\n"), 0600))

		cfg, err := config.LoadConfigFromFile(path)
		require.NoError(t, err)
		cfg.APIBaseURL = server.URL
		Config = *cfg

		lc := listenCmdWithCliKeyFlag(t, "sk_test_bogus_key_9999")
		Config.Profile.APIKey = "sk_test_bogus_key_9999"

		require.Error(t, lc.applyCliKey(lc.cmd), "a key that fails validation must not be adopted")

		after, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.NotContains(t, string(after), "sk_test_bogus_key_9999",
			"an unvalidated key must never reach disk")
	})
}

// The flag has to be declared on listen itself: the root declaration is hidden,
// so it appears in no help output and tooling that introspects the CLI cannot
// discover it.
func TestListenDeclaresCliKeyFlag(t *testing.T) {
	lc := newListenCmd()

	flag := lc.cmd.Flags().Lookup("cli-key")
	require.NotNil(t, flag, "listen must declare --cli-key")
	assert.False(t, flag.Hidden, "--cli-key must be visible in `hookdeck listen --help`")
}
