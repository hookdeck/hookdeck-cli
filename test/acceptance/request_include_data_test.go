//go:build request

package acceptance

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRequestIncludeData checks --include data on `gateway request list` and
// `gateway request events` against the live API. Each command calls the
// shared helper separately, so each is checked, with a negative control: the
// API ignores query parameters it does not declare, so only the difference
// between with and without shows the flag is doing the work.
func TestRequestIncludeData(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	connID, _ := createConnectionAndTriggerEvent(t, cli)
	t.Cleanup(func() { deleteConnection(t, cli, connID) })

	var conn Connection
	require.NoError(t, cli.RunJSON(&conn, "gateway", "connection", "get", connID))
	requestID := pollForRequestsBySourceID(t, cli, conn.Source.ID)[0].ID

	type model struct {
		ID   string         `json:"id"`
		Data map[string]any `json:"data"`
	}
	// request list prints {models, pagination}; request events prints a bare
	// array, so it has no page size to read back. The default limit on request
	// events is pinned by TestRequestEventsIncludeData instead.
	type listed struct {
		Models     []model `json:"models"`
		Pagination struct {
			Limit int `json:"limit"`
		} `json:"pagination"`
	}
	listModels := func(t *testing.T, args []string, bareArray bool) []model {
		t.Helper()
		if bareArray {
			var models []model
			require.NoError(t, cli.RunJSON(&models, args...))
			return models
		}
		var resp listed
		require.NoError(t, cli.RunJSON(&resp, args...))
		return resp.Models
	}

	commands := []struct {
		name      string
		args      []string
		bareArray bool
	}{
		{"request list", []string{"gateway", "request", "list", "--source-id", conn.Source.ID}, false},
		{"request events", []string{"gateway", "request", "events", requestID}, true},
	}

	t.Run("request list with include data and no limit the page size is 10", func(t *testing.T) {
		var resp listed
		require.NoError(t, cli.RunJSON(&resp, "gateway", "request", "list", "--source-id", conn.Source.ID, "--include", "data"))
		assert.Equal(t, 10, resp.Pagination.Limit)
	})

	for _, c := range commands {
		t.Run(c.name+" without include there is no payload", func(t *testing.T) {
			models := listModels(t, append(c.args, "--limit", "1"), c.bareArray)
			require.NotEmpty(t, models)
			assert.Nil(t, models[0].Data)
		})

		t.Run(c.name+" with include data the payload comes back", func(t *testing.T) {
			models := listModels(t, append(c.args, "--limit", "1", "--include", "data"), c.bareArray)
			require.NotEmpty(t, models)
			require.NotNil(t, models[0].Data, "include data must put a data object on the model")
			// triggerTestEvent posts {"test":true}.
			assert.Equal(t, map[string]any{"test": true}, models[0].Data["body"])
		})

		t.Run(c.name+" include without output json is refused", func(t *testing.T) {
			stdout, stderr, err := cli.Run(append(c.args, "--include", "data")...)
			require.Error(t, err)
			assert.Contains(t, stdout+stderr, "--output json")
		})
	}
}
