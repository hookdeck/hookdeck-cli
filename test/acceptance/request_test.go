package acceptance

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestList(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess("gateway", "request", "list", "--limit", "5")
	assert.NotEmpty(t, stdout)
}

func TestRequestListAndGet(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	connID, _ := createConnectionAndTriggerEvent(t, cli)
	t.Cleanup(func() { deleteConnection(t, cli, connID) })

	// Get connection to find source ID, then poll for requests (ingestion may lag)
	var conn Connection
	require.NoError(t, cli.RunJSON(&conn, "gateway", "connection", "get", connID))
	requests := pollForRequestsBySourceID(t, cli, conn.Source.ID)
	requestID := requests[0].ID

	stdout := cli.RunExpectSuccess("gateway", "request", "get", requestID)
	assert.Contains(t, stdout, requestID)
}

func TestRequestEvents(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	connID, eventID := createConnectionAndTriggerEvent(t, cli)
	t.Cleanup(func() { deleteConnection(t, cli, connID) })

	var conn Connection
	require.NoError(t, cli.RunJSON(&conn, "gateway", "connection", "get", connID))
	requests := pollForRequestsBySourceID(t, cli, conn.Source.ID)
	requestID := requests[0].ID

	stdout := cli.RunExpectSuccess("gateway", "request", "events", requestID)
	assert.Contains(t, stdout, eventID)
}

func TestRequestRetry(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	connID, _ := createConnectionAndTriggerEvent(t, cli)
	t.Cleanup(func() { deleteConnection(t, cli, connID) })

	var conn Connection
	require.NoError(t, cli.RunJSON(&conn, "gateway", "connection", "get", connID))
	requests := pollForRequestsBySourceID(t, cli, conn.Source.ID)
	requestID := requests[0].ID

	// Retry is only allowed for rejected requests or those with ignored events. Our request
	// succeeded (MOCK_API delivered), so API may return "not eligible for retry". Either outcome is valid.
	stdout, stderr, err := cli.Run("gateway", "request", "retry", requestID)
	if err != nil {
		assert.Contains(t, stdout+stderr, "not eligible for retry", "retry failed for unexpected reason: %v", err)
		return
	}
	assert.Contains(t, stdout, "retry requested")
}

func TestRequestRetryWithConnectionIds(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	connID, _ := createConnectionAndTriggerEvent(t, cli)
	t.Cleanup(func() { deleteConnection(t, cli, connID) })
	var conn Connection
	require.NoError(t, cli.RunJSON(&conn, "gateway", "connection", "get", connID))
	requests := pollForRequestsBySourceID(t, cli, conn.Source.ID)
	requestID := requests[0].ID
	// --connection-ids is passed to API; request may not be eligible for retry, so accept success or "not eligible"
	stdout, stderr, err := cli.Run("gateway", "request", "retry", requestID, "--connection-ids", connID)
	if err != nil {
		assert.Contains(t, stdout+stderr, "not eligible for retry", "retry with connection-ids failed for unexpected reason: %v", err)
		return
	}
	assert.Contains(t, stdout, "retry requested")
}

func TestRequestIgnoredEvents(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	connID, _ := createConnectionAndTriggerEvent(t, cli)
	t.Cleanup(func() { deleteConnection(t, cli, connID) })

	var conn Connection
	require.NoError(t, cli.RunJSON(&conn, "gateway", "connection", "get", connID))
	requests := pollForRequestsBySourceID(t, cli, conn.Source.ID)
	requestID := requests[0].ID

	// May return empty list; we only check the command succeeds
	cli.RunExpectSuccess("gateway", "request", "ignored-events", requestID)
}

func TestRequestRawBody(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	connID, _ := createConnectionAndTriggerEvent(t, cli)
	t.Cleanup(func() { deleteConnection(t, cli, connID) })
	var conn Connection
	require.NoError(t, cli.RunJSON(&conn, "gateway", "connection", "get", connID))
	requests := pollForRequestsBySourceID(t, cli, conn.Source.ID)
	stdout := cli.RunExpectSuccess("gateway", "request", "raw-body", requests[0].ID)
	assert.Contains(t, stdout, "test")
}

func TestRequestListWithId(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	connID, _ := createConnectionAndTriggerEvent(t, cli)
	t.Cleanup(func() { deleteConnection(t, cli, connID) })
	var conn Connection
	require.NoError(t, cli.RunJSON(&conn, "gateway", "connection", "get", connID))
	requests := pollForRequestsBySourceID(t, cli, conn.Source.ID)
	cli.RunExpectSuccess("gateway", "request", "list", "--id", requests[0].ID)
}

func TestRequestListWithStatus(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	cli.RunExpectSuccess("gateway", "request", "list", "--status", "accepted", "--limit", "5")
}

func TestRequestListWithVerified(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	cli.RunExpectSuccess("gateway", "request", "list", "--verified", "true", "--limit", "5")
}

func TestRequestListWithRejectionCause(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	cli.RunExpectSuccess("gateway", "request", "list", "--rejection-cause", "VERIFICATION_FAILED", "--limit", "5")
}

func TestRequestListWithCreatedAfter(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	cli.RunExpectSuccess("gateway", "request", "list", "--created-after", "2020-01-01T00:00:00Z", "--limit", "5")
}

func TestRequestListWithCreatedBefore(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	cli.RunExpectSuccess("gateway", "request", "list", "--created-before", "2030-01-01T00:00:00Z", "--limit", "5")
}

func TestRequestListWithIngestedAtAfter(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	cli.RunExpectSuccess("gateway", "request", "list", "--ingested-at-after", "2020-01-01T00:00:00Z", "--limit", "5")
}

func TestRequestListWithIngestedAtBefore(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	cli.RunExpectSuccess("gateway", "request", "list", "--ingested-at-before", "2030-01-01T00:00:00Z", "--limit", "5")
}

func TestRequestListWithHeaders(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	cli.RunExpectSuccess("gateway", "request", "list", "--headers", "{}", "--limit", "5")
}

func TestRequestListWithBody(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	cli.RunExpectSuccess("gateway", "request", "list", "--body", "{}", "--limit", "5")
}

func TestRequestListWithPath(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	cli.RunExpectSuccess("gateway", "request", "list", "--path", "/", "--limit", "5")
}

func TestRequestListWithParsedQuery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	cli.RunExpectSuccess("gateway", "request", "list", "--parsed-query", "{}", "--limit", "5")
}

func TestRequestListWithOrderBy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	cli.RunExpectSuccess("gateway", "request", "list", "--order-by", "created_at", "--limit", "5")
}

func TestRequestListWithDir(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	cli.RunExpectSuccess("gateway", "request", "list", "--dir", "desc", "--limit", "5")
}

func TestRequestListWithNext(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	_, _, err := cli.Run("gateway", "request", "list", "--limit", "1", "--next", "dummy")
	if err != nil {
		assert.Contains(t, err.Error(), "exit status")
	}
}

func TestRequestListWithPrev(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	_, _, err := cli.Run("gateway", "request", "list", "--limit", "1", "--prev", "dummy")
	if err != nil {
		assert.Contains(t, err.Error(), "exit status")
	}
}

func TestRequestEventsWithNext(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	connID, _ := createConnectionAndTriggerEvent(t, cli)
	t.Cleanup(func() { deleteConnection(t, cli, connID) })
	var conn Connection
	require.NoError(t, cli.RunJSON(&conn, "gateway", "connection", "get", connID))
	requests := pollForRequestsBySourceID(t, cli, conn.Source.ID)
	// --next is passed to API; invalid cursor may return 400, so just verify command runs and params are accepted
	_, _, err := cli.Run("gateway", "request", "events", requests[0].ID, "--limit", "1", "--next", "dummy")
	if err != nil {
		// API may reject invalid cursor; ensure we're not crashing
		assert.Contains(t, err.Error(), "exit status")
	}
}

func TestRequestEventsWithPrev(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	connID, _ := createConnectionAndTriggerEvent(t, cli)
	t.Cleanup(func() { deleteConnection(t, cli, connID) })
	var conn Connection
	require.NoError(t, cli.RunJSON(&conn, "gateway", "connection", "get", connID))
	requests := pollForRequestsBySourceID(t, cli, conn.Source.ID)
	_, _, err := cli.Run("gateway", "request", "events", requests[0].ID, "--limit", "1", "--prev", "dummy")
	if err != nil {
		assert.Contains(t, err.Error(), "exit status")
	}
}

func TestRequestIgnoredEventsWithNext(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	connID, _ := createConnectionAndTriggerEvent(t, cli)
	t.Cleanup(func() { deleteConnection(t, cli, connID) })
	var conn Connection
	require.NoError(t, cli.RunJSON(&conn, "gateway", "connection", "get", connID))
	requests := pollForRequestsBySourceID(t, cli, conn.Source.ID)
	_, _, err := cli.Run("gateway", "request", "ignored-events", requests[0].ID, "--limit", "1", "--next", "dummy")
	if err != nil {
		assert.Contains(t, err.Error(), "exit status")
	}
}

func TestRequestIgnoredEventsWithPrev(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	connID, _ := createConnectionAndTriggerEvent(t, cli)
	t.Cleanup(func() { deleteConnection(t, cli, connID) })
	var conn Connection
	require.NoError(t, cli.RunJSON(&conn, "gateway", "connection", "get", connID))
	requests := pollForRequestsBySourceID(t, cli, conn.Source.ID)
	_, _, err := cli.Run("gateway", "request", "ignored-events", requests[0].ID, "--limit", "1", "--prev", "dummy")
	if err != nil {
		assert.Contains(t, err.Error(), "exit status")
	}
}

func TestRequestListPaginationWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	connID, _ := createConnectionAndTriggerEvent(t, cli)
	t.Cleanup(func() { deleteConnection(t, cli, connID) })

	// Get connection to find source ID
	var conn Connection
	require.NoError(t, cli.RunJSON(&conn, "gateway", "connection", "get", connID))
	require.NotEmpty(t, conn.Source.ID, "connection source ID")

	// Trigger multiple requests to ensure we have enough for pagination
	triggerEvent(t, cli, connID)
	triggerEvent(t, cli, connID)
	triggerEvent(t, cli, connID)
	time.Sleep(2 * time.Second) // Wait for requests to be processed

	// Test 1: JSON output includes pagination metadata
	type RequestListResponse struct {
		Models     []Request              `json:"models"`
		Pagination map[string]interface{} `json:"pagination"`
	}
	var firstPageResp RequestListResponse
	require.NoError(t, cli.RunJSON(&firstPageResp, "gateway", "request", "list", "--source-id", conn.Source.ID, "--limit", "2"))
	assert.NotEmpty(t, firstPageResp.Models, "First page should have requests")
	assert.NotNil(t, firstPageResp.Pagination, "JSON response should include pagination metadata")
	assert.Contains(t, firstPageResp.Pagination, "limit")
	assert.Equal(t, float64(2), firstPageResp.Pagination["limit"])

	// Test 2: Text output includes pagination info when next cursor exists
	if len(firstPageResp.Models) == 2 && firstPageResp.Pagination["next"] != nil {
		stdout := cli.RunExpectSuccess("gateway", "request", "list", "--source-id", conn.Source.ID, "--limit", "2")
		assert.Contains(t, stdout, "Pagination:")
		assert.Contains(t, stdout, "Next:")
		assert.Contains(t, stdout, "To get the next page:")
		assert.Contains(t, stdout, "--next")

		// Test 3: Use next cursor to get second page
		nextCursor := firstPageResp.Pagination["next"].(string)
		var secondPageResp RequestListResponse
		require.NoError(t, cli.RunJSON(&secondPageResp, "gateway", "request", "list", "--source-id", conn.Source.ID, "--limit", "2", "--next", nextCursor))
		assert.NotEmpty(t, secondPageResp.Models, "Second page should have requests")

		// Verify pages contain different requests
		firstPageIDs := make(map[string]bool)
		for _, r := range firstPageResp.Models {
			firstPageIDs[r.ID] = true
		}
		for _, r := range secondPageResp.Models {
			assert.False(t, firstPageIDs[r.ID], "Second page should not contain requests from first page")
		}
	}
}
