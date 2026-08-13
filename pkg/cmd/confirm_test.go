package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfirmDestructiveActionWithoutTerminal covers the fix for destructive
// commands exiting 0 on a skipped confirmation. `source delete`, `destination
// delete`, `connection delete`, `transformation delete` and `issue dismiss` all
// called fmt.Scanln and discarded its error, so with no terminal the response
// stayed empty, they printed "cancelled" and returned nil — a CI job that deleted
// nothing exited 0 and looked like a success.
func TestConfirmDestructiveActionWithoutTerminal(t *testing.T) {
	original := stdinIsTerminal
	t.Cleanup(func() { stdinIsTerminal = original })

	stdinIsTerminal = func() bool { return false }

	proceed, err := confirmDestructiveAction("Are you sure?", "Deletion cancelled.", "force")

	require.Error(t, err, "a confirmation that cannot be asked must be an error, not a silent no-op")
	assert.False(t, proceed, "must never proceed with a destructive action it could not confirm")

	assert.Contains(t, err.Error(), "--force",
		"the error must name the flag that makes this work non-interactively")
	assert.Contains(t, err.Error(), "no terminal is attached",
		"the error must explain why the prompt was skipped")
}

// TestConfirmDestructiveActionNamesTheRightFlag guards the caller contract: the
// helper takes the flag name so the message stays correct if a command ever uses
// something other than --force.
func TestConfirmDestructiveActionNamesTheRightFlag(t *testing.T) {
	original := stdinIsTerminal
	t.Cleanup(func() { stdinIsTerminal = original })

	stdinIsTerminal = func() bool { return false }

	_, err := confirmDestructiveAction("Are you sure?", "Cancelled.", "yes")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--yes")
}
