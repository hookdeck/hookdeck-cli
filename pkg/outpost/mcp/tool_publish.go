package mcp

import (
	"context"
	"fmt"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/mcpcore"
)

var publishActions = mcpcore.ActionSet{
	{Name: "publish", Desc: "publish an event to a topic", Write: true, Destructive: true},
}

// publishSpec builds the publish tool for a given Project API key.
//
// Publishing needs a Hookdeck Project API key: the publish API does not accept
// the credentials stored by `hookdeck login`. The tool is therefore only
// registered when a key is available, rather than being offered and then
// failing on every call.
func publishSpec(apiKey string) mcpcore.ToolSpec {
	return mcpcore.ToolSpec{
		Resource: "publish",
		Summary:  "Publish an event to a topic, for delivery to a tenant's matching destinations. Publishing is asynchronous: a successful response means the event was accepted, not that it has been delivered — check outpost_attempts for that. This delivers real events to real destinations.",
		Actions:  publishActions,
		Required: []string{"tenant_id", "topic"},
		Props: map[string]mcpcore.Prop{
			"tenant_id":          {Type: "string", Desc: "Tenant to publish for (required)."},
			"topic":              {Type: "string", Desc: "Topic to publish on (required). Must be one of the project's topics — see outpost_topics."},
			"data":               {Type: "object", Desc: "Event payload as a JSON object."},
			"destination_id":     {Type: "string", Desc: "Deliver only to this destination instead of every matching one."},
			"event_id":           {Type: "string", Desc: "Event ID, for idempotent publishing. Republishing the same ID reports a duplicate instead of creating a second event."},
			"metadata":           {Type: "object", Desc: "Event metadata as a JSON object of string values."},
			"eligible_for_retry": {Type: "boolean", Desc: "Whether failed deliveries should be retried. Omit to use the project default."},
		},
		Handler: func(srv *mcpcore.Server) mcpsdk.ToolHandler {
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

		if _, blocked := mcpcore.Dispatch(srv, publishActions, in.String("action")); blocked != nil {
			return blocked, nil
		}

		tenantID, err := mcpcore.RequireString(in, "tenant_id", "publish")
		if err != nil {
			return mcpcore.ErrorResult(err.Error()), nil
		}
		topic, err := mcpcore.RequireString(in, "topic", "publish")
		if err != nil {
			return mcpcore.ErrorResult(err.Error()), nil
		}
		data, err := mcpcore.Object(in, "data")
		if err != nil {
			return mcpcore.ErrorResult(err.Error()), nil
		}
		metadata, err := mcpcore.StringMap(in, "metadata")
		if err != nil {
			return mcpcore.ErrorResult(err.Error()), nil
		}

		// Publishing follows the credential, not the active project, and the two
		// can disagree: the credential is fixed at startup while the active
		// project moves with hookdeck_projects use. When they disagree the event
		// is accepted, matches nothing, and leaves no trace — a success response
		// for something that never happened.
		//
		// Checking the tenant with the publish credential resolves to the same
		// project the event would go to, so it catches that and a mistyped or
		// unprovisioned tenant alike.
		exists, checkErr := client.TenantExistsForPublish(ctx, apiKey, tenantID)
		if checkErr == nil && !exists {
			return mcpcore.ErrorResult(fmt.Sprintf(
				"tenant %q does not exist in the project the publish credential belongs to, so this event "+
					"would be accepted, delivered nowhere, and leave no trace. Publishing follows the credential "+
					"rather than the active project (%s), and the two can differ. Check the tenant id, or restart "+
					"the server with a publish key for the project you are working in.",
				tenantID, client.ProjectID,
			)), nil
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

		// An event matching nothing is accepted, given an id, and then leaves no
		// trace: it is not delivered and does not appear in the events list. A
		// bare success response is indistinguishable from one that was delivered,
		// so say plainly that nothing will happen.
		payload := map[string]any{"result": result}
		if len(result.DestinationIDs) == 0 {
			payload["warning"] = "This event matched no destinations, so it will not be delivered and will not " +
				"appear in the events list. Check that the tenant exists and has a destination subscribed to this topic — " +
				"publishing for a tenant that does not exist is accepted rather than rejected."
		}

		return mcpcore.JSONResultEnvelopeForClient(payload, client)
	}
}
