package acceptance

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// NOTE: This file contains automated acceptance tests for the --local flag on login and ci commands.
// Tests that require human browser-based authentication would follow the same pattern as
// project_use_manual_test.go and would use the //go:build manual tag.
//
// Automated tests in this file (run in CI with HOOKDECK_CLI_TESTING_API_KEY):
// - TestLoginLocalAndConfigFlagConflictAcceptance
// - TestCILocalAndConfigFlagConflictAcceptance
// - TestCILocalCreatesConfig

// TestLoginLocalAndConfigFlagConflictAcceptance tests that login --local and --config cannot be
// used together. Flag validation happens before any API call, so no auth is needed for the
// actual test command (but NewCLIRunner requires HOOKDECK_CLI_TESTING_API_KEY).
func TestLoginLocalAndConfigFlagConflictAcceptance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)

	tempDir, cleanup := createTempWorkingDir(t)
	defer cleanup()

	t.Logf("Testing in temp directory: %s", tempDir)

	dummyConfigPath := filepath.Join(tempDir, "custom-config.toml")

	stdout, stderr, err := cli.Run("login", "--local", "--config", dummyConfigPath)

	require.Error(t, err, "Using both --local and --config should fail")
	combinedOutput := stdout + stderr
	assert.Contains(t, combinedOutput, "cannot be used together",
		"Error message should indicate flags cannot be used together")

	t.Logf("Successfully verified login --local --config conflict: %s", combinedOutput)
}

// TestCILocalAndConfigFlagConflictAcceptance tests that ci --local and --config cannot be
// used together. Flag validation happens before any API call.
func TestCILocalAndConfigFlagConflictAcceptance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)

	tempDir, cleanup := createTempWorkingDir(t)
	defer cleanup()

	t.Logf("Testing in temp directory: %s", tempDir)

	dummyConfigPath := filepath.Join(tempDir, "custom-config.toml")

	stdout, stderr, err := cli.Run("ci", "--api-key", "test_key", "--local", "--config", dummyConfigPath)

	require.Error(t, err, "Using both --local and --config should fail")
	combinedOutput := stdout + stderr
	assert.Contains(t, combinedOutput, "cannot be used together",
		"Error message should indicate flags cannot be used together")

	t.Logf("Successfully verified ci --local --config conflict: %s", combinedOutput)
}

// TestCILocalCreatesConfig tests that `hookdeck ci --api-key XXX --local` creates
// .hookdeck/config.toml in the current working directory with the correct content.
func TestCILocalCreatesConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)

	tempDir, cleanup := createTempWorkingDir(t)
	defer cleanup()

	t.Logf("Testing in temp directory: %s", tempDir)

	// Verify no local config exists initially
	require.False(t, hasLocalConfig(t), "Local config should not exist initially")

	// Run ci --local from the temp working directory
	stdout, stderr, err := cli.RunFromCwd("ci", "--api-key", cli.apiKey, "--local")
	if err != nil {
		t.Logf("STDOUT: %s", stdout)
		t.Logf("STDERR: %s", stderr)
	}
	require.NoError(t, err, "ci --local should succeed")

	// Verify local config was created
	require.True(t, hasLocalConfig(t), "Local config should exist at .hookdeck/config.toml")

	// Verify the .hookdeck directory was created
	hookdeckDir := filepath.Join(tempDir, ".hookdeck")
	info, err := os.Stat(hookdeckDir)
	require.NoError(t, err, ".hookdeck directory should exist")
	assert.True(t, info.IsDir(), ".hookdeck should be a directory")

	// Parse and verify config contents
	config := readLocalConfigTOML(t)
	defaultSection, ok := config["default"].(map[string]interface{})
	require.True(t, ok, "Config should have 'default' section")

	projectId, ok := defaultSection["project_id"].(string)
	require.True(t, ok && projectId != "", "Config should have non-empty project_id in default section")

	t.Logf("Successfully verified ci --local creates .hookdeck/config.toml with project_id=%s", projectId)
}

// TestCILocalCreatesNewFileOutputsCreated tests that the "Created:" message is shown
// when ci --local creates a new config file.
func TestCILocalCreatesNewFileOutputsCreated(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)

	_, cleanup := createTempWorkingDir(t)
	defer cleanup()

	// Run ci --local in a fresh directory
	stdout, stderr, err := cli.RunFromCwd("ci", "--api-key", cli.apiKey, "--local")
	if err != nil {
		t.Logf("STDOUT: %s", stdout)
		t.Logf("STDERR: %s", stderr)
	}
	require.NoError(t, err, "ci --local should succeed")

	assert.Contains(t, stdout, "Created:", "Should print 'Created:' when new local config is created")
	t.Logf("Output: %s", stdout)
}

// TestCILocalUpdatesExistingFileOutputsUpdated tests that the "Updated:" message is shown
// when ci --local updates an existing config file.
func TestCILocalUpdatesExistingFileOutputsUpdated(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)

	tempDir, cleanup := createTempWorkingDir(t)
	defer cleanup()

	// Pre-create local config so this is an update, not a create
	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, ".hookdeck"), 0755))
	existingConfig := "[default]\nproject_id = 'existing_proj'\nproject_mode = 'test'\n"
	require.NoError(t, os.WriteFile(
		filepath.Join(tempDir, ".hookdeck", "config.toml"),
		[]byte(existingConfig),
		0644,
	))

	// Run ci --local - should update, not create
	stdout, stderr, err := cli.RunFromCwd("ci", "--api-key", cli.apiKey, "--local")
	if err != nil {
		t.Logf("STDOUT: %s", stdout)
		t.Logf("STDERR: %s", stderr)
	}
	require.NoError(t, err, "ci --local should succeed")

	assert.Contains(t, stdout, "Updated:", "Should print 'Updated:' when existing local config is updated")
	assert.NotContains(t, stdout, "Created:", "Should not print 'Created:' when updating")
	t.Logf("Output: %s", stdout)
}

// TestCILocalSecurityWarning tests that the security warning is shown when creating a new
// local config file via ci --local.
func TestCILocalSecurityWarning(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)

	_, cleanup := createTempWorkingDir(t)
	defer cleanup()

	stdout, stderr, err := cli.RunFromCwd("ci", "--api-key", cli.apiKey, "--local")
	if err != nil {
		t.Logf("STDOUT: %s", stdout)
		t.Logf("STDERR: %s", stderr)
	}
	require.NoError(t, err, "ci --local should succeed")

	assert.Contains(t, stdout, "Security:", "Should display security warning header")
	assert.Contains(t, stdout, ".gitignore", "Should mention .gitignore in security warning")
	t.Logf("Output: %s", stdout)
}

// readLocalConfigTOMLFromDir parses .hookdeck/config.toml in the given directory
func readLocalConfigTOMLFromDir(t *testing.T, dir string) map[string]interface{} {
	t.Helper()

	var config map[string]interface{}
	_, err := toml.DecodeFile(filepath.Join(dir, ".hookdeck", "config.toml"), &config)
	require.NoError(t, err, "Failed to parse local config")

	return config
}
