package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// persistedConfig builds a Config backed by a real config file on disk, so the
// difference between clearing in memory and clearing on disk is observable.
func persistedConfig(t *testing.T) (*Config, string) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(path, []byte("[default]\n  api_key = \"sk_test_123456789012\"\n  project_id = \"proj_1\"\n"), 0o600))

	v := viper.New()
	v.SetConfigType("toml")
	v.SetConfigFile(path)
	require.NoError(t, v.ReadInConfig())

	c := &Config{viper: v, fs: newConfigFS()}
	c.Profile.Name = "default"
	c.Profile.Config = c
	c.Profile.APIKey = "sk_test_123456789012"
	c.Profile.ProjectId = "proj_1"
	c.Profile.ProjectMode = "inbound"
	c.Profile.ProjectType = ProjectTypeGateway
	c.Profile.GuestURL = "https://example.test/guest"
	return c, path
}

// A sign-in that is started and never finished must not sign the user out.
//
// The MCP reauth path clears credentials before opening a browser flow that the
// user may abandon. Clearing them on disk at that point gains nothing — a
// completed sign-in overwrites every field anyway — and costs the user their
// session in every terminal if the flow never completes.
func TestClearInMemoryLeavesTheStoredCredentialsAlone(t *testing.T) {
	c, path := persistedConfig(t)
	before, err := os.ReadFile(path)
	require.NoError(t, err)

	c.ClearActiveProfileCredentialsInMemory()

	t.Run("this process stops using the old credentials", func(t *testing.T) {
		assert.Empty(t, c.Profile.APIKey)
		assert.Empty(t, c.Profile.ProjectId)
		assert.Empty(t, c.Profile.ProjectMode)
		assert.Empty(t, c.Profile.ProjectType)
		assert.Empty(t, c.Profile.GuestURL)
	})

	t.Run("the stored credentials survive", func(t *testing.T) {
		after, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, string(before), string(after),
			"an abandoned sign-in must leave the user logged in")
		assert.Contains(t, string(after), "sk_test_123456789012")
	})
}

// The persisting variant is what `hookdeck logout` uses, and it must keep
// removing the stored credentials. This is the half that must not change.
func TestClearActiveProfileCredentialsStillRemovesThemFromDisk(t *testing.T) {
	c, path := persistedConfig(t)

	require.NoError(t, c.ClearActiveProfileCredentials())

	assert.Empty(t, c.Profile.APIKey)

	after, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.NotContains(t, string(after), "sk_test_123456789012",
		"logout must still remove the stored key")
}

// Both clear the same in-memory fields; they differ only in whether the stored
// credentials are removed. Pinning that here so the two cannot be collapsed
// back into one.
func TestBothClearTheSameInMemoryFields(t *testing.T) {
	inMemory, _ := persistedConfig(t)
	persisted, _ := persistedConfig(t)

	inMemory.ClearActiveProfileCredentialsInMemory()
	require.NoError(t, persisted.ClearActiveProfileCredentials())

	assert.Equal(t, persisted.Profile.APIKey, inMemory.Profile.APIKey)
	assert.Equal(t, persisted.Profile.ProjectId, inMemory.Profile.ProjectId)
	assert.Equal(t, persisted.Profile.ProjectMode, inMemory.Profile.ProjectMode)
	assert.Equal(t, persisted.Profile.ProjectType, inMemory.Profile.ProjectType)
	assert.Equal(t, persisted.Profile.GuestURL, inMemory.Profile.GuestURL)
}
