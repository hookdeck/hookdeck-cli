package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoginLocalAndConfigFlagConflict verifies that --local and --hookdeck-config cannot be combined.
func TestLoginLocalAndConfigFlagConflict(t *testing.T) {
	lc := newLoginCmd()
	lc.local = true

	origFlag := Config.ConfigFileFlag
	Config.ConfigFileFlag = "/some/custom/path.toml"
	defer func() { Config.ConfigFileFlag = origFlag }()

	err := lc.runLoginCmd(nil, []string{})
	require.Error(t, err, "--local and --hookdeck-config together should return an error")
	assert.Contains(t, err.Error(), "cannot be used together")
}

// TestCILocalAndConfigFlagConflict verifies that --local and --hookdeck-config cannot be combined on ci.
func TestCILocalAndConfigFlagConflict(t *testing.T) {
	lc := newCICmd()
	lc.local = true

	origFlag := Config.ConfigFileFlag
	Config.ConfigFileFlag = "/some/custom/path.toml"
	defer func() { Config.ConfigFileFlag = origFlag }()

	err := lc.runCICmd(nil, []string{})
	require.Error(t, err, "--local and --hookdeck-config together should return an error")
	assert.Contains(t, err.Error(), "cannot be used together")
}

// TestSaveLocalConfig_CreatesDirectoryAndFile verifies that saveLocalConfig writes
// .hookdeck/config.toml in the current working directory with correct profile fields.
func TestSaveLocalConfig_CreatesDirectoryAndFile(t *testing.T) {
	// Use a temp directory as the working directory so the test is isolated.
	tempDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tempDir))
	defer os.Chdir(origDir)

	// Write a minimal global config file so Config can be initialized.
	globalConfigPath := filepath.Join(tempDir, "global-config.toml")
	require.NoError(t, os.WriteFile(globalConfigPath, []byte(
		"[default]\napi_key = 'test_api_key'\nproject_id = 'global_proj'\nproject_mode = 'global_mode'\n",
	), 0644))

	// Save and restore the global Config variable.
	origConfig := Config
	defer func() { Config = origConfig }()

	Config = config.Config{
		LogLevel:       "info",
		ConfigFileFlag: globalConfigPath,
	}
	Config.InitConfig()

	// Override project fields to what we want in the local config.
	Config.Profile.ProjectId = "local_proj_123"
	Config.Profile.ProjectMode = "local_mode"

	err = saveLocalConfig()
	require.NoError(t, err, "saveLocalConfig should not return an error")

	// Verify the local config file was created.
	localConfigPath := filepath.Join(tempDir, ".hookdeck", "config.toml")
	_, statErr := os.Stat(localConfigPath)
	require.NoError(t, statErr, ".hookdeck/config.toml should exist after saveLocalConfig")

	// Verify the content of the local config.
	var configData map[string]interface{}
	_, decodeErr := toml.DecodeFile(localConfigPath, &configData)
	require.NoError(t, decodeErr, "local config should be valid TOML")

	defaultSection, ok := configData["default"].(map[string]interface{})
	require.True(t, ok, "config should have a 'default' section")
	assert.Equal(t, "local_proj_123", defaultSection["project_id"], "project_id should match")
	assert.Equal(t, "local_mode", defaultSection["project_mode"], "project_mode should match")
}

// TestSaveLocalConfig_ShowsCreatedForNewFile verifies the "Created:" message is printed for new files.
func TestSaveLocalConfig_ShowsCreatedForNewFile(t *testing.T) {
	tempDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tempDir))
	defer os.Chdir(origDir)

	globalConfigPath := filepath.Join(tempDir, "global-config.toml")
	require.NoError(t, os.WriteFile(globalConfigPath, []byte(
		"[default]\napi_key = 'k'\nproject_id = 'p'\nproject_mode = 'm'\n",
	), 0644))

	origConfig := Config
	defer func() { Config = origConfig }()
	Config = config.Config{LogLevel: "info", ConfigFileFlag: globalConfigPath}
	Config.InitConfig()
	Config.Profile.ProjectId = "proj_a"
	Config.Profile.ProjectMode = "mode_a"

	// Capture stdout
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	saveErr := saveLocalConfig()

	w.Close()
	os.Stdout = oldStdout

	var buf [4096]byte
	n, _ := r.Read(buf[:])
	output := string(buf[:n])

	require.NoError(t, saveErr)
	assert.Contains(t, output, "Created:", "should print 'Created:' for a new local config")
}

// TestSaveLocalConfig_ShowsUpdatedForExistingFile verifies the "Updated:" message is printed
// when the local config already exists.
func TestSaveLocalConfig_ShowsUpdatedForExistingFile(t *testing.T) {
	tempDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tempDir))
	defer os.Chdir(origDir)

	// Pre-create the local config
	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, ".hookdeck"), 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(tempDir, ".hookdeck", "config.toml"),
		[]byte("[default]\nproject_id = 'existing_proj'\n"),
		0644,
	))

	globalConfigPath := filepath.Join(tempDir, "global-config.toml")
	require.NoError(t, os.WriteFile(globalConfigPath, []byte(
		"[default]\napi_key = 'k'\nproject_id = 'p'\nproject_mode = 'm'\n",
	), 0644))

	origConfig := Config
	defer func() { Config = origConfig }()
	Config = config.Config{LogLevel: "info", ConfigFileFlag: globalConfigPath}
	Config.InitConfig()
	Config.Profile.ProjectId = "proj_updated"
	Config.Profile.ProjectMode = "mode_b"

	// Capture stdout
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	saveErr := saveLocalConfig()

	w.Close()
	os.Stdout = oldStdout

	var buf [4096]byte
	n, _ := r.Read(buf[:])
	output := string(buf[:n])

	require.NoError(t, saveErr)
	assert.Contains(t, output, "Updated:", "should print 'Updated:' for an existing local config")
	assert.NotContains(t, output, "Created:", "should not print 'Created:' for an existing local config")
}

// TestSaveLocalConfig_ShowsSecurityWarningForNewFile verifies the security warning is shown
// only when a new config file is created.
func TestSaveLocalConfig_ShowsSecurityWarningForNewFile(t *testing.T) {
	tempDir := t.TempDir()
	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tempDir))
	defer os.Chdir(origDir)

	globalConfigPath := filepath.Join(tempDir, "global-config.toml")
	require.NoError(t, os.WriteFile(globalConfigPath, []byte(
		"[default]\napi_key = 'k'\nproject_id = 'p'\nproject_mode = 'm'\n",
	), 0644))

	origConfig := Config
	defer func() { Config = origConfig }()
	Config = config.Config{LogLevel: "info", ConfigFileFlag: globalConfigPath}
	Config.InitConfig()
	Config.Profile.ProjectId = "proj_new"
	Config.Profile.ProjectMode = "mode_new"

	// Capture stdout
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	saveErr := saveLocalConfig()

	w.Close()
	os.Stdout = oldStdout

	var buf [4096]byte
	n, _ := r.Read(buf[:])
	output := string(buf[:n])

	require.NoError(t, saveErr)
	assert.Contains(t, output, "Security:", "should display security warning for new config")
	assert.Contains(t, output, ".gitignore", "should mention .gitignore in security warning")
}
