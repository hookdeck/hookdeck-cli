package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveOutpostFieldMapDottedPaths(t *testing.T) {
	t.Parallel()

	t.Run("flat keys are unchanged", func(t *testing.T) {
		got, err := resolveOutpostFieldMap([]string{"url=https://example.com"}, "", "config")
		require.NoError(t, err)
		assert.Equal(t, map[string]interface{}{"url": "https://example.com"}, got)
	})

	t.Run("a dotted key builds a nested object", func(t *testing.T) {
		got, err := resolveOutpostFieldMap([]string{"auth.type=BEARER", "auth.token=xyz"}, "", "config")
		require.NoError(t, err)
		assert.Equal(t, map[string]interface{}{
			"auth": map[string]interface{}{"type": "BEARER", "token": "xyz"},
		}, got)
	})

	t.Run("paths nest arbitrarily deep", func(t *testing.T) {
		got, err := resolveOutpostFieldMap([]string{"a.b.c.d=v"}, "", "config")
		require.NoError(t, err)
		assert.Equal(t, map[string]interface{}{
			"a": map[string]interface{}{"b": map[string]interface{}{"c": map[string]interface{}{"d": "v"}}},
		}, got)
	})

	t.Run("an escaped dot stays part of the key", func(t *testing.T) {
		got, err := resolveOutpostFieldMap([]string{`custom\.header=value`}, "", "config")
		require.NoError(t, err)
		assert.Equal(t, map[string]interface{}{"custom.header": "value"}, got)
	})

	t.Run("a value containing dots is untouched", func(t *testing.T) {
		// Only the key is a path; values routinely contain dots.
		got, err := resolveOutpostFieldMap([]string{"url=https://a.b.example.com/x"}, "", "config")
		require.NoError(t, err)
		assert.Equal(t, map[string]interface{}{"url": "https://a.b.example.com/x"}, got)
	})

	t.Run("a value containing = is untouched", func(t *testing.T) {
		got, err := resolveOutpostFieldMap([]string{"url=https://example.com?a=1&b=2"}, "", "config")
		require.NoError(t, err)
		assert.Equal(t, map[string]interface{}{"url": "https://example.com?a=1&b=2"}, got)
	})

	t.Run("a scalar and a path cannot claim the same key", func(t *testing.T) {
		_, err := resolveOutpostFieldMap([]string{"a=1", "a.b=2"}, "", "config")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "conflicts with an earlier value")
	})

	t.Run("an empty path segment is rejected", func(t *testing.T) {
		_, err := resolveOutpostFieldMap([]string{"a..b=1"}, "", "config")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "empty path segment")
	})

	t.Run("a pair without = is rejected", func(t *testing.T) {
		_, err := resolveOutpostFieldMap([]string{"justakey"}, "", "config")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must be in key=value form")
	})
}

func TestSplitDottedPath(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []string{"a"}, splitDottedPath("a"))
	assert.Equal(t, []string{"a", "b"}, splitDottedPath("a.b"))
	assert.Equal(t, []string{"a.b"}, splitDottedPath(`a\.b`))
	assert.Equal(t, []string{"a.b", "c"}, splitDottedPath(`a\.b.c`))
}
