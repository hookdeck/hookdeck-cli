//go:build project_use

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
// - TestLoginLocalAndConfigFlagConflictAcceptance (--local vs --hookdeck-config)
// - TestCILocalAndConfigFlagConflictAcceptance (--local vs --hookdeck-config)
// - TestCILocalCreatesConfig

// TestLoginLocalAndConfigFlagConflictAcceptance tests that login --local and --hookdeck-config cannot be
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

	stdout, stderr, err := cli.Run("login", "--local", "--hookdeck-config", dummyConfigPath)

	require.Error(t, err, "Using both --local and --hookdeck-config should fail")
	combinedOutput := stdout + stderr
	assert.Contains(t, combinedOutput, "cannot be used together",
		"Error message should indicate flags cannot be used together")

	t.Logf("Successfully verified login --local --hookdeck-config conflict: %s", combinedOutput)
}

// TestCILocalAndConfigFlagConflictAcceptance tests that ci --local and --hookdeck-config cannot be
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

	stdout, stderr, err := cli.Run("ci", "--api-key", "test_key", "--local", "--hookdeck-config", dummyConfigPath)

	require.Error(t, err, "Using both --local and --hookdeck-config should fail")
	combinedOutput := stdout + stderr
	assert.Contains(t, combinedOutput, "cannot be used together",
		"Error message should indicate flags cannot be used together")

	t.Logf("Successfully verified ci --local --hookdeck-config conflict: %s", combinedOutput)
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

// TestCILocalDoesNotWriteGlobalConfig is the regression test for #332: `hookdeck ci
// --local` used to call CILogin (which saves the profile to the resolved, usually
// global, config) and only then honour --local, so the flag added a second write
// instead of redirecting the first. The visible symptom was the machine's active
// project silently switching to the --local project.
//
// The test pins HOOKDECK_CONFIG_FILE to a temp file acting as the "global" config,
// so it is meaningful whether or not ACCEPTANCE_SLICE is set.
func TestCILocalDoesNotWriteGlobalConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)

	tempDir, cleanup := createTempWorkingDir(t)
	defer cleanup()

	// A stand-in global config, pre-populated with a project that must survive.
	globalConfigPath := filepath.Join(tempDir, "global-config.toml")
	originalGlobal := "profile = 'default'\n\n[default]\napi_key = 'cli_key_original_do_not_touch'\nproject_id = 'tm_original_project'\nproject_mode = 'inbound'\nproject_type = 'Gateway'\n"
	require.NoError(t, os.WriteFile(globalConfigPath, []byte(originalGlobal), 0600))

	stdout, stderr, err := cli.RunFromCwdWithEnv(
		map[string]string{"HOOKDECK_CONFIG_FILE": globalConfigPath},
		"ci", "--api-key", cli.apiKey, "--local",
	)
	if err != nil {
		t.Logf("STDOUT: %s", stdout)
		t.Logf("STDERR: %s", stderr)
	}
	require.NoError(t, err, "ci --local should succeed")

	globalAfter, err := os.ReadFile(globalConfigPath)
	require.NoError(t, err, "global config should still exist")
	assert.Equal(t, originalGlobal, string(globalAfter),
		"ci --local must leave the global config byte-identical (#332)")

	// And the local config must still have been written with real credentials.
	require.True(t, hasLocalConfig(t), "Local config should exist at .hookdeck/config.toml")
	localConfig := readLocalConfigTOMLFromDir(t, tempDir)
	defaultSection, ok := localConfig["default"].(map[string]interface{})
	require.True(t, ok, "Local config should have 'default' section")

	projectId, _ := defaultSection["project_id"].(string)
	require.NotEmpty(t, projectId, "Local config should have a project_id")
	assert.NotEqual(t, "tm_original_project", projectId,
		"local config should hold the newly authenticated project, not the global one")

	apiKey, _ := defaultSection["api_key"].(string)
	require.NotEmpty(t, apiKey, "Local config should carry the CLI client key")
	assert.NotEqual(t, "cli_key_original_do_not_touch", apiKey,
		"local config should hold the exchanged CI key")

	t.Logf("Verified ci --local wrote only the local config (project_id=%s)", projectId)
}

// TestCILocalDoesNotSwitchActiveProject asserts the user-visible symptom of #332,
// not just the file contents. The report was not "a file changed" — it was that
// every other `hookdeck` invocation on the machine silently moved to a different
// project. `whoami` is where a user would actually notice, so that is what this
// checks: after `ci --local` in some other directory, the global session must
// still report the project it had before.
func TestCILocalDoesNotSwitchActiveProject(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)

	tempDir, cleanup := createTempWorkingDir(t)
	defer cleanup()

	// Establish a "global" session in its own config file.
	globalConfigPath := filepath.Join(tempDir, "global-config.toml")
	globalEnv := map[string]string{"HOOKDECK_CONFIG_FILE": globalConfigPath}

	_, _, err := cli.RunFromCwdWithEnv(globalEnv, "ci", "--api-key", cli.apiKey)
	require.NoError(t, err, "setting up the global session should succeed")

	beforeStdout, _, err := cli.RunFromCwdWithEnv(globalEnv, "whoami")
	require.NoError(t, err, "whoami should work after ci")
	t.Logf("whoami before:\n%s", beforeStdout)

	globalBefore, err := os.ReadFile(globalConfigPath)
	require.NoError(t, err)

	// Now run ci --local in a subdirectory, as a project-local setup would.
	subDir := filepath.Join(tempDir, "some-project")
	require.NoError(t, os.MkdirAll(subDir, 0755))

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(subDir))
	defer os.Chdir(origDir)

	_, _, err = cli.RunFromCwdWithEnv(globalEnv, "ci", "--api-key", cli.apiKey, "--local")
	require.NoError(t, err, "ci --local should succeed")

	require.NoError(t, os.Chdir(origDir))

	globalAfter, err := os.ReadFile(globalConfigPath)
	require.NoError(t, err)
	assert.Equal(t, string(globalBefore), string(globalAfter),
		"ci --local must leave the global config byte-identical (#332)")

	afterStdout, _, err := cli.RunFromCwdWithEnv(globalEnv, "whoami")
	require.NoError(t, err, "whoami should still work")
	t.Logf("whoami after:\n%s", afterStdout)

	assert.Equal(t, beforeStdout, afterStdout,
		"the active project reported by whoami must be unchanged after ci --local (#332)")
}

// TestLoginLocalDoesNotWriteGlobalConfigOnFailure covers the twin path in #332.
// `hookdeck login --local` shares the same SaveProfile shape as ci. A full login
// needs a browser, so this asserts the narrower invariant that matters for
// automation: an unauthenticated `login --local` must not mutate the global config.
func TestLoginLocalDoesNotWriteGlobalConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)

	tempDir, cleanup := createTempWorkingDir(t)
	defer cleanup()

	globalConfigPath := filepath.Join(tempDir, "global-config.toml")
	originalGlobal := "profile = 'default'\n\n[default]\napi_key = 'cli_key_original_do_not_touch'\nproject_id = 'tm_original_project'\nproject_mode = 'inbound'\nproject_type = 'Gateway'\n"
	require.NoError(t, os.WriteFile(globalConfigPath, []byte(originalGlobal), 0600))

	// --cli-key with an invalid key fails validation before any browser flow.
	stdout, stderr, _ := cli.RunFromCwdWithEnv(
		map[string]string{"HOOKDECK_CONFIG_FILE": globalConfigPath},
		"login", "--local", "--cli-key", "invalid_key_for_test_0000",
	)
	t.Logf("STDOUT: %s\nSTDERR: %s", stdout, stderr)

	globalAfter, err := os.ReadFile(globalConfigPath)
	require.NoError(t, err)
	assert.Equal(t, originalGlobal, string(globalAfter),
		"login --local must not mutate the global config (#332)")
}

// readLocalConfigTOMLFromDir parses .hookdeck/config.toml in the given directory
func readLocalConfigTOMLFromDir(t *testing.T, dir string) map[string]interface{} {
	t.Helper()

	var config map[string]interface{}
	_, err := toml.DecodeFile(filepath.Join(dir, ".hookdeck", "config.toml"), &config)
	require.NoError(t, err, "Failed to parse local config")

	return config
}
