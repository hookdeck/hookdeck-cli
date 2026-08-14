package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

func TestIsOutpostProject(t *testing.T) {
	t.Parallel()

	for _, value := range []string{ProjectTypeOutpost, "outpost"} {
		assert.True(t, IsOutpostProject(value), "expected %q to be an Outpost project", value)
	}

	// Gateway and Console types must not satisfy the Outpost gate, and nor must
	// an unset type — an unknown project should be resolved, not assumed.
	for _, value := range []string{ProjectTypeGateway, ProjectTypeConsole, "inbound", "outbound", "console", ""} {
		assert.False(t, IsOutpostProject(value), "expected %q not to be an Outpost project", value)
	}
}

func TestOutpostAPIClientIsSeparateFromGatewayClient(t *testing.T) {
	ResetAPIClientForTesting()
	t.Cleanup(ResetAPIClientForTesting)

	cfg := &Config{
		APIBaseURL:        "https://api.example.test",
		OutpostAPIBaseURL: "https://outpost.example.test",
	}
	cfg.Profile.APIKey = "key-1"
	cfg.Profile.ProjectId = "tm_1"

	gateway := cfg.GetAPIClient()
	outpost := cfg.GetOutpostAPIClient()

	require.NotSame(t, gateway, outpost, "the two clients must be distinct instances")
	assert.Equal(t, "api.example.test", gateway.BaseURL.Host)
	assert.Equal(t, "outpost.example.test", outpost.BaseURL.Host)
	assert.Equal(t, "key-1", outpost.APIKey)
	assert.Equal(t, "tm_1", outpost.ProjectID)

	// Tool handlers switch projects by mutating the client in place, so a change
	// to one product's client must not move the other.
	outpost.ProjectID = "tm_2"
	assert.Equal(t, "tm_1", gateway.ProjectID)
}

func TestRefreshCachedAPIClientRefreshesOutpostClient(t *testing.T) {
	ResetAPIClientForTesting()
	t.Cleanup(ResetAPIClientForTesting)

	cfg := &Config{
		APIBaseURL:        "https://api.example.test",
		OutpostAPIBaseURL: "https://outpost.example.test",
	}
	cfg.Profile.APIKey = "old-key"
	cfg.Profile.ProjectId = "tm_1"

	outpost := cfg.GetOutpostAPIClient()
	require.Equal(t, "old-key", outpost.APIKey)

	// Simulates signing in, or switching project, after the client was built.
	cfg.Profile.APIKey = "new-key"
	cfg.Profile.ProjectId = "tm_2"
	cfg.RefreshCachedAPIClient()

	assert.Equal(t, "new-key", outpost.APIKey, "a stale key here would fail every Outpost call after login")
	assert.Equal(t, "tm_2", outpost.ProjectID)
}

func TestRefreshCachedAPIClientHandlesUnbuiltClients(t *testing.T) {
	ResetAPIClientForTesting()
	t.Cleanup(ResetAPIClientForTesting)

	cfg := &Config{
		APIBaseURL:        "https://api.example.test",
		OutpostAPIBaseURL: "https://outpost.example.test",
	}

	// Only the gateway client has been built; refreshing must not panic on the
	// Outpost one, which is nil until a command asks for it.
	_ = cfg.GetAPIClient()
	assert.NotPanics(t, cfg.RefreshCachedAPIClient)
}

func TestResetAPIClientForTestingClearsOutpostClient(t *testing.T) {
	ResetAPIClientForTesting()
	t.Cleanup(ResetAPIClientForTesting)

	cfg := &Config{OutpostAPIBaseURL: "https://outpost.example.test"}
	first := cfg.GetOutpostAPIClient()

	ResetAPIClientForTesting()

	second := cfg.GetOutpostAPIClient()
	assert.NotSame(t, first, second, "reset must clear the Outpost singleton, not just the gateway one")
}

func TestOutpostAPIBaseURLDefaultsAndOverrides(t *testing.T) {
	t.Parallel()

	t.Run("falls back to the published Outpost host", func(t *testing.T) {
		cfg := newTestConfigForOutpost(t, "", "")
		assert.Equal(t, hookdeck.DefaultOutpostAPIBaseURL, cfg.OutpostAPIBaseURL)
	})

	t.Run("a config file value overrides the default", func(t *testing.T) {
		cfg := newTestConfigForOutpost(t, "", `outpost_api_base = "https://outpost.from-config.test"`)
		assert.Equal(t, "https://outpost.from-config.test", cfg.OutpostAPIBaseURL)
	})

	t.Run("an explicit flag value beats the config file", func(t *testing.T) {
		cfg := newTestConfigForOutpost(t, "https://outpost.from-flag.test", `outpost_api_base = "https://outpost.from-config.test"`)
		assert.Equal(t, "https://outpost.from-flag.test", cfg.OutpostAPIBaseURL)
	})
}

// newTestConfigForOutpost resolves a Config through constructConfig, the same
// flags > config file > default chain the real CLI uses. InitConfig is avoided
// here because it terminates the process when LogLevel is unset.
func newTestConfigForOutpost(t *testing.T, outpostBaseFlag, configContents string) *Config {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(path, []byte(configContents+"\n"), 0o600))

	cfg, err := LoadConfigFromFile(path)
	require.NoError(t, err)

	if outpostBaseFlag != "" {
		cfg.OutpostAPIBaseURL = outpostBaseFlag
		cfg.constructConfig()
	}

	return cfg
}
