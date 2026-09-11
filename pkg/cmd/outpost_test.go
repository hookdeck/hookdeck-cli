package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

// The wrong-project-type error is on the first-run path, and its advice used to
// be circular for anyone holding a project-scoped key: it sent them to
// `hookdeck project use`, which refuses a project-scoped key and tells them to
// log in, which with the same key returns them here.
func TestRequireOutpostProjectExplainsAProjectScopedKey(t *testing.T) {
	validate := func(t *testing.T, response map[string]any) *config.Config {
		t.Helper()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(response)
		}))
		t.Cleanup(server.Close)

		cfg := &config.Config{APIBaseURL: server.URL}
		cfg.Profile.APIKey = "sk_xxx"
		cfg.Profile.ProjectId = "proj_1"
		cfg.Profile.ProjectType = config.ProjectTypeGateway

		// GetAPIClient is a process-wide singleton, so it has to be rebuilt for
		// this config or the request goes wherever an earlier test pointed it.
		config.ResetAPIClientForTesting()
		t.Cleanup(config.ResetAPIClientForTesting)

		return cfg
	}

	t.Run("a project-scoped key is named as the reason", func(t *testing.T) {
		// No user_id is how a single-project credential is recognised.
		cfg := validate(t, map[string]any{"team_id": "proj_1", "team_mode": "inbound"})

		err := requireOutpostProject(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot switch away from it")
		assert.Contains(t, err.Error(), "hookdeck login")
		assert.Contains(t, err.Error(), "hookdeck ci --api-key")
		// `project use` may be mentioned, but only to rule it out — offering it
		// as the fix is the dead end this replaced.
		assert.NotContains(t, err.Error(), "Use 'hookdeck project use' to switch")
	})

	t.Run("an account-wide key is still told to switch project", func(t *testing.T) {
		cfg := validate(t, map[string]any{
			"user_id": "usr_1", "team_id": "proj_1", "team_mode": "inbound",
		})

		err := requireOutpostProject(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "hookdeck project use")
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
