package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUseProjectLocal tests the UseProjectLocal method of Config.
// These tests change the working directory and must NOT run in parallel.
func TestUseProjectLocal(t *testing.T) {
	t.Run("creates .hookdeck directory when it does not exist", func(t *testing.T) {
		tempDir := t.TempDir()
		origDir, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(tempDir))
		defer os.Chdir(origDir)

		c := Config{LogLevel: "info"}
		c.ConfigFileFlag = setupTempConfig(t, "./testdata/default-profile.toml")
		c.InitConfig()

		_, statErr := os.Stat(filepath.Join(tempDir, ".hookdeck"))
		require.True(t, os.IsNotExist(statErr), ".hookdeck directory should not exist before UseProjectLocal")

		_, err = c.UseProjectLocal("proj_abc", "test_mode")
		require.NoError(t, err)

		info, statErr := os.Stat(filepath.Join(tempDir, ".hookdeck"))
		require.NoError(t, statErr, ".hookdeck directory should be created")
		assert.True(t, info.IsDir(), ".hookdeck should be a directory")
	})

	t.Run("creates config.toml with correct content", func(t *testing.T) {
		tempDir := t.TempDir()
		origDir, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(tempDir))
		defer os.Chdir(origDir)

		c := Config{LogLevel: "info"}
		c.ConfigFileFlag = setupTempConfig(t, "./testdata/default-profile.toml")
		c.InitConfig()

		_, err = c.UseProjectLocal("proj_new_123", "test_mode")
		require.NoError(t, err)

		localConfigPath := filepath.Join(tempDir, ".hookdeck", "config.toml")
		_, statErr := os.Stat(localConfigPath)
		require.NoError(t, statErr, "config.toml should be created")

		var configData map[string]interface{}
		_, decodeErr := toml.DecodeFile(localConfigPath, &configData)
		require.NoError(t, decodeErr, "config.toml should be valid TOML")

		defaultSection, ok := configData["default"].(map[string]interface{})
		require.True(t, ok, "config should have a 'default' section")
		assert.Equal(t, "proj_new_123", defaultSection["project_id"], "project_id should match")
		assert.Equal(t, "test_mode", defaultSection["project_mode"], "project_mode should match")
	})

	t.Run("returns isNewFile=true when creating a new config", func(t *testing.T) {
		tempDir := t.TempDir()
		origDir, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(tempDir))
		defer os.Chdir(origDir)

		c := Config{LogLevel: "info"}
		c.ConfigFileFlag = setupTempConfig(t, "./testdata/default-profile.toml")
		c.InitConfig()

		isNew, err := c.UseProjectLocal("proj_abc", "mode_a")
		require.NoError(t, err)
		assert.True(t, isNew, "should return true when creating a new config file")
	})

	t.Run("returns isNewFile=false when updating an existing config", func(t *testing.T) {
		tempDir := t.TempDir()
		origDir, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(tempDir))
		defer os.Chdir(origDir)

		require.NoError(t, os.MkdirAll(filepath.Join(tempDir, ".hookdeck"), 0755))
		require.NoError(t, os.WriteFile(
			filepath.Join(tempDir, ".hookdeck", "config.toml"),
			[]byte("[default]\nproject_id = 'old_proj'\n"),
			0644,
		))

		c := Config{LogLevel: "info"}
		c.ConfigFileFlag = setupTempConfig(t, "./testdata/default-profile.toml")
		c.InitConfig()

		isNew, err := c.UseProjectLocal("proj_updated", "mode_b")
		require.NoError(t, err)
		assert.False(t, isNew, "should return false when updating an existing config file")
	})

	t.Run("updates project fields in existing config file", func(t *testing.T) {
		tempDir := t.TempDir()
		origDir, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(tempDir))
		defer os.Chdir(origDir)

		require.NoError(t, os.MkdirAll(filepath.Join(tempDir, ".hookdeck"), 0755))
		require.NoError(t, os.WriteFile(
			filepath.Join(tempDir, ".hookdeck", "config.toml"),
			[]byte("[default]\nproject_id = 'old_proj'\nproject_mode = 'old_mode'\n"),
			0644,
		))

		c := Config{LogLevel: "info"}
		c.ConfigFileFlag = setupTempConfig(t, "./testdata/default-profile.toml")
		c.InitConfig()

		_, err = c.UseProjectLocal("proj_updated", "new_mode")
		require.NoError(t, err)

		localConfigPath := filepath.Join(tempDir, ".hookdeck", "config.toml")
		var configData map[string]interface{}
		_, decodeErr := toml.DecodeFile(localConfigPath, &configData)
		require.NoError(t, decodeErr)

		defaultSection, ok := configData["default"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "proj_updated", defaultSection["project_id"], "project_id should be updated")
		assert.Equal(t, "new_mode", defaultSection["project_mode"], "project_mode should be updated")
	})
}
