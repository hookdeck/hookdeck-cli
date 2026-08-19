package mcp

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/mcpcore"
)

var transformationsActions = mcpcore.ActionSet{
	{Name: "list", Desc: "list transformations"},
	{Name: "get", Desc: "get one transformation, including its code"},
	{Name: "create", Desc: "create a transformation", Write: true},
	{Name: "upsert", Desc: "create a transformation or update the existing one with the same name", Write: true},
	{Name: "update", Desc: "update a transformation's code or environment", Write: true},
	{Name: "delete", Desc: "delete a transformation", Write: true, Destructive: true},

	// run is marked Write even though it stores nothing.
	//
	// It executes caller-supplied JavaScript on Hookdeck's transformation
	// runtime. Treating "changes no records" as "is a read" would let a
	// read-only session run arbitrary code, which is not what a user asking for
	// read-only is asking for. The gate is about what the session can cause to
	// happen, not only about what it persists.
	{Name: "run", Desc: "execute transformation code against a sample request and return the result", Write: true},
}

var transformationsSpec = mcpcore.ToolSpec{
	Resource: "transformations",
	Summary:  "Inspect and manage JavaScript transformations applied to event payloads, and try code out against a sample request before saving it.",
	Actions:  transformationsActions,
	Props: map[string]mcpcore.Prop{
		"id":            {Type: "string", Desc: "Transformation ID. Required for get/update/delete; optional on run to execute a stored transformation."},
		"name":          {Type: "string", Desc: "Transformation name. Filters on list; required on create/upsert."},
		"code":          {Type: "string", Desc: "JavaScript source (create/upsert/update, or run to execute unsaved code)"},
		"env":           {Type: "object", Desc: "Environment variables as a JSON object of string values (create/upsert/update/run)"},
		"connection_id": {Type: "string", Desc: "Connection to run against (run, maps to webhook_id)"},
		"request":       {Type: "object", Desc: "Sample request for run: { headers, body, path, query, parsed_query }. headers is required by the API and may be an empty object."},
		"limit":         {Type: "integer", Desc: "Max results (list)"},
		"next":          {Type: "string", Desc: "Next page cursor"},
		"prev":          {Type: "string", Desc: "Previous page cursor"},
	},
	Handler: handleTransformations,
}

func handleTransformations(srv *mcpcore.Server) mcpsdk.ToolHandler {
	client := srv.Client()
	return func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		if r := srv.RequireAuth(); r != nil {
			return r, nil
		}

		in, err := mcpcore.ParseInput(req.Params.Arguments)
		if err != nil {
			return mcpcore.ErrorResult(err.Error()), nil
		}

		action, blocked := mcpcore.DispatchWithDefault(srv, transformationsActions, in.String("action"), "list")
		if blocked != nil {
			return blocked, nil
		}

		switch action {
		case "list":
			return transformationsList(ctx, client, in)
		case "get":
			return transformationsGet(ctx, client, in)
		case "create", "upsert":
			return transformationsWrite(ctx, client, in, action)
		case "update":
			return transformationsUpdate(ctx, client, in)
		case "delete":
			return transformationsDelete(ctx, client, in)
		default:
			return transformationsRun(ctx, client, in)
		}
	}
}

// transformationsWrite handles create and upsert, which share a request body.
func transformationsWrite(ctx context.Context, client *hookdeck.Client, in mcpcore.Input, action string) (*mcpsdk.CallToolResult, error) {
	name, err := mcpcore.RequireString(in, "name", action)
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	code, err := mcpcore.RequireString(in, "code", action)
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	env, err := mcpcore.StringMap(in, "env")
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}

	call := client.CreateTransformation
	if action == "upsert" {
		call = client.UpsertTransformation
	}
	t, err := call(ctx, &hookdeck.TransformationCreateRequest{Name: name, Code: code, Env: env})
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(t, client)
}

func transformationsUpdate(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	id, err := mcpcore.RequireString(in, "id", "update")
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	env, err := mcpcore.StringMap(in, "env")
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	t, err := client.UpdateTransformation(ctx, id, &hookdeck.TransformationUpdateRequest{
		Name: in.String("name"),
		Code: in.String("code"),
		Env:  env,
	})
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(t, client)
}

func transformationsDelete(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	id, err := mcpcore.RequireString(in, "id", "delete")
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	if err := client.DeleteTransformation(ctx, id); err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(map[string]string{
		"transformation_id": id,
		"status":            "deleted",
	}, client)
}

func transformationsRun(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	code := in.String("code")
	id := in.String("id")
	if code == "" && id == "" {
		return mcpcore.ErrorResult("code or id is required for the run action: supply code to try unsaved source, or id to run a stored transformation"), nil
	}
	env, err := mcpcore.StringMap(in, "env")
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	request, err := transformationRunInput(in)
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}

	result, err := client.RunTransformation(ctx, &hookdeck.TransformationRunRequest{
		Code:             code,
		TransformationID: id,
		WebhookID:        in.String("connection_id"),
		Env:              env,
		Request:          request,
	})
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(result, client)
}

// transformationRunInput builds the sample request the run action executes
// against. The API requires a headers object, so an omitted request still
// produces one rather than being sent as null.
func transformationRunInput(in mcpcore.Input) (*hookdeck.TransformationRunRequestInput, error) {
	raw, err := mcpcore.Object(in, "request")
	if err != nil {
		return nil, err
	}
	out := &hookdeck.TransformationRunRequestInput{Headers: map[string]string{}}
	if raw == nil {
		return out, nil
	}

	nested := mcpcore.Input(raw)
	headers, err := mcpcore.StringMap(nested, "headers")
	if err != nil {
		return nil, err
	}
	if headers != nil {
		out.Headers = headers
	}
	parsedQuery, err := mcpcore.Object(nested, "parsed_query")
	if err != nil {
		return nil, err
	}
	out.Body = raw["body"]
	out.Path = nested.String("path")
	out.Query = nested.String("query")
	out.ParsedQuery = parsedQuery
	return out, nil
}

func transformationsList(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	params := make(map[string]string)
	mcpcore.SetIfNonEmpty(params, "name", in.String("name"))
	mcpcore.SetInt(params, "limit", in.Int("limit", 0))
	mcpcore.SetIfNonEmpty(params, "next", in.String("next"))
	mcpcore.SetIfNonEmpty(params, "prev", in.String("prev"))

	result, err := client.ListTransformations(ctx, params)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(result, client)
}

func transformationsGet(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	id := in.String("id")
	if id == "" {
		return mcpcore.ErrorResult("id is required for the get action"), nil
	}
	t, err := client.GetTransformation(ctx, id)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(t, client)
}
