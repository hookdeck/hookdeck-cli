package cmd

import (
	"context"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
	gatewaymcp "github.com/hookdeck/hookdeck-cli/pkg/gateway/mcp"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/mcpcore"
	outpostmcp "github.com/hookdeck/hookdeck-cli/pkg/outpost/mcp"
)

func TestArgvContainsMCP(t *testing.T) {
	tests := []struct {
		name string
		argv []string
		want bool
	}{
		{"minimal", []string{"hookdeck", "gateway", "mcp"}, true},
		{"with profile long", []string{"hookdeck", "--profile", "p1", "gateway", "mcp"}, true},
		{"profile equals", []string{"hookdeck", "--profile=p1", "gateway", "mcp"}, true},
		{"short p", []string{"hookdeck", "-p", "p1", "gateway", "mcp"}, true},
		{"double dash positional", []string{"hookdeck", "--", "gateway", "mcp"}, true},
		{"not mcp", []string{"hookdeck", "gateway", "connection", "list"}, false},
		{"wrong order", []string{"hookdeck", "mcp", "gateway"}, false},
		{"too short", []string{"hookdeck", "gateway"}, false},
		{"api key flag before", []string{"hookdeck", "--api-key", "k", "gateway", "mcp"}, true},
		// Boolean flags (--insecure, --version) are not in flagNeedsNextArg, so
		// globalPositionalArgs treats them as single-token flags and skips them.
		{"bool flag before gateway", []string{"hookdeck", "--insecure", "gateway", "mcp"}, true},
		{"bool flag between gateway and mcp", []string{"hookdeck", "gateway", "--insecure", "mcp"}, false},

		// Outpost has its own MCP server and needs the same stdout hygiene.
		{"outpost minimal", []string{"hookdeck", "outpost", "mcp"}, true},
		{"outpost with profile", []string{"hookdeck", "--profile", "p1", "outpost", "mcp"}, true},
		{"outpost with allow-write", []string{"hookdeck", "outpost", "mcp", "--allow-write"}, true},
		{"outpost not mcp", []string{"hookdeck", "outpost", "tenant", "list"}, false},

		{"unrelated group", []string{"hookdeck", "project", "mcp"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, argvContainsMCP(tt.argv))
		})
	}
}

func TestArgvMCPGroup(t *testing.T) {
	assert.Equal(t, "gateway", argvMCPGroup([]string{"hookdeck", "gateway", "mcp"}))
	assert.Equal(t, "outpost", argvMCPGroup([]string{"hookdeck", "outpost", "mcp"}))
	assert.Equal(t, "", argvMCPGroup([]string{"hookdeck", "listen", "3000"}))
}

// The pre-startup auth error tells an agent which tool to call, so the name has
// to be one the server it is about to start actually registers.
//
// Asserting the literal the function already returns proved nothing: it read
// "outpost_login" for the Outpost group while that server registers
// hookdeck_login, and the test passed throughout. Starting the real servers and
// listing their tools is what catches the drift.
func TestMCPLoginToolNameIsRegisteredByEveryServer(t *testing.T) {
	for _, tc := range []struct {
		group  string
		server *mcpcore.Server
	}{
		{"gateway", gatewaymcp.NewServer(gatewaymcp.ServerOptions{
			Client: &hookdeck.Client{}, Config: &config.Config{},
		})},
		{"outpost", outpostmcp.NewServer(outpostmcp.ServerOptions{
			Client: &hookdeck.Client{}, AccountClient: &hookdeck.Client{}, Config: &config.Config{},
		})},
	} {
		t.Run(tc.group, func(t *testing.T) {
			registered := registeredToolNames(t, tc.server)
			assert.Contains(t, registered, mcpLoginToolName(),
				"the auth error names a login tool the %s server does not register", tc.group)
		})
	}
}

// registeredToolNames starts a server over an in-memory transport and returns
// the tools it advertises.
func registeredToolNames(t *testing.T, srv *mcpcore.Server) []string {
	t.Helper()

	serverTransport, clientTransport := mcpsdk.NewInMemoryTransports()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = srv.Run(ctx, serverTransport) }()

	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test-client", Version: "0.0.1"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })

	result, err := session.ListTools(ctx, nil)
	require.NoError(t, err)
	names := make([]string, 0, len(result.Tools))
	for _, tool := range result.Tools {
		names = append(names, tool.Name)
	}
	return names
}
