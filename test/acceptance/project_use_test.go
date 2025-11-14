package acceptance

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// NOTE: This file contains only the automated project use tests that can run in CI.
// Tests requiring human browser-based authentication are in project_use_manual_test.go
// with the //go:build manual tag.
//
// Automated tests (in this file):
// - TestProjectUseLocalAndConfigFlagConflict (flag validation occurs before API call)
// - TestLocalConfigHelpers (no API calls, tests helper functions)
//
// Manual tests (in project_use_manual_test.go):
// - TestProjectUseLocalCreatesConfig (requires /teams endpoint access)
// - TestProjectUseSmartDefault (requires /teams endpoint access)
// - TestProjectUseLocalCreateDirectory (requires /teams endpoint access)
// - TestProjectUseLocalSecurityWarning (requires /teams endpoint access)
//
// To run manual tests: go test -tags=manual -v ./test/acceptance/

// createTempWorkingDir creates a temporary directory, changes to it,
// and returns a cleanup function that restores original directory
func createTempWorkingDir(t *testing.T) (string, func()) {
	t.Helper()

	// Save original directory
	origDir, err := os.Getwd()
	require.NoError(t, err, "Failed to get current directory")

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "hookdeck-test-*")
	require.NoError(t, err, "Failed to create temp directory")

	// Change to temp directory
	err = os.Chdir(tempDir)
	require.NoError(t, err, "Failed to change to temp directory")

	cleanup := func() {
		// Restore original directory
		os.Chdir(origDir)
		// Clean up temp directory
		os.RemoveAll(tempDir)
	}

	return tempDir, cleanup
}

// hasLocalConfig checks if .hookdeck/config.toml exists in current directory
func hasLocalConfig(t *testing.T) bool {
	t.Helper()
	_, err := os.Stat(".hookdeck/config.toml")
	return err == nil
}

// readLocalConfigTOML parses the local config file as TOML
func readLocalConfigTOML(t *testing.T) map[string]interface{} {
	t.Helper()

	var config map[string]interface{}
	_, err := toml.DecodeFile(".hookdeck/config.toml", &config)
	require.NoError(t, err, "Failed to parse local config")

	return config
}

// TestProjectUseLocalAndConfigFlagConflict tests that using both --local and --config flags returns error
// This test doesn't require API calls since it validates flag conflicts before any API interaction
func TestProjectUseLocalAndConfigFlagConflict(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)

	// Create temp directory and change to it
	tempDir, cleanup := createTempWorkingDir(t)
	defer cleanup()

	t.Logf("Testing in temp directory: %s", tempDir)

	// Create a dummy config file path
	dummyConfigPath := filepath.Join(tempDir, "custom-config.toml")

	// Run with both --local and --config flags (should error)
	// Use placeholder values for org/project since the error occurs before API validation
	stdout, stderr, err := cli.Run("project", "use", "test-org", "test-project", "--local", "--config", dummyConfigPath)

	// Should return an error
	require.Error(t, err, "Using both --local and --config should fail")

	// Verify error message contains expected text
	combinedOutput := stdout + stderr
	assert.Contains(t, combinedOutput, "cannot be used together",
		"Error message should indicate flags cannot be used together")

	t.Logf("Successfully verified conflict error: %s", combinedOutput)
}

// TestLocalConfigHelpers tests the helper functions for working with local config
// This test doesn't require API access and verifies the test infrastructure works
func TestLocalConfigHelpers(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	// Create temp directory and change to it
	tempDir, cleanup := createTempWorkingDir(t)
	defer cleanup()

	t.Logf("Testing in temp directory: %s", tempDir)

	// Verify local config doesn't exist initially
	require.False(t, hasLocalConfig(t), "Local config should not exist initially")

	// Create .hookdeck directory and config file manually
	err := os.MkdirAll(".hookdeck", 0755)
	require.NoError(t, err, "Should be able to create .hookdeck directory")

	// Write a test config file
	testConfig := `[default]
project_id = "test_project_123"
api_key = "test_key_456"
`
	err = os.WriteFile(".hookdeck/config.toml", []byte(testConfig), 0644)
	require.NoError(t, err, "Should be able to write config file")

	// Verify hasLocalConfig detects it
	require.True(t, hasLocalConfig(t), "Local config should exist after creation")

	// Verify readLocalConfigTOML can parse it
	config := readLocalConfigTOML(t)
	require.NotNil(t, config, "Config should be parsed")

	defaultSection, ok := config["default"].(map[string]interface{})
	require.True(t, ok, "Config should have 'default' section")

	projectId, ok := defaultSection["project_id"].(string)
	require.True(t, ok, "Should have project_id field")
	assert.Equal(t, "test_project_123", projectId, "Project ID should match")

	t.Log("Successfully verified local config helper functions work correctly")
}
