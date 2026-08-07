package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// HasStoredAPIKey exists so commands can tell "this machine already has a
// login" apart from "a key was supplied for this run". Profile.APIKey cannot
// answer that on its own, because InitConfig coalesces the flag value over the
// stored one.
func TestHasStoredAPIKey(t *testing.T) {
	t.Parallel()

	t.Run("false when the config file holds no key", func(t *testing.T) {
		t.Parallel()

		c := Config{
			LogLevel:       "info",
			ConfigFileFlag: "./testdata/empty.toml",
		}
		c.InitConfig()

		assert.False(t, c.HasStoredAPIKey)
	})

	t.Run("true when the config file holds a key", func(t *testing.T) {
		t.Parallel()

		c := Config{
			LogLevel:       "info",
			ConfigFileFlag: "./testdata/default-profile.toml",
		}
		c.InitConfig()

		assert.True(t, c.HasStoredAPIKey)
		assert.Equal(t, "test_api_key", c.Profile.APIKey)
	})

	// The case the flag guard depends on: a key arriving by flag must not look
	// like an existing login, or `listen --cli-key` would decline to save on a
	// machine that has never been authenticated.
	t.Run("false when the key came from a flag and none was stored", func(t *testing.T) {
		t.Parallel()

		c := Config{
			LogLevel:       "info",
			ConfigFileFlag: "./testdata/empty.toml",
		}
		// Mirrors the flag binding: --cli-key writes here before InitConfig runs.
		c.Profile.APIKey = "key_from_flag"
		c.InitConfig()

		assert.False(t, c.HasStoredAPIKey, "a flag-supplied key is not a stored login")
		assert.Equal(t, "key_from_flag", c.Profile.APIKey)
	})

	// And the converse: a flag value wins the coalesce, so Profile.APIKey alone
	// can no longer reveal that a different key is still on disk.
	t.Run("true when a flag overrides a stored key", func(t *testing.T) {
		t.Parallel()

		c := Config{
			LogLevel:       "info",
			ConfigFileFlag: "./testdata/default-profile.toml",
		}
		c.Profile.APIKey = "key_from_flag"
		c.InitConfig()

		assert.True(t, c.HasStoredAPIKey, "a stored login is still present behind the flag")
		assert.Equal(t, "key_from_flag", c.Profile.APIKey, "the flag value wins for this run")
	})
}
