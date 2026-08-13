package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSetLocalOnly covers the fix for #332: `hookdeck ci --local` and
// `hookdeck login --local` used to write the resolved (global) config file as
// well as the local one, silently repointing the machine's active project.
func TestSetLocalOnly(t *testing.T) {
	t.Run("SaveProfile leaves the global config untouched when localOnly is set", func(t *testing.T) {
		c := Config{LogLevel: "info"}
		c.ConfigFileFlag = setupTempConfig(t, "./testdata/default-profile.toml")
		c.InitConfig()

		before, err := os.ReadFile(c.GetConfigFile())
		require.NoError(t, err)

		c.SetLocalOnly(true)

		c.Profile.APIKey = "cli_key_that_must_not_be_persisted_globally"
		c.Profile.ProjectId = "tm_should_not_be_persisted"
		require.NoError(t, c.Profile.SaveProfile())
		require.NoError(t, c.Profile.UseProfile())

		after, err := os.ReadFile(c.GetConfigFile())
		require.NoError(t, err)

		assert.Equal(t, string(before), string(after),
			"global config must be byte-identical after SaveProfile/UseProfile with --local")
	})

	t.Run("SaveProfile still writes the global config by default", func(t *testing.T) {
		c := Config{LogLevel: "info"}
		c.ConfigFileFlag = setupTempConfig(t, "./testdata/default-profile.toml")
		c.InitConfig()

		before, err := os.ReadFile(c.GetConfigFile())
		require.NoError(t, err)

		c.Profile.ProjectId = "tm_written_globally"
		require.NoError(t, c.Profile.SaveProfile())

		after, err := os.ReadFile(c.GetConfigFile())
		require.NoError(t, err)

		assert.NotEqual(t, string(before), string(after),
			"without --local the global config should still be written")
		assert.Contains(t, string(after), "tm_written_globally")
	})

	t.Run("UseProjectLocal still writes the local file when localOnly is set", func(t *testing.T) {
		tempDir := t.TempDir()
		origDir, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(tempDir))
		defer os.Chdir(origDir)

		c := Config{LogLevel: "info"}
		c.ConfigFileFlag = setupTempConfig(t, "./testdata/default-profile.toml")
		c.InitConfig()
		c.SetLocalOnly(true)

		globalBefore, err := os.ReadFile(c.GetConfigFile())
		require.NoError(t, err)

		c.Profile.APIKey = "cli_key_local_only"
		isNew, err := c.UseProjectLocal("tm_local_project", "inbound")
		require.NoError(t, err)
		assert.True(t, isNew)

		localBytes, err := os.ReadFile(tempDir + "/.hookdeck/config.toml")
		require.NoError(t, err)
		assert.Contains(t, string(localBytes), "tm_local_project",
			"local config must still receive the project")
		assert.Contains(t, string(localBytes), "cli_key_local_only",
			"local config must still receive the credentials")

		globalAfter, err := os.ReadFile(c.GetConfigFile())
		require.NoError(t, err)
		assert.Equal(t, string(globalBefore), string(globalAfter),
			"global config must remain untouched")
	})
}
