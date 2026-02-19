package acceptance

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
