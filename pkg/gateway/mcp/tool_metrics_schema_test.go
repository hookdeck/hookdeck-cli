package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// metricsSchemaProperty returns one property of the live gateway_metrics tool
// schema, as an MCP client would read it off tools/list.
func metricsSchemaProperty(t *testing.T, name string) map[string]any {
	t.Helper()
	client := newTestClient("https://api.hookdeck.com", "test-api-key")
	session := connectInMemory(t, client)

	listed, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)

	for _, tool := range listed.Tools {
		if tool.Name != "gateway_metrics" {
			continue
		}
		raw, err := json.Marshal(tool.InputSchema)
		require.NoError(t, err)
		var schema struct {
			Properties map[string]map[string]any `json:"properties"`
		}
		require.NoError(t, json.Unmarshal(raw, &schema))
		prop, ok := schema.Properties[name]
		require.True(t, ok, "gateway_metrics schema has no %q property", name)
		return prop
	}
	t.Fatal("gateway_metrics not listed")
	return nil
}

// TestMetricsSchemaMeasuresAreAccuratePerAction pins the measures contract.
//
// There is no enum and no per-action breakdown on this property, so its
// description IS what a client plans against. It used to read "Common: count,
// successful_count, failed_count, error_count" - one list standing in for four
// endpoints, of which only count works on all four: error_count is valid on
// transformations alone, and successful_count and failed_count both 422 on
// requests. The CLI documents the right list per subcommand; this puts the same
// four lists, from the same constants, into the tool schema.
func TestMetricsSchemaMeasuresAreAccuratePerAction(t *testing.T) {
	desc, _ := metricsSchemaProperty(t, "measures")["description"].(string)
	require.NotEmpty(t, desc)

	for action, measures := range map[string]string{
		"events":          hookdeck.EventMetricsMeasures,
		"requests":        hookdeck.RequestMetricsMeasures,
		"attempts":        hookdeck.AttemptMetricsMeasures,
		"transformations": hookdeck.TransformationMetricsMeasures,
	} {
		assert.Contains(t, desc, action+": "+measures,
			"measures description must carry the real %s list", action)
	}

	// The specific claims that were wrong. error_count is a transformations
	// measure only, and the requests endpoint has neither success nor failure
	// counts - it counts accepted and rejected.
	assert.NotContains(t, desc, "Common: count, successful_count, failed_count, error_count",
		"the inaccurate blanket list must be gone")
	requestsPart := actionSlice(t, desc, "requests: ", "; attempts:")
	for _, absent := range []string{"successful_count", "failed_count", "error_count"} {
		assert.NotContains(t, requestsPart, absent,
			"requests metrics do not accept %s", absent)
	}
	assert.Contains(t, actionSlice(t, desc, "transformations: ", ". Only count"), "error_count",
		"error_count is valid on transformations")
}

// TestMetricsSchemaDimensionsAreAccuratePerAction is the same guard for
// dimensions, plus the cross-field rule a client cannot discover any other way.
func TestMetricsSchemaDimensionsAreAccuratePerAction(t *testing.T) {
	desc, _ := metricsSchemaProperty(t, "dimensions")["description"].(string)
	require.NotEmpty(t, desc)

	for action, dimensions := range map[string]string{
		"events":          hookdeck.EventMetricsDimensions,
		"requests":        hookdeck.RequestMetricsDimensions,
		"attempts":        hookdeck.AttemptMetricsDimensions,
		"transformations": hookdeck.TransformationMetricsDimensions,
	} {
		assert.Contains(t, desc, action+": "+dimensions,
			"dimensions description must carry the real %s list", action)
	}

	assert.Contains(t, desc, "delivery_group also requires destination_id",
		"the API's cross-field rule must be documented, not discovered as a 422")
	assert.NotEqual(t, "Grouping dimensions", desc, "the placeholder description must be gone")
}

// TestMetricsSchemaStatusIsAccuratePerAction pins the third parameter that
// decides whether a call succeeds. status means something different on each
// action - accepted/rejected at the edge, a delivery status on events and
// attempts, nothing at all on transformations - and the schema said only
// "Filter by status".
func TestMetricsSchemaStatusIsAccuratePerAction(t *testing.T) {
	desc, _ := metricsSchemaProperty(t, "status")["description"].(string)
	require.NotEmpty(t, desc)

	assert.Contains(t, desc, "events: "+hookdeck.EventStatusValues)
	assert.Contains(t, desc, "requests: "+hookdeck.RequestStatusValues)
	assert.Contains(t, desc, "attempts: "+hookdeck.AttemptStatusValues)
	assert.Contains(t, desc, "Not supported on transformations")
}

// actionSlice cuts the part of a description belonging to one action, so a
// value can be asserted absent from that action without tripping over another
// action that legitimately has it.
func actionSlice(t *testing.T, desc, from, to string) string {
	t.Helper()
	start := strings.Index(desc, from)
	require.GreaterOrEqual(t, start, 0, "description has no %q section", from)
	rest := desc[start+len(from):]
	end := strings.Index(rest, to)
	require.GreaterOrEqual(t, end, 0, "description section %q is not terminated by %q", from, to)
	return rest[:end]
}

// TestAPIValidationErrorReachesTheClientReadable pins the wiring, not just the
// helper: a 422 has to arrive at the MCP client as the one line worth reading.
//
// These bodies carry no top-level "message", so the whole thing was pasted into
// the error text - {"level":"info","handled":true,"report":true,...} and all -
// with the useful part buried mid-string. Every gated error in this tool that
// the client side does not catch first ends up on this path.
func TestAPIValidationErrorReachesTheClientReadable(t *testing.T) {
	session := mockAPIWithClient(t, map[string]http.HandlerFunc{
		hookdeck.APIPathPrefix + "/metrics/events": func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"level":"info","handled":true,"report":true,"data":["granularity must match the required pattern"],"status":422,"code":"UNPROCESSABLE_ENTITY"}`))
		},
	})

	result := callTool(t, session, "gateway_metrics", map[string]any{
		"action":      "events",
		"start":       "2025-01-01T00:00:00Z",
		"end":         "2025-01-02T00:00:00Z",
		"measures":    []any{"count"},
		"granularity": "not-a-granularity",
	})

	require.True(t, result.IsError)
	body := textContent(t, result)
	assert.Equal(t, "granularity must match the required pattern", body,
		"the client should get the message and nothing else")
	for _, leaked := range []string{"level", "handled", "report", "UNPROCESSABLE_ENTITY", "status"} {
		assert.NotContains(t, body, leaked, "internal field %q must not reach the client", leaked)
	}
}
