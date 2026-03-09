package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/version"
)

// Server wraps the MCP SDK server and the Hookdeck API client.
type Server struct {
	client    *hookdeck.Client
	mcpServer *mcpsdk.Server
}

// NewServer creates an MCP server with all Hookdeck tools registered.
// The supplied client is shared across all tool handlers; changing its
// ProjectID (e.g. via the projects.use action) affects subsequent calls
// within the same session.
func NewServer(client *hookdeck.Client) *Server {
	s := &Server{client: client}

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
		s.mcpServer.AddTool(td.tool, td.handler)
	}
}

// RunStdio starts the MCP server on stdin/stdout and blocks until the
// connection is closed (i.e. stdin reaches EOF).
func (s *Server) RunStdio(ctx context.Context) error {
	return s.mcpServer.Run(ctx, &mcpsdk.StdioTransport{})
}
