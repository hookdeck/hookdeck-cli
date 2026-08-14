package mcp

import (
	"fmt"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/mcpcore"
)

// Tool names. The Outpost server namespaces its tools with "outpost_" so it can
// be configured alongside the Event Gateway server without colliding.
const (
	toolPrefix      = "outpost"
	helpToolName    = toolPrefix + "_help"
	helpTopicPrefix = toolPrefix + "_"

	loginToolDesc    = "Authenticate the Hookdeck CLI or sign in again. Without arguments, returns a URL for browser login when not yet authenticated, or confirms if already signed in. Set reauth: true to clear the current session and start a new browser login (use when outpost_projects list fails and the stored key may be a single-project or dashboard API key)."
	projectsToolDesc = "Always call this first when the user references a specific project by name. List available Outpost projects to find the matching project ID, then use the `use` action to switch to it before calling any other tools. Every other tool is scoped to the active project — if the wrong project is active, all results will be wrong. Only Outpost projects are listed: this server has no access to Event Gateway projects. If list or use fails (especially 401/403), the error may suggest outpost_login with reauth: true. JSON successes use a standard data/meta envelope; see outpost_help."
)

// ServerOptions configure the Outpost MCP server.
type ServerOptions struct {
	// Client must be the Outpost API client. Tool handlers mutate it in place
	// (the projects and login tools set ProjectID), so passing the Event Gateway
	// client would leave every Outpost call pointed at the previous project.
	Client *hookdeck.Client

	// AccountClient is the Hookdeck API client. Listing projects and validating
	// credentials are account-level requests, which the Outpost host does not
	// serve, so they need a client for the main API.
	AccountClient *hookdeck.Client

	// Config is the CLI configuration, used by the login tool.
	Config *config.Config

	// WriteEnabled turns on the actions that change data or return a credential.
	WriteEnabled bool

	// PublishAPIKey is a Hookdeck Project API key. The publish tool is only
	// registered when one is available, because the publish API does not accept
	// the credentials stored by `hookdeck login`.
	PublishAPIKey string
}

// NewServer creates an MCP server exposing the Outpost tools.
func NewServer(opts ServerOptions) *mcpcore.Server {
	return mcpcore.NewServer(mcpcore.Options{
		Name:          "hookdeck-outpost",
		ToolPrefix:    toolPrefix,
		Client:        opts.Client,
		AccountClient: opts.AccountClient,
		Config:        opts.Config,
		WriteEnabled:  opts.WriteEnabled,
		ProjectFilter: config.ProjectTypeOutpost,
		ToolDefs: func(srv *mcpcore.Server) []mcpcore.ToolDef {
			return toolDefs(srv, opts)
		},
	})
}

// action is one action a tool supports.
//
// write marks an action that a read-only server must not offer. That covers
// anything that changes data, and also the reads that hand back a credential:
// a tenant token and a portal URL are both reusable access to a tenant's data,
// so treating them as reads would let a read-only session mint them at will.
//
// destructive drives the client-facing DestructiveHint annotation.
type action struct {
	name        string
	desc        string
	write       bool
	destructive bool
}

// enabled reports whether the action is available in this mode.
func (a action) enabled(writeEnabled bool) bool { return writeEnabled || !a.write }

// actionSet is a tool's action list.
type actionSet []action

// available returns the actions offered in this mode.
func (as actionSet) available(writeEnabled bool) actionSet {
	out := make(actionSet, 0, len(as))
	for _, a := range as {
		if a.enabled(writeEnabled) {
			out = append(out, a)
		}
	}
	return out
}

// names returns the action names, for the schema enum.
func (as actionSet) names() []string {
	out := make([]string, len(as))
	for i, a := range as {
		out[i] = a.name
	}
	return out
}

// summary renders "list — …, get — …" for a tool description.
func (as actionSet) summary() string {
	parts := make([]string, 0, len(as))
	for _, a := range as {
		if a.desc == "" {
			parts = append(parts, a.name)
			continue
		}
		parts = append(parts, fmt.Sprintf("%s (%s)", a.name, a.desc))
	}
	return strings.Join(parts, ", ")
}

// find returns the named action.
func (as actionSet) find(name string) (action, bool) {
	for _, a := range as {
		if a.name == name {
			return a, true
		}
	}
	return action{}, false
}

// hasWrite reports whether any action in the set is a write.
func (as actionSet) hasWrite() bool {
	for _, a := range as {
		if a.write {
			return true
		}
	}
	return false
}

// hasDestructive reports whether any action in the set is destructive.
func (as actionSet) hasDestructive() bool {
	for _, a := range as {
		if a.destructive {
			return true
		}
	}
	return false
}

// toolSpec describes one Outpost tool before write mode is applied.
type toolSpec struct {
	resource string    // e.g. "tenants" — the tool is named "outpost_<resource>"
	summary  string    // what the tool is for, without listing actions
	actions  actionSet // every action, including the write-only ones
	props    map[string]mcpcore.Prop
	required []string
	handler  func(*mcpcore.Server) mcpsdk.ToolHandler
}

// define builds the tool definition for the current write mode.
//
// The schema is the primary gate: in read-only mode the write actions are
// absent from the enum and from the description, so an agent is never told
// about an action it cannot use. Tools whose every action is a write are not
// registered at all rather than registered to always fail.
func (spec toolSpec) define(srv *mcpcore.Server) (mcpcore.ToolDef, bool) {
	available := spec.actions.available(srv.WriteEnabled())
	if len(available) == 0 {
		return mcpcore.ToolDef{}, false
	}

	props := make(map[string]mcpcore.Prop, len(spec.props)+1)
	for k, v := range spec.props {
		props[k] = v
	}
	props["action"] = mcpcore.Prop{
		Type: "string",
		Desc: "Action: " + available.summary(),
		Enum: available.names(),
	}

	description := spec.summary + " Actions: " + available.summary() + "."
	if spec.actions.hasWrite() && !srv.WriteEnabled() {
		description += " This server is running in read-only mode, so only the actions listed above are available; see outpost_help for how to enable the rest."
	}

	destructive := available.hasDestructive()
	return mcpcore.ToolDef{
		Tool: &mcpsdk.Tool{
			Name:        srv.ToolName(spec.resource),
			Description: description,
			InputSchema: mcpcore.Schema(props, append([]string{"action"}, spec.required...)...),
			Annotations: &mcpsdk.ToolAnnotations{
				ReadOnlyHint:    !available.hasWrite(),
				DestructiveHint: &destructive,
			},
		},
		Handler: spec.handler(srv),
	}, true
}

// dispatch validates and gates an action before a handler runs it.
//
// The schema already hides write actions in read-only mode; this is the second
// line of defence, for a client that calls one anyway.
func dispatch(srv *mcpcore.Server, actions actionSet, name string) (string, *mcpsdk.CallToolResult) {
	a, ok := actions.find(name)
	if !ok {
		available := actions.available(srv.WriteEnabled())
		return "", mcpcore.ErrorResult(fmt.Sprintf(
			"unknown action %q; expected one of: %s",
			name, strings.Join(available.names(), ", "),
		))
	}
	if a.write {
		if r := mcpcore.RequireWrite(srv.WriteEnabled(), name); r != nil {
			return "", r
		}
	}
	return a.name, nil
}

// toolDefs lists every tool the Outpost MCP server exposes.
func toolDefs(srv *mcpcore.Server, opts ServerOptions) []mcpcore.ToolDef {
	specs := []toolSpec{
		tenantsSpec,
		destinationsSpec,
		eventsSpec,
		attemptsSpec,
		topicsSpec,
		destinationTypesSpec,
		metricsSpec,
		configSpec,
		statusSpec,
	}

	defs := []mcpcore.ToolDef{srv.ProjectsToolDef(projectsToolDesc)}
	for _, spec := range specs {
		if def, ok := spec.define(srv); ok {
			defs = append(defs, def)
		}
	}

	// Publishing needs both write mode and a Project API key, so the tool is
	// only offered when it can actually work. outpost_help explains its absence.
	if srv.WriteEnabled() && opts.PublishAPIKey != "" {
		if def, ok := publishSpec(opts.PublishAPIKey).define(srv); ok {
			defs = append(defs, def)
		}
	}

	defs = append(defs,
		mcpcore.ToolDef{
			Tool: &mcpsdk.Tool{
				Name:        helpToolName,
				Description: "Get an overview of all available Outpost tools or detailed help for a specific tool. Use this when unsure which tool to use for a task, or to find out which actions this session is allowed to perform. The overview reports the current mode (read-only or write) and documents the common JSON response shape (data + meta).",
				InputSchema: mcpcore.Schema(map[string]mcpcore.Prop{
					"topic": {Type: "string", Desc: "Tool name for detailed help (e.g. outpost_events). Omit for overview."},
				}),
				Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: true},
			},
			Handler: handleHelp(srv, opts),
		},
		srv.LoginToolDef(loginToolDesc),
	)

	return defs
}

// Shared property descriptions.
const (
	descTimeAfter  = "Only records at or after this ISO 8601 datetime."
	descTimeBefore = "Only records at or before this ISO 8601 datetime."
	descListValue  = "Accepts an array of strings or a comma-separated string."
)
