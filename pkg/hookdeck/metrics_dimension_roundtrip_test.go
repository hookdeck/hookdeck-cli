package hookdeck

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The API spells the connection dimension webhook_id. The CLI and the MCP tool
// both accept connection_id and translate outbound, but neither translated the
// response, so a caller who asked for connection_id got results keyed
// webhook_id and had to know an internal name to read their own answer (#442).
//
// These cover the round trip, which is the property that matters: whatever
// dimension name went out is the one that comes back.
func TestDimensionNamesRoundTrip(t *testing.T) {
	asked := []string{"connection_id", "status"}
	sent := MapDimensionsToAPI(asked)
	assert.Equal(t, []string{"webhook_id", "status"}, sent, "the API is asked in its own spelling")

	// What the API answers with, keyed as it sent it.
	response := MetricsResponse{
		{Dimensions: map[string]interface{}{"webhook_id": "web_123", "status": "SUCCESSFUL"}},
	}
	got := RestoreDimensionNames(response)

	assert.Equal(t, "web_123", got[0].Dimensions["connection_id"],
		"the answer must use the name the caller asked with")
	assert.NotContains(t, got[0].Dimensions, "webhook_id",
		"the internal name must not leak back to the caller")
	assert.Equal(t, "SUCCESSFUL", got[0].Dimensions["status"],
		"untranslated dimensions are left alone")
}

func TestRestoreDimensionNamesEdgeCases(t *testing.T) {
	t.Run("nil dimensions are left alone", func(t *testing.T) {
		got := RestoreDimensionNames(MetricsResponse{{Dimensions: nil}})
		assert.Nil(t, got[0].Dimensions)
	})

	t.Run("no webhook_id is a no-op", func(t *testing.T) {
		got := RestoreDimensionNames(MetricsResponse{
			{Dimensions: map[string]interface{}{"status": "FAILED"}},
		})
		assert.Equal(t, map[string]interface{}{"status": "FAILED"}, got[0].Dimensions)
	})

	// If the API ever answers with both, the one it called connection_id wins;
	// silently overwriting it would be the same class of bug this fixes.
	t.Run("an existing connection_id is never clobbered", func(t *testing.T) {
		got := RestoreDimensionNames(MetricsResponse{
			{Dimensions: map[string]interface{}{"webhook_id": "web_1", "connection_id": "conn_real"}},
		})
		assert.Equal(t, "conn_real", got[0].Dimensions["connection_id"])
		assert.Equal(t, "web_1", got[0].Dimensions["webhook_id"])
	})

	t.Run("empty response", func(t *testing.T) {
		assert.Empty(t, RestoreDimensionNames(MetricsResponse{}))
	})
}

func TestMapDimensionsToAPIIsIdempotent(t *testing.T) {
	once := MapDimensionsToAPI([]string{"connection_id"})
	assert.Equal(t, once, MapDimensionsToAPI(once),
		"mapping an already-mapped slice must not change it")
	assert.Nil(t, MapDimensionsToAPI(nil))
}
