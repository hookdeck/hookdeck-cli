package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProfile_ApplyValidateAPIKeyResponse(t *testing.T) {
	t.Run("nil response is no-op", func(t *testing.T) {
		p := &Profile{ProjectId: "keep", GuestURL: "https://guest"}
		p.ApplyValidateAPIKeyResponse(nil, true)
		require.Equal(t, "keep", p.ProjectId)
		require.Equal(t, "https://guest", p.GuestURL)
	})

	t.Run("sets project fields and clears guest when requested", func(t *testing.T) {
		p := &Profile{GuestURL: "https://guest"}
		p.ApplyValidateAPIKeyResponse(&hookdeck.ValidateAPIKeyResponse{
			ProjectID:   "team_1",
			ProjectType: "event_gateway",
		}, true)
		require.Equal(t, "team_1", p.ProjectId)
		require.Equal(t, "event_gateway", p.ProjectType)
		require.Equal(t, "inbound", p.ProjectMode)
		require.Equal(t, ProjectTypeEventGateway, p.ProjectType)
		require.Empty(t, p.GuestURL)
	})

	t.Run("preserves guest URL when clearGuestURL is false", func(t *testing.T) {
		p := &Profile{GuestURL: "https://guest.example/x"}
		p.ApplyValidateAPIKeyResponse(&hookdeck.ValidateAPIKeyResponse{
			ProjectID:   "team_2",
			ProjectType: "console",
		}, false)
		require.Equal(t, "team_2", p.ProjectId)
		require.Equal(t, ProjectTypeConsole, p.ProjectType)
		require.Equal(t, "https://guest.example/x", p.GuestURL)
	})
}

// TestProfile_LegacyModeFallback covers a response without team_type. Without
// the fallback the profile is blanked and every gateway command fails.
func TestProfile_LegacyModeFallback(t *testing.T) {
	t.Run("validate response falls back to team_mode", func(t *testing.T) {
		p := &Profile{}
		p.ApplyValidateAPIKeyResponse(&hookdeck.ValidateAPIKeyResponse{
			ProjectID:   "team_legacy",
			ProjectMode: "outbound",
		}, false)
		require.Equal(t, "event_gateway", p.ProjectType)
		require.Equal(t, ProjectTypeEventGateway, p.ProjectType)
	})

	t.Run("poll response falls back to team_mode", func(t *testing.T) {
		p := &Profile{}
		p.ApplyPollAPIKeyResponse(&hookdeck.PollAPIKeyResponse{
			APIKey:      "key",
			ProjectID:   "team_legacy",
			ProjectMode: "console",
		}, "")
		require.Equal(t, "console", p.ProjectType)
		require.Equal(t, ProjectTypeConsole, p.ProjectType)
	})

	t.Run("ci client falls back to team_mode", func(t *testing.T) {
		p := &Profile{}
		p.ApplyCIClient(hookdeck.CIClient{
			APIKey:      "key",
			ProjectID:   "team_legacy",
			ProjectMode: "outpost",
		})
		require.Equal(t, "outpost", p.ProjectType)
		require.Equal(t, ProjectTypeOutpost, p.ProjectType)
	})

	t.Run("product wins when both are present", func(t *testing.T) {
		p := &Profile{}
		p.ApplyValidateAPIKeyResponse(&hookdeck.ValidateAPIKeyResponse{
			ProjectID:   "team_both",
			ProjectType: "outpost",
			ProjectMode: "inbound",
		}, false)
		require.Equal(t, "outpost", p.ProjectType)
		require.Equal(t, ProjectTypeOutpost, p.ProjectType)
	})

	t.Run("both absent leaves the type empty", func(t *testing.T) {
		p := &Profile{}
		p.ApplyValidateAPIKeyResponse(&hookdeck.ValidateAPIKeyResponse{ProjectID: "team_none"}, false)
		require.Empty(t, p.ProjectType)
		require.Empty(t, p.ProjectType)
	})
}

func TestProfile_ApplyPollAPIKeyResponse(t *testing.T) {
	t.Run("nil response is no-op", func(t *testing.T) {
		p := &Profile{APIKey: "k", ProjectId: "p"}
		p.ApplyPollAPIKeyResponse(nil, "")
		require.Equal(t, "k", p.APIKey)
		require.Equal(t, "p", p.ProjectId)
	})

	t.Run("sets credentials and guest URL", func(t *testing.T) {
		p := &Profile{}
		p.ApplyPollAPIKeyResponse(&hookdeck.PollAPIKeyResponse{
			APIKey:      "key_from_poll",
			ProjectID:   "team_p",
			ProjectType: "event_gateway",
		}, "https://guest")
		require.Equal(t, "key_from_poll", p.APIKey)
		require.Equal(t, "team_p", p.ProjectId)
		require.Equal(t, ProjectTypeEventGateway, p.ProjectType)
		require.Equal(t, "https://guest", p.GuestURL)
	})

	t.Run("clears guest URL when empty string passed", func(t *testing.T) {
		p := &Profile{GuestURL: "old"}
		p.ApplyPollAPIKeyResponse(&hookdeck.PollAPIKeyResponse{
			APIKey:      "k123456789012",
			ProjectID:   "t",
			ProjectType: "event_gateway",
		}, "")
		require.Empty(t, p.GuestURL)
	})
}

func TestProfile_ApplyCIClient(t *testing.T) {
	p := &Profile{}
	p.ApplyCIClient(hookdeck.CIClient{
		APIKey:      "ci_key_123456",
		ProjectID:   "team_ci",
		ProjectType: "event_gateway",
	})
	require.Equal(t, "ci_key_123456", p.APIKey)
	require.Equal(t, "team_ci", p.ProjectId)
	require.Equal(t, ProjectTypeEventGateway, p.ProjectType)
	require.Empty(t, p.GuestURL)
}

func TestSaveProfile_RemovesLegacyWorkspaceKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	content := `profile = "default"
workspace_id = "legacy_team"
workspace_mode = "inbound"
team_id = "legacy_team"
team_mode = "inbound"

[default]
api_key = "hk_test_123456789012"
project_id = "proj_new"
project_mode = "inbound"
workspace_id = "legacy_profile_team"
workspace_mode = "inbound"
team_id = "legacy_profile_team"
team_mode = "inbound"
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0600))

	c, err := LoadConfigFromFile(path)
	require.NoError(t, err)
	c.Profile.Config = c

	require.NoError(t, c.Profile.SaveProfile())

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	tomlText := string(raw)
	assert.NotContains(t, tomlText, "workspace_id")
	assert.NotContains(t, tomlText, "workspace_mode")
	assert.NotContains(t, tomlText, "team_id")
	assert.NotContains(t, tomlText, "team_mode")
	assert.Contains(t, tomlText, "project_id")
	assert.Contains(t, tomlText, `project_type = 'Gateway'`)
}

// TestProfile_UnrecognizedTypeIsNotDiscarded covers a project type this CLI does
// not know about. Deriving from an unresolved type wrote empty values over what
// the API sent, leaving "current project type is ." on every gateway command.
func TestProfile_UnrecognizedTypeIsNotDiscarded(t *testing.T) {
	p := &Profile{}
	p.ApplyValidateAPIKeyResponse(&hookdeck.ValidateAPIKeyResponse{
		ProjectID:   "team_future",
		ProjectType: "some_future_product",
	}, false)

	// The raw value survives for a CLI that understands it.
	require.Equal(t, "some_future_product", p.ProjectType)

	// It still does not resolve, which keeps gateway commands from acting on it.
	require.Empty(t, p.ResolveProjectType())
	require.False(t, IsGatewayProject(p.ProjectType))
}

// TestUnknownProjectTypeSurvivesDisk covers both routes an unrecognized project
// type takes to config.toml; both wrote an empty project_type before this.
// Matters for forward compatibility: if the API adds a fourth type, running this
// CLI once would erase the setting a newer CLI depends on.
func TestUnknownProjectTypeSurvivesDisk(t *testing.T) {
	writeAndReload := func(t *testing.T, c *Config) string {
		t.Helper()
		require.NoError(t, c.Profile.SaveProfile())
		written, err := os.ReadFile(c.viper.ConfigFileUsed())
		require.NoError(t, err)
		return string(written)
	}

	t.Run("arriving from an auth response", func(t *testing.T) {
		c := Config{LogLevel: "info"}
		c.ConfigFileFlag = setupTempConfig(t, "./testdata/default-profile.toml")
		c.InitConfig()

		c.Profile.ApplyValidateAPIKeyResponse(&hookdeck.ValidateAPIKeyResponse{
			ProjectID:   "tm_future",
			ProjectType: "future_type",
		}, false)

		assert.Contains(t, writeAndReload(t, &c), "project_type = 'future_type'")
	})

	t.Run("already on disk, then rewritten", func(t *testing.T) {
		path := setupTempConfig(t, "./testdata/default-profile.toml")
		require.NoError(t, os.WriteFile(path, []byte(`profile = "default"

[default]
api_key = "test_key"
project_id = "tm_future"
project_type = "future_type"
`), 0o600))

		c := Config{LogLevel: "info", ConfigFileFlag: path}
		c.InitConfig()

		// Loading must not normalize it away, which is the half that persistence
		// alone could not fix.
		require.Equal(t, "future_type", c.Profile.ProjectType)
		assert.Contains(t, writeAndReload(t, &c), "project_type = 'future_type'")
	})

	t.Run("a recognized type is still stored as its label", func(t *testing.T) {
		c := Config{LogLevel: "info"}
		c.ConfigFileFlag = setupTempConfig(t, "./testdata/default-profile.toml")
		c.InitConfig()

		c.Profile.ApplyValidateAPIKeyResponse(&hookdeck.ValidateAPIKeyResponse{
			ProjectID:   "tm_known",
			ProjectType: ProjectTypeEventGateway,
		}, false)

		out := writeAndReload(t, &c)
		assert.Contains(t, out, "project_type = 'Gateway'", "older CLIs read the label")
		assert.Contains(t, out, "project_mode = 'inbound'")
	})
}
