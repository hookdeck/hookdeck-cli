package cmd

import (
	"strings"
	"testing"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
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
