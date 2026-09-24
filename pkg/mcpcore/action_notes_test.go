package mcpcore

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A property shared by a resource's read and write tools had one description
// naming every action of the original, unsplit tool — so outpost_tenants_write
// told an agent its id "On list, filters by tenant ID(s)" on a tool with no
// list. ActionNotes scope each clause to the actions it is about, so a tool can
// only ever be shown the clauses that apply to it.
func TestActionNotesRenderPerTool(t *testing.T) {
	spec := ToolSpec{
		Resource: "tenants",
		Actions: ActionSet{
			{Name: "list"},
			{Name: "get"},
			{Name: "upsert", Write: true},
			{Name: "delete", Write: true},
		},
		Props: map[string]Prop{
			"id": {Type: "string", Desc: "Tenant ID.", ActionNotes: []ActionNote{
				{On: []string{"get", "upsert", "delete"}, Text: "Required for %s."},
				{On: []string{"list"}, Text: "On list, filters by tenant ID(s)."},
			}},
		},
	}

	read := spec.VisibleProps(GroupRead)
	assert.Equal(t, "Tenant ID. Required for get. On list, filters by tenant ID(s).", read["id"].Desc)

	write := spec.VisibleProps(GroupWrite)
	assert.Equal(t, "Tenant ID. Required for upsert/delete.", write["id"].Desc)

	// The spec is the shared source both tools render from, so rendering must
	// not consume it.
	require.Len(t, spec.Props["id"].ActionNotes, 2, "rendering must not mutate the spec")
}

func TestActionNotesOmitClausesWithNoActionOnThisTool(t *testing.T) {
	spec := ToolSpec{
		Resource: "destinations",
		Actions:  ActionSet{{Name: "list"}, {Name: "create", Write: true}},
		Props: map[string]Prop{
			"type": {Type: "string", Desc: "Destination type.", ActionNotes: []ActionNote{
				{On: []string{"create"}, Text: "Required for %s."},
				{On: []string{"list"}, Text: "On list, filters by type(s)."},
				// An action this resource does not have at all contributes
				// nothing anywhere, rather than silently naming itself.
				{On: []string{"nonesuch"}, Text: "Ignored on %s."},
			}},
		},
	}

	assert.Equal(t, "Destination type. On list, filters by type(s).", spec.VisibleProps(GroupRead)["type"].Desc)
	assert.Equal(t, "Destination type. Required for create.", spec.VisibleProps(GroupWrite)["type"].Desc)
}

// A note without a %s is appended as written, for a clause whose wording does
// not read as a list ("On run, executes this unsaved code.").
func TestActionNoteWithoutPlaceholder(t *testing.T) {
	spec := ToolSpec{
		Resource: "transformations",
		Actions:  ActionSet{{Name: "run"}, {Name: "update", Write: true}},
		Props: map[string]Prop{
			"code": {Type: "string", Desc: "JavaScript source.", ActionNotes: []ActionNote{
				{On: []string{"run"}, Text: "On run, executes this unsaved code."},
			}},
		},
	}

	assert.Equal(t, "JavaScript source. On run, executes this unsaved code.", spec.VisibleProps(GroupRead)["code"].Desc)
	assert.Equal(t, "JavaScript source.", spec.VisibleProps(GroupWrite)["code"].Desc)
}
