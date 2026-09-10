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
			ProjectID:      "team_1",
			ProjectProduct: "event_gateway",
		}, true)
		require.Equal(t, "team_1", p.ProjectId)
		require.Equal(t, "event_gateway", p.ProjectProduct)
		require.Equal(t, "inbound", p.ProjectMode)
		require.Equal(t, ProjectTypeGateway, p.ProjectType)
		require.Empty(t, p.GuestURL)
	})

	t.Run("preserves guest URL when clearGuestURL is false", func(t *testing.T) {
		p := &Profile{GuestURL: "https://guest.example/x"}
		p.ApplyValidateAPIKeyResponse(&hookdeck.ValidateAPIKeyResponse{
			ProjectID:      "team_2",
			ProjectProduct: "console",
		}, false)
		require.Equal(t, "team_2", p.ProjectId)
		require.Equal(t, ProjectTypeConsole, p.ProjectType)
		require.Equal(t, "https://guest.example/x", p.GuestURL)
	})
}

// TestProfile_LegacyModeFallback covers a response that predates team_product,
// or one where the field is absent for any other reason. Without the fallback
// the profile is blanked: ProjectType becomes "", IsGatewayProject("") is false,
// and every `hookdeck gateway ...` command fails with an empty project type.
func TestProfile_LegacyModeFallback(t *testing.T) {
	t.Run("validate response falls back to team_mode", func(t *testing.T) {
		p := &Profile{}
		p.ApplyValidateAPIKeyResponse(&hookdeck.ValidateAPIKeyResponse{
			ProjectID:   "team_legacy",
			ProjectMode: "outbound",
		}, false)
		require.Equal(t, "event_gateway", p.ProjectProduct)
		require.Equal(t, ProjectTypeGateway, p.ProjectType)
	})

	t.Run("poll response falls back to team_mode", func(t *testing.T) {
		p := &Profile{}
		p.ApplyPollAPIKeyResponse(&hookdeck.PollAPIKeyResponse{
			APIKey:      "key",
			ProjectID:   "team_legacy",
			ProjectMode: "console",
		}, "")
		require.Equal(t, "console", p.ProjectProduct)
		require.Equal(t, ProjectTypeConsole, p.ProjectType)
	})

	t.Run("ci client falls back to team_mode", func(t *testing.T) {
		p := &Profile{}
		p.ApplyCIClient(hookdeck.CIClient{
			APIKey:      "key",
			ProjectID:   "team_legacy",
			ProjectMode: "outpost",
		})
		require.Equal(t, "outpost", p.ProjectProduct)
		require.Equal(t, ProjectTypeOutpost, p.ProjectType)
	})

	t.Run("product wins when both are present", func(t *testing.T) {
		p := &Profile{}
		p.ApplyValidateAPIKeyResponse(&hookdeck.ValidateAPIKeyResponse{
			ProjectID:      "team_both",
			ProjectProduct: "outpost",
			ProjectMode:    "inbound",
		}, false)
		require.Equal(t, "outpost", p.ProjectProduct)
		require.Equal(t, ProjectTypeOutpost, p.ProjectType)
	})

	t.Run("both absent leaves the type empty", func(t *testing.T) {
		p := &Profile{}
		p.ApplyValidateAPIKeyResponse(&hookdeck.ValidateAPIKeyResponse{ProjectID: "team_none"}, false)
		require.Empty(t, p.ProjectProduct)
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
			APIKey:         "key_from_poll",
			ProjectID:      "team_p",
			ProjectProduct: "event_gateway",
		}, "https://guest")
		require.Equal(t, "key_from_poll", p.APIKey)
		require.Equal(t, "team_p", p.ProjectId)
		require.Equal(t, ProjectTypeGateway, p.ProjectType)
		require.Equal(t, "https://guest", p.GuestURL)
	})

	t.Run("clears guest URL when empty string passed", func(t *testing.T) {
		p := &Profile{GuestURL: "old"}
		p.ApplyPollAPIKeyResponse(&hookdeck.PollAPIKeyResponse{
			APIKey:         "k123456789012",
			ProjectID:      "t",
			ProjectProduct: "event_gateway",
		}, "")
		require.Empty(t, p.GuestURL)
	})
}

func TestProfile_ApplyCIClient(t *testing.T) {
	p := &Profile{}
	p.ApplyCIClient(hookdeck.CIClient{
		APIKey:         "ci_key_123456",
		ProjectID:      "team_ci",
		ProjectProduct: "event_gateway",
	})
	require.Equal(t, "ci_key_123456", p.APIKey)
	require.Equal(t, "team_ci", p.ProjectId)
	require.Equal(t, ProjectTypeGateway, p.ProjectType)
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
	assert.Contains(t, tomlText, `project_product = 'event_gateway'`)
}
