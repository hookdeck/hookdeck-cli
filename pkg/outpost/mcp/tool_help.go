package mcp

import (
	"context"
	"fmt"
	"sort"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/mcpcore"
)

func handleHelp(srv *mcpcore.Server, opts ServerOptions) mcpsdk.ToolHandler {
	client := srv.Client()
	return func(_ context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		in, err := mcpcore.ParseInput(req.Params.Arguments)
		if err != nil {
			return mcpcore.ErrorResult(err.Error()), nil
		}

		topic := in.String("topic")
		if topic == "" {
			return helpOverview(srv, opts, client), nil
		}
		return mcpcore.HelpTopic(helpTopicPrefix, toolHelp(srv), topic, jsonResponseShapeHelp), nil
	}
}

// jsonResponseShapeHelp documents the envelope every resource tool returns.
// Keep in sync with mcpcore.JSONResultEnvelope.
const jsonResponseShapeHelp = `Common JSON response shape (all resource tools)
Successful tool calls that return JSON share one envelope. Parse the tool result body as JSON:

  • "data" — Domain payload for this tool and action (the same shapes as the Outpost list/get APIs;
    list actions that are paginated return { "models": [...], "pagination": {...} }).
  • "meta" — Cross-cutting fields. When a project is in scope: "active_project_id" (string) and
    "active_project_name" (string, short name without org) are always present; name may be "" if
    unresolved. "active_project_org" (string) is included when known; omitted when empty.
    If no project id is set, "meta" is {}.

Plain text (not this shape): outpost_help text, outpost_login prompts, and error messages.
Errors use the host error flag; bodies are plain text, not JSON envelopes.`

// formatCurrentProject builds a display label from org + short name, and
// appends the project id in parentheses when set.
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

// modeHelp explains what this session may do and, in read-only mode, how to
// change that.
func modeHelp(srv *mcpcore.Server, opts ServerOptions) string {
	if srv.WriteEnabled() {
		text := `Mode: write enabled. Every action below is available, including the ones that create,
change or delete data. Destructive actions (delete, config set, publish) are real and immediate.`
		if opts.PublishAPIKey == "" {
			text += "\n\noutpost_publish is not registered in this session: publishing needs a Hookdeck Project API key,\n" +
				"which the credentials stored by 'hookdeck login' cannot substitute for. Restart the server with\n" +
				"--api-key <project-api-key>, or set HOOKDECK_API_KEY, to publish."
		}
		return text
	}

	return `Mode: read-only. Actions that change data are not offered, and the tools above list only
the actions this session can perform. Two reads are treated as writes and are also unavailable:
outpost_tenants token mints a tenant-scoped access token, and outpost_tenants portal returns a URL
granting access to a tenant's portal — both hand back reusable credentials, so a read-only session
must not be able to produce them. outpost_publish is not registered at all.

To enable everything, restart the server with --allow-write, or set HOOKDECK_MCP_ALLOW_WRITE=true
(the flag wins). Publishing additionally needs a Hookdeck Project API key via --api-key or
HOOKDECK_API_KEY.`
}

func helpOverview(srv *mcpcore.Server, opts ServerOptions, client *hookdeck.Client) *mcpsdk.CallToolResult {
	var tools strings.Builder
	for _, line := range toolSummaryLines(srv, opts) {
		tools.WriteString(line)
		tools.WriteString("\n")
	}

	text := fmt.Sprintf(`Hookdeck Outpost MCP Server — Available Tools

Current project: %s

%s

%s

All tools operate on the active project, which must be an Outpost project. Call outpost_projects
first when the user references a project by name, or when unsure which project is active.

%s
Use outpost_help with topic="<tool_name>" for detailed help on a specific tool.`,
		formatCurrentProject(client),
		modeHelp(srv, opts),
		jsonResponseShapeHelp,
		tools.String(),
	)

	return mcpcore.TextResult(text)
}

// toolSummaryLines renders one line per registered tool, listing only the
// actions this session can perform.
func toolSummaryLines(srv *mcpcore.Server, opts ServerOptions) []string {
	type entry struct {
		name    string
		summary string
	}

	entries := []entry{
		{srv.ProjectsToolName(), "List or switch the active Outpost project (actions: list, use)"},
		{srv.LoginToolName(), "Sign in, or reauth: true for a fresh browser session when listing projects fails"},
	}

	specs := []toolSpec{
		tenantsSpec, destinationsSpec, eventsSpec, attemptsSpec,
		topicsSpec, destinationTypesSpec, metricsSpec, configSpec, statusSpec,
	}
	for _, spec := range specs {
		available := spec.actions.available(srv.WriteEnabled())
		if len(available) == 0 {
			continue
		}
		entries = append(entries, entry{
			name:    srv.ToolName(spec.resource),
			summary: "Actions: " + strings.Join(available.names(), ", "),
		})
	}
	if srv.WriteEnabled() && opts.PublishAPIKey != "" {
		entries = append(entries, entry{srv.ToolName("publish"), "Publish an event (actions: publish)"})
	}
	entries = append(entries, entry{helpToolName, "This help text"})

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

// toolHelp builds the per-tool help topics for the current mode, so a topic
// never documents an action this session cannot perform.
func toolHelp(srv *mcpcore.Server) map[string]string {
	topics := map[string]string{
		srv.ProjectsToolName(): `outpost_projects — List or switch the active project

Always call this first when the user references a specific project by name. Every other tool is
scoped to the active project. Only Outpost projects are listed and only an Outpost project can be
switched to: this server talks to the Outpost API and has no access to Event Gateway projects.

Actions:
  list  — List the Outpost projects available to your credentials
  use   — Switch the active project for this session (in-memory only)

Parameters:
  action      (string, required) — "list" or "use"
  project_id  (string)           — Required for "use"`,

		srv.LoginToolName(): `outpost_login — Browser sign-in for the Hookdeck CLI inside MCP

Without arguments when already authenticated: confirms the session is active.
When not authenticated: returns a URL the user opens in a browser; poll by calling this tool again.

Note: signing in here does not supply a Project API key, which outpost_publish needs separately.

Parameters:
  reauth  (boolean) — If true, clears stored credentials and starts a new browser login. Use when
                      outpost_projects list fails and the key may be a single-project or dashboard
                      API key that cannot list projects.`,

		helpToolName: `outpost_help — Overview of the Outpost tools, or detailed help for one

The overview reports the current mode (read-only or write) and which tools are registered.

Parameters:
  topic  (string) — Tool name for detailed help (e.g. "outpost_events"). Omit for the overview.`,
	}

	specs := []toolSpec{
		tenantsSpec, destinationsSpec, eventsSpec, attemptsSpec,
		topicsSpec, destinationTypesSpec, metricsSpec, configSpec, statusSpec,
		publishSpec(""),
	}
	for _, spec := range specs {
		available := spec.actions.available(srv.WriteEnabled())
		if len(available) == 0 {
			continue
		}
		topics[srv.ToolName(spec.resource)] = specHelp(srv, spec, available)
	}

	return topics
}

// specHelp renders a tool's help from its definition, so help cannot drift from
// the schema the agent is actually given.
func specHelp(srv *mcpcore.Server, spec toolSpec, available actionSet) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n%s\n\nActions:\n", srv.ToolName(spec.resource), spec.summary)

	width := 0
	for _, a := range available {
		if len(a.name) > width {
			width = len(a.name)
		}
	}
	for _, a := range available {
		fmt.Fprintf(&b, "  %-*s — %s\n", width, a.name, a.desc)
	}

	if hidden := spec.actions.hasWrite() && !srv.WriteEnabled(); hidden {
		b.WriteString("\nFurther actions exist but are unavailable in read-only mode. See outpost_help for how to enable them.\n")
	}

	if len(spec.props) > 0 {
		b.WriteString("\nParameters:\n")
		names := make([]string, 0, len(spec.props))
		for name := range spec.props {
			names = append(names, name)
		}
		sort.Strings(names)

		width = 0
		for _, name := range names {
			if len(name) > width {
				width = len(name)
			}
		}
		for _, name := range names {
			prop := spec.props[name]
			required := ""
			for _, r := range spec.required {
				if r == name {
					required = ", required"
					break
				}
			}
			fmt.Fprintf(&b, "  %-*s (%s%s) — %s\n", width, name, prop.Type, required, prop.Desc)
		}
	}

	return strings.TrimRight(b.String(), "\n")
}
