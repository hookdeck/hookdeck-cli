package mcp

import (
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

	loginToolDesc    = "Authenticate the Hookdeck CLI or sign in again. Without arguments, returns a URL for browser login when not yet authenticated, or confirms if already signed in. Set reauth: true to clear the current session and start a new browser login (use when hookdeck_projects list fails and the stored key may be a single-project or dashboard API key)."
	projectsToolDesc = "Always call this first when the user references a specific project by name. List available Outpost projects to find the matching project ID, then use the `use` action to switch to it before calling any other tools. Every other tool is scoped to the active project — if the wrong project is active, all results will be wrong. Only Outpost projects are listed: this server has no access to Event Gateway projects. If list or use fails (especially 401/403), the error may suggest hookdeck_login with reauth: true. JSON successes use a standard data/meta envelope; see outpost_help."
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

// toolDefs lists every tool the Outpost MCP server exposes.
func toolDefs(srv *mcpcore.Server, opts ServerOptions) []mcpcore.ToolDef {
	specs := []mcpcore.ToolSpec{
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
		if def, ok := spec.Define(srv); ok {
			defs = append(defs, def)
		}
	}

	// Publishing needs both write mode and a Project API key, so the tool is
	// only offered when it can actually work. outpost_help explains its absence.
	if srv.WriteEnabled() && opts.PublishAPIKey != "" {
		if def, ok := publishSpec(opts.PublishAPIKey).Define(srv); ok {
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
