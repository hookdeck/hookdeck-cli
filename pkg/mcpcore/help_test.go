package mcpcore

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHelpTopic(t *testing.T) {
	topics := map[string]string{
		"outpost_events":  "events help",
		"outpost_tenants": "tenants help",
	}

	t.Run("qualified name resolves", func(t *testing.T) {
		result := HelpTopic("outpost_", topics, "outpost_events", "")
		assert.False(t, result.IsError)
		assert.Equal(t, "events help", firstText(t, result))
	})

	t.Run("bare name is prefixed", func(t *testing.T) {
		result := HelpTopic("outpost_", topics, "events", "")
		assert.False(t, result.IsError)
		assert.Equal(t, "events help", firstText(t, result))
	})

	t.Run("suffix is appended", func(t *testing.T) {
		result := HelpTopic("outpost_", topics, "events", "shared docs")
		assert.Equal(t, "events help\n\nshared docs", firstText(t, result))
	})

	t.Run("unknown topic lists the available tools", func(t *testing.T) {
		result := HelpTopic("outpost_", topics, "how do I retry", "")
		assert.True(t, result.IsError)
		text := firstText(t, result)
		assert.Contains(t, text, "No help found")
		assert.Contains(t, text, "outpost_events")
		assert.Contains(t, text, "outpost_tenants")
	})

	t.Run("another server's prefix does not resolve these topics", func(t *testing.T) {
		result := HelpTopic("hookdeck_", topics, "events", "")
		assert.True(t, result.IsError)
	})
}
