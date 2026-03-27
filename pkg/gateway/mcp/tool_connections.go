package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

func handleConnections(client *hookdeck.Client) mcpsdk.ToolHandler {
	return func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		if r := requireAuth(client); r != nil {
			return r, nil
		}

		in, err := parseInput(req.Params.Arguments)
		if err != nil {
			return ErrorResult(err.Error()), nil
		}

		action := in.String("action")
		switch action {
		case "list", "":
			return connectionsList(ctx, client, in)
		case "get":
			return connectionsGet(ctx, client, in)
		case "pause":
			return connectionsPause(ctx, client, in)
		case "unpause":
			return connectionsUnpause(ctx, client, in)
		default:
			return ErrorResult(fmt.Sprintf("unknown action %q; expected list, get, pause, or unpause", action)), nil
		}
	}
}

func connectionsList(ctx context.Context, client *hookdeck.Client, in input) (*mcpsdk.CallToolResult, error) {
	params := make(map[string]string)
	setIfNonEmpty(params, "name", in.String("name"))
	setIfNonEmpty(params, "source_id", in.String("source_id"))
	setIfNonEmpty(params, "destination_id", in.String("destination_id"))
	setInt(params, "limit", in.Int("limit", 0))
	setIfNonEmpty(params, "next", in.String("next"))
	setIfNonEmpty(params, "prev", in.String("prev"))

	if bp := in.BoolPtr("disabled"); bp != nil {
		if *bp {
			params["disabled_at[any]"] = "true"
		}
	}

	result, err := client.ListConnections(ctx, params)
	if err != nil {
		return ErrorResult(TranslateAPIError(err)), nil
	}
	return JSONResult(result)
}

func connectionsGet(ctx context.Context, client *hookdeck.Client, in input) (*mcpsdk.CallToolResult, error) {
	idOrName := in.String("id")
	if idOrName == "" {
		return ErrorResult("id or name is required for the get action"), nil
	}
	id, err := resolveMCPConnectionID(ctx, client, idOrName)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}
	conn, err := client.GetConnection(ctx, id)
	if err != nil {
		return ErrorResult(TranslateAPIError(err)), nil
	}
	return JSONResult(conn)
}

func connectionsPause(ctx context.Context, client *hookdeck.Client, in input) (*mcpsdk.CallToolResult, error) {
	idOrName := in.String("id")
	if idOrName == "" {
		return ErrorResult("id or name is required for the pause action"), nil
	}
	id, err := resolveMCPConnectionID(ctx, client, idOrName)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}
	conn, err := client.PauseConnection(ctx, id)
	if err != nil {
		return ErrorResult(TranslateAPIError(err)), nil
	}
	return JSONResult(conn)
}

func connectionsUnpause(ctx context.Context, client *hookdeck.Client, in input) (*mcpsdk.CallToolResult, error) {
	idOrName := in.String("id")
	if idOrName == "" {
		return ErrorResult("id or name is required for the unpause action"), nil
	}
	id, err := resolveMCPConnectionID(ctx, client, idOrName)
	if err != nil {
		return ErrorResult(err.Error()), nil
	}
	conn, err := client.UnpauseConnection(ctx, id)
	if err != nil {
		return ErrorResult(TranslateAPIError(err)), nil
	}
	return JSONResult(conn)
}

// resolveMCPConnectionID resolves a connection ID or name to an ID.
// If the value looks like an ID (starts with conn_ or web_), it is returned as-is after
// verifying it exists; otherwise a name lookup is performed.
func resolveMCPConnectionID(ctx context.Context, client *hookdeck.Client, idOrName string) (string, error) {
	if strings.HasPrefix(idOrName, "conn_") || strings.HasPrefix(idOrName, "web_") {
		_, err := client.GetConnection(ctx, idOrName)
		if err == nil {
			return idOrName, nil
		}
		if !hookdeck.IsNotFoundError(err) {
			return "", errors.New(TranslateAPIError(err))
		}
	}

	params := map[string]string{"name": idOrName}
	result, err := client.ListConnections(ctx, params)
	if err != nil {
		return "", errors.New(TranslateAPIError(err))
	}
	if result.Pagination.Limit == 0 || len(result.Models) == 0 {
		return "", fmt.Errorf("connection not found: '%s'", idOrName)
	}
	if len(result.Models) > 1 {
		return "", fmt.Errorf("multiple connections found with name '%s', please use the connection ID instead", idOrName)
	}
	return result.Models[0].ID, nil
}
