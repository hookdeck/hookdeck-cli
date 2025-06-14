package config

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemoveKey(t *testing.T) {
	v := viper.New()
	v.Set("remove", "me")
	v.Set("stay", "here")

	nv, err := removeKey(v, "remove")
	require.NoError(t, err)

	require.EqualValues(t, []string{"stay"}, nv.AllKeys())
	require.ElementsMatch(t, []string{"stay", "remove"}, v.AllKeys())
}

func TestGetConfigPath(t *testing.T) {
	t.Parallel()

	t.Run("with no config - should return global config path", func(t *testing.T) {
		t.Parallel()

		fs := &globalNoLocalConfigFS{}
		c := Config{fs: fs}
		customPathInput := ""
		expectedPath := filepath.Join(getSystemConfigFolder(os.Getenv("XDG_CONFIG_HOME")), "config.toml")

		path, isGlobalConfig := c.getConfigPath(customPathInput)
		assert.True(t, isGlobalConfig)
		assert.Equal(t, expectedPath, path)
	})

	t.Run("with no local or custom config - should return global config path", func(t *testing.T) {
		t.Parallel()

		fs := &noConfigFS{}
		c := Config{fs: fs}
		customPathInput := ""
		expectedPath := filepath.Join(getSystemConfigFolder(os.Getenv("XDG_CONFIG_HOME")), "config.toml")

		path, isGlobalConfig := c.getConfigPath(customPathInput)
		assert.True(t, isGlobalConfig)
		assert.Equal(t, expectedPath, path)
	})

	t.Run("with local and custom config - should return custom config path", func(t *testing.T) {
		t.Parallel()

		fs := &globalAndLocalConfigFS{}
		c := Config{fs: fs}
		customPathInput := "/absolute/custom/config.toml"
		expectedPath := customPathInput

		path, isGlobalConfig := c.getConfigPath(customPathInput)
		assert.False(t, isGlobalConfig)
		assert.Equal(t, expectedPath, path)
	})

	t.Run("with local only - should return local config path", func(t *testing.T) {
		t.Parallel()

		fs := &globalAndLocalConfigFS{}
		c := Config{fs: fs}
		customPathInput := ""
		pwd, _ := os.Getwd()
		expectedPath := filepath.Join(pwd, "./.hookdeck/config.toml")

		path, isGlobalConfig := c.getConfigPath(customPathInput)
		assert.False(t, isGlobalConfig)
		assert.Equal(t, expectedPath, path)
	})

	t.Run("with absolute custom config - should return custom config path", func(t *testing.T) {
		t.Parallel()

		fs := &noConfigFS{}
		c := Config{fs: fs}
		customPathInput := "/absolute/custom/config.toml"
		expectedPath := customPathInput

		path, isGlobalConfig := c.getConfigPath(customPathInput)
		assert.False(t, isGlobalConfig)
		assert.Equal(t, expectedPath, path)
	})

	t.Run("with relative custom config - should return custom config path", func(t *testing.T) {
		t.Parallel()

		fs := &noConfigFS{}
		c := Config{fs: fs}
		customPathInput := "absolute/custom/config.toml"
		pwd, _ := os.Getwd()
		expectedPath := filepath.Join(pwd, customPathInput)

		path, isGlobalConfig := c.getConfigPath(customPathInput)
		assert.False(t, isGlobalConfig)
		assert.Equal(t, expectedPath, path)
	})
}

func TestInitConfig(t *testing.T) {
	t.Parallel()

	t.Run("empty config", func(t *testing.T) {
		t.Parallel()

		c := Config{
			LogLevel:       "info",
			ConfigFileFlag: "./testdata/empty.toml",
		}
		c.InitConfig()

		assert.Equal(t, "default", c.Profile.Name)
		assert.Equal(t, "", c.Profile.APIKey)
		assert.Equal(t, "", c.Profile.ProjectId)
		assert.Equal(t, "", c.Profile.ProjectMode)
	})

	t.Run("default profile", func(t *testing.T) {
		t.Parallel()

		c := Config{
			LogLevel:       "info",
			ConfigFileFlag: "./testdata/default-profile.toml",
		}
		c.InitConfig()

		assert.Equal(t, "default", c.Profile.Name)
		assert.Equal(t, "test_api_key", c.Profile.APIKey)
		assert.Equal(t, "test_project_id", c.Profile.ProjectId)
		assert.Equal(t, "test_project_mode", c.Profile.ProjectMode)
	})

	t.Run("multiple profile", func(t *testing.T) {
		t.Parallel()

		c := Config{
			LogLevel:       "info",
			ConfigFileFlag: "./testdata/multiple-profiles.toml",
		}
		c.InitConfig()

		assert.Equal(t, "account_2", c.Profile.Name)
		assert.Equal(t, "account_2_test_api_key", c.Profile.APIKey)
		assert.Equal(t, "account_2_test_project_id", c.Profile.ProjectId)
		assert.Equal(t, "account_2_test_project_mode", c.Profile.ProjectMode)
	})

	t.Run("custom profile", func(t *testing.T) {
		t.Parallel()

		c := Config{
			LogLevel:       "info",
			ConfigFileFlag: "./testdata/multiple-profiles.toml",
		}
		c.Profile.Name = "account_3"
		c.InitConfig()

		assert.Equal(t, "account_3", c.Profile.Name)
		assert.Equal(t, "account_3_test_api_key", c.Profile.APIKey)
		assert.Equal(t, "account_3_test_project_id", c.Profile.ProjectId)
		assert.Equal(t, "account_3_test_project_mode", c.Profile.ProjectMode)
	})

	t.Run("local full", func(t *testing.T) {
		t.Parallel()

		c := Config{
			LogLevel:       "info",
			ConfigFileFlag: "./testdata/local-full.toml",
		}
		c.InitConfig()

		assert.Equal(t, "default", c.Profile.Name)
		assert.Equal(t, "local_api_key", c.Profile.APIKey)
		assert.Equal(t, "local_project_id", c.Profile.ProjectId)
		assert.Equal(t, "local_project_mode", c.Profile.ProjectMode)
	})

	t.Run("backwards compatible", func(t *testing.T) {
		t.Parallel()

		c := Config{
			LogLevel:       "info",
			ConfigFileFlag: "./testdata/local-full-workspace.toml",
		}
		c.InitConfig()

		assert.Equal(t, "default", c.Profile.Name)
		assert.Equal(t, "local_api_key", c.Profile.APIKey)
		assert.Equal(t, "local_workspace_id", c.Profile.ProjectId)
		assert.Equal(t, "local_workspace_mode", c.Profile.ProjectMode)
	})

	// TODO: Consider this case. This is a breaking change.
	// BREAKINGCHANGE
	t.Run("local workspace only", func(t *testing.T) {
		t.Parallel()

		c := Config{
			LogLevel:       "info",
			ConfigFileFlag: "./testdata/local-workspace-only.toml",
		}
		c.InitConfig()

		assert.Equal(t, "default", c.Profile.Name)
		assert.Equal(t, "", c.Profile.APIKey)
		assert.Equal(t, "local_workspace_id", c.Profile.ProjectId)
		assert.Equal(t, "", c.Profile.ProjectMode)
	})

	t.Run("api key override", func(t *testing.T) {
		t.Parallel()

		c := Config{
			LogLevel:       "info",
			ConfigFileFlag: "./testdata/default-profile.toml",
		}
		apiKey := "overridden_api_key"
		c.Profile.APIKey = apiKey
		c.InitConfig()

		assert.Equal(t, "default", c.Profile.Name)
		assert.Equal(t, apiKey, c.Profile.APIKey)
		assert.Equal(t, "test_project_id", c.Profile.ProjectId)
		assert.Equal(t, "test_project_mode", c.Profile.ProjectMode)
	})
}

func TestWriteConfig(t *testing.T) {
	t.Parallel()

	t.Run("save profile", func(t *testing.T) {
		t.Parallel()

		// Arrange
		c := Config{LogLevel: "info"}
		c.ConfigFileFlag = setupTempConfig(t, "./testdata/default-profile.toml")
		c.InitConfig()

		// Act
		c.Profile.ProjectMode = "new_team_mode"
		err := c.Profile.SaveProfile()

		// Assert
		assert.NoError(t, err)
		contentBytes, _ := ioutil.ReadFile(c.viper.ConfigFileUsed())
		assert.Contains(t, string(contentBytes), `project_mode = "new_team_mode"`)
	})

	t.Run("use project", func(t *testing.T) {
		t.Parallel()

		// Arrange
		c := Config{LogLevel: "info"}
		c.ConfigFileFlag = setupTempConfig(t, "./testdata/default-profile.toml")
		c.InitConfig()

		// Act
		err := c.UseProject("new_team_id", "new_team_mode")

		// Assert
		assert.NoError(t, err)
		contentBytes, _ := ioutil.ReadFile(c.viper.ConfigFileUsed())
		assert.Contains(t, string(contentBytes), `project_id = "new_team_id"`)
	})

	t.Run("use profile", func(t *testing.T) {
		t.Parallel()

		// Arrange
		c := Config{LogLevel: "info"}
		c.ConfigFileFlag = setupTempConfig(t, "./testdata/multiple-profiles.toml")
		c.InitConfig()

		// Act
		c.Profile.Name = "account_3"
		err := c.Profile.UseProfile()

		// Assert
		assert.NoError(t, err)
		contentBytes, _ := ioutil.ReadFile(c.viper.ConfigFileUsed())
		assert.Contains(t, string(contentBytes), `profile = "account_3"`)
	})

	t.Run("remove profile", func(t *testing.T) {
		t.Parallel()

		// Arrange
		c := Config{LogLevel: "info"}
		c.ConfigFileFlag = setupTempConfig(t, "./testdata/multiple-profiles.toml")
		c.InitConfig()

		// Act
		err := c.Profile.RemoveProfile()

		// Assert
		assert.NoError(t, err)
		contentBytes, _ := ioutil.ReadFile(c.viper.ConfigFileUsed())
		assert.NotContains(t, string(contentBytes), "account_2", `default profile "account_2" should be cleared`)
		assert.NotContains(t, string(contentBytes), `profile =`, `profile key should be cleared`)
	})

	t.Run("remove profile multiple times", func(t *testing.T) {
		t.Parallel()

		// Arrange
		c := Config{LogLevel: "info"}
		c.ConfigFileFlag = setupTempConfig(t, "./testdata/multiple-profiles.toml")
		c.InitConfig()

		// Act
		err := c.Profile.RemoveProfile()

		// Assert
		assert.NoError(t, err)
		contentBytes, _ := ioutil.ReadFile(c.viper.ConfigFileUsed())
		assert.NotContains(t, string(contentBytes), "account_2", `default profile "account_2" should be cleared`)
		assert.NotContains(t, string(contentBytes), `profile =`, `profile key should be cleared`)

		// Remove profile again

		c2 := Config{LogLevel: "info"}
		c2.ConfigFileFlag = c.ConfigFileFlag
		c2.InitConfig()
		err = c2.Profile.RemoveProfile()

		contentBytes, _ = ioutil.ReadFile(c2.viper.ConfigFileUsed())
		assert.NoError(t, err)
		assert.NotContains(t, string(contentBytes), "[default]", `default profile "default" should be cleared`)
		assert.NotContains(t, string(contentBytes), `api_key = "test_api_key"`, `default profile "default" should be cleared`)

		// Now even though there are some profiles (account_1, account_3), when reading config
		// we won't register any profile.
		// TODO: Consider this case. It's not great UX. This may be an edge case only power users run into
		// given it requires users to be using multiple profiles.

		c3 := Config{LogLevel: "info"}
		c3.ConfigFileFlag = c.ConfigFileFlag
		c3.InitConfig()
		assert.Equal(t, "default", c3.Profile.Name, `profile should be "default"`)
		assert.Equal(t, "", c3.Profile.APIKey, "api key should be empty even though there are other profiles")
	})
}

// ===== Test helpers =====

func setupTempConfig(t *testing.T, sourceConfigPath string) string {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	srcFile, _ := os.Open(sourceConfigPath)
	defer srcFile.Close()
	destFile, _ := os.Create(configPath)
	defer destFile.Close()
	io.Copy(destFile, srcFile)
	return configPath
}

// ===== Mock FS =====

// Mock fs where there's no config file, whether global or local
type noConfigFS struct{}

var _ ConfigFS = &noConfigFS{}

func (fs *noConfigFS) makePath(path string) error {
	return nil
}
func (fs *noConfigFS) fileExists(path string) (bool, error) {
	return false, nil
}

// Mock fs where there's global and local config file
type globalAndLocalConfigFS struct{}

var _ ConfigFS = &globalAndLocalConfigFS{}

func (fs *globalAndLocalConfigFS) makePath(path string) error {
	return nil
}
func (fs *globalAndLocalConfigFS) fileExists(path string) (bool, error) {
	return true, nil
}

// Mock fs where there's global but no local config file
type globalNoLocalConfigFS struct{}

var _ ConfigFS = &globalNoLocalConfigFS{}

func (fs *globalNoLocalConfigFS) makePath(path string) error {
	return nil
}
func (fs *globalNoLocalConfigFS) fileExists(path string) (bool, error) {
	globalConfigFolder := getSystemConfigFolder(os.Getenv("XDG_CONFIG_HOME"))
	globalPath := filepath.Join(globalConfigFolder, "config.toml")
	if path == globalPath {
		return true, nil
	}
	return false, nil
}

// Mock fs where there's no global and yes local config file
type noGlobalYesLocalConfigFS struct{}

var _ ConfigFS = &noGlobalYesLocalConfigFS{}

func (fs *noGlobalYesLocalConfigFS) makePath(path string) error {
	return nil
}
func (fs *noGlobalYesLocalConfigFS) fileExists(path string) (bool, error) {
	workspaceFolder, _ := os.Getwd()
	localPath := filepath.Join(workspaceFolder, ".hookdeck/config.toml")
	if path == localPath {
		return true, nil
	}
	return false, nil
}

// Mock fs where there's only custom local config at ${PWD}/customconfig.toml
type onlyCustomConfigFS struct{}

var _ ConfigFS = &onlyCustomConfigFS{}

func (fs *onlyCustomConfigFS) makePath(path string) error {
	return nil
}
func (fs *onlyCustomConfigFS) fileExists(path string) (bool, error) {
	workspaceFolder, _ := os.Getwd()
	customConfigPath := filepath.Join(workspaceFolder, "customconfig.toml")
	if path == customConfigPath {
		return true, nil
	}
	return false, nil
}
