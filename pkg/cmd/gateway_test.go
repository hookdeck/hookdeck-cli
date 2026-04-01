package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequireGatewayProject(t *testing.T) {
	t.Run("no API key", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.Profile.ProjectId = "proj_1"
		cfg.Profile.ProjectType = config.ProjectTypeGateway
		err := requireGatewayProject(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "authenticated")
	})

	t.Run("no project selected", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.Profile.APIKey = "sk_xxx"
		cfg.Profile.ProjectId = ""
		err := requireGatewayProject(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no project selected")
	})

	t.Run("Gateway type passes", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.Profile.APIKey = "sk_xxx"
		cfg.Profile.ProjectId = "proj_1"
		cfg.Profile.ProjectType = config.ProjectTypeGateway
		err := requireGatewayProject(cfg)
		assert.NoError(t, err)
	})

	t.Run("Console type passes", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.Profile.APIKey = "sk_xxx"
		cfg.Profile.ProjectId = "proj_1"
		cfg.Profile.ProjectType = config.ProjectTypeConsole
		err := requireGatewayProject(cfg)
		assert.NoError(t, err)
	})

	t.Run("inbound mode passes when type empty", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.Profile.APIKey = "sk_xxx"
		cfg.Profile.ProjectId = "proj_1"
		cfg.Profile.ProjectMode = "inbound"
		err := requireGatewayProject(cfg)
		assert.NoError(t, err)
	})

	t.Run("outbound mode passes when type empty (same as inbound)", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.Profile.APIKey = "sk_xxx"
		cfg.Profile.ProjectId = "proj_1"
		cfg.Profile.ProjectMode = "outbound"
		err := requireGatewayProject(cfg)
		assert.NoError(t, err)
	})

	t.Run("Outpost type fails", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.Profile.APIKey = "sk_xxx"
		cfg.Profile.ProjectId = "proj_1"
		cfg.Profile.ProjectType = config.ProjectTypeOutpost
		err := requireGatewayProject(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "requires a Gateway project")
		assert.Contains(t, err.Error(), "Outpost")
		assert.Contains(t, err.Error(), "hookdeck project use")
	})

	t.Run("unknown type fails", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.Profile.APIKey = "sk_xxx"
		cfg.Profile.ProjectId = "proj_1"
		cfg.Profile.ProjectMode = "outpost"
		err := requireGatewayProject(cfg)
		require.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "requires a Gateway project") || strings.Contains(err.Error(), "Outpost"))
	})
}

func TestRequireGatewayProject_resolveFromValidate(t *testing.T) {
	config.ResetAPIClientForTesting()
	t.Cleanup(config.ResetAPIClientForTesting)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != hookdeck.APIPathPrefix+"/cli-auth/validate" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(hookdeck.ValidateAPIKeyResponse{
			ProjectID:   "team_from_validate",
			ProjectMode: "inbound",
		})
	}))
	t.Cleanup(server.Close)

	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	toml := `profile = "default"

[default]
api_key = "sk_test_123456789012"
project_id = "stale_team_should_be_replaced"
guest_url = "https://guest.example/keep-me"
`
	require.NoError(t, os.WriteFile(path, []byte(toml), 0600))

	cfg, err := config.LoadConfigFromFile(path)
	require.NoError(t, err)
	cfg.APIBaseURL = server.URL

	err = requireGatewayProject(cfg)
	require.NoError(t, err)
	require.Equal(t, "team_from_validate", cfg.Profile.ProjectId)
	require.Equal(t, config.ProjectTypeGateway, cfg.Profile.ProjectType)
	require.Equal(t, "inbound", cfg.Profile.ProjectMode)
	require.Equal(t, "https://guest.example/keep-me", cfg.Profile.GuestURL, "gateway validate path must not clear guest_url")
}

func TestGatewayPersistentPreRunE_MCP(t *testing.T) {
	old := Config
	t.Cleanup(func() { Config = old })

	gw := newGatewayCmd().cmd
	mcpLeaf, _, err := gw.Find([]string{"mcp"})
	require.NoError(t, err)
	require.True(t, isGatewayMCPLeafCommand(mcpLeaf))

	t.Run("no API key skips requireGatewayProject", func(t *testing.T) {
		Config = config.Config{}
		Config.Profile.ProjectId = ""
		err := gatewayPersistentPreRunE(mcpLeaf, nil)
		assert.NoError(t, err)
	})

	t.Run("with API key and no project fails", func(t *testing.T) {
		Config = config.Config{}
		Config.Profile.APIKey = "sk_test_123456789012"
		Config.Profile.ProjectId = ""
		err := gatewayPersistentPreRunE(mcpLeaf, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no project selected")
	})
}
