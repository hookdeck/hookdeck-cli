package mcp

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/internal/toolprose"
	"github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/mcpcore"
)

// TestAgentFacingTextNamesOnlyRealTools mirrors the Event Gateway guard.
//
// Help output, tool descriptions and error hints told agents to call
// `hookdeck_projects`, `outpost_tenants` and `outpost_destinations` — all names
// that stopped existing when each tool was split into `_read` and `_write`.
// Nothing failed to compile, because the names were either built by a helper or
// written as prose, so this reads every tool name an agent is shown and
// requires it to be a tool that exists.
func TestAgentFacingTextNamesOnlyRealTools(t *testing.T) {
	api := mockAPI(t, nil)

	// Write mode with a publish key, because that advertises the superset.
	session := connect(t, ServerOptions{
		Client:        newTestClient(t, api.URL),
		Config:        &config.Config{},
		WriteEnabled:  true,
		PublishAPIKey: "pk",
	})
	tools := listTools(t, session)

	real := make(map[string]bool, len(tools))
	for name := range tools {
		real[name] = true
	}
	// Read-only names too: help legitimately mentions the write tools while
	// describing what --allow-write unlocks, and vice versa.
	readOnly := connect(t, ServerOptions{Client: newTestClient(t, api.URL), Config: &config.Config{}})
	for name := range listTools(t, readOnly) {
		real[name] = true
	}

	mention := regexp.MustCompile(`\b(?:gateway|hookdeck|outpost)_[a-z_]+\b`)

	check := func(where, text string) {
		for _, name := range mention.FindAllString(text, -1) {
			assert.True(t, real[name],
				"%s names %q, which is not a tool this server advertises; "+
					"an agent following that text gets `unknown tool`", where, name)
		}
	}

	for name, tool := range tools {
		check("the description of "+name, tool.Description)
		if tool.InputSchema != nil {
			schema, err := json.Marshal(tool.InputSchema)
			require.NoError(t, err)
			check("the input schema of "+name, string(schema))
		}
	}

	check("outpost_help overview", resultText(t, callTool(t, session, "outpost_help", map[string]any{})))
	for name := range tools {
		check("outpost_help topic="+name,
			resultText(t, callTool(t, session, "outpost_help", map[string]any{"topic": name})))
	}
}

// allSpecs is every ToolSpec this server can render, including the two platform
// tools and the publish tool that only exists with a publish key.
func allSpecs(t *testing.T) []mcpcore.ToolSpec {
	t.Helper()
	srv := NewServer(ServerOptions{Config: &config.Config{}, WriteEnabled: true, PublishAPIKey: "pk"})
	specs := append([]mcpcore.ToolSpec{}, srv.PlatformSpecs(projectsToolDesc)...)
	specs = append(specs, resourceSpecs()...)
	specs = append(specs, publishSpec("pk"))
	return specs
}

// TestToolPropertiesDoNotPromiseArrays is the sibling of
// TestAgentFacingTextNamesOnlyRealTools for schema shape rather than tool names.
//
// Thirteen properties said "Accepts an array of strings or a comma-separated
// string" over "type": "string", and mcpcore.checkArgumentTypes refuses an
// array for a scalar property — so outpost_events_read with
// topic: ["user.created", "user.updated"] was answered with "topic takes a
// single value, not an array". An agent that reads the description before the
// schema is led straight into that.
func TestToolPropertiesDoNotPromiseArrays(t *testing.T) {
	for _, spec := range allSpecs(t) {
		for _, group := range spec.Actions.Groups() {
			for _, problem := range toolprose.ArrayPromises(spec.VisibleProps(group)) {
				assert.Fail(t, "description offers a shape the validator refuses",
					"%s_%s: %s", spec.Resource, group, problem)
			}
		}
	}
}

// TestAgentFacingTextNamesOnlyRealActions is the action-name half of
// TestAgentFacingTextNamesOnlyRealTools above.
//
// Splitting each tool into _read and _write left the shared property
// descriptions naming the whole original action list, so outpost_tenants_write
// said its id "On list, filters by tenant ID(s)" and outpost_destinations_write
// said its type filters "On list" — on tools with no list action. Same failure
// as naming a tool that does not exist, one level down: the text describes a
// call the tool will refuse.
func TestAgentFacingTextNamesOnlyRealActions(t *testing.T) {
	for _, spec := range allSpecs(t) {
		vocabulary := spec.Actions.Names()
		for _, group := range spec.Actions.Groups() {
			offered := spec.Actions.InGroup(group).Names()
			where := spec.Resource + "_" + group

			check := func(what, text string) {
				for _, name := range toolprose.UnknownActionMentions(text, offered, vocabulary) {
					assert.Fail(t, "prose names an action the tool does not offer",
						"%s: %s names %q; this tool offers: %s",
						where, what, name, strings.Join(offered, ", "))
				}
			}

			check("the summary", spec.Summary)
			check("the notes", spec.Notes)
			for name, prop := range spec.VisibleProps(group) {
				check("the description of "+name, prop.Desc)
			}
		}
	}
}

// TestAgentFacingTextNamesOnlyRealToolActions is the cross-tool half: prose
// that points a caller at another tool's action has to name an action that
// tool has. gateway_event_read sent callers to "a request's events action on
// gateway_request_read", which offers get, raw_body and retry.
func TestAgentFacingTextNamesOnlyRealToolActions(t *testing.T) {
	srv := NewServer(ServerOptions{Config: &config.Config{}, WriteEnabled: true, PublishAPIKey: "pk"})
	actions := map[string][]string{}
	for _, spec := range allSpecs(t) {
		for _, group := range spec.Actions.Groups() {
			actions[spec.GroupToolName(srv, group)] = spec.Actions.InGroup(group).Names()
		}
	}

	for _, spec := range allSpecs(t) {
		for _, group := range spec.Actions.Groups() {
			where := spec.GroupToolName(srv, group)
			texts := map[string]string{"the summary": spec.Summary, "the notes": spec.Notes}
			for name, prop := range spec.VisibleProps(group) {
				texts["the description of "+name] = prop.Desc
			}
			for what, text := range texts {
				for _, problem := range toolprose.CrossToolActionMentions(text, actions) {
					assert.Fail(t, "prose points at an action another tool does not have",
						"%s: %s %s", where, what, problem)
				}
			}
		}
	}
}
