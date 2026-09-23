package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// These tests pin down which config file `hookdeck project use` writes, for
// every combination of --hookdeck-config, --local and a cwd-local
// .hookdeck/config.toml. The rule is that an explicit flag beats implicit
// discovery, and that the file written is the file read:
//
//  1. --hookdeck-config <path> -> exactly that file, never a cwd-local one
//  2. --local                  -> ./.hookdeck/config.toml
//  3. neither                  -> cwd-local if it exists, else the global config
//  4. --local + --hookdeck-config -> an error, and nothing written
//
// Case 1 is the regression test for #424: `project use` used to check only
// whether a cwd-local config existed, so it reported success and then wrote the
// project somewhere other than the file the flag named.

const (
	testProjectIDFirst  = "tm_first_project"
	testProjectIDSecond = "tm_second_project"
)

// projectUseAPIStub serves the two endpoints `project use` calls: the
// credential check (which must report a user, or the command refuses to list
// projects) and the project list itself.
func projectUseAPIStub(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case hookdeck.APIPathPrefix + "/cli-auth/validate":
			_ = json.NewEncoder(w).Encode(hookdeck.ValidateAPIKeyResponse{
				UserID:           "usr_123",
				UserName:         "Test User",
				OrganizationName: "Acme",
				ProjectID:        testProjectIDFirst,
				ProjectType:      config.ProjectTypeEventGateway,
			})
		case hookdeck.APIPathPrefix + "/projects":
			_ = json.NewEncoder(w).Encode([]hookdeck.Project{
				{Id: testProjectIDFirst, Name: "[Acme] First Project", Type: config.ProjectTypeEventGateway},
				{Id: testProjectIDSecond, Name: "[Acme] Second Project", Type: config.ProjectTypeEventGateway},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

// seedConfig writes a config file holding a usable CLI key and the "first"
// project, so a `project use` of the second project is a visible change.
func seedConfig(t *testing.T, path string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	contents := "profile = 'default'\n\n[default]\napi_key = 'cli_key_for_test'\nproject_id = '" +
		testProjectIDFirst + "'\nproject_mode = 'inbound'\nproject_type = 'Gateway'\n"
	require.NoError(t, os.WriteFile(path, []byte(contents), 0600))
}

// projectIDIn returns the active profile's project_id recorded in a config file.
func projectIDIn(t *testing.T, path string) string {
	t.Helper()
	var data map[string]interface{}
	_, err := toml.DecodeFile(path, &data)
	require.NoError(t, err, "config at %s should be valid TOML", path)
	section, ok := data["default"].(map[string]interface{})
	require.True(t, ok, "config at %s should have a [default] section", path)
	id, _ := section["project_id"].(string)
	return id
}

// runProjectUse runs `project use Acme "Second Project"` against a stubbed API
// with the given working directory, config flag and --local setting, and
// returns what the command printed.
func runProjectUse(t *testing.T, workingDir, configFileFlag string, local bool) (string, error) {
	t.Helper()

	server := projectUseAPIStub(t)

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(workingDir))
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	// HOOKDECK_CONFIG_FILE sits at the same precedence as --hookdeck-config, so
	// it has to be out of the way for the cases that exercise discovery.
	t.Setenv("HOOKDECK_CONFIG_FILE", "")

	origConfig := Config
	t.Cleanup(func() { Config = origConfig })
	config.ResetAPIClientForTesting()
	t.Cleanup(config.ResetAPIClientForTesting)

	Config = config.Config{
		LogLevel:       "info",
		APIBaseURL:     server.URL,
		ConfigFileFlag: configFileFlag,
	}
	Config.InitConfig()

	lc := newProjectUseCmd()
	lc.local = local

	origStdout := os.Stdout
	r, w, pipeErr := os.Pipe()
	require.NoError(t, pipeErr)
	os.Stdout = w

	runErr := lc.runProjectUseCmd(lc.cmd, []string{"Acme", "Second Project"})

	require.NoError(t, w.Close())
	os.Stdout = origStdout

	var buf [8192]byte
	n, _ := r.Read(buf[:])
	return string(buf[:n]), runErr
}

// TestProjectUseHookdeckConfigFlagBeatsCwdLocalConfig is the regression test for
// #424: with --hookdeck-config given, the named file is the one written, even
// when the working directory happens to contain .hookdeck/config.toml. Before
// the fix the local file was overwritten and the named file left untouched,
// while the command reported success — so the next command run with the same
// flag still reported the old project.
func TestProjectUseHookdeckConfigFlagBeatsCwdLocalConfig(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(tempDir, "xdg"))

	localConfigPath := filepath.Join(tempDir, ".hookdeck", "config.toml")
	seedConfig(t, localConfigPath)
	localBefore, err := os.ReadFile(localConfigPath)
	require.NoError(t, err)

	explicitConfigPath := filepath.Join(tempDir, "explicit.toml")
	seedConfig(t, explicitConfigPath)

	stdout, err := runProjectUse(t, tempDir, explicitConfigPath, false)
	require.NoError(t, err, "project use should succeed; output: %s", stdout)

	assert.Equal(t, testProjectIDSecond, projectIDIn(t, explicitConfigPath),
		"--hookdeck-config must write the file it names (#424)")

	localAfter, err := os.ReadFile(localConfigPath)
	require.NoError(t, err)
	assert.Equal(t, string(localBefore), string(localAfter),
		"--hookdeck-config must leave a cwd-local config byte-identical (#424)")

	assert.Contains(t, stdout, explicitConfigPath,
		"the reported path should be the file that was actually written")
}

// TestProjectUseLocalFlagWritesCwdLocalConfig covers rule 2: --local pins the
// write to ./.hookdeck/config.toml and leaves the global config alone.
func TestProjectUseLocalFlagWritesCwdLocalConfig(t *testing.T) {
	tempDir := t.TempDir()
	xdgHome := filepath.Join(tempDir, "xdg")
	t.Setenv("XDG_CONFIG_HOME", xdgHome)

	globalConfigPath := filepath.Join(xdgHome, "hookdeck", "config.toml")
	seedConfig(t, globalConfigPath)
	globalBefore, err := os.ReadFile(globalConfigPath)
	require.NoError(t, err)

	localConfigPath := filepath.Join(tempDir, ".hookdeck", "config.toml")
	require.NoFileExists(t, localConfigPath)

	stdout, err := runProjectUse(t, tempDir, "", true)
	require.NoError(t, err, "project use --local should succeed; output: %s", stdout)

	assert.Equal(t, testProjectIDSecond, projectIDIn(t, localConfigPath),
		"--local must write ./.hookdeck/config.toml")

	globalAfter, err := os.ReadFile(globalConfigPath)
	require.NoError(t, err)
	assert.Equal(t, string(globalBefore), string(globalAfter),
		"--local must not touch the global config")

	assert.Contains(t, stdout, "Created:", "a new local config should be reported as created")
}

// TestProjectUseWithoutFlagsPrefersExistingCwdLocalConfig covers rule 3 with a
// cwd-local config present: it is both read and written, and the global config
// is left alone.
func TestProjectUseWithoutFlagsPrefersExistingCwdLocalConfig(t *testing.T) {
	tempDir := t.TempDir()
	xdgHome := filepath.Join(tempDir, "xdg")
	t.Setenv("XDG_CONFIG_HOME", xdgHome)

	globalConfigPath := filepath.Join(xdgHome, "hookdeck", "config.toml")
	seedConfig(t, globalConfigPath)
	globalBefore, err := os.ReadFile(globalConfigPath)
	require.NoError(t, err)

	localConfigPath := filepath.Join(tempDir, ".hookdeck", "config.toml")
	seedConfig(t, localConfigPath)

	stdout, err := runProjectUse(t, tempDir, "", false)
	require.NoError(t, err, "project use should succeed; output: %s", stdout)

	assert.Equal(t, testProjectIDSecond, projectIDIn(t, localConfigPath),
		"an existing cwd-local config should be the one updated")

	globalAfter, err := os.ReadFile(globalConfigPath)
	require.NoError(t, err)
	assert.Equal(t, string(globalBefore), string(globalAfter),
		"the global config should be untouched while a cwd-local config exists")

	assert.Contains(t, stdout, "Updated:", "an existing local config should be reported as updated")
}

// TestProjectUseWithoutFlagsFallsBackToGlobalConfig covers rule 3 with no
// cwd-local config: the global config is written and no local one is created.
func TestProjectUseWithoutFlagsFallsBackToGlobalConfig(t *testing.T) {
	tempDir := t.TempDir()
	xdgHome := filepath.Join(tempDir, "xdg")
	t.Setenv("XDG_CONFIG_HOME", xdgHome)

	globalConfigPath := filepath.Join(xdgHome, "hookdeck", "config.toml")
	seedConfig(t, globalConfigPath)

	stdout, err := runProjectUse(t, tempDir, "", false)
	require.NoError(t, err, "project use should succeed; output: %s", stdout)

	assert.Equal(t, testProjectIDSecond, projectIDIn(t, globalConfigPath),
		"with no cwd-local config the global config should be written")
	assert.NoFileExists(t, filepath.Join(tempDir, ".hookdeck", "config.toml"),
		"project use without --local must not create a local config")
}

// TestProjectUseLocalAndHookdeckConfigConflict covers rule 4: the combination
// stays an error, and nothing is written.
func TestProjectUseLocalAndHookdeckConfigConflict(t *testing.T) {
	tempDir := t.TempDir()
	explicitConfigPath := filepath.Join(tempDir, "explicit.toml")
	seedConfig(t, explicitConfigPath)
	explicitBefore, err := os.ReadFile(explicitConfigPath)
	require.NoError(t, err)

	origConfig := Config
	t.Cleanup(func() { Config = origConfig })
	Config = config.Config{LogLevel: "info", ConfigFileFlag: explicitConfigPath}

	lc := newProjectUseCmd()
	lc.local = true

	runErr := lc.runProjectUseCmd(lc.cmd, []string{"Acme", "Second Project"})
	require.Error(t, runErr, "--local and --hookdeck-config together should be rejected")
	assert.Contains(t, runErr.Error(), "cannot be used together")

	explicitAfter, err := os.ReadFile(explicitConfigPath)
	require.NoError(t, err)
	assert.Equal(t, string(explicitBefore), string(explicitAfter),
		"a rejected invocation must not write any config")
	assert.NoFileExists(t, filepath.Join(tempDir, ".hookdeck", "config.toml"),
		"a rejected invocation must not create a local config")
}
