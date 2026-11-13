//go:build event

package acceptance

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEventListIncludeData checks --include data on `gateway event list`
// against the live API.
//
// The negative control matters as much as the positive one: the API ignores
// query parameters it does not declare, so a dropped include would return a
// list with no payloads and exit zero. Asserting both directions is what shows
// the flag is doing the work.
func TestEventListIncludeData(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	connID, _ := createConnectionAndTriggerEvent(t, cli)
	t.Cleanup(func() { deleteConnection(t, cli, connID) })

	type listed struct {
		Models []struct {
			ID   string         `json:"id"`
			Data map[string]any `json:"data"`
		} `json:"models"`
		Pagination struct {
			Limit int `json:"limit"`
		} `json:"pagination"`
	}

	t.Run("without include there is no payload", func(t *testing.T) {
		var resp listed
		require.NoError(t, cli.RunJSON(&resp, "gateway", "event", "list", "--connection-id", connID, "--limit", "1"))
		require.NotEmpty(t, resp.Models)
		assert.Nil(t, resp.Models[0].Data)
	})

	t.Run("with include data the payload comes back", func(t *testing.T) {
		var resp listed
		require.NoError(t, cli.RunJSON(&resp, "gateway", "event", "list", "--connection-id", connID, "--limit", "1", "--include", "data"))
		require.NotEmpty(t, resp.Models)
		require.NotNil(t, resp.Models[0].Data, "include data must put a data object on the event")
		// triggerTestEvent posts {"test":true}.
		assert.Equal(t, map[string]any{"test": true}, resp.Models[0].Data["body"])
	})

	t.Run("with include data and no limit the page size is 10", func(t *testing.T) {
		var resp listed
		require.NoError(t, cli.RunJSON(&resp, "gateway", "event", "list", "--connection-id", connID, "--include", "data"))
		assert.Equal(t, 10, resp.Pagination.Limit)
	})

	t.Run("include without output json is refused", func(t *testing.T) {
		stdout, stderr, err := cli.Run("gateway", "event", "list", "--connection-id", connID, "--include", "data")
		require.Error(t, err)
		assert.Contains(t, stdout+stderr, "--output json")
	})

	// The API owns the value set. This pins that a wrong value fails loudly
	// rather than returning a list without payloads.
	t.Run("an unknown include value is rejected", func(t *testing.T) {
		_, _, err := cli.Run("gateway", "event", "list", "--connection-id", connID, "--include", "bogus", "--output", "json")
		require.Error(t, err)
	})
}
