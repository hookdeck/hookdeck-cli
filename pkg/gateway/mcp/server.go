package mcp

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

// Server wraps the MCP SDK server and the Hookdeck API client.
type Server struct {
	client    *hookdeck.Client
	cfg       *config.Config
	mcpServer *mcpsdk.Server

	// sessionCtx is the context passed to RunStdio. It is cancelled when the
	// MCP transport closes (stdin EOF). Background goroutines (e.g. login
	// polling) should select on this — NOT on the per-request ctx passed to
	// tool handlers, which is cancelled when the handler returns.
	sessionCtx context.Context
}

// NewServer creates an MCP server with all Hookdeck tools registered.
// The supplied client is shared across all tool handlers; changing its
// ProjectID (e.g. via the projects.use action) affects subsequent calls
// within the same session.
//
// hookdeck_login is always registered: it signs in when unauthenticated, or
// with reauth: true clears stored credentials and starts a fresh browser login.
func NewServer(client *hookdeck.Client, cfg *config.Config) *Server {
	s := &Server{client: client, cfg: cfg}

	s.mcpServer = mcpsdk.NewServer(
		&mcpsdk.Implementation{
			Name:    "hookdeck-gateway",
			Version: version.Version,
		},
		nil, // default options; tools capability is inferred from AddTool calls
	)

	s.registerTools()
	return s
}

// registerTools adds all tool definitions to the MCP server.
func (s *Server) registerTools() {
	for _, td := range toolDefs(s.client) {
		s.mcpServer.AddTool(td.tool, s.wrapWithTelemetry(td.tool.Name, td.handler))
	}

	s.mcpServer.AddTool(
		&mcpsdk.Tool{
			Name:        "hookdeck_login",
			Description: "Authenticate the Hookdeck CLI or sign in again. Without arguments, returns a URL for browser login when not yet authenticated, or confirms if already signed in. Set reauth: true to clear the current session and start a new browser login (use when hookdeck_projects list fails and the stored key may be a single-project or dashboard API key).",
			InputSchema: schema(map[string]prop{
				"reauth": {Type: "boolean", Desc: "If true, clear stored credentials and start a new browser login. Use when project listing fails — complete login in the browser, then retry hookdeck_projects."},
			}),
		},
		s.wrapWithTelemetry("hookdeck_login", handleLogin(s)),
	)
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

		fillProjectDisplayNameIfNeeded(s.client)

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
