package mcp

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/mcpcore"
)

// TestToolSurfaceIsWhatWeThinkItIs renders the whole advertised surface in both
// modes and asserts it, as a golden list in the test itself.
//
// The plan's final verification step was "start both servers in both modes and
// diff tools/list". Doing that by hand once proves nothing a month from now, so
// it lives here: any tool added, removed, renamed or re-annotated shows up as a
// diff on this list, and the reviewer has to agree the change was intended.
func TestToolSurfaceIsWhatWeThinkItIs(t *testing.T) {
	want := map[bool]string{
		false: `gateway_attempts_read           ro  .  [list get]
gateway_bulk_read               ro  .  [list get plan]
gateway_connections_pause       rw  .  [pause unpause]
gateway_connections_read        ro  .  [list get]
gateway_destinations_read       ro  .  [list get]
gateway_event_read              ro  .  [get raw_body]
gateway_events_read             ro  .  [list list_ignored]
gateway_help                    ro  .  []
gateway_issues_read             ro  .  [list get]
gateway_metrics_read            ro  .  [events requests attempts transformations]
gateway_request_read            ro  .  [get raw_body]
gateway_requests_read           ro  .  [list]
gateway_sources_read            ro  .  [list get]
gateway_transformations_read    ro  .  [list get run]
hookdeck_login                  rw  .  []
hookdeck_projects_read          ro  .  [list]
hookdeck_projects_use           rw  .  [use]`,

		true: `gateway_attempts_read           ro  .  [list get]
gateway_bulk_read               ro  .  [list get plan]
gateway_bulk_write              rw  D  [create cancel]
gateway_connections_pause       rw  .  [pause unpause]
gateway_connections_read        ro  .  [list get]
gateway_connections_write       rw  D  [create upsert update delete enable disable]
gateway_destinations_read       ro  .  [list get]
gateway_destinations_write      rw  D  [create upsert update delete enable disable]
gateway_event_read              ro  .  [get raw_body]
gateway_event_write             rw  D  [retry cancel mute]
gateway_events_read             ro  .  [list list_ignored]
gateway_help                    ro  .  []
gateway_issues_read             ro  .  [list get]
gateway_issues_write            rw  D  [update dismiss]
gateway_metrics_read            ro  .  [events requests attempts transformations]
gateway_request_read            ro  .  [get raw_body]
gateway_request_write           rw  .  [retry]
gateway_requests_read           ro  .  [list]
gateway_sources_read            ro  .  [list get]
gateway_sources_write           rw  D  [create upsert update delete enable disable]
gateway_transformations_read    ro  .  [list get run]
gateway_transformations_write   rw  D  [create upsert update delete]
hookdeck_login                  rw  .  []
hookdeck_projects_read          ro  .  [list]
hookdeck_projects_use           rw  .  [use]`,
	}

	for _, writeEnabled := range []bool{false, true} {
		mode := "read-only"
		if writeEnabled {
			mode = "write"
		}
		t.Run(mode, func(t *testing.T) {
			got := renderSurface(t, writeEnabled)
			assert.Equal(t, want[writeEnabled], got,
				"the advertised tool surface changed. If that was intended, update this list "+
					"— and check the change against plans/mcp_read_write_tool_split.md")
		})
	}
}

// Every name ends in a group suffix, or is one of the two documented
// exceptions. This is what makes `*_read` a usable permission rule.
func TestEveryToolNameCarriesItsPosture(t *testing.T) {
	exceptions := map[string]string{
		"gateway_help":              "read-only by construction, and not a resource tool",
		"hookdeck_login":            "authenticates; it is not a read or a write of project data",
		"gateway_connections_pause": "ungated mutation, deliberately its own tool",
		"hookdeck_projects_use":     "ungated mutation, deliberately its own tool",
	}

	tools := listTools(t, connectInMemoryWithMode(t, &hookdeck.Client{APIKey: "k"}, true))
	for name := range tools {
		if _, ok := exceptions[name]; ok {
			continue
		}
		assert.True(t,
			strings.HasSuffix(name, "_"+mcpcore.GroupRead) || strings.HasSuffix(name, "_"+mcpcore.GroupWrite),
			"%s ends in neither _read nor _write; a client granting `*_read` would miss it, "+
				"and it is not one of the documented exceptions", name)
	}
}

func renderSurface(t *testing.T, writeEnabled bool) string {
	t.Helper()
	tools := listTools(t, connectInMemoryWithMode(t, &hookdeck.Client{APIKey: "k"}, writeEnabled))

	names := make([]string, 0, len(tools))
	for n := range tools {
		names = append(names, n)
	}
	sort.Strings(names)

	lines := make([]string, 0, len(names))
	for _, n := range names {
		// A tool with no annotations is itself worth seeing in this list: an
		// unset readOnlyHint defaults to false, which makes clients prompt on
		// safe reads.
		posture, destructive := "??", "?"
		if a := tools[n].Annotations; a != nil {
			posture, destructive = "ro", "."
			if !a.ReadOnlyHint {
				posture = "rw"
			}
			if a.DestructiveHint != nil && *a.DestructiveHint {
				destructive = "D"
			}
		}
		lines = append(lines, fmt.Sprintf("%-31s %s  %s  %v", n, posture, destructive, actionEnumOrEmpty(t, tools[n])))
	}
	return strings.Join(lines, "\n")
}

func actionEnumOrEmpty(t *testing.T, tool *mcpsdk.Tool) []string {
	t.Helper()
	var schema struct {
		Properties struct {
			Action struct {
				Enum []string `json:"enum"`
			} `json:"action"`
		} `json:"properties"`
	}
	raw, err := json.Marshal(tool.InputSchema)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &schema))
	if schema.Properties.Action.Enum == nil {
		return []string{}
	}
	return schema.Properties.Action.Enum
}

// TestAgentFacingTextNamesOnlyRealTools is the guard for a failure that shipped:
// help output, tool descriptions and error hints all told agents to call
// `hookdeck_projects`, which stopped existing when the tool was split into
// `_read` and `_use`. Calling what the help advertised returned
// `unknown tool "hookdeck_projects"`.
//
// The names were produced by a helper rather than written literally, so nothing
// failed to compile and no test noticed. This reads every tool name mentioned
// in text an agent actually sees and requires it to be a tool that exists.
func TestAgentFacingTextNamesOnlyRealTools(t *testing.T) {
	// Write mode, because it advertises the superset. A name that only exists
	// with --allow-write is still a real name to mention.
	session := connectInMemoryWithMode(t, &hookdeck.Client{APIKey: "k"}, true)
	tools := listTools(t, session)

	real := make(map[string]bool, len(tools))
	for name := range tools {
		real[name] = true
	}
	// Read-only-mode names too: help text legitimately mentions write tools
	// while describing what --allow-write unlocks, and vice versa.
	for name := range listTools(t, connectInMemoryWithMode(t, &hookdeck.Client{APIKey: "k"}, false)) {
		real[name] = true
	}

	mention := regexp.MustCompile(`\b(?:gateway|hookdeck|outpost)_[a-z_]+\b`)

	check := func(t *testing.T, where, text string) {
		t.Helper()
		for _, name := range mention.FindAllString(text, -1) {
			assert.True(t, real[name],
				"%s names %q, which is not a tool this server advertises; "+
					"an agent following that text gets `unknown tool`", where, name)
		}
	}

	for name, tool := range tools {
		check(t, "the description of "+name, tool.Description)
		if tool.InputSchema != nil {
			schema, err := json.Marshal(tool.InputSchema)
			require.NoError(t, err)
			check(t, "the input schema of "+name, string(schema))
		}
	}

	// The help tool's own output is the text an agent is most likely to act on.
	check(t, "gateway_help overview", textContent(t, callTool(t, session, "gateway_help", map[string]any{})))
	for name := range tools {
		topic := textContent(t, callTool(t, session, "gateway_help", map[string]any{"topic": name}))
		check(t, "gateway_help topic="+name, topic)
	}
}
