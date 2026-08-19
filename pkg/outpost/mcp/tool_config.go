package mcp

import (
	"context"
	"fmt"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/mcpcore"
)

var configActions = mcpcore.ActionSet{
	{Name: "get", Desc: "show the project configuration"},
	{Name: "set", Desc: "change configuration values", Write: true, Destructive: true},
	{Name: "custom_domain_get", Desc: "show the tenant portal's custom domain"},
	{Name: "custom_domain_set", Desc: "configure a custom domain for the tenant portal", Write: true},
	{Name: "custom_domain_delete", Desc: "remove the custom domain", Write: true, Destructive: true},
}

var configSpec = mcpcore.ToolSpec{
	Resource: "config",
	Summary:  "Read and change this project's Outpost configuration. These settings apply to the whole project — every tenant and every destination — so a change here affects all delivery, and takes a short while to reach the deployment (check outpost_status). Some keys are managed for you and are rejected if set directly.",
	Actions:  configActions,
	Props: map[string]mcpcore.Prop{
		"key":      {Type: "string", Desc: "A single configuration key to read (get). Omit to read everything that is set."},
		"values":   {Type: "object", Desc: `Configuration values to set, as {"KEY": "value"} (set). Only the keys given are changed.`},
		"unset":    {Type: "array", Desc: "Configuration keys to return to their default (set).", Items: &mcpcore.Prop{Type: "string"}},
		"hostname": {Type: "string", Desc: "Hostname to serve the tenant portal from (required for custom_domain_set)."},
	},
	Handler: handleConfig,
}

func handleConfig(srv *mcpcore.Server) mcpsdk.ToolHandler {
	client := srv.Client()
	return func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		if r := srv.RequireAuth(); r != nil {
			return r, nil
		}
		in, err := mcpcore.ParseInput(req.Params.Arguments)
		if err != nil {
			return mcpcore.ErrorResult(err.Error()), nil
		}

		action, blocked := mcpcore.Dispatch(srv, configActions, in.String("action"))
		if blocked != nil {
			return blocked, nil
		}

		switch action {
		case "get":
			return configGet(ctx, client, in)
		case "set":
			return configSet(ctx, client, in)
		case "custom_domain_get":
			domain, err := client.GetOutpostCustomDomain(ctx)
			if err != nil {
				return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
			}
			return mcpcore.JSONResultEnvelopeForClient(domain, client)
		case "custom_domain_set":
			hostname, err := mcpcore.RequireString(in, "hostname", "custom_domain_set")
			if err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}
			domain, err := client.AddOutpostCustomDomain(ctx, hostname)
			if err != nil {
				return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
			}
			return mcpcore.JSONResultEnvelopeForClient(domain, client)
		default:
			if err := client.DeleteOutpostCustomDomain(ctx); err != nil {
				return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
			}
			return mcpcore.JSONResultEnvelopeForClient(map[string]string{"status": "deleted"}, client)
		}
	}
}

func configGet(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	cfg, err := client.GetOutpostConfig(ctx)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	if key := in.String("key"); key != "" {
		value, present := cfg[key]
		if !present {
			return mcpcore.ErrorResult(fmt.Sprintf("no configuration key named %q", key)), nil
		}
		return mcpcore.JSONResultEnvelopeForClient(map[string]*string{key: value}, client)
	}
	return mcpcore.JSONResultEnvelopeForClient(cfg, client)
}

func configSet(ctx context.Context, client *hookdeck.Client, in mcpcore.Input) (*mcpsdk.CallToolResult, error) {
	update := hookdeck.OutpostManagedConfig{}

	values, err := mcpcore.Object(in, "values")
	if err != nil {
		return mcpcore.ErrorResult(err.Error()), nil
	}
	for key, raw := range values {
		switch v := raw.(type) {
		case string:
			value := v
			update[key] = &value
		case nil:
			// A null clears the key back to its default, same as unset.
			update[key] = nil
		default:
			return mcpcore.ErrorResult(fmt.Sprintf("values.%s must be a string, or null to clear it", key)), nil
		}
	}

	for _, key := range mcpcore.StringList(in, "unset") {
		update[key] = nil
	}

	if len(update) == 0 {
		return mcpcore.ErrorResult("nothing to change: pass values, unset, or both"), nil
	}

	updated, err := client.UpdateOutpostConfig(ctx, update)
	if err != nil {
		return mcpcore.ErrorResult(mcpcore.TranslateAPIError(err)), nil
	}
	return mcpcore.JSONResultEnvelopeForClient(map[string]any{
		"config":  updated,
		"changed": len(update),
		"note":    "Changes take a short while to reach the deployment. Check outpost_status.",
	}, client)
}
