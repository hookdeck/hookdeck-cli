package mcp

import (
	"context"
	"encoding/json"

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
}

// NewServer creates an MCP server with all Hookdeck tools registered.
// The supplied client is shared across all tool handlers; changing its
// ProjectID (e.g. via the projects.use action) affects subsequent calls
// within the same session.
//
// When the client has no API key (unauthenticated), the server additionally
// registers a hookdeck_login tool that initiates browser-based device auth.
// Resource tool handlers will return an auth error until login completes.
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
// If the client is not yet authenticated, the hookdeck_login tool is also
// registered so that AI agents can initiate authentication in-band.
func (s *Server) registerTools() {
	for _, td := range toolDefs(s.client) {
		s.mcpServer.AddTool(td.tool, td.handler)
	}

	if s.client.APIKey == "" {
		s.mcpServer.AddTool(
			&mcpsdk.Tool{
				Name:        "hookdeck_login",
				Description: "Authenticate the Hookdeck CLI. Returns a URL that the user must open in their browser to complete login. The tool will wait for the user to complete authentication before returning.",
				InputSchema: json.RawMessage(`{"type":"object","properties":{},"additionalProperties":false}`),
			},
			handleLogin(s.client, s.cfg, s.mcpServer),
		)
	}
}

// RunStdio starts the MCP server on stdin/stdout and blocks until the
// connection is closed (i.e. stdin reaches EOF).
func (s *Server) RunStdio(ctx context.Context) error {
	return s.mcpServer.Run(ctx, &mcpsdk.StdioTransport{})
}
