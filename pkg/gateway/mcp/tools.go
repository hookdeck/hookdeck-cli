package mcp

import (
	"context"
	"fmt"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// toolDefs lists every tool the MCP server exposes. Each entry pairs a Tool
// definition with a low-level ToolHandler. Part 4 of the implementation plan
// will fill in the real handlers; for now each handler returns a "not yet
// implemented" error so the skeleton compiles and registers cleanly.
func toolDefs(client *hookdeck.Client) []struct {
	tool    *mcpsdk.Tool
	handler mcpsdk.ToolHandler
} {
	placeholder := func(name string) mcpsdk.ToolHandler {
		return func(_ context.Context, _ *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			return ErrorResult(fmt.Sprintf("tool %q is not yet implemented", name)), nil
		}
	}

	return []struct {
		tool    *mcpsdk.Tool
		handler mcpsdk.ToolHandler
	}{
		{
			tool: &mcpsdk.Tool{
				Name:        "hookdeck_projects",
				Description: "List available Hookdeck projects or switch the active project for this session.",
			},
			handler: placeholder("hookdeck_projects"),
		},
		{
			tool: &mcpsdk.Tool{
				Name:        "hookdeck_connections",
				Description: "Manage connections (webhook routes) that link sources to destinations.",
			},
			handler: placeholder("hookdeck_connections"),
		},
		{
			tool: &mcpsdk.Tool{
				Name:        "hookdeck_sources",
				Description: "Manage inbound webhook sources.",
			},
			handler: placeholder("hookdeck_sources"),
		},
		{
			tool: &mcpsdk.Tool{
				Name:        "hookdeck_destinations",
				Description: "Manage webhook delivery destinations.",
			},
			handler: placeholder("hookdeck_destinations"),
		},
		{
			tool: &mcpsdk.Tool{
				Name:        "hookdeck_transformations",
				Description: "Manage JavaScript transformations applied to webhook payloads.",
			},
			handler: placeholder("hookdeck_transformations"),
		},
		{
			tool: &mcpsdk.Tool{
				Name:        "hookdeck_requests",
				Description: "Query inbound webhook requests received by Hookdeck.",
			},
			handler: placeholder("hookdeck_requests"),
		},
		{
			tool: &mcpsdk.Tool{
				Name:        "hookdeck_events",
				Description: "Query events (processed webhook deliveries) and manage retries.",
			},
			handler: placeholder("hookdeck_events"),
		},
		{
			tool: &mcpsdk.Tool{
				Name:        "hookdeck_attempts",
				Description: "Query delivery attempts for webhook events.",
			},
			handler: placeholder("hookdeck_attempts"),
		},
		{
			tool: &mcpsdk.Tool{
				Name:        "hookdeck_issues",
				Description: "List, inspect, and manage Hookdeck issues (delivery failures, transformation errors, etc.).",
			},
			handler: placeholder("hookdeck_issues"),
		},
		{
			tool: &mcpsdk.Tool{
				Name:        "hookdeck_metrics",
				Description: "Query metrics for events, requests, attempts, and transformations.",
			},
			handler: placeholder("hookdeck_metrics"),
		},
		{
			tool: &mcpsdk.Tool{
				Name:        "hookdeck_help",
				Description: "Describe available tools and their actions.",
			},
			handler: placeholder("hookdeck_help"),
		},
	}
}
