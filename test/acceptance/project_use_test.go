//go:build project_use

package acceptance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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
// - TestProjectUseLocalCreatesConfig (requires /projects endpoint access)
// - TestProjectUseSmartDefault (requires /projects endpoint access)
// - TestProjectUseLocalCreateDirectory (requires /projects endpoint access)
// - TestProjectUseLocalSecurityWarning (requires /projects endpoint access)
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

// TestProjectUseLocalAndConfigFlagConflict tests that using both --local and --hookdeck-config flags returns error
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

	// Run with both --local and --hookdeck-config flags (should error)
	// Use placeholder values for org/project since the error occurs before API validation
	stdout, stderr, err := cli.Run("project", "use", "test-org", "test-project", "--local", "--hookdeck-config", dummyConfigPath)

	// Should return an error
	require.Error(t, err, "Using both --local and --hookdeck-config should fail")

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

// TestProjectListShowsType asserts that project list output includes project type (Gateway, Outpost, or Console).
// Requires HOOKDECK_CLI_TESTING_CLI_KEY (only CLI keys can list projects; API/CI keys cannot).
func TestProjectListShowsType(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cliKey := os.Getenv("HOOKDECK_CLI_TESTING_CLI_KEY")
	if cliKey == "" {
		t.Skip("Skipping project list test: HOOKDECK_CLI_TESTING_CLI_KEY must be set (CLI key required for listing projects; API and CI keys cannot list or switch projects)")
	}
	cli := NewCLIRunnerWithKey(t, cliKey)
	stdout, _, err := cli.Run("project", "list")
	// Deliberately not echoing stdout into any failure message: this listing is
	// every project the credential can see, and Actions logs on a public repo
	// are world-readable. Assert on derived values instead.
	require.NoError(t, err, "project list should succeed with an account-wide CLI key")

	assert.True(t, strings.Contains(stdout, "|"), "project list should show the type separator")
	assert.True(t,
		strings.Contains(stdout, "Gateway") || strings.Contains(stdout, "Outpost") || strings.Contains(stdout, "Console"),
		"project list should show at least one project type (Gateway, Outpost, or Console)")
}

// TestProjectListJSONOutput asserts that project list --output json returns valid JSON with id, org, project, type, current.
// Requires HOOKDECK_CLI_TESTING_CLI_KEY (only CLI keys can list projects; API/CI keys cannot).
func TestProjectListJSONOutput(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cliKey := os.Getenv("HOOKDECK_CLI_TESTING_CLI_KEY")
	if cliKey == "" {
		t.Skip("Skipping project list test: HOOKDECK_CLI_TESTING_CLI_KEY must be set (CLI key required for listing projects; API and CI keys cannot list or switch projects)")
	}
	cli := NewCLIRunnerWithKey(t, cliKey)
	stdout, _, err := cli.Run("project", "list", "--output", "json")
	// As above: never put this payload in a failure message.
	require.NoError(t, err, "project list --output json should succeed")
	var list []struct {
		Id      string `json:"id"`
		Org     string `json:"org"`
		Project string `json:"project"`
		Type    string `json:"type"`
		Current bool   `json:"current"`
	}
	require.NoError(t, json.Unmarshal([]byte(stdout), &list),
		"project list --output json should return valid JSON array")
	for i, item := range list {
		assert.NotEmpty(t, item.Id, "item %d should have id", i)
		assert.NotEmpty(t, item.Type, "item %d should have type", i)
		assert.True(t, item.Type == "gateway" || item.Type == "outpost" || item.Type == "console",
			"item %d type should be gateway, outpost, or console", i)
	}
}

// TestProjectListInvalidType asserts that project list --type <invalid> returns an error.
// Does not require CLI key (validation runs before listing).
func TestProjectListInvalidType(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout, stderr, err := cli.Run("project", "list", "--type", "invalid")
	require.Error(t, err)
	combined := stdout + stderr
	assert.Contains(t, combined, "invalid", "error should mention invalid type")
	assert.Contains(t, combined, "gateway", "error should list valid types")
}

// TestProjectListFilterByType asserts that project list --type <type> returns only projects of that type.
// Requires HOOKDECK_CLI_TESTING_CLI_KEY.
func TestProjectListFilterByType(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cliKey := os.Getenv("HOOKDECK_CLI_TESTING_CLI_KEY")
	if cliKey == "" {
		t.Skip("Skipping project list test: HOOKDECK_CLI_TESTING_CLI_KEY must be set (CLI key required for listing projects; API and CI keys cannot list or switch projects)")
	}
	cli := NewCLIRunnerWithKey(t, cliKey)
	stdout, _, err := cli.Run("project", "list", "--type", "gateway", "--output", "json")
	// No payload in the failure message: see TestProjectListShowsType.
	require.NoError(t, err, "project list should succeed with an account-wide CLI key")
	var list []struct {
		Id      string `json:"id"`
		Org     string `json:"org"`
		Project string `json:"project"`
		Type    string `json:"type"`
		Current bool   `json:"current"`
	}
	err = json.Unmarshal([]byte(stdout), &list)
	require.NoError(t, err, "project list --type gateway --output json should return valid JSON array")
	for i, item := range list {
		assert.Equal(t, "gateway", item.Type, "item %d should have type gateway when filtering by --type gateway", i)
	}
}

// TestProjectListFilterByOrgProject asserts that project list with org/project substrings filters results.
// Requires HOOKDECK_CLI_TESTING_CLI_KEY. Runs list with a substring and checks output shape and that items match.
func TestProjectListFilterByOrgProject(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cliKey := os.Getenv("HOOKDECK_CLI_TESTING_CLI_KEY")
	if cliKey == "" {
		t.Skip("Skipping project list test: HOOKDECK_CLI_TESTING_CLI_KEY must be set (CLI key required for listing projects; API and CI keys cannot list or switch projects)")
	}
	cli := NewCLIRunnerWithKey(t, cliKey)
	// Get full list first to derive a substring that matches at least one project
	full, _, err := cli.Run("project", "list", "--output", "json")
	// No payload in the failure message: see TestProjectListShowsType.
	require.NoError(t, err, "project list should succeed with an account-wide CLI key")
	var fullList []struct {
		Org     string `json:"org"`
		Project string `json:"project"`
	}
	require.NoError(t, json.Unmarshal([]byte(full), &fullList), "full list should be valid JSON")
	if len(fullList) == 0 {
		t.Skip("No projects to filter; skipping filter test")
	}
	// Use first character of first org as substring so we get a non-empty filtered result
	firstOrg := strings.TrimSpace(fullList[0].Org)
	if firstOrg == "" {
		t.Skip("First project has no org; skipping filter test")
	}
	substring := string([]rune(firstOrg)[0])
	stdout, _, err := cli.Run("project", "list", substring, "--output", "json")
	// No payload in the failure message: see TestProjectListShowsType.
	require.NoError(t, err, "project list should succeed with an account-wide CLI key")
	var filtered []struct {
		Id      string `json:"id"`
		Org     string `json:"org"`
		Project string `json:"project"`
		Type    string `json:"type"`
		Current bool   `json:"current"`
	}
	err = json.Unmarshal([]byte(stdout), &filtered)
	require.NoError(t, err, "project list with org substring should return valid JSON array")
	for i, item := range filtered {
		assert.Contains(t, strings.ToLower(item.Org), strings.ToLower(substring),
			"item %d org should contain substring %q", i, substring)
		assert.NotEmpty(t, item.Type, "item %d should have type", i)
	}
}

// TestProjectListFilterByOrgAndProject asserts that project list with two args (org and project substrings) filters correctly.
// Requires HOOKDECK_CLI_TESTING_CLI_KEY.
func TestProjectListFilterByOrgAndProject(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cliKey := os.Getenv("HOOKDECK_CLI_TESTING_CLI_KEY")
	if cliKey == "" {
		t.Skip("Skipping project list test: HOOKDECK_CLI_TESTING_CLI_KEY must be set (CLI key required for listing projects; API and CI keys cannot list or switch projects)")
	}
	cli := NewCLIRunnerWithKey(t, cliKey)
	full, _, err := cli.Run("project", "list", "--output", "json")
	// No payload in the failure message: see TestProjectListShowsType.
	require.NoError(t, err, "project list should succeed with an account-wide CLI key")
	var fullList []struct {
		Org     string `json:"org"`
		Project string `json:"project"`
	}
	require.NoError(t, json.Unmarshal([]byte(full), &fullList), "full list should be valid JSON")
	if len(fullList) == 0 {
		t.Skip("No projects to filter; skipping filter test")
	}
	first := fullList[0]
	orgSub := strings.TrimSpace(first.Org)
	projSub := strings.TrimSpace(first.Project)
	if orgSub == "" || projSub == "" {
		t.Skip("First project missing org or project name; skipping filter test")
	}
	// Use first character of each so filter matches at least one project
	orgChar := string([]rune(orgSub)[0])
	projChar := string([]rune(projSub)[0])
	stdout, _, err := cli.Run("project", "list", orgChar, projChar, "--output", "json")
	// No payload in the failure message: see TestProjectListShowsType.
	require.NoError(t, err, "project list should succeed with an account-wide CLI key")
	var filtered []struct {
		Org     string `json:"org"`
		Project string `json:"project"`
		Type    string `json:"type"`
	}
	err = json.Unmarshal([]byte(stdout), &filtered)
	require.NoError(t, err, "project list with org and project substrings should return valid JSON array")
	for i, item := range filtered {
		assert.Contains(t, strings.ToLower(item.Org), strings.ToLower(orgChar),
			"item %d org should contain org substring", i)
		assert.Contains(t, strings.ToLower(item.Project), strings.ToLower(projChar),
			"item %d project should contain project substring", i)
		assert.NotEmpty(t, item.Type, "item %d should have type", i)
	}
}

// TestProjectListFailsWithCIKeyAcceptance asserts that project list fails with a clear
// message when the config holds a CI-scoped key (from hookdeck ci), not a 500 Fatal Error.
func TestProjectListFailsWithCIKeyAcceptance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)

	stdout, stderr, err := cli.Run("project", "list")
	require.Error(t, err, "project list should fail with CI-scoped credentials")

	combined := stdout + stderr
	assert.NotContains(t, combined, "Fatal Error")
	assert.NotContains(t, combined, "status=500")
	assert.Contains(t, combined, "single project")
	// The reason alone is not actionable: the message has to name the command
	// that fixes it, because the two kinds of CLI key are not visible to the
	// user otherwise. `hookdeck ci` issues a project-scoped key with no user and
	// core rejects it for this endpoint; `hookdeck login` issues an account-wide
	// one that works.
	assert.Contains(t, combined, "hookdeck login",
		"the error should tell the user how to get an account-wide key")
}

// TestProjectUseHookdeckConfigWritesNamedFileNotCwdLocal is the acceptance-level
// regression test for #424: `hookdeck project use --hookdeck-config <path>` used
// to report success and then write a .hookdeck/config.toml that merely happened
// to be in the working directory, leaving the file the flag named untouched. The
// flag exists to keep a command off other configuration, so the named file must
// be the one written, and the local one must be byte-identical afterwards.
//
// Requires HOOKDECK_CLI_TESTING_CLI_KEY: switching projects needs an
// account-wide CLI key, which API and CI keys are not.
func TestProjectUseHookdeckConfigWritesNamedFileNotCwdLocal(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cliKey := os.Getenv("HOOKDECK_CLI_TESTING_CLI_KEY")
	if cliKey == "" {
		t.Skip("Skipping project use test: HOOKDECK_CLI_TESTING_CLI_KEY must be set (CLI key required for switching projects; API and CI keys cannot list or switch projects)")
	}

	cli := NewCLIRunnerWithKey(t, cliKey)

	// `project use <org> <project>` requires the pair to be unambiguous, so pick
	// one that appears exactly once. As elsewhere in this file, the listing is
	// never echoed into logs or failure messages.
	listOut, _, err := cli.Run("project", "list", "--output", "json")
	require.NoError(t, err, "project list should succeed with an account-wide CLI key")
	var projects []struct {
		Id      string `json:"id"`
		Org     string `json:"org"`
		Project string `json:"project"`
	}
	require.NoError(t, json.Unmarshal([]byte(listOut), &projects),
		"project list --output json should return valid JSON array")

	counts := map[string]int{}
	for _, p := range projects {
		counts[strings.ToLower(p.Org)+"/"+strings.ToLower(p.Project)] = counts[strings.ToLower(p.Org)+"/"+strings.ToLower(p.Project)] + 1
	}
	var org, projectName, expectedID string
	for _, p := range projects {
		if p.Org == "" || p.Project == "" {
			continue
		}
		if counts[strings.ToLower(p.Org)+"/"+strings.ToLower(p.Project)] == 1 {
			org, projectName, expectedID = p.Org, p.Project, p.Id
			break
		}
	}
	if org == "" {
		t.Skip("Skipping project use test: no uniquely named project available for this credential")
	}

	tempDir, cleanup := createTempWorkingDir(t)
	defer cleanup()

	// A cwd-local config the command must not touch. Its project id could not
	// have come from any real switch, so finding it afterwards is conclusive.
	require.NoError(t, os.MkdirAll(filepath.Join(tempDir, ".hookdeck"), 0755))
	localConfigPath := filepath.Join(tempDir, ".hookdeck", "config.toml")
	originalLocal := "profile = 'default'\n\n[default]\napi_key = 'cli_key_local_do_not_touch'\nproject_id = 'tm_local_do_not_touch'\nproject_mode = 'inbound'\nproject_type = 'Gateway'\n"
	require.NoError(t, os.WriteFile(localConfigPath, []byte(originalLocal), 0600))

	// Authenticate into the named file only. login honours --hookdeck-config the
	// same way, which the local-config check below also asserts.
	explicitConfigPath := filepath.Join(tempDir, "explicit-config.toml")
	loginOut, loginErr, err := cli.RunFromCwd("login", "--api-key", cliKey, "--hookdeck-config", explicitConfigPath)
	if err != nil {
		t.Logf("STDERR: %s", loginErr)
	}
	require.NoError(t, err, "login --hookdeck-config should succeed")
	require.NotEmpty(t, loginOut, "login should report what it did")

	afterLogin, err := os.ReadFile(localConfigPath)
	require.NoError(t, err)
	require.Equal(t, originalLocal, string(afterLogin),
		"login --hookdeck-config must leave a cwd-local config byte-identical")

	stdout, stderr, err := cli.RunFromCwd("project", "use", org, projectName, "--hookdeck-config", explicitConfigPath)
	if err != nil {
		t.Logf("STDERR: %s", stderr)
	}
	require.NoError(t, err, "project use --hookdeck-config should succeed")

	// The named file is the one that was written.
	explicitConfig := map[string]interface{}{}
	_, decodeErr := toml.DecodeFile(explicitConfigPath, &explicitConfig)
	require.NoError(t, decodeErr, "the file named by --hookdeck-config should be valid TOML")
	defaultSection, ok := explicitConfig["default"].(map[string]interface{})
	require.True(t, ok, "the file named by --hookdeck-config should have a 'default' section")
	writtenID, _ := defaultSection["project_id"].(string)
	assert.Equal(t, expectedID, writtenID,
		"--hookdeck-config must write the selected project to the file it names (#424)")

	// And the cwd-local config is untouched.
	localAfter, err := os.ReadFile(localConfigPath)
	require.NoError(t, err, "the cwd-local config should still exist")
	assert.Equal(t, originalLocal, string(localAfter),
		"--hookdeck-config must leave a cwd-local config byte-identical (#424)")

	// The reported path must be the file that was actually written: the issue
	// was a success message that described a switch which had not happened.
	assert.Contains(t, stdout, explicitConfigPath,
		"project use should report the file named by --hookdeck-config")
	assert.NotContains(t, stdout, localConfigPath,
		"project use must not report writing the cwd-local config")
}
