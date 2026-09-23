package hookdeck

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBulkFiltersMatchTheSpec reads the filter matrix out of the OpenAPI
// document and compares it with the one this package hardcodes.
//
// The matrix is the guard against a filter being sent to a family that does not
// declare it — the API ignores it and the operation runs across everything the
// remaining filters match. A hardcoded matrix that drifts from the document is
// therefore worse than none, because it reads as authoritative.
func TestBulkFiltersMatchTheSpec(t *testing.T) {
	doc := loadSpec(t)

	routes := map[string]string{
		BulkEventsRetry:        "/bulk/events/retry",
		BulkEventsCancel:       "/bulk/events/cancel",
		BulkIgnoredEventsRetry: "/bulk/ignored-events/retry",
		BulkRequestsRetry:      "/bulk/requests/retry",
		BulkRequestsReplay:     "/bulk/requests/replay",
	}

	for family, path := range routes {
		t.Run(family, func(t *testing.T) {
			declared := bulkQueryFilters(t, doc, path)
			ours := append([]string{}, BulkFilters[family]...)
			sort.Strings(declared)
			sort.Strings(ours)
			assert.Equal(t, declared, ours,
				"the %s filter list has drifted from the OpenAPI document", family)
		})
	}
}

// The asymmetry that makes the matrix necessary, stated as a test so nobody
// "tidies" the families into one list.
func TestIgnoredEventsTakesFarFewerFilters(t *testing.T) {
	assert.Len(t, BulkFilters[BulkIgnoredEventsRetry], 3)
	assert.Greater(t, len(BulkFilters[BulkEventsRetry]), 20,
		"the event families take the full event filter set")
}

func TestRejectUnsupportedBulkFilters(t *testing.T) {
	// A filter from a sibling family is the realistic mistake.
	err := RejectUnsupportedBulkFilters(BulkIgnoredEventsRetry,
		map[string]interface{}{"status": "FAILED", "webhook_id": "web_1"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status")
	assert.NotContains(t, err.Error(), "webhook_id is not", "webhook_id is valid here")
	assert.Contains(t, err.Error(), "cause", "the message should list what the family does take")

	assert.NoError(t, RejectUnsupportedBulkFilters(BulkIgnoredEventsRetry,
		map[string]interface{}{"cause": "x", "webhook_id": "web_1"}))

	// target belongs to replay and nothing else.
	assert.NoError(t, RejectUnsupportedBulkFilters(BulkRequestsReplay,
		map[string]interface{}{"target": map[string]any{"source_id": "src_1"}}))
	assert.Error(t, RejectUnsupportedBulkFilters(BulkRequestsRetry,
		map[string]interface{}{"target": map[string]any{"source_id": "src_1"}}))
}

// events_cancel is the one family with no cancel route. Reporting that is more
// use than letting the call 404.
func TestEventsCancelCannotBeCancelled(t *testing.T) {
	assert.False(t, BulkCancellable(BulkEventsCancel))
	for _, f := range []string{BulkEventsRetry, BulkIgnoredEventsRetry, BulkRequestsRetry, BulkRequestsReplay} {
		assert.True(t, BulkCancellable(f), "%s should be cancellable", f)
	}
}

func loadSpec(t *testing.T) map[string]interface{} {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(dir)
		require.NotEqual(t, parent, dir, "no go.mod above the test")
		dir = parent
	}
	raw, err := os.ReadFile(filepath.Join(dir, "test", "openapi", "openapi_2026-09-01.json"))
	require.NoError(t, err)
	var doc map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &doc))
	return doc
}

// bulkQueryFilters pulls the properties of the `query` object a family's POST
// body declares.
func bulkQueryFilters(t *testing.T, doc map[string]interface{}, path string) []string {
	t.Helper()
	paths := doc["paths"].(map[string]interface{})
	op, ok := paths[path].(map[string]interface{})
	require.True(t, ok, "%s is not in the document", path)
	post := op["post"].(map[string]interface{})
	body := post["requestBody"].(map[string]interface{})
	content := body["content"].(map[string]interface{})
	schema := content["application/json"].(map[string]interface{})["schema"].(map[string]interface{})
	props := schema["properties"].(map[string]interface{})
	query := props["query"].(map[string]interface{})
	qprops, ok := query["properties"].(map[string]interface{})
	require.True(t, ok, "%s query declares no properties", path)

	out := make([]string, 0, len(qprops))
	for k := range qprops {
		out = append(out, k)
	}
	return out
}

// The API reads the query as an object in bracket notation, the same
// convention the metrics routes use. A JSON string is accepted by any
// hand-written mock and rejected by the API with "query must be of type
// object" — which is how this was found.
func TestBulkQueryUsesBracketNotation(t *testing.T) {
	got := encodeBulkQuery(map[string]interface{}{
		"status":     "FAILED",
		"webhook_id": "web_1",
	})
	assert.Contains(t, got, "query%5Bstatus%5D=FAILED")
	assert.Contains(t, got, "query%5Bwebhook_id%5D=web_1")
	assert.NotContains(t, got, "%7B", "a JSON object must not be sent as a string")
}

func TestBulkQueryEncodesNestedAndListValues(t *testing.T) {
	got := encodeBulkQuery(map[string]interface{}{
		"target":     map[string]interface{}{"source_id": "src_1"},
		"created_at": map[string]interface{}{"gte": "2026-01-01T00:00:00Z"},
		"id":         []interface{}{"evt_1", "evt_2"},
	})
	assert.Contains(t, got, "query%5Btarget%5D%5Bsource_id%5D=src_1")
	assert.Contains(t, got, "query%5Bcreated_at%5D%5Bgte%5D=2026-01-01T00%3A00%3A00Z")
	assert.Contains(t, got, "query%5Bid%5D%5B%5D=evt_1")
	assert.Contains(t, got, "query%5Bid%5D%5B%5D=evt_2")
}

// The connection dimension is webhook_id on the wire and connection_id
// everywhere a caller types it. Bulk was the one surface that leaked the wire
// name, so an agent had to spell it connection_id for every other tool and
// webhook_id only here.
func TestBulkQueryTakesConnectionIDLikeEveryOtherTool(t *testing.T) {
	t.Run("connection_id is rewritten to the API's spelling", func(t *testing.T) {
		got := CanonicalBulkQuery(map[string]interface{}{
			"connection_id": "web_1",
			"status":        "FAILED",
		})
		assert.Equal(t, map[string]interface{}{
			"webhook_id": "web_1",
			"status":     "FAILED",
		}, got)
	})

	t.Run("webhook_id still works", func(t *testing.T) {
		got := CanonicalBulkQuery(map[string]interface{}{"webhook_id": "web_1"})
		assert.Equal(t, map[string]interface{}{"webhook_id": "web_1"}, got)
	})

	t.Run("connection_id passes validation once canonicalised", func(t *testing.T) {
		query := CanonicalBulkQuery(map[string]interface{}{"connection_id": "web_1"})
		assert.NoError(t, RejectUnsupportedBulkFilters(BulkIgnoredEventsRetry, query))
	})

	// The refusal has to advertise the spelling the caller is told to use
	// everywhere else, or it names a filter they were never offered.
	t.Run("the refusal names connection_id, not webhook_id", func(t *testing.T) {
		err := RejectUnsupportedBulkFilters(
			BulkIgnoredEventsRetry,
			map[string]interface{}{"status": "FAILED"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "connection_id")
		assert.NotContains(t, err.Error(), "webhook_id")
	})
}
