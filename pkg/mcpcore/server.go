package mcpcore

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/version"
)

// ToolDef pairs a tool definition (with its JSON Schema) with the handler that
// serves it.
type ToolDef struct {
	Tool    *mcpsdk.Tool
	Handler mcpsdk.ToolHandler
}

// Options configure a product-specific MCP server built on this package.
type Options struct {
	// Name is the MCP server identity reported at initialize
	// (e.g. "hookdeck-gateway", "hookdeck-outpost").
	Name string

	// ToolPrefix namespaces every tool this server exposes (e.g. "hookdeck",
	// "outpost") so several Hookdeck MCP servers can be configured in one client
	// without colliding.
	ToolPrefix string

	// Client is the API client shared by every tool handler. Handlers mutate it
	// in place (e.g. ProjectID on a project switch), so each server must be given
	// the client for its own API.
	Client *hookdeck.Client

	// Config is the CLI configuration, used by the login tool to persist
	// credentials.
	Config *config.Config

	// WriteEnabled reports whether write actions are available in this session.
	WriteEnabled bool

	// ProjectFilter, when set, is the project type (see pkg/config) the projects
	// tool lists and allows switching to. Empty means no filtering.
	ProjectFilter string

	// ToolDefs supplies the tools to register. It receives the constructed
	// server so definitions can reach the client, write mode and tool names.
	ToolDefs func(*Server) []ToolDef
}

// Server wraps the MCP SDK server and the Hookdeck API client.
type Server struct {
	opts      Options
	client    *hookdeck.Client
	cfg       *config.Config
	mcpServer *mcpsdk.Server

	// sessionCtx is the context passed to RunStdio. It is cancelled when the
	// MCP transport closes (stdin EOF). Background goroutines (e.g. login
	// polling) should select on this — NOT on the per-request ctx passed to
	// tool handlers, which is cancelled when the handler returns.
	sessionCtx context.Context
}

// NewServer creates an MCP server from the given options and registers the
// tools returned by Options.ToolDefs.
//
// The client is shared across all tool handlers; changing its ProjectID (e.g.
// via the projects tool's use action) affects subsequent calls within the same
// session.
func NewServer(opts Options) *Server {
	s := &Server{opts: opts, client: opts.Client, cfg: opts.Config}

	s.mcpServer = mcpsdk.NewServer(
		&mcpsdk.Implementation{
			Name:    opts.Name,
			Version: version.Version,
		},
		nil, // default options; tools capability is inferred from AddTool calls
	)

	if opts.ToolDefs != nil {
		for _, td := range opts.ToolDefs(s) {
			s.mcpServer.AddTool(td.Tool, s.wrapWithTelemetry(td.Tool.Name, td.Handler))
		}
	}

	return s
}

// Client returns the API client shared by this server's tool handlers.
func (s *Server) Client() *hookdeck.Client { return s.client }

// Config returns the CLI configuration this server was built with.
func (s *Server) Config() *config.Config { return s.cfg }

// WriteEnabled reports whether write actions are available in this session.
func (s *Server) WriteEnabled() bool { return s.opts.WriteEnabled }

// ProjectFilter returns the project type this server serves, or "" when it
// serves any project type.
func (s *Server) ProjectFilter() string { return s.opts.ProjectFilter }

// ToolName returns the fully qualified name for a resource, e.g. "outpost_events".
func (s *Server) ToolName(resource string) string {
	if s.opts.ToolPrefix == "" {
		return resource
	}
	return s.opts.ToolPrefix + "_" + resource
}

// ToolPrefix returns the tool-name prefix including the separator, e.g. "outpost_".
func (s *Server) ToolPrefix() string {
	if s.opts.ToolPrefix == "" {
		return ""
	}
	return s.opts.ToolPrefix + "_"
}

// LoginToolName returns the name of this server's login tool.
func (s *Server) LoginToolName() string { return s.ToolName("login") }

// ProjectsToolName returns the name of this server's projects tool.
func (s *Server) ProjectsToolName() string { return s.ToolName("projects") }

// RequireAuth guards a handler on an unauthenticated session, naming this
// server's login tool.
func (s *Server) RequireAuth() *mcpsdk.CallToolResult {
	return RequireAuth(s.client, s.LoginToolName())
}

// mcpClientInfo extracts the MCP client name/version string from the
// session's initialize params. Returns "" if unavailable.
func mcpClientInfo(req *mcpsdk.CallToolRequest) string {
	if req.Session == nil {
		return ""
	}
	params := req.Session.InitializeParams()
	if params == nil || params.ClientInfo == nil {
		return ""
	}
	ci := params.ClientInfo
	if ci.Version != "" {
		return fmt.Sprintf("%s/%s", ci.Name, ci.Version)
	}
	return ci.Name
}

// wrapWithTelemetry returns a handler that sets per-invocation telemetry on the
// shared client before delegating to the original handler. The stdio transport
// processes tool calls sequentially, so setting telemetry on the shared client
// is safe (no concurrent access).
func (s *Server) wrapWithTelemetry(toolName string, handler mcpsdk.ToolHandler) mcpsdk.ToolHandler {
	return func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		// Extract the action from the request arguments for command_path.
		action := extractAction(req)
		commandPath := toolName
		if action != "" {
			commandPath = toolName + "/" + action
		}

		deviceName, _ := os.Hostname()

		s.client.Telemetry = &hookdeck.CLITelemetry{
			Source:       "mcp",
			Environment:  hookdeck.DetectEnvironment(),
			CommandPath:  commandPath,
			InvocationID: hookdeck.NewInvocationID(),
			DeviceName:   deviceName,
			MCPClient:    mcpClientInfo(req),
		}
		defer func() { s.client.Telemetry = nil }()

		FillProjectDisplayNameIfNeeded(s.client)

		return handler(ctx, req)
	}
}

// extractAction parses the "action" field from the tool call arguments.
func extractAction(req *mcpsdk.CallToolRequest) string {
	if req.Params.Arguments == nil {
		return ""
	}
	var args map[string]interface{}
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return ""
	}
	if action, ok := args["action"].(string); ok {
		return action
	}
	return ""
}

// RunStdio starts the MCP server on stdin/stdout and blocks until the
// connection is closed (i.e. stdin reaches EOF).
func (s *Server) RunStdio(ctx context.Context) error {
	return s.Run(ctx, &mcpsdk.StdioTransport{})
}

// Run starts the MCP server on the given transport. It stores ctx as the
// session-level context so background goroutines (e.g. login polling) can
// detect when the session ends.
func (s *Server) Run(ctx context.Context, transport mcpsdk.Transport) error {
	s.sessionCtx = ctx
	return s.mcpServer.Run(ctx, transport)
}
