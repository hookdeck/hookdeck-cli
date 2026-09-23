package mcpcore

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
)

// The platform specs split like any other, and keep the hookdeck_ prefix in
// every server. A client with both servers configured sees one name for one
// operation, which is only true if the prefix does not follow the product.
func TestPlatformSpecsSplitAndKeepTheirPrefix(t *testing.T) {
	api := projectsAPI(t)
	srv, _ := newProjectsServer(t, api, config.ProjectTypeOutpost)

	got := map[string][]string{}
	for _, spec := range srv.PlatformSpecs("summary") {
		for _, group := range spec.Actions.Groups() {
			got[spec.GroupToolName(srv, group)] = spec.Actions.InGroup(group).Names()
		}
	}

	assert.Equal(t, map[string][]string{
		"hookdeck_projects_read": {"list"},
		"hookdeck_projects_use":  {"use"},
	}, got, "the product prefix must not reach a platform tool")
}

// use changes which project every later call targets, so it cannot claim to be
// a pure read — but it stays available without --allow-write, because a
// read-only session stuck in one project cannot investigate another.
func TestProjectsUseIsUngatedButNotARead(t *testing.T) {
	use, ok := projectsActions.Find("use")
	require.True(t, ok)

	assert.False(t, use.Write, "gating use would strand a read-only session in one project")
	assert.True(t, use.Changes(), "use changes what later calls target")
	assert.Equal(t, "use", use.Group(), "use belongs on its own tool, not the read one")

	read := projectsActions.InGroup(GroupRead)
	assert.False(t, read.HasChanging(), "the read tool must stay annotatable as read-only")
}
