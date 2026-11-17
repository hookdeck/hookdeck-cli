package mcp

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAttemptsResponseDiscoverable(t *testing.T) {
	session := connect(t, ServerOptions{Client: newTestClient(t, "https://example.com")})
	tool := listTools(t, session)["outpost_attempts_read"]
	require.NotNil(t, tool)
	raw, err := json.Marshal(tool.InputSchema)
	require.NoError(t, err)
	var schema struct {
		Properties map[string]struct {
			Description string `json:"description"`
		} `json:"properties"`
	}
	require.NoError(t, json.Unmarshal(raw, &schema))
	for _, value := range []string{"event", "event.data", "response_data", "destination"} {
		assert.Contains(t, schema.Properties["include"].Description, `"`+value+`"`)
	}
	assert.Contains(t, schema.Properties["action"].Description, "include response_data")
}
