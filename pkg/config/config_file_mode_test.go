//go:build !windows

package config

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The config file holds the CLI key, which can be account-wide. The global
// config was kept owner-only, but a --hookdeck-config path was created with the
// umask (0644 under 022) and a later login wrote the key into it, readable by
// every other local user (#445).
//
// The umask is pinned to 022 because that is what produces the bug; under a
// stricter umask these would pass without the fix. Not parallel: the umask is
// process-wide.

func withUmask022(t *testing.T) {
	t.Helper()
	old := syscall.Umask(0o022)
	t.Cleanup(func() { syscall.Umask(old) })
}

func mode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	require.NoError(t, err)
	return info.Mode().Perm()
}

func TestHookdeckConfigPathIsCreatedOwnerOnly(t *testing.T) {
	withUmask022(t)
	path := filepath.Join(t.TempDir(), "mine.toml")

	c := Config{LogLevel: "info", ConfigFileFlag: path}
	c.InitConfig()

	assert.Equal(t, os.FileMode(0o600), mode(t, path),
		"a config file created at a --hookdeck-config path must be owner-only")
}

func TestSavingAKeyLeavesTheFileOwnerOnly(t *testing.T) {
	withUmask022(t)
	path := filepath.Join(t.TempDir(), "mine.toml")

	c := Config{LogLevel: "info", ConfigFileFlag: path}
	c.InitConfig()
	c.Profile.APIKey = "cli_test_key_not_real"
	require.NoError(t, c.Profile.SaveProfile())

	assert.Equal(t, os.FileMode(0o600), mode(t, path),
		"the file a key is written into must be owner-only")
}

// A file that already exists keeps its mode through viper's WriteConfig, so a
// user-supplied 0644 file would hold the key world-readable. Tightened on write.
func TestSavingAKeyTightensAnExistingWorldReadableFile(t *testing.T) {
	withUmask022(t)
	path := filepath.Join(t.TempDir(), "existing.toml")
	require.NoError(t, os.WriteFile(path, []byte{}, 0o644))
	require.NoError(t, os.Chmod(path, 0o644))

	c := Config{LogLevel: "info", ConfigFileFlag: path}
	c.InitConfig()
	c.Profile.APIKey = "cli_test_key_not_real"
	require.NoError(t, c.Profile.SaveProfile())

	assert.Equal(t, os.FileMode(0o600), mode(t, path))
}
