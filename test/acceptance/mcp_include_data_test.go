//go:build mcp

package acceptance

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// firstModelData lists through an MCP tool and returns the first model's data
// object, or nil when the model has none.
func firstModelData(t *testing.T, cli *CLIRunner, tool string, args map[string]any) map[string]any {
	t.Helper()
	result := CallGatewayMCPTool(t, cli.projectRoot, cli.configPath, tool, args, 30*time.Second)
	require.False(t, result.IsError, "tool error: %s", result.Text)

	var envelope struct {
		Data struct {
			Models []map[string]any `json:"models"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(result.Text), &envelope), result.Text)
	require.NotEmpty(t, envelope.Data.Models, "expected at least one model from %s: %s", tool, result.Text)

	data, _ := envelope.Data.Models[0]["data"].(map[string]any)
	return data
}

// TestMCPListIncludeData is the check a mock cannot make: that the live API
// honours include=data on the routes the MCP list actions query, and that the
// payload it returns comes back out of the tool.
//
// The negative control matters as much as the positive one. The API ignores
// query parameters it does not declare, so a dropped include would return 200
// and a list with no payloads, and a test that only asked for data would never
// notice whether it was the parameter or the default that put it there.
func TestMCPListIncludeData(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)

	connID, _ := createConnectionAndTriggerEvent(t, cli)
	t.Cleanup(func() { deleteConnection(t, cli, connID) })

	var conn Connection
	require.NoError(t, cli.RunJSON(&conn, "gateway", "connection", "get", connID))
	sourceID := conn.Source.ID
	require.NotEmpty(t, sourceID)

	// triggerTestEvent posts {"test":true}.
	assertPayload := func(t *testing.T, data map[string]any) {
		t.Helper()
		require.NotNil(t, data, "include=data must put a data object on the model")
		assert.Equal(t, map[string]any{"test": true}, data["body"],
			"data.body should be the payload that was sent")
	}

	cases := []struct {
		name string
		tool string
		args map[string]any
	}{
		{"events", "gateway_events_read", map[string]any{"action": "list", "connection_id": connID, "limit": 1}},
		{"requests", "gateway_requests_read", map[string]any{"action": "list", "source_id": sourceID, "limit": 1}},
	}

	for _, tc := range cases {
		t.Run(tc.name+" without include returns no payload", func(t *testing.T) {
			assert.Nil(t, firstModelData(t, cli, tc.tool, tc.args),
				"payloads must be opt-in: without include the model should carry no data")
		})

		t.Run(tc.name+" with include=data returns the payload", func(t *testing.T) {
			args := map[string]any{"include": "data"}
			for k, v := range tc.args {
				args[k] = v
			}
			assertPayload(t, firstModelData(t, cli, tc.tool, args))
		})
	}

	// The request-scoped events route declares include too.
	t.Run("events scoped to a request with include=data returns the payload", func(t *testing.T) {
		result := CallGatewayMCPTool(t, cli.projectRoot, cli.configPath, "gateway_requests_read",
			map[string]any{"action": "list", "source_id": sourceID, "limit": 1}, 30*time.Second)
		require.False(t, result.IsError, result.Text)
		var envelope struct {
			Data struct {
				Models []struct {
					ID string `json:"id"`
				} `json:"models"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal([]byte(result.Text), &envelope))
		require.NotEmpty(t, envelope.Data.Models)

		assertPayload(t, firstModelData(t, cli, "gateway_events_read", map[string]any{
			"action": "list", "request_id": envelope.Data.Models[0].ID, "include": "data", "limit": 1,
		}))
	})
}
