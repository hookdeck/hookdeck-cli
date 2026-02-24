package acceptance

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventList(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess("gateway", "event", "list", "--limit", "5")
	assert.NotEmpty(t, stdout)
}

func TestEventListWithConnectionID(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	connID, eventID := createConnectionAndTriggerEvent(t, cli)
	t.Cleanup(func() { deleteConnection(t, cli, connID) })

	stdout := cli.RunExpectSuccess("gateway", "event", "list", "--connection-id", connID)
	assert.Contains(t, stdout, eventID)
	assert.Contains(t, stdout, connID)
}

func TestEventGet(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	connID, eventID := createConnectionAndTriggerEvent(t, cli)
	t.Cleanup(func() { deleteConnection(t, cli, connID) })

	stdout := cli.RunExpectSuccess("gateway", "event", "get", eventID)
	assert.Contains(t, stdout, eventID)
	assert.Contains(t, stdout, connID)
}

func TestEventRetry(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	connID, eventID := createConnectionAndTriggerEvent(t, cli)
	t.Cleanup(func() { deleteConnection(t, cli, connID) })

	stdout := cli.RunExpectSuccess("gateway", "event", "retry", eventID)
	assert.Contains(t, stdout, "retry requested")
}

func TestEventCancel(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	connID, eventID := createConnectionAndTriggerEvent(t, cli)
	t.Cleanup(func() { deleteConnection(t, cli, connID) })

	stdout := cli.RunExpectSuccess("gateway", "event", "cancel", eventID)
	assert.Contains(t, stdout, "cancelled")
}

func TestEventMute(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	connID, eventID := createConnectionAndTriggerEvent(t, cli)
	t.Cleanup(func() { deleteConnection(t, cli, connID) })

	stdout := cli.RunExpectSuccess("gateway", "event", "mute", eventID)
	assert.Contains(t, stdout, "muted")
}

func TestEventListJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	connID, _ := createConnectionAndTriggerEvent(t, cli)
	t.Cleanup(func() { deleteConnection(t, cli, connID) })

	type EventListResponse struct {
		Models     []Event                `json:"models"`
		Pagination map[string]interface{} `json:"pagination"`
	}
	var resp EventListResponse
	require.NoError(t, cli.RunJSON(&resp, "gateway", "event", "list", "--connection-id", connID, "--limit", "5"))
	assert.NotEmpty(t, resp.Models)
	assert.NotEmpty(t, resp.Models[0].ID)
	assert.NotEmpty(t, resp.Models[0].Status)
	assert.NotNil(t, resp.Pagination)
}

func TestEventRawBody(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	connID, eventID := createConnectionAndTriggerEvent(t, cli)
	t.Cleanup(func() { deleteConnection(t, cli, connID) })

	stdout := cli.RunExpectSuccess("gateway", "event", "raw-body", eventID)
	// We triggered with {"test":true}
	assert.Contains(t, stdout, "test")
}

func TestEventListWithId(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	connID, eventID := createConnectionAndTriggerEvent(t, cli)
	t.Cleanup(func() { deleteConnection(t, cli, connID) })
	cli.RunExpectSuccess("gateway", "event", "list", "--id", eventID, "--limit", "5")
}

func TestEventListWithAttempts(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	cli.RunExpectSuccess("gateway", "event", "list", "--attempts", "1", "--limit", "5")
}

func TestEventListWithResponseStatus(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	cli.RunExpectSuccess("gateway", "event", "list", "--response-status", "200", "--limit", "5")
}

func TestEventListWithErrorCode(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	cli.RunExpectSuccess("gateway", "event", "list", "--error-code", "TIMEOUT", "--limit", "5")
}

func TestEventListWithCliID(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	cli.RunExpectSuccess("gateway", "event", "list", "--cli-id", "cli_xxx", "--limit", "5")
}

func TestEventListWithIssueID(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	cli.RunExpectSuccess("gateway", "event", "list", "--issue-id", "iss_xxx", "--limit", "5")
}

func TestEventListWithCreatedAfter(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	cli.RunExpectSuccess("gateway", "event", "list", "--created-after", "2020-01-01T00:00:00Z", "--limit", "5")
}

func TestEventListWithCreatedBefore(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	cli.RunExpectSuccess("gateway", "event", "list", "--created-before", "2030-01-01T00:00:00Z", "--limit", "5")
}

func TestEventListWithSuccessfulAtAfter(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	cli.RunExpectSuccess("gateway", "event", "list", "--successful-at-after", "2020-01-01T00:00:00Z", "--limit", "5")
}

func TestEventListWithSuccessfulAtBefore(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	cli.RunExpectSuccess("gateway", "event", "list", "--successful-at-before", "2030-01-01T00:00:00Z", "--limit", "5")
}

func TestEventListWithLastAttemptAtAfter(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	cli.RunExpectSuccess("gateway", "event", "list", "--last-attempt-at-after", "2020-01-01T00:00:00Z", "--limit", "5")
}

func TestEventListWithLastAttemptAtBefore(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	cli.RunExpectSuccess("gateway", "event", "list", "--last-attempt-at-before", "2030-01-01T00:00:00Z", "--limit", "5")
}

func TestEventListWithHeaders(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	cli.RunExpectSuccess("gateway", "event", "list", "--headers", "{}", "--limit", "5")
}

func TestEventListWithBody(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	cli.RunExpectSuccess("gateway", "event", "list", "--body", "{}", "--limit", "5")
}

func TestEventListWithPath(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	cli.RunExpectSuccess("gateway", "event", "list", "--path", "/webhooks", "--limit", "5")
}

func TestEventListWithParsedQuery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	cli.RunExpectSuccess("gateway", "event", "list", "--parsed-query", "{}", "--limit", "5")
}

func TestEventListWithSourceID(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	connID, _ := createConnectionAndTriggerEvent(t, cli)
	t.Cleanup(func() { deleteConnection(t, cli, connID) })
	var conn Connection
	require.NoError(t, cli.RunJSON(&conn, "gateway", "connection", "get", connID))
	cli.RunExpectSuccess("gateway", "event", "list", "--source-id", conn.Source.ID, "--limit", "5")
}

func TestEventListWithDestinationID(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	connID, _ := createConnectionAndTriggerEvent(t, cli)
	t.Cleanup(func() { deleteConnection(t, cli, connID) })
	var conn Connection
	require.NoError(t, cli.RunJSON(&conn, "gateway", "connection", "get", connID))
	cli.RunExpectSuccess("gateway", "event", "list", "--destination-id", conn.Destination.ID, "--limit", "5")
}

func TestEventListWithStatus(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	cli.RunExpectSuccess("gateway", "event", "list", "--status", "SUCCESSFUL", "--limit", "5")
}

func TestEventListWithOrderBy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	cli.RunExpectSuccess("gateway", "event", "list", "--order-by", "created_at", "--limit", "5")
}

func TestEventListWithDir(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	cli.RunExpectSuccess("gateway", "event", "list", "--dir", "desc", "--limit", "5")
}

func TestEventListWithNext(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	_, _, err := cli.Run("gateway", "event", "list", "--limit", "1", "--next", "dummy")
	if err != nil {
		assert.Contains(t, err.Error(), "exit status")
	}
}

func TestEventListWithPrev(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	_, _, err := cli.Run("gateway", "event", "list", "--limit", "1", "--prev", "dummy")
	if err != nil {
		assert.Contains(t, err.Error(), "exit status")
	}
}

func TestEventListPaginationWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	connID, _ := createConnectionAndTriggerEvent(t, cli)
	t.Cleanup(func() { deleteConnection(t, cli, connID) })

	// Trigger multiple events to ensure we have enough for pagination
	triggerEvent(t, cli, connID)
	triggerEvent(t, cli, connID)
	triggerEvent(t, cli, connID)
	time.Sleep(2 * time.Second) // Wait for events to be processed

	// Test 1: JSON output includes pagination metadata
	type EventListResponse struct {
		Models     []Event                `json:"models"`
		Pagination map[string]interface{} `json:"pagination"`
	}
	var firstPageResp EventListResponse
	require.NoError(t, cli.RunJSON(&firstPageResp, "gateway", "event", "list", "--connection-id", connID, "--limit", "2"))
	assert.NotEmpty(t, firstPageResp.Models, "First page should have events")
	assert.NotNil(t, firstPageResp.Pagination, "JSON response should include pagination metadata")
	assert.Contains(t, firstPageResp.Pagination, "limit")
	assert.Equal(t, float64(2), firstPageResp.Pagination["limit"])

	// Test 2: Text output includes pagination info when next cursor exists
	if len(firstPageResp.Models) == 2 && firstPageResp.Pagination["next"] != nil {
		stdout := cli.RunExpectSuccess("gateway", "event", "list", "--connection-id", connID, "--limit", "2")
		assert.Contains(t, stdout, "Pagination:")
		assert.Contains(t, stdout, "Next:")
		assert.Contains(t, stdout, "To get the next page:")
		assert.Contains(t, stdout, "--next")

		// Test 3: Use next cursor to get second page
		nextCursor := firstPageResp.Pagination["next"].(string)
		var secondPageResp EventListResponse
		require.NoError(t, cli.RunJSON(&secondPageResp, "gateway", "event", "list", "--connection-id", connID, "--limit", "2", "--next", nextCursor))
		assert.NotEmpty(t, secondPageResp.Models, "Second page should have events")

		// Verify pages contain different events
		firstPageIDs := make(map[string]bool)
		for _, e := range firstPageResp.Models {
			firstPageIDs[e.ID] = true
		}
		for _, e := range secondPageResp.Models {
			assert.False(t, firstPageIDs[e.ID], "Second page should not contain events from first page")
		}
	}
}

// triggerEvent is a helper function to trigger an additional event on an existing connection
func triggerEvent(t *testing.T, cli *CLIRunner, connID string) {
	t.Helper()
	var conn Connection
	require.NoError(t, cli.RunJSON(&conn, "gateway", "connection", "get", connID))
	require.NotEmpty(t, conn.Source.ID, "connection source ID")

	var src Source
	require.NoError(t, cli.RunJSON(&src, "gateway", "source", "get", conn.Source.ID))
	require.NotEmpty(t, src.URL, "source URL")

	triggerTestEvent(t, src.URL)
}
