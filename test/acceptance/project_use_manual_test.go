//go:build manual

package acceptance

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// NOTE: These tests require human browser-based authentication and must be run with:
// go test -tags=manual -v ./test/acceptance/
//
// Each test will:
// 1. Clear existing authentication
// 2. Run `hookdeck login` and prompt you to complete browser authentication
// 3. Wait for you to press Enter after completing authentication
// 4. Verify authentication succeeded
// 5. Run the actual test
//
// The authentication helper runs once per test run (shared across all tests in this file).

// TestProjectUseLocalCreatesConfig tests creating a local config with --local flag
func TestProjectUseLocalCreatesConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping manual test in short mode")
	}

	// Require fresh CLI authentication (human interactive)
	whoamiOutput := RequireCLIAuthenticationOnce(t)

	cli := NewManualCLIRunner(t)
	tempDir, cleanup := createTempWorkingDir(t)
	defer cleanup()

	t.Logf("Testing in temp directory: %s", tempDir)

	// Parse actual org/project from whoami output
	org, project := ParseOrgAndProjectFromWhoami(t, whoamiOutput)
	t.Logf("Using organization: %s, project: %s", org, project)

	// Run project use --local with org/project (from current working directory)
	stdout, stderr, err := cli.RunFromCwd("project", "use", org, project, "--local")
	if err != nil {
		t.Logf("STDOUT: %s", stdout)
		t.Logf("STDERR: %s", stderr)
	}
	require.NoError(t, err, "project use --local should succeed")

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
func TestProjectUseSmartDefault(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping manual test in short mode")
	}

	// Require fresh CLI authentication (human interactive)
	whoamiOutput := RequireCLIAuthenticationOnce(t)

	cli := NewManualCLIRunner(t)
	tempDir, cleanup := createTempWorkingDir(t)
	defer cleanup()

	t.Logf("Testing in temp directory: %s", tempDir)

	// Parse actual org/project from whoami output
	org, project := ParseOrgAndProjectFromWhoami(t, whoamiOutput)
	t.Logf("Using organization: %s, project: %s", org, project)

	// Create local config first with --local (from current working directory)
	stdout1, stderr1, err := cli.RunFromCwd("project", "use", org, project, "--local")
	require.NoError(t, err, "Initial project use --local should succeed: stderr=%s", stderr1)
	require.Contains(t, stdout1, "Created:", "First use should create config")

	// Verify local config exists
	require.True(t, hasLocalConfig(t), "Local config should exist after first use")

	// Run project use again WITHOUT --local (smart default should detect local config)
	stdout2, stderr2, err := cli.RunFromCwd("project", "use", org, project)
	require.NoError(t, err, "Second project use should succeed: stderr=%s", stderr2)

	// Verify it says "Updated:" not "Created:"
	assert.Contains(t, stdout2, "Updated:", "Second use should update existing config")
	assert.NotContains(t, stdout2, "Created:", "Second use should not say created")
}

// TestProjectUseLocalCreateDirectory tests that .hookdeck directory is created if it doesn't exist
func TestProjectUseLocalCreateDirectory(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping manual test in short mode")
	}

	// Require fresh CLI authentication (human interactive)
	whoamiOutput := RequireCLIAuthenticationOnce(t)

	cli := NewManualCLIRunner(t)
	tempDir, cleanup := createTempWorkingDir(t)
	defer cleanup()

	t.Logf("Testing in temp directory: %s", tempDir)

	// Verify .hookdeck directory doesn't exist yet
	hookdeckDir := filepath.Join(tempDir, ".hookdeck")
	_, err := os.Stat(hookdeckDir)
	require.True(t, os.IsNotExist(err), ".hookdeck directory should not exist initially")

	// Parse actual org/project from whoami output
	org, project := ParseOrgAndProjectFromWhoami(t, whoamiOutput)
	t.Logf("Using organization: %s, project: %s", org, project)

	// Run project use --local (from current working directory)
	stdout, stderr, err := cli.RunFromCwd("project", "use", org, project, "--local")
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
func TestProjectUseLocalSecurityWarning(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping manual test in short mode")
	}

	// Require fresh CLI authentication (human interactive)
	whoamiOutput := RequireCLIAuthenticationOnce(t)

	cli := NewManualCLIRunner(t)
	tempDir, cleanup := createTempWorkingDir(t)
	defer cleanup()

	t.Logf("Testing in temp directory: %s", tempDir)

	// Parse actual org/project from whoami output
	org, project := ParseOrgAndProjectFromWhoami(t, whoamiOutput)
	t.Logf("Using organization: %s, project: %s", org, project)

	// Run project use --local (from current working directory)
	stdout, stderr, err := cli.RunFromCwd("project", "use", org, project, "--local")
	require.NoError(t, err, "project use --local should succeed: stderr=%s", stderr)

	// Verify security warning components
	assert.Contains(t, stdout, "Security:", "Should display security header")
	assert.Contains(t, stdout, "source control", "Should warn about source control")
	assert.Contains(t, stdout, ".gitignore", "Should mention .gitignore")

	t.Log("Successfully verified security warning is displayed")
}
