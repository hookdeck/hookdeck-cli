package mcp

import (
	"context"
	"fmt"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/mcpcore"
)

// gateway_event is the single-record half of the events pair: every action here
// addresses one event by id and needs nothing else. Searching for events lives
// on gateway_events — see tool_events.go.
var eventActions = mcpcore.ActionSet{
	{Name: "get", Desc: "get this event's metadata and headers"},
	{Name: "raw_body", Desc: "get this event's payload"},
	{Name: "retry", Desc: "queue another delivery attempt for this event", Write: true},
	{Name: "cancel", Desc: "stop this scheduled event from being delivered", Write: true, Destructive: true},
	{Name: "mute", Desc: "mute this failed event so it stops raising issues", Write: true, Destructive: true},
}

var eventSpec = mcpcore.ToolSpec{
	Resource: "event",
	Summary: "ONE event by ID — singular, single-record. Use this when you already have an event ID. Takes an id and nothing else. " +
		"To find events in the first place — by status, source, destination, date range or payload — use " + eventsToolName + " (plural), which takes the filters and returns IDs. This tool has no filters and cannot search. " +
		"Results are scoped to the active project — call the projects tool first if the user has specified a project.",
	Actions:  eventActions,
	Required: []string{"id"},
	Props: map[string]mcpcore.Prop{
		"id": {Type: "string", Desc: "Event ID (required, one event). Get one from " + eventsToolName + " list, or from a request's events action on " + requestToolName + "."},
	},
	Notes: `Plural vs singular — which of the two event tools to use:
  ` + eventToolName + `  (this tool, singular) — you already have an event ID and want to read or act on it.
  ` + eventsToolName + ` (plural)              — you have filters and want to find matching events.
  The usual flow is ` + eventsToolName + ` to find an ID, then ` + eventToolName + ` with that ID.

Getting the payload:
  get returns metadata and headers only. Use raw_body for the payload — there is no need to go via
  the request tools when you already have an event id.
  Example: {"action":"raw_body","id":"evt_abc"}

Acting on a failure (write mode):
  retry queues another delivery attempt and is the usual follow-up to investigating a failed event.
  cancel stops a scheduled event from ever being delivered; mute stops a failed event raising
  further issues without retrying it.

From an event to its request:
  An event carries request_id. Pass that to ` + requestToolName + ` with action get to see the raw
  inbound request. Events cannot be filtered by request_id — for the other direction, call
  ` + requestToolName + ` with action events.`,
	Handler: handleEvent,
}

func handleEvent(srv *mcpcore.Server) mcpsdk.ToolHandler {
	client := srv.Client()
	return func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		if r := srv.RequireAuth(); r != nil {
			return r, nil
		}

		in, err := mcpcore.ParseInput(req.Params.Arguments)
		if err != nil {
			return mcpcore.ErrorResult(err.Error()), nil
		}

		action, blocked := mcpcore.DispatchWithDefault(srv, eventActions, in.String("action"), "get",
			"To search for events by status, source, date range or payload, use "+eventsToolName+" (plural)")
		if blocked != nil {
			return blocked, nil
		}

		if wrong := wrongIDKind(in.String("id"), "evt_", eventToolName, map[string]string{
			"req_": requestToolName,
			"web_": "gateway_connections",
			"src_": "gateway_sources",
			"des_": "gateway_destinations",
		}); wrong != nil {
			return wrong, nil
		}

		switch action {
		case "get":
			return eventGet(ctx, client, in)
		case "raw_body":
			return eventRawBody(ctx, client, in)
		case "retry":
			return eventRetry(ctx, client, in)
		case "cancel":
			return eventCancel(ctx, client, in)
		default:
			return eventMute(ctx, client, in)
		}
	}
}

func eventGet(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	id := in.String("id")
	if id == "" {
		return mcpcore.ErrorResult("id is required for the get action"), nil
	}
	event, err := client.GetEvent(ctx, id, nil)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(event, client)
}

func eventRawBody(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	id := in.String("id")
	if id == "" {
		return mcpcore.ErrorResult("id is required for the raw_body action"), nil
	}
	body, err := client.GetEventRawBody(ctx, id)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	text := string(body)
	if len(body) > maxRawBodyBytes {
		text = string(body[:maxRawBodyBytes]) + "\n... [truncated]"
	}
	return mcpcore.JSONResultEnvelopeForClient(map[string]string{"raw_body": text}, client)
}

func eventRetry(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	return eventAction(ctx, client, in, "retry", client.RetryEvent)
}

func eventCancel(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	return eventAction(ctx, client, in, "cancel", client.CancelEvent)
}

func eventMute(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	return eventAction(ctx, client, in, "mute", client.MuteEvent)
}

// eventAction runs one of the by-id event mutations and returns the event the
// API answers with.
//
// It used to return a hardcoded status — "cancelled", "muted" — whenever the
// call did not error. The API answers 200 for a no-op, so cancelling an
// already-delivered event reported {"status":"cancelled"} while the event stayed
// SUCCESSFUL. The caller then told the user something that had not happened, and
// nothing in the response contradicted it.
//
// Returning the event means the agent can see the real status and say so. This
// is what the connection pause/unpause actions have always done.
func eventAction(
	ctx context.Context,
	client *hookdeck.Client,
	in mcpcore.Input,
	action string,
	call func(context.Context, string) (*hookdeck.Event, error),
) (*mcpsdk.CallToolResult, error) {
	id, err := mcpcore.RequireString(in, "id", action)
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	event, err := call(ctx, id)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(event, client)
}

// wrongIDKind catches an id belonging to a different resource before it becomes
// a bare "not found".
//
// Hookdeck ids carry their type as a prefix, and splitting events and requests
// into plural and singular tools means an agent routinely holds both kinds at
// once. Passing a req_ id to the event tool used to answer "Resource not
// found", so the agent reported that a request did not exist when it did.
func wrongIDKind(id, want, wantTool string, others map[string]string) *mcpsdk.CallToolResult {
	if id == "" || strings.HasPrefix(id, want) {
		return nil
	}
	for prefix, tool := range others {
		if strings.HasPrefix(id, prefix) {
			return mcpcore.ErrorResult(fmt.Sprintf(
				"%q is a %s id, not a %s id. Use %s for that, or pass an id beginning %q to %s.",
				id, strings.TrimSuffix(prefix, "_"), strings.TrimSuffix(want, "_"),
				tool, want, wantTool,
			))
		}
	}
	return nil
}
