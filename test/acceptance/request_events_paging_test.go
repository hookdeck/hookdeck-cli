//go:build request

package acceptance

import (
	"encoding/json"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRequestEventsPointsAtTheNextPage checks the stderr paging note on
// `gateway request events` against the live API.
//
// That command's JSON is a bare array with no cursor, so a page the API cut
// short looks complete. The note is driven by the API's pagination.next, and
// what a mock cannot tell us is whether the API sets next exactly when more
// events exist: not on a page that is merely full. Two connections on one
// source make one request fan out to two events, which gives both cases.
func TestRequestEventsPointsAtTheNextPage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)

	firstConnID := createTestConnectionWithMockDestination(t, cli)
	t.Cleanup(func() { deleteConnection(t, cli, firstConnID) })

	var first Connection
	require.NoError(t, cli.RunJSON(&first, "gateway", "connection", "get", firstConnID))
	sourceID := first.Source.ID
	require.NotEmpty(t, sourceID)

	var second Connection
	stamp := generateTimestamp()
	require.NoError(t, cli.RunJSON(&second,
		"gateway", "connection", "create",
		"--name", "test-conn-paging-"+stamp,
		"--source-id", sourceID,
		"--destination-name", "test-dst-paging-"+stamp,
		"--destination-type", "MOCK_API",
	))
	require.NotEmpty(t, second.ID)
	t.Cleanup(func() { deleteConnection(t, cli, second.ID) })

	var src Source
	require.NoError(t, cli.RunJSON(&src, "gateway", "source", "get", sourceID))
	triggerTestEvent(t, src.URL)

	// Wait for the request to show both of its events.
	requestID := ""
	for i := 0; i < propagationAttempts && requestID == ""; i++ {
		time.Sleep(propagationInterval)
		var resp struct {
			Models []Request `json:"models"`
		}
		require.NoError(t, cli.RunJSON(&resp, "gateway", "request", "list", "--source-id", sourceID, "--limit", "5"))
		for _, r := range resp.Models {
			if r.EventsCount >= 2 {
				requestID = r.ID
				break
			}
		}
	}
	require.NotEmpty(t, requestID, "no request reached two events within %s", propagationAttempts*propagationInterval)

	// eventIDs runs request events with --output json and returns the event
	// IDs from stdout and whatever was written to stderr.
	eventIDs := func(t *testing.T, args ...string) ([]string, string) {
		t.Helper()
		full := append([]string{"gateway", "request", "events", requestID, "--output", "json"}, args...)
		stdout, stderr, err := cli.Run(full...)
		require.NoError(t, err, "stdout=%s stderr=%s", stdout, stderr)
		var events []struct {
			ID string `json:"id"`
		}
		require.NoError(t, json.Unmarshal([]byte(stdout), &events), "stdout must stay valid JSON: %s", stdout)
		ids := make([]string, len(events))
		for i, e := range events {
			ids[i] = e.ID
		}
		return ids, stderr
	}

	// events_count reaches two before the second event is listable on the
	// request's events route, so wait on the route itself.
	listed := 0
	for i := 0; i < propagationAttempts && listed < 2; i++ {
		ids, _ := eventIDs(t, "--limit", "5")
		listed = len(ids)
		if listed < 2 {
			time.Sleep(propagationInterval)
		}
	}
	require.Equal(t, 2, listed, "request events never listed both events within %s", propagationAttempts*propagationInterval)

	cursorRE := regexp.MustCompile(`--next (\S+)`)

	t.Run("a cut-short page gives the cursor and the cursor reaches the rest", func(t *testing.T) {
		firstPage, stderr := eventIDs(t, "--limit", "1")
		require.Len(t, firstPage, 1)
		m := cursorRE.FindStringSubmatch(stderr)
		require.NotNil(t, m, "expected a --next cursor on stderr, got: %q", stderr)

		secondPage, stderr := eventIDs(t, "--limit", "1", "--next", m[1])
		require.Len(t, secondPage, 1)
		assert.NotEqual(t, firstPage[0], secondPage[0], "the cursor must lead to a different event")
		assert.NotContains(t, stderr, "--next", "the last page must not point at another one")
	})

	t.Run("a page that is exactly full gets no note", func(t *testing.T) {
		ids, stderr := eventIDs(t, "--limit", "2")
		require.Len(t, ids, 2)
		assert.NotContains(t, stderr, "--next", fmt.Sprintf("no more events exist, so no cursor should be offered: %q", stderr))
	})
}
