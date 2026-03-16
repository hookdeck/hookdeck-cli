package version

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNeedsToUpgrade(t *testing.T) {
	// Same version (with and without v prefix) — no upgrade
	require.False(t, needsToUpgrade("1.2.3", "v1.2.3"))
	require.False(t, needsToUpgrade("1.2.3", "1.2.3"))
	require.False(t, needsToUpgrade("v1.2.3", "v1.2.3"))

	// GA → newer GA — prompt
	require.True(t, needsToUpgrade("1.2.3", "1.2.4"))
	require.True(t, needsToUpgrade("1.2.3", "v1.2.4"))
	require.True(t, needsToUpgrade("v1.2.3", "v1.2.4"))

	// Multi-digit minor version: 1.9.x → 1.10.x — prompt (was broken with string comparison)
	require.True(t, needsToUpgrade("1.9.1", "1.10.0"))
	require.True(t, needsToUpgrade("1.9.1", "v1.10.0"))

	// Pre-release running beta → newer GA — prompt
	require.True(t, needsToUpgrade("1.10.0-beta.4", "1.10.0"))
	require.True(t, needsToUpgrade("1.10.0-beta.4", "v1.10.0"))

	// Pre-release running beta → newer beta — prompt
	require.True(t, needsToUpgrade("1.10.0-beta.3", "1.10.0-beta.4"))
	require.True(t, needsToUpgrade("1.10.0-beta.3", "v1.10.0-beta.4"))

	// GA → beta (older stable should not be pushed to pre-release) — no prompt
	require.False(t, needsToUpgrade("1.9.1", "1.10.0-beta.4"))
	require.False(t, needsToUpgrade("1.9.1", "v1.10.0-beta.4"))

	// Original bug: beta 1.10.0-beta.4 vs stable v1.9.1 — no upgrade (current is newer)
	require.False(t, needsToUpgrade("1.10.0-beta.4", "v1.9.1"))
	require.False(t, needsToUpgrade("1.10.0-beta.4", "1.9.1"))

	// Older version — no upgrade
	require.False(t, needsToUpgrade("1.2.4", "1.2.3"))
	require.False(t, needsToUpgrade("1.10.0", "1.9.1"))

	// Empty latest — no upgrade
	require.False(t, needsToUpgrade("1.2.3", ""))
}
