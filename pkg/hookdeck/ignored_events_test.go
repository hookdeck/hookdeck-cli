package hookdeck

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// GET /requests/{id}/ignored_events returns IgnoredEvent records: id, team_id,
// webhook_id, cause, request_id, meta, created_at. They have no status, source
// or destination, and the cause is the only field that says why the event was
// ignored. meta is an array, an object or null depending on the cause.
const ignoredEventsPage = `{"models":[
  {"id":"ign_1","team_id":"t","webhook_id":"web_1","cause":"FILTERED","request_id":"req_1","meta":["body"],"created_at":"2026-11-01T10:00:00Z"},
  {"id":"ign_2","team_id":"t","webhook_id":"web_1","cause":"TRANSFORMATION_FAILED","request_id":"req_1","meta":{"error":"boom"},"created_at":"2026-11-01T10:00:01Z"},
  {"id":"ign_3","team_id":"t","webhook_id":"web_2","cause":"CLI_DISCONNECTED","request_id":"req_1","meta":null,"created_at":"2026-11-01T10:00:02Z"}
],"pagination":{"limit":10}}`

func TestGetRequestIgnoredEventsKeepsCauseAndMeta(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(ignoredEventsPage))
	}))
	t.Cleanup(srv.Close)
	base, err := url.Parse(srv.URL)
	require.NoError(t, err)
	c := &Client{BaseURL: base}

	resp, err := c.GetRequestIgnoredEvents(context.Background(), "req_1", nil)
	require.NoError(t, err)
	require.Len(t, resp.Models, 3)

	assert.Equal(t, "FILTERED", resp.Models[0].Cause)
	assert.Equal(t, []any{"body"}, resp.Models[0].Meta)
	assert.Equal(t, "TRANSFORMATION_FAILED", resp.Models[1].Cause)
	assert.Equal(t, map[string]any{"error": "boom"}, resp.Models[1].Meta)
	assert.Equal(t, "CLI_DISCONNECTED", resp.Models[2].Cause)
	assert.Nil(t, resp.Models[2].Meta)

	// What the CLI and MCP print: cause and meta present, no phantom status.
	out, err := json.Marshal(resp.Models[0])
	require.NoError(t, err)
	var fields map[string]any
	require.NoError(t, json.Unmarshal(out, &fields))
	assert.Equal(t, "FILTERED", fields["cause"])
	assert.Contains(t, fields, "meta")
	assert.NotContains(t, fields, "status")
}
