package mcpcore

import (
	"testing"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// project_id belongs to use and nothing else. Advertised on the read half it
// was accepted by list and then ignored, so {"action":"list","project_id":…}
// returned every project as though it had been filtered.
func TestProjectsProjectIDBelongsToUse(t *testing.T) {
	spec := (&Server{}).ProjectsSpec("projects")

	prop, ok := spec.Props["project_id"]
	require.True(t, ok, "the projects tool must still advertise project_id")
	require.Equal(t, []string{"use"}, prop.Actions,
		"project_id is read only by the use action; list ignores it")

	assert.True(t, prop.appliesTo("use"))
	assert.False(t, prop.appliesTo("list"),
		"list must reject project_id rather than silently returning an unfiltered set")
}

// The read half must not carry it at all: use lives on its own tool, so the
// property has no action to belong to there.
func TestProjectsReadHalfDoesNotAdvertiseProjectID(t *testing.T) {
	spec := (&Server{}).ProjectsSpec("projects")

	assert.NotContains(t, spec.VisibleProps(GroupRead), "project_id",
		"the read tool offers only list, which never reads project_id")
	assert.Contains(t, spec.VisibleProps("use"), "project_id",
		"the use tool still needs it")
}

// A browser login lands on whichever project the user was last in. The Outpost
// server must not adopt a Gateway one: ProjectFilter is enforced by the
// projects tools alone, so every later resource call would reach the wrong
// product and return 404 — data that reads as missing rather than a project
// that was never right.
func TestLoginRefusesAProjectThisServerCannotServe(t *testing.T) {
	gateway := &hookdeck.PollAPIKeyResponse{
		UserName: "someone", ProjectName: "Acme / Web", ProjectType: "event_gateway",
	}

	t.Run("outpost server refuses a gateway project", func(t *testing.T) {
		err := loginProjectMismatch(config.ProjectTypeOutpost, gateway, "hookdeck_login")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Nothing was saved",
			"the caller must be told no half-state was written")
		assert.Contains(t, err.Error(), "hookdeck_login", "and how to retry")
	})

	t.Run("outpost server accepts an outpost project", func(t *testing.T) {
		outpost := &hookdeck.PollAPIKeyResponse{ProjectType: "outpost"}
		assert.NoError(t, loginProjectMismatch(config.ProjectTypeOutpost, outpost, "hookdeck_login"))
	})

	// A server with no filter serves either, which is what the un-split CLI
	// login does; it must not start refusing logins it used to accept.
	t.Run("an unfiltered server accepts anything", func(t *testing.T) {
		assert.NoError(t, loginProjectMismatch("", gateway, "hookdeck_login"))
	})

	// The type is whatever the API said. An unrecognised one is not a match, so
	// it is refused rather than adopted on the assumption it is fine.
	t.Run("an unrecognised type is refused, not assumed", func(t *testing.T) {
		unknown := &hookdeck.PollAPIKeyResponse{ProjectName: "X", ProjectType: "something_new"}
		err := loginProjectMismatch(config.ProjectTypeOutpost, unknown, "hookdeck_login")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unrecognised type")
	})
}
