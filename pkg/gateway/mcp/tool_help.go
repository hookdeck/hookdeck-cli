package mcp

import (
	"context"
	"fmt"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/mcpcore"
)

func handleHelp(srv *mcpcore.Server) mcpsdk.ToolHandler {
	client := srv.Client()
	return func(_ context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		in, err := mcpcore.ParseInput(req.Params.Arguments)
		if err != nil {
			return mcpcore.ErrorResult(err.Error()), nil
		}

		topic := in.String("topic")
		if topic == "" {
			return helpOverview(srv, client), nil
		}
		return mcpcore.HelpTopic(helpTopicPrefix, toolHelp(srv), topic, mcpJSONSuccessResponseHelp), nil
	}
}

// formatCurrentProject builds a display label from org + short name (or a legacy
// combined ProjectName), and appends the project id in parentheses when set.
func formatCurrentProject(client *hookdeck.Client) string {
	if client.ProjectID == "" && client.ProjectName == "" && client.ProjectOrg == "" {
		return "not set"
	}
	var label string
	switch {
	case client.ProjectOrg != "" && client.ProjectName != "":
		label = client.ProjectOrg + " / " + client.ProjectName
	case client.ProjectName != "":
		label = client.ProjectName
	case client.ProjectOrg != "":
		label = client.ProjectOrg
	}
	if client.ProjectID != "" {
		if label != "" {
			return fmt.Sprintf("%s (%s)", label, client.ProjectID)
		}
		return client.ProjectID
	}
	return label
}

// mcpJSONSuccessResponseHelp documents the envelope returned by every resource tool.
// Keep in sync with JSONResultEnvelope in mcpcore/response.go.
const mcpJSONSuccessResponseHelp = `Common JSON response shape (all resource tools)
Successful tool calls that return JSON share one envelope. Parse the tool result body as JSON:

  • "data" — Domain payload for this tool and action (same shapes as Hookdeck list/get APIs,
    or { "raw_body": "..." } for raw_body actions, or { "projects": [...] } for hookdeck_projects list).
  • "meta" — Cross-cutting fields. When a Hookdeck project is in scope: "active_project_id" (string)
    and "active_project_name" (string, short name without org) are always present; name may be "" if
    unresolved. "active_project_org" (string) is included when known; omitted when empty.
    If no project id is set, "meta" is {}.

Plain text (not this shape): gateway_help text, hookdeck_login prompts, and error messages.
Errors use the host error flag; bodies are plain text, not JSON envelopes.`

// modeHelp explains what this session may do and, in read-only mode, how to
// change that.
func modeHelp(srv *mcpcore.Server) string {
	if srv.WriteEnabled() {
		return `Mode: write enabled. The ` + "`_write`" + ` tools are registered alongside the
` + "`_read`" + ` ones, so every action listed above is available, including the ones that create,
change and delete data. Destructive actions (delete, cancel, mute, dismiss) are real and
immediate.`
	}

	return `Mode: read-only. The tools that create, change or delete data are unavailable in
read-only mode: each resource has a ` + "`_write`" + ` tool, and none of them is registered. The
` + "`_read`" + ` tools above are the whole surface, and they are the same tools, with the same
schemas, in either mode.

Pausing and unpausing a connection are the exception. They change delivery but stay available,
on their own tool, because stopping a misbehaving connection is the natural end of an
investigation, and both are reversible and drop nothing.

To register the write tools, restart the server with --allow-write, or set
HOOKDECK_MCP_ALLOW_WRITE=true (the flag wins).`
}

// toolSummaryLines renders one line per registered tool, listing only the
// actions this session can perform.
func toolSummaryLines(srv *mcpcore.Server) []string {
	type entry struct {
		name    string
		summary string
	}

	entries := []entry{
		{srv.ProjectsToolName(), "List or switch the active project (actions: list, use)"},
		{srv.LoginToolName(), "Sign in, or reauth: true for a fresh browser session when listing projects fails"},
	}

	for _, spec := range append(srv.PlatformSpecs(projectsToolDesc), resourceSpecs()...) {
		for _, group := range spec.Actions.Groups() {
			actions := spec.Actions.InGroup(group)
			if len(actions) == 0 {
				continue
			}
			// The gated tool is not registered in read-only mode, so a topic or
			// an overview line for it would document something tools/list does
			// not offer.
			if group == mcpcore.GroupWrite && !srv.WriteEnabled() {
				continue
			}
			entries = append(entries, entry{
				name:    spec.GroupToolName(srv, group),
				summary: "Actions: " + strings.Join(actions.Names(), ", "),
			})
		}
	}
	entries = append(entries, entry{srv.HelpToolName(), "This help text"})

	width := 0
	for _, e := range entries {
		if len(e.name) > width {
			width = len(e.name)
		}
	}

	lines := make([]string, len(entries))
	for i, e := range entries {
		lines[i] = fmt.Sprintf("%-*s — %s", width, e.name, e.summary)
	}
	return lines
}

func helpOverview(srv *mcpcore.Server, client *hookdeck.Client) *mcpsdk.CallToolResult {
	var tools strings.Builder
	for _, line := range toolSummaryLines(srv) {
		tools.WriteString(line)
		tools.WriteString("\n")
	}

	text := fmt.Sprintf(`Hookdeck Event Gateway MCP Server — Available Tools

Current project: %s

%s

%s

All tools operate on the active project. Call %s first when the user references a project by
name, or when unsure which project is active.

%s
Use %s with topic="<tool_name>" for detailed help on a specific tool; each topic
repeats the common JSON response shape above for convenience.`,
		formatCurrentProject(client),
		modeHelp(srv),
		mcpJSONSuccessResponseHelp,
		srv.ProjectsToolName(),
		tools.String(),
		srv.HelpToolName(),
	)

	return mcpcore.TextResult(text)
}

// toolHelp builds the per-tool help topics for the current mode, so a topic
// never documents an action this session cannot perform.
func toolHelp(srv *mcpcore.Server) map[string]string {
	topics := map[string]string{

		srv.LoginToolName(): `hookdeck_login — Browser sign-in for the Hookdeck CLI inside MCP

Without arguments when already authenticated: confirms the session is active.
When not authenticated: returns a URL the user opens in a browser; poll by calling this tool again.

Parameters:
  reauth  (boolean) — If true, clears stored credentials and starts a new browser login. Use when
                      hookdeck_projects list fails and the key may be a single-project or dashboard
                      API key that cannot list teams.`,

		srv.HelpToolName(): fmt.Sprintf(`%s — Overview of the Event Gateway tools, or detailed help for one

The overview reports the current mode (read-only or write) and which actions are registered.

Note: all tools operate on the active project — use %s to verify or switch project
context before querying.

Parameters:
  topic  (string) — Tool name for detailed help (e.g. "%s"). Omit for the overview.`,
			srv.HelpToolName(), srv.ProjectsToolName(), srv.ToolName("events")),
	}

	for _, spec := range append(srv.PlatformSpecs(projectsToolDesc), resourceSpecs()...) {
		for _, group := range spec.Actions.Groups() {
			actions := spec.Actions.InGroup(group)
			if len(actions) == 0 {
				continue
			}
			// The gated tool is not registered in read-only mode, so a topic or
			// an overview line for it would document something tools/list does
			// not offer.
			if group == mcpcore.GroupWrite && !srv.WriteEnabled() {
				continue
			}
			topics[spec.GroupToolName(srv, group)] = spec.Help(srv, group, actions)
		}
	}

	return topics
}
