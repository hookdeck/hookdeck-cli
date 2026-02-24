package acceptance

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAttemptList(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	connID, eventID := createConnectionAndTriggerEvent(t, cli)
	t.Cleanup(func() { deleteConnection(t, cli, connID) })

	stdout := cli.RunExpectSuccess("gateway", "attempt", "list", "--event-id", eventID)
	assert.NotEmpty(t, stdout)
}

func TestAttemptGet(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	connID, eventID := createConnectionAndTriggerEvent(t, cli)
	t.Cleanup(func() { deleteConnection(t, cli, connID) })

	attempts := pollForAttemptsByEventID(t, cli, eventID)
	attemptID := attempts[0].ID

	stdout := cli.RunExpectSuccess("gateway", "attempt", "get", attemptID)
	assert.Contains(t, stdout, attemptID)
	assert.Contains(t, stdout, eventID)
}

func TestAttemptListJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	connID, eventID := createConnectionAndTriggerEvent(t, cli)
	t.Cleanup(func() { deleteConnection(t, cli, connID) })

	attempts := pollForAttemptsByEventID(t, cli, eventID)
	assert.NotEmpty(t, attempts[0].ID)
	assert.Equal(t, eventID, attempts[0].EventID)
}

func TestAttemptListWithOrderBy(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	connID, eventID := createConnectionAndTriggerEvent(t, cli)
	t.Cleanup(func() { deleteConnection(t, cli, connID) })
	cli.RunExpectSuccess("gateway", "attempt", "list", "--event-id", eventID, "--order-by", "created_at", "--limit", "5")
}

func TestAttemptListWithDir(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	connID, eventID := createConnectionAndTriggerEvent(t, cli)
	t.Cleanup(func() { deleteConnection(t, cli, connID) })
	cli.RunExpectSuccess("gateway", "attempt", "list", "--event-id", eventID, "--dir", "desc", "--limit", "5")
}

func TestAttemptListWithLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	connID, eventID := createConnectionAndTriggerEvent(t, cli)
	t.Cleanup(func() { deleteConnection(t, cli, connID) })
	cli.RunExpectSuccess("gateway", "attempt", "list", "--event-id", eventID, "--limit", "2")
}

func TestAttemptListWithNext(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	connID, eventID := createConnectionAndTriggerEvent(t, cli)
	t.Cleanup(func() { deleteConnection(t, cli, connID) })
	_, _, err := cli.Run("gateway", "attempt", "list", "--event-id", eventID, "--limit", "1", "--next", "dummy")
	if err != nil {
		assert.Contains(t, err.Error(), "exit status")
	}
}

func TestAttemptListWithPrev(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	connID, eventID := createConnectionAndTriggerEvent(t, cli)
	t.Cleanup(func() { deleteConnection(t, cli, connID) })
	_, _, err := cli.Run("gateway", "attempt", "list", "--event-id", eventID, "--limit", "1", "--prev", "dummy")
	if err != nil {
		assert.Contains(t, err.Error(), "exit status")
	}
}

func TestAttemptListPaginationWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	cli := NewCLIRunner(t)
	connID, eventID := createConnectionAndTriggerEvent(t, cli)
	t.Cleanup(func() { deleteConnection(t, cli, connID) })

	// Retry the event multiple times to create multiple attempts
	for i := 0; i < 3; i++ {
		cli.RunExpectSuccess("gateway", "event", "retry", eventID)
		time.Sleep(2 * time.Second) // Wait for retry to be processed
	}

	// Poll for attempts to ensure we have multiple
	attempts := pollForAttemptsByEventID(t, cli, eventID)
	require.GreaterOrEqual(t, len(attempts), 2, "Need at least 2 attempts for pagination test")

	// Test 1: JSON output includes pagination metadata
	type AttemptListResponse struct {
		Models     []Attempt              `json:"models"`
		Pagination map[string]interface{} `json:"pagination"`
	}
	var firstPageResp AttemptListResponse
	require.NoError(t, cli.RunJSON(&firstPageResp, "gateway", "attempt", "list", "--event-id", eventID, "--limit", "2"))
	assert.NotEmpty(t, firstPageResp.Models, "First page should have attempts")
	assert.NotNil(t, firstPageResp.Pagination, "JSON response should include pagination metadata")
	assert.Contains(t, firstPageResp.Pagination, "limit")
	assert.Equal(t, float64(2), firstPageResp.Pagination["limit"])

	// Test 2: Text output includes pagination info when next cursor exists
	if len(firstPageResp.Models) == 2 && firstPageResp.Pagination["next"] != nil {
		stdout := cli.RunExpectSuccess("gateway", "attempt", "list", "--event-id", eventID, "--limit", "2")
		assert.Contains(t, stdout, "Pagination:")
		assert.Contains(t, stdout, "Next:")
		assert.Contains(t, stdout, "To get the next page:")
		assert.Contains(t, stdout, "--next")

		// Test 3: Use next cursor to get second page
		nextCursor := firstPageResp.Pagination["next"].(string)
		var secondPageResp AttemptListResponse
		require.NoError(t, cli.RunJSON(&secondPageResp, "gateway", "attempt", "list", "--event-id", eventID, "--limit", "2", "--next", nextCursor))
		assert.NotEmpty(t, secondPageResp.Models, "Second page should have attempts")

		// Verify pages contain different attempts
		firstPageIDs := make(map[string]bool)
		for _, a := range firstPageResp.Models {
			firstPageIDs[a.ID] = true
		}
		for _, a := range secondPageResp.Models {
			assert.False(t, firstPageIDs[a.ID], "Second page should not contain attempts from first page")
		}
	}
}
