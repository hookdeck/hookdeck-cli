package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
)

func TestRequireOutpostProject(t *testing.T) {
	t.Run("no API key", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.Profile.ProjectId = "proj_1"
		cfg.Profile.ProjectType = config.ProjectTypeOutpost
		err := requireOutpostProject(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "authenticated")
	})

	t.Run("no project selected", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.Profile.APIKey = "sk_xxx"
		err := requireOutpostProject(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no project selected")
	})

	t.Run("Outpost type passes", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.Profile.APIKey = "sk_xxx"
		cfg.Profile.ProjectId = "proj_1"
		cfg.Profile.ProjectType = config.ProjectTypeOutpost
		assert.NoError(t, requireOutpostProject(cfg))
	})

	t.Run("outpost mode passes when type is empty", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.Profile.APIKey = "sk_xxx"
		cfg.Profile.ProjectId = "proj_1"
		cfg.Profile.ProjectMode = "outpost"
		assert.NoError(t, requireOutpostProject(cfg))
	})

	// The point of the gate: without it these produce a 404 from the API, which
	// reads as "no such resource" rather than "wrong project".
	for name, projectType := range map[string]string{
		"Gateway type fails": config.ProjectTypeGateway,
		"Console type fails": config.ProjectTypeConsole,
	} {
		t.Run(name, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Profile.APIKey = "sk_xxx"
			cfg.Profile.ProjectId = "proj_1"
			cfg.Profile.ProjectType = projectType

			err := requireOutpostProject(cfg)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "requires an Outpost project")
			assert.Contains(t, err.Error(), "hookdeck project use", "the error should say how to fix it")
		})
	}

	t.Run("inbound mode fails when type is empty", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.Profile.APIKey = "sk_xxx"
		cfg.Profile.ProjectId = "proj_1"
		cfg.Profile.ProjectMode = "inbound"

		err := requireOutpostProject(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "requires an Outpost project")
	})
}

func TestIsOutpostMCPLeafCommand(t *testing.T) {
	t.Parallel()

	outpost := &cobra.Command{Use: "outpost"}
	mcp := &cobra.Command{Use: "mcp"}
	outpost.AddCommand(mcp)

	tenant := &cobra.Command{Use: "tenant"}
	outpost.AddCommand(tenant)

	gateway := &cobra.Command{Use: "gateway"}
	gatewayMCP := &cobra.Command{Use: "mcp"}
	gateway.AddCommand(gatewayMCP)

	assert.True(t, isOutpostMCPLeafCommand(mcp))
	assert.False(t, isOutpostMCPLeafCommand(tenant))
	assert.False(t, isOutpostMCPLeafCommand(gatewayMCP), "the gateway MCP command has its own handling")
	assert.False(t, isOutpostMCPLeafCommand(outpost))
	assert.False(t, isOutpostMCPLeafCommand(nil))
}

func TestOutpostCommandIsRegistered(t *testing.T) {
	t.Parallel()

	cmd, _, err := RootCmd().Find([]string{"outpost"})
	require.NoError(t, err)
	assert.Equal(t, "outpost", cmd.Name())
	assert.NotNil(t, cmd.PersistentPreRunE, "the project gate must run before every subcommand")
}
