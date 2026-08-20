package mcp

import (
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/mcpcore"
)

// Tool names. The Event Gateway server namespaces its product tools with
// "gateway_" so it can be configured alongside the Outpost server without
// colliding, and so a tool name says which product it acts on.
//
// The platform tools (hookdeck_login, hookdeck_projects) deliberately do not
// take this prefix — see mcpcore.DefaultPlatformPrefix.
const (
	toolPrefix       = "gateway"
	helpToolName     = toolPrefix + "_help"
	helpTopicPrefix  = toolPrefix + "_"
	eventsToolName   = toolPrefix + "_events"
	eventToolName    = toolPrefix + "_event"
	requestsToolName = toolPrefix + "_requests"
	requestToolName  = toolPrefix + "_request"
	loginToolDesc    = "Authenticate the Hookdeck CLI or sign in again. Without arguments, returns a URL for browser login when not yet authenticated, or confirms if already signed in. Set reauth: true to clear the current session and start a new browser login (use when hookdeck_projects list fails and the stored key may be a single-project or dashboard API key)."
	projectsToolDesc = "Always call this first when the user references a specific project by name. List available projects to find the matching project ID, then use the `use` action to switch to it before calling any other tools. All queries (events, issues, connections, metrics, requests) are scoped to the active project — if the wrong project is active, all results will be wrong. Also use this when unsure which project is currently active. If list or use fails (especially 401/403), the error may suggest hookdeck_login with reauth: true. JSON successes use a standard data/meta envelope; see gateway_help (overview or any tool topic)."
)

// ServerOptions configure the Event Gateway MCP server.
type ServerOptions struct {
	// Client is the Hookdeck API client shared by every tool handler. Handlers
	// mutate it in place (the projects and login tools set ProjectID).
	Client *hookdeck.Client

	// Config is the CLI configuration, used by the login tool.
	Config *config.Config

	// WriteEnabled turns on the actions that create, change or delete data.
	WriteEnabled bool
}

// NewServer creates an MCP server exposing the Event Gateway tools.
//
// The supplied client is shared across all tool handlers; changing its
// ProjectID (e.g. via the projects tool's use action) affects subsequent calls
// within the same session.
//
// hookdeck_login is always registered: it signs in when unauthenticated, or
// with reauth: true clears stored credentials and starts a fresh browser login.
func NewServer(opts ServerOptions) *mcpcore.Server {
	return mcpcore.NewServer(mcpcore.Options{
		Name:         "hookdeck-gateway",
		ToolPrefix:   toolPrefix,
		Client:       opts.Client,
		Config:       opts.Config,
		WriteEnabled: opts.WriteEnabled,
		ToolDefs:     toolDefs,
	})
}

// resourceSpecs lists every product tool the Event Gateway server exposes.
// Registration order is the order tools are advertised in.
//
// Events and requests are split into a plural collection tool and a singular
// single-record tool. Their actions share no parameters beyond an id: list
// carries ~20 filters that no by-id action can use, so a single tool would
// show every one of them to a caller that only has an id. The pairs are
// registered next to each other so the naming distinction is visible where an
// agent reads the tool list.
func resourceSpecs() []mcpcore.ToolSpec {
	return []mcpcore.ToolSpec{
		connectionsSpec,
		sourcesSpec,
		destinationsSpec,
		transformationsSpec,
		requestsSpec,
		requestSpec,
		eventsSpec,
		eventSpec,
		attemptsSpec,
		issuesSpec,
		metricsSpec,
	}
}

// toolDefs builds the tool definitions for the current write mode. Each spec
// renders its own schema, so read-only sessions never advertise a write action.
func toolDefs(srv *mcpcore.Server) []mcpcore.ToolDef {
	defs := []mcpcore.ToolDef{srv.ProjectsToolDef(projectsToolDesc)}

	for _, spec := range resourceSpecs() {
		if def, ok := spec.Define(srv); ok {
			defs = append(defs, def)
		}
	}

	defs = append(defs,
		mcpcore.ToolDef{
			Tool: &mcpsdk.Tool{
				Name:        srv.HelpToolName(),
				Description: "Get an overview of all available Event Gateway tools or detailed help for a specific tool. Use this when unsure which tool to use for a task, or to find out which actions this session is allowed to perform. The overview reports the current mode (read-only or write) and documents the common JSON response shape (data + meta). Note: all tools operate on the active project — use `hookdeck_projects` to verify or switch project context before querying.",
				InputSchema: mcpcore.Schema(map[string]mcpcore.Prop{
					"topic": {Type: "string", Desc: "Tool name for detailed help (e.g. gateway_events). Omit for overview."},
				}),
				Annotations: &mcpsdk.ToolAnnotations{ReadOnlyHint: true},
			},
			Handler: handleHelp(srv),
		},
		srv.LoginToolDef(loginToolDesc),
	)

	return defs
}

const (
	descDateAfter  = "ISO 8601 datetime lower bound (list). Maps to API field[gte]; do not pass bracket keys in MCP args. Combinable with the matching *_before param."
	descDateBefore = "ISO 8601 datetime upper bound (list). Maps to API field[lte]; do not pass bracket keys in MCP args."
	descJSONFilter = "Hookdeck JSON filter (object or string). Same syntax as hookdeck listen --filter-body."
	descPathFilter = "Partial URL path match (string)."

	// One filter that searches body, headers, parsed_query and path together,
	// for when the caller knows the value but not which field carries it.
	descSearchTerm = "Partial match against the body, headers, parsed_query or path at once (minimum 3 characters). " +
		"Use when you know the value but not which field holds it; use body/headers/parsed_query for a structured match."

	// The API schema is nullable and says null matches events with no delivery
	// group. A query string cannot carry a JSON null, and the string "null" is
	// read as a group name — verified against the live API, it returns nothing
	// in a project whose events all have no delivery group. Saying so here stops
	// an agent burning calls on a query that cannot work.
	descDeliveryGroup = "Filter by delivery group; comma-separate several. " +
		"The API documents null as matching events without a delivery group, but that null cannot be expressed " +
		"in a query string — passing \"null\" filters for a group of that name, so there is no way to search for ungrouped events."

	// Count filters accept the same operator objects as the date filters
	// (attempts is the existing precedent). Modelling that in the schema would
	// need a second shape per parameter for no gain, so the value is passed
	// through as written.
	descCountFilter = "Integer or API operator syntax; pass through as string."
)
