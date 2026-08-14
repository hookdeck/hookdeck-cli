package mcpcore

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

func TestRequireAuth(t *testing.T) {
	t.Run("no API key names the server's login tool", func(t *testing.T) {
		result := RequireAuth(&hookdeck.Client{}, "outpost_login")
		require.NotNil(t, result)
		assert.True(t, result.IsError)
		assert.Contains(t, firstText(t, result), "outpost_login")
	})

	t.Run("API key present passes", func(t *testing.T) {
		assert.Nil(t, RequireAuth(&hookdeck.Client{APIKey: "key"}, "hookdeck_login"))
	})
}

func TestRequireWrite(t *testing.T) {
	t.Run("write mode enabled passes", func(t *testing.T) {
		assert.Nil(t, RequireWrite(true, "delete"))
	})

	t.Run("read-only mode names the action and the flag", func(t *testing.T) {
		result := RequireWrite(false, "delete")
		require.NotNil(t, result)
		assert.True(t, result.IsError)
		text := firstText(t, result)
		assert.Contains(t, text, `"delete"`)
		assert.Contains(t, text, "--allow-write")
		assert.Contains(t, text, "read-only mode")
	})
}
