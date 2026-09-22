package mcp

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/mcpcore"
)

// These four are the invariants the read/write split exists to create. They are
// deliberately written against the registered tools rather than the specs, so
// they check what a client is actually handed.

// TestNoToolMixesReadAndWriteActions is the one that will silently regress.
//
// MCP clients grant per tool NAME and cannot match on arguments, so a single
// write action on a read tool makes that tool ungrantable — the caller is back
// to "allow everything or be prompted on every read". Adding an action is the
// moment that happens, and nothing else would catch it.
func TestNoToolMixesReadAndWriteActions(t *testing.T) {
	for _, spec := range resourceSpecs() {
		for _, group := range spec.Actions.Groups() {
			actions := spec.Actions.InGroup(group)
			require.NotEmpty(t, actions)

			writes := 0
			for _, a := range actions {
				if a.Write {
					writes++
				}
			}
			assert.True(t, writes == 0 || writes == len(actions),
				"%s_%s mixes gated and ungated actions (%v); an action enum must be homogeneous "+
					"or the tool cannot be granted as either a read or a write",
				toolPrefix, group, actions.Names())

			if group == mcpcore.GroupRead {
				for _, a := range actions {
					assert.False(t, a.Changes(),
						"%s_%s carries %q, which changes state; it belongs on the write tool, or on "+
							"its own via Action.Tool if it is deliberately ungated",
						spec.Resource, group, a.Name)
				}
			}
		}
	}
}

// TestAnnotationsMatchToolContents: the hints are what a client gates on, so a
// tool that under-reports is worse than one that says nothing.
func TestAnnotationsMatchToolContents(t *testing.T) {
	for _, writeEnabled := range []bool{false, true} {
		session := connectInMemoryWithMode(t, newTestClient("https://api.hookdeck.com", "k"), writeEnabled)
		tools := listTools(t, session)

		for _, spec := range resourceSpecs() {
			for _, group := range spec.Actions.Groups() {
				name := toolPrefix + "_" + spec.Resource + "_" + group
				tool, ok := tools[name]
				if !ok {
					continue
				}
				actions := spec.Actions.InGroup(group)
				require.NotNil(t, tool.Annotations, "%s has no annotations", name)

				assert.Equal(t, !actions.HasChanging(), tool.Annotations.ReadOnlyHint,
					"%s: ReadOnlyHint must be true exactly when no action changes state", name)
				require.NotNil(t, tool.Annotations.DestructiveHint, "%s: DestructiveHint must be set", name)
				assert.Equal(t, actions.HasDestructive(), *tool.Annotations.DestructiveHint,
					"%s: DestructiveHint must be true exactly when a destructive action is present", name)
			}
		}
	}
}

// TestReadToolsAreIdenticalInBothModes is what makes a grant durable. If a read
// tool's schema or description shifts when the server is restarted with
// --allow-write, a permission granted against it no longer describes the same
// thing.
func TestReadToolsAreIdenticalInBothModes(t *testing.T) {
	ro := listTools(t, connectInMemoryWithMode(t, newTestClient("https://api.hookdeck.com", "k"), false))
	rw := listTools(t, connectInMemoryWithMode(t, newTestClient("https://api.hookdeck.com", "k"), true))

	for name, a := range ro {
		b, ok := rw[name]
		require.True(t, ok, "%s disappeared when write mode was enabled", name)

		assert.Equal(t, a.Description, b.Description, "%s: description differs between modes", name)
		wantSchema, err := json.Marshal(a.InputSchema)
		require.NoError(t, err)
		gotSchema, err := json.Marshal(b.InputSchema)
		require.NoError(t, err)
		assert.JSONEq(t, string(wantSchema), string(gotSchema), "%s: schema differs between modes", name)
		// gateway_help and the platform tools are not built by ToolSpec.Define
		// and may leave DestructiveHint unset, so compare what is there rather
		// than requiring it here — TestAnnotationsMatchToolContents is where
		// the resource tools are required to set both.
		if a.Annotations == nil || b.Annotations == nil {
			assert.Equal(t, a.Annotations == nil, b.Annotations == nil,
				"%s: annotations appear in one mode and not the other", name)
			continue
		}
		assert.Equal(t, a.Annotations.ReadOnlyHint, b.Annotations.ReadOnlyHint,
			"%s: ReadOnlyHint differs between modes", name)
		assert.Equal(t, a.Annotations.DestructiveHint == nil, b.Annotations.DestructiveHint == nil,
			"%s: DestructiveHint is set in one mode and not the other", name)
		if a.Annotations.DestructiveHint != nil && b.Annotations.DestructiveHint != nil {
			assert.Equal(t, *a.Annotations.DestructiveHint, *b.Annotations.DestructiveHint,
				"%s: DestructiveHint differs between modes", name)
		}
	}
}

// TestWriteToolsAreAbsentInReadOnlyMode, and a write action asked for on the
// read tool still gets the mode message — not "unknown action", which would
// send the caller looking for a typo, and not silent success.
func TestWriteToolsAreAbsentInReadOnlyMode(t *testing.T) {
	tools := listTools(t, connectInMemoryWithMode(t, newTestClient("https://api.hookdeck.com", "k"), false))

	for _, spec := range resourceSpecs() {
		if !spec.Actions.HasWrite() {
			continue
		}
		name := toolPrefix + "_" + spec.Resource + "_" + mcpcore.GroupWrite
		assert.NotContains(t, tools, name, "%s must not be registered without --allow-write", name)
	}

	session := connectInMemory(t, newTestClient("https://api.hookdeck.com", "k"))
	result := callTool(t, session, toolPrefix+"_connections_read", map[string]any{"action": "delete", "id": "web_1"})
	require.True(t, result.IsError)
	assert.Contains(t, textContent(t, result), "--allow-write",
		"a gated action must name the flag that enables it, not read as a typo")
}

// TestListFiltersAreRefusedOnByIDActions closes the gap left when main's
// per-action argument whitelist was deleted in the v2.6.0 merge.
//
// Splitting by group stops a write-only property reaching a read-only caller.
// It does not stop a LIST filter being accepted on a by-id action of the same
// tool: {action:"get", id:"web_1", disabled:true} was taken, `disabled` was
// ignored, and one connection came back as though the filter had applied.
func TestListFiltersAreRefusedOnByIDActions(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/connections/web_1": func(w http.ResponseWriter, r *http.Request) {
			t.Fatalf("a list filter on a by-id action must not reach the API")
		},
	})

	for _, filter := range []string{"disabled", "limit", "next", "prev"} {
		t.Run(filter, func(t *testing.T) {
			args := map[string]any{"action": "get", "id": "web_1", filter: "1"}
			if filter == "disabled" {
				args[filter] = true
			}
			result := callTool(t, session, "gateway_connections_read", args)
			require.True(t, result.IsError, "%s must be refused on get", filter)
			body := textContent(t, result)
			assert.Contains(t, body, filter)
			assert.Contains(t, body, "list", "the message should name the action that does take it")
		})
	}
}

// The other half: a filter that genuinely serves both actions must still work.
// source_id filters the listing and links the source on create, so refusing it
// would be the same bug inverted — a real argument reported as wrong.
func TestFiltersServingSeveralActionsStillWork(t *testing.T) {
	var saw string
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/connections": func(w http.ResponseWriter, r *http.Request) {
			saw = r.URL.Query().Get("source_id")
			_ = json.NewEncoder(w).Encode(listResponse())
		},
	})
	result := callTool(t, session, "gateway_connections_read",
		map[string]any{"action": "list", "source_id": "src_1"})
	assert.False(t, result.IsError, textContent(t, result))
	assert.Equal(t, "src_1", saw)
}
