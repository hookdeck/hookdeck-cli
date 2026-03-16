package version

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNeedsToUpgrade(t *testing.T) {
	// Basic same-version checks (v-prefix normalisation).
	require.False(t, needsToUpgrade("4.2.4.2", "v4.2.4.2"))
	require.False(t, needsToUpgrade("4.2.4.2", "4.2.4.2"))

	// GA to newer GA — should upgrade.
	require.True(t, needsToUpgrade("4.2.4.2", "4.2.4.3"))
	require.True(t, needsToUpgrade("4.2.4.2", "v4.2.4.3"))
	require.True(t, needsToUpgrade("v4.2.4.2", "v4.2.4.3"))

	// Minor/major version bump where numeric comparison matters
	// (string comparison would wrongly treat 1.9.x > 1.10.x).
	require.True(t, needsToUpgrade("1.9.1", "1.10.0"))
	require.False(t, needsToUpgrade("1.10.0", "1.9.1"))

	// GA current, pre-release latest — must NOT suggest upgrade.
	require.False(t, needsToUpgrade("1.9.1", "1.10.0-beta.4"))
	require.False(t, needsToUpgrade("1.9.1", "v1.10.0-beta.4"))
	require.False(t, needsToUpgrade("1.10.0", "1.10.1-beta.1"))

	// Pre-release current, newer GA latest — should upgrade.
	require.True(t, needsToUpgrade("1.9.0-beta.4", "1.9.0"))
	require.True(t, needsToUpgrade("1.10.0-beta.4", "1.10.0"))

	// Pre-release current, older GA latest — should NOT upgrade.
	require.False(t, needsToUpgrade("1.10.0-beta.4", "1.9.1"))

	// Pre-release to newer pre-release — should upgrade.
	require.True(t, needsToUpgrade("1.9.0-beta.3", "1.9.0-beta.4"))
	require.True(t, needsToUpgrade("1.9.0-beta.9", "1.9.0-beta.10"))

	// Pre-release to older pre-release — should NOT upgrade.
	require.False(t, needsToUpgrade("1.9.0-beta.4", "1.9.0-beta.3"))

	// Same pre-release — should NOT upgrade.
	require.False(t, needsToUpgrade("1.9.0-beta.4", "1.9.0-beta.4"))
}
