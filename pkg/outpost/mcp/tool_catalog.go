package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/mcpcore"
)

// Topics and destination types are both read-only catalogues describing what a
// destination may be created with, which is why they live together here.

var topicsActions = actionSet{
	{name: "list", desc: "list the topics configured for this project"},
}

var topicsSpec = toolSpec{
	resource: "topics",
	summary:  "List the topics destinations can subscribe to and events can be published on. Topics are project configuration rather than a resource, so they are changed with outpost_config, not created here.",
	actions:  topicsActions,
	handler:  handleTopics,
}

func handleTopics(srv *mcpcore.Server) mcpsdk.ToolHandler {
	client := srv.Client()
	return func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		if r := srv.RequireAuth(); r != nil {
			return r, nil
		}
		in, err := mcpcore.ParseInput(req.Params.Arguments)
		if err != nil {
			return mcpcore.ErrorResult(err.Error()), nil
		}
		if _, blocked := dispatch(srv, topicsActions, in.String("action")); blocked != nil {
			return blocked, nil
		}

		topics, err := client.ListOutpostTopics(ctx)
		if err != nil {
			return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
		}
		return mcpcore.JSONResultEnvelopeForClient(map[string]any{"topics": topics}, client)
	}
}

var destinationTypesActions = actionSet{
	{name: "list", desc: "list the available destination types"},
	{name: "get", desc: "get one type's full field schema"},
}

var destinationTypesSpec = toolSpec{
	resource: "destination_types",
	summary:  "Describe the destination types available in this project and the config and credential fields each one accepts. Call this before outpost_destinations create or update so the payload matches the type's schema.",
	actions:  destinationTypesActions,
	props: map[string]mcpcore.Prop{
		"type":               {Type: "string", Desc: "Destination type, e.g. webhook (required for get)."},
		"include_setup_docs": {Type: "boolean", Desc: "Include the provider setup instructions and icon. These are long and meant for rendering a setup UI, so they are omitted by default."},
	},
	handler: handleDestinationTypes,
}

func handleDestinationTypes(srv *mcpcore.Server) mcpsdk.ToolHandler {
	client := srv.Client()
	return func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		if r := srv.RequireAuth(); r != nil {
			return r, nil
		}
		in, err := mcpcore.ParseInput(req.Params.Arguments)
		if err != nil {
			return mcpcore.ErrorResult(err.Error()), nil
		}

		action, blocked := dispatch(srv, destinationTypesActions, in.String("action"))
		if blocked != nil {
			return blocked, nil
		}
		verbose := in.Bool("include_setup_docs")

		if action == "get" {
			destinationType, err := requireString(in, "type", "get")
			if err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			schema, err := client.GetOutpostDestinationType(ctx, destinationType)
			if err != nil {
				return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
			}
			return mcpcore.JSONResultEnvelopeForClient(trimSetupDocs(*schema, verbose), client)
		}

		schemas, err := client.ListOutpostDestinationTypes(ctx)
		if err != nil {
			return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
		}
		trimmed := make([]hookdeck.OutpostDestinationTypeSchema, len(schemas))
		for i, schema := range schemas {
			trimmed[i] = trimSetupDocs(schema, verbose)
		}
		return mcpcore.JSONResultEnvelopeForClient(trimmed, client)
	}
}

// trimSetupDocs drops the icon and setup instructions unless they were asked
// for. Both are sized for a setup UI and would otherwise dominate the response.
func trimSetupDocs(schema hookdeck.OutpostDestinationTypeSchema, verbose bool) hookdeck.OutpostDestinationTypeSchema {
	if verbose {
		return schema
	}
	schema.Icon = ""
	schema.Instructions = ""
	return schema
}

var statusActions = actionSet{
	{name: "get", desc: "report the deployment status for this project"},
}

var statusSpec = toolSpec{
	resource: "status",
	summary:  "Report the state of this project's Outpost deployment, including the portal hostname. Configuration changes take a short while to reach the deployment, so check here after outpost_config set.",
	actions:  statusActions,
	handler:  handleStatus,
}

func handleStatus(srv *mcpcore.Server) mcpsdk.ToolHandler {
	client := srv.Client()
	return func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		if r := srv.RequireAuth(); r != nil {
			return r, nil
		}
		in, err := mcpcore.ParseInput(req.Params.Arguments)
		if err != nil {
			return mcpcore.ErrorResult(err.Error()), nil
		}
		if _, blocked := dispatch(srv, statusActions, in.String("action")); blocked != nil {
			return blocked, nil
		}

		status, err := client.GetOutpostStatus(ctx)
		if err != nil {
			return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
		}
		return mcpcore.JSONResultEnvelopeForClient(status, client)
	}
}
