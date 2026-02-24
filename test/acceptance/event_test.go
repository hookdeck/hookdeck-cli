package acceptance

import (
	"testing"

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

	var events []Event
	require.NoError(t, cli.RunJSON(&events, "gateway", "event", "list", "--connection-id", connID, "--limit", "5"))
	assert.NotEmpty(t, events)
	assert.NotEmpty(t, events[0].ID)
	assert.NotEmpty(t, events[0].Status)
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
