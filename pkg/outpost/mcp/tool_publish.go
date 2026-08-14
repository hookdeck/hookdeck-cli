package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/mcpcore"
)

var publishActions = actionSet{
	{name: "publish", desc: "publish an event to a topic", write: true, destructive: true},
}

// publishSpec builds the publish tool for a given Project API key.
//
// Publishing needs a Hookdeck Project API key: the publish API does not accept
// the credentials stored by `hookdeck login`. The tool is therefore only
// registered when a key is available, rather than being offered and then
// failing on every call.
func publishSpec(apiKey string) toolSpec {
	return toolSpec{
		resource: "publish",
		summary:  "Publish an event to a topic, for delivery to a tenant's matching destinations. Publishing is asynchronous: a successful response means the event was accepted, not that it has been delivered — check outpost_attempts for that. This delivers real events to real destinations.",
		actions:  publishActions,
		required: []string{"tenant_id", "topic"},
		props: map[string]mcpcore.Prop{
			"tenant_id":          {Type: "string", Desc: "Tenant to publish for (required)."},
			"topic":              {Type: "string", Desc: "Topic to publish on (required). Must be one of the project's topics — see outpost_topics."},
			"data":               {Type: "object", Desc: "Event payload as a JSON object."},
			"destination_id":     {Type: "string", Desc: "Deliver only to this destination instead of every matching one."},
			"event_id":           {Type: "string", Desc: "Event ID, for idempotent publishing. Republishing the same ID reports a duplicate instead of creating a second event."},
			"metadata":           {Type: "object", Desc: "Event metadata as a JSON object of string values."},
			"eligible_for_retry": {Type: "boolean", Desc: "Whether failed deliveries should be retried. Omit to use the project default."},
		},
		handler: func(srv *mcpcore.Server) mcpsdk.ToolHandler {
			return handlePublish(srv, apiKey)
		},
	}
}

func handlePublish(srv *mcpcore.Server, apiKey string) mcpsdk.ToolHandler {
	client := srv.Client()
	return func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		if r := srv.RequireAuth(); r != nil {
			return r, nil
		}
		in, err := mcpcore.ParseInput(req.Params.Arguments)
		if err != nil {
			return mcpcore.ErrorResult(err.Error()), nil
		}

		if _, blocked := dispatch(srv, publishActions, in.String("action")); blocked != nil {
			return blocked, nil
		}

		tenantID, err := requireString(in, "tenant_id", "publish")
		if err != nil {
			return mcpcore.ErrorResult(err.Error()), nil
		}
		topic, err := requireString(in, "topic", "publish")
		if err != nil {
			return mcpcore.ErrorResult(err.Error()), nil
		}
		data, err := object(in, "data")
		if err != nil {
			return mcpcore.ErrorResult(err.Error()), nil
		}
		metadata, err := stringMap(in, "metadata")
		if err != nil {
			return mcpcore.ErrorResult(err.Error()), nil
		}

		result, err := client.PublishOutpostEvent(ctx, apiKey, &hookdeck.OutpostPublishRequest{
			ID:               in.String("event_id"),
			TenantID:         tenantID,
			Topic:            topic,
			DestinationID:    in.String("destination_id"),
			EligibleForRetry: in.BoolPtr("eligible_for_retry"),
			Metadata:         metadata,
			Data:             data,
		})
		if err != nil {
			return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
		}
		return mcpcore.JSONResultEnvelopeForClient(result, client)
	}
}
