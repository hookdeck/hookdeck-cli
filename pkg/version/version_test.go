package version

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNeedsToUpgrade(t *testing.T) {
	// Test basic version comparison
	require.False(t, needsToUpgrade("1.9.1", "v1.9.1"), "same version should not need upgrade")
	require.False(t, needsToUpgrade("v1.9.1", "1.9.1"), "same version with different v prefix should not need upgrade")
	require.True(t, needsToUpgrade("1.9.1", "1.9.2"), "older patch version should need upgrade")
	require.True(t, needsToUpgrade("1.9.1", "v1.9.2"), "older patch version with v prefix should need upgrade")
	require.True(t, needsToUpgrade("v1.9.1", "v1.9.2"), "older patch version both with v prefix should need upgrade")

	// Test major/minor version comparison - the key bug fix
	require.True(t, needsToUpgrade("1.9.1", "1.10.0"), "1.9.1 is older than 1.10.0, should need upgrade")
	require.True(t, needsToUpgrade("v1.9.1", "v1.10.0"), "v1.9.1 is older than v1.10.0, should need upgrade")
	require.False(t, needsToUpgrade("1.10.0", "1.9.1"), "1.10.0 is newer than 1.9.1, should not need upgrade")

	// Test pre-release/beta version comparison - the key bug fix for issue #232
	// Case 1: On beta, newer stable exists - should suggest upgrade
	require.True(t, needsToUpgrade("1.10.0-beta.4", "1.10.0"), "beta should upgrade to stable release")
	require.True(t, needsToUpgrade("v1.10.0-beta.4", "v1.10.0"), "beta with v prefix should upgrade to stable")

	// Case 2: On beta, newer beta exists - should suggest upgrade
	require.True(t, needsToUpgrade("1.10.0-beta.4", "1.10.0-beta.5"), "older beta should upgrade to newer beta")
	require.True(t, needsToUpgrade("v1.10.0-beta.4", "v1.10.0-beta.5"), "older beta with v prefix should upgrade to newer beta")

	// Case 3: On stable, newer beta exists - should NOT suggest upgrade
	require.False(t, needsToUpgrade("1.9.1", "1.10.0-beta.4"), "stable should not upgrade to beta")
	require.False(t, needsToUpgrade("v1.9.1", "v1.10.0-beta.4"), "stable with v prefix should not upgrade to beta")

	// Case 4: Beta vs older stable - the original bug case
	require.False(t, needsToUpgrade("1.10.0-beta.4", "1.9.1"), "newer beta base version should not downgrade to older stable")
	require.False(t, needsToUpgrade("v1.10.0-beta.4", "v1.9.1"), "newer beta base version with v prefix should not downgrade to older stable")

	// Test with RC (release candidate) versions
	require.True(t, needsToUpgrade("1.10.0-rc.1", "1.10.0"), "RC should upgrade to stable")
	require.True(t, needsToUpgrade("1.10.0-rc.1", "1.10.0-rc.2"), "older RC should upgrade to newer RC")
	require.False(t, needsToUpgrade("1.9.1", "1.10.0-rc.1"), "stable should not upgrade to RC")

	// Test edge cases
	require.False(t, needsToUpgrade("1.9.1", ""), "empty latest version should not need upgrade")
	require.False(t, needsToUpgrade("2.0.0", "1.9.1"), "newer current version should not need upgrade")

	// Test double-digit minor/patch versions
	require.True(t, needsToUpgrade("1.9.0", "1.10.0"), "1.9.0 < 1.10.0")
	require.True(t, needsToUpgrade("1.9.99", "1.10.0"), "1.9.99 < 1.10.0")
	require.True(t, needsToUpgrade("1.10.0", "1.11.0"), "1.10.0 < 1.11.0")
}
