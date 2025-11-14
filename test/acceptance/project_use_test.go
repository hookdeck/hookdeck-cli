package acceptance

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// NOTE: The project use --local integration tests are SKIPPED in CI because they require
// the /teams endpoint which is only accessible with CLI keys (from `hookdeck login`),
// not CI keys (from `hookdeck ci`).
//
// TO RUN THESE TESTS LOCALLY:
// 1. Run: hookdeck login
// 2. Copy the API key from: ~/.config/hookdeck/config.toml
// 3. Set environment variable: export HOOKDECK_CLI_TESTING_API_KEY=<your-cli-key>
// 4. Run tests: cd test/acceptance && go test -v -run TestProjectUse
//
// Tests that CAN run in CI:
// - TestProjectUseLocalAndConfigFlagConflict (flag validation occurs before API call)
// - TestLocalConfigHelpers (no API calls, tests helper functions)

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

// TestProjectUseLocal tests creating a local config with --local flag
// SKIPPED in CI: Requires /teams endpoint access
func TestProjectUseLocal(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	t.Skip("Skipped in CI: Requires CLI key from 'hookdeck login' (CI keys don't have /teams access)")

	// TODO: Implement org/project parsing from whoami output
	// This would require parsing: "Logged in as ... on project PROJECT_NAME in organization ORG_NAME"

	cli := NewCLIRunner(t)
	tempDir, cleanup := createTempWorkingDir(t)
	defer cleanup()

	t.Logf("Testing in temp directory: %s", tempDir)

	// For now, using placeholder values - in real test would parse from whoami
	org := "test-org"
	project := "test-project"

	// Run project use --local with org/project
	stdout, stderr, err := cli.Run("project", "use", org, project, "--local")
	require.NoError(t, err, "project use --local should succeed: stderr=%s", stderr)

	// Verify local config was created
	require.True(t, hasLocalConfig(t), "Local config should exist at .hookdeck/config.toml")

	// Verify security warning in output
	assert.Contains(t, stdout, "Security:", "Should display security warning")
	assert.Contains(t, stdout, "Created:", "Should indicate config was created")

	// Parse and verify config contents
	config := readLocalConfigTOML(t)
	defaultSection, ok := config["default"].(map[string]interface{})
	require.True(t, ok, "Config should have 'default' section")

	projectId, ok := defaultSection["project_id"].(string)
	require.True(t, ok && projectId != "", "Config should have project_id in default section")
}

// TestProjectUseSmartDefault tests that the smart default updates local config when it exists
// SKIPPED in CI: Requires /teams endpoint access
func TestProjectUseSmartDefault(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	t.Skip("Skipped in CI: Requires CLI key from 'hookdeck login' (CI keys don't have /teams access)")

	cli := NewCLIRunner(t)
	tempDir, cleanup := createTempWorkingDir(t)
	defer cleanup()

	t.Logf("Testing in temp directory: %s", tempDir)

	// Placeholder values - would parse from whoami in real test
	org := "test-org"
	project := "test-project"

	// Create local config first with --local
	stdout1, stderr1, err := cli.Run("project", "use", org, project, "--local")
	require.NoError(t, err, "Initial project use --local should succeed: stderr=%s", stderr1)
	require.Contains(t, stdout1, "Created:", "First use should create config")

	// Verify local config exists
	require.True(t, hasLocalConfig(t), "Local config should exist after first use")

	// Run project use again WITHOUT --local (smart default should detect local config)
	stdout2, stderr2, err := cli.Run("project", "use", org, project)
	require.NoError(t, err, "Second project use should succeed: stderr=%s", stderr2)

	// Verify it says "Updated:" not "Created:"
	assert.Contains(t, stdout2, "Updated:", "Second use should update existing config")
	assert.NotContains(t, stdout2, "Created:", "Second use should not say created")
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

// TestProjectUseLocalCreateDirectory tests that .hookdeck directory is created if it doesn't exist
// SKIPPED in CI: Requires /teams endpoint access
func TestProjectUseLocalCreateDirectory(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	t.Skip("Skipped in CI: Requires CLI key from 'hookdeck login' (CI keys don't have /teams access)")

	cli := NewCLIRunner(t)
	tempDir, cleanup := createTempWorkingDir(t)
	defer cleanup()

	t.Logf("Testing in temp directory: %s", tempDir)

	// Verify .hookdeck directory doesn't exist yet
	hookdeckDir := filepath.Join(tempDir, ".hookdeck")
	_, err := os.Stat(hookdeckDir)
	require.True(t, os.IsNotExist(err), ".hookdeck directory should not exist initially")

	// Placeholder values - would parse from whoami in real test
	org := "test-org"
	project := "test-project"

	// Run project use --local
	stdout, stderr, err := cli.Run("project", "use", org, project, "--local")
	require.NoError(t, err, "project use --local should succeed: stderr=%s", stderr)

	// Verify .hookdeck directory was created
	info, err := os.Stat(hookdeckDir)
	require.NoError(t, err, ".hookdeck directory should be created")
	require.True(t, info.IsDir(), ".hookdeck should be a directory")

	// Verify config file was created inside
	require.True(t, hasLocalConfig(t), "Local config should exist at .hookdeck/config.toml")

	t.Logf("Successfully verified directory creation: %s", stdout)
}

// TestProjectUseLocalSecurityWarning tests that security warning is displayed
// SKIPPED in CI: Requires /teams endpoint access
func TestProjectUseLocalSecurityWarning(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	t.Skip("Skipped in CI: Requires CLI key from 'hookdeck login' (CI keys don't have /teams access)")

	cli := NewCLIRunner(t)
	tempDir, cleanup := createTempWorkingDir(t)
	defer cleanup()

	t.Logf("Testing in temp directory: %s", tempDir)

	// Placeholder values - would parse from whoami in real test
	org := "test-org"
	project := "test-project"

	// Run project use --local
	stdout, stderr, err := cli.Run("project", "use", org, project, "--local")
	require.NoError(t, err, "project use --local should succeed: stderr=%s", stderr)

	// Verify security warning components
	assert.Contains(t, stdout, "Security:", "Should display security header")
	assert.Contains(t, stdout, "source control", "Should warn about source control")
	assert.Contains(t, stdout, ".gitignore", "Should mention .gitignore")

	t.Log("Successfully verified security warning is displayed")
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
