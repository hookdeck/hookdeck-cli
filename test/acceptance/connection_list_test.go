package acceptance

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConnectionListFilters tests the various filtering flags for connection list
func TestConnectionListFilters(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}

	t.Run("FilterDisabledConnections", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping acceptance test in short mode")
		}

		cli := NewCLIRunner(t)
		timestamp := generateTimestamp()

		connName := "test-disabled-filter-" + timestamp
		sourceName := "test-disabled-src-" + timestamp
		destName := "test-disabled-dst-" + timestamp

		// Create a connection
		var conn Connection
		err := cli.RunJSON(&conn,
			"gateway", "connection", "create",
			"--name", connName,
			"--source-name", sourceName,
			"--source-type", "WEBHOOK",
			"--destination-name", destName,
			"--destination-type", "CLI",
			"--destination-cli-path", "/webhooks",
		)
		require.NoError(t, err, "Should create connection")
		require.NotEmpty(t, conn.ID, "Connection should have an ID")

		// Cleanup
		t.Cleanup(func() {
			deleteConnection(t, cli, conn.ID)
		})

		// Verify connection is NOT in disabled list
		stdout, stderr, err := cli.Run("gateway", "connection", "list", "--disabled", "--output", "json")
		require.NoError(t, err, "Should list disabled connections: stderr=%s", stderr)

		type ConnectionListResponse struct {
			Models []Connection `json:"models"`
		}
		var disabledConnsResp ConnectionListResponse
		err = json.Unmarshal([]byte(stdout), &disabledConnsResp)
		require.NoError(t, err, "Should parse JSON response")

		// Check that our connection IS in the disabled list (inclusive filtering)
		// When --disabled is used, it shows ALL connections (both active and disabled)
		found := false
		for _, c := range disabledConnsResp.Models {
			if c.ID == conn.ID {
				found = true
				break
			}
		}
		assert.True(t, found, "Active connection should appear when --disabled flag is used (inclusive filtering)")

		// Disable the connection
		_, stderr, err = cli.Run("gateway", "connection", "disable", conn.ID)
		require.NoError(t, err, "Should disable connection: stderr=%s", stderr)

		// Verify connection IS in disabled list
		stdout, stderr, err = cli.Run("gateway", "connection", "list", "--disabled", "--output", "json")
		require.NoError(t, err, "Should list disabled connections: stderr=%s", stderr)

		err = json.Unmarshal([]byte(stdout), &disabledConnsResp)
		require.NoError(t, err, "Should parse JSON response")

		// Check that our connection IS now in the disabled list
		found = false
		for _, c := range disabledConnsResp.Models {
			if c.ID == conn.ID {
				found = true
				break
			}
		}
		assert.True(t, found, "Disabled connection should appear when filtering for disabled connections")

		// Verify connection is NOT in default list (without --disabled flag)
		stdout, stderr, err = cli.Run("gateway", "connection", "list", "--output", "json")
		require.NoError(t, err, "Should list connections: stderr=%s", stderr)

		var activeConnsResp ConnectionListResponse
		err = json.Unmarshal([]byte(stdout), &activeConnsResp)
		require.NoError(t, err, "Should parse JSON response")

		// Check that our disabled connection is NOT in the default list
		found = false
		for _, c := range activeConnsResp.Models {
			if c.ID == conn.ID {
				found = true
				break
			}
		}
		assert.False(t, found, "Disabled connection should not appear in default connection list")

		t.Logf("Successfully tested --disabled flag filtering: %s", conn.ID)
	})

	t.Run("FilterByName", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping acceptance test in short mode")
		}

		cli := NewCLIRunner(t)
		timestamp := generateTimestamp()

		connName := "test-name-filter-unique-" + timestamp
		sourceName := "test-name-filter-src-" + timestamp
		destName := "test-name-filter-dst-" + timestamp

		// Create a connection with a unique name
		var conn Connection
		err := cli.RunJSON(&conn,
			"gateway", "connection", "create",
			"--name", connName,
			"--source-name", sourceName,
			"--source-type", "WEBHOOK",
			"--destination-name", destName,
			"--destination-type", "CLI",
			"--destination-cli-path", "/webhooks",
		)
		require.NoError(t, err, "Should create connection")
		require.NotEmpty(t, conn.ID, "Connection should have an ID")

		// Cleanup
		t.Cleanup(func() {
			deleteConnection(t, cli, conn.ID)
		})

		// Filter by exact name
		stdout, stderr, err := cli.Run("gateway", "connection", "list", "--name", connName, "--output", "json")
		require.NoError(t, err, "Should filter by name: stderr=%s", stderr)

		var filteredConnsResp ConnectionListResponse
		err = json.Unmarshal([]byte(stdout), &filteredConnsResp)
		require.NoError(t, err, "Should parse JSON response")

		// Should find exactly our connection
		found := false
		for _, c := range filteredConnsResp.Models {
			if c.ID == conn.ID {
				found = true
				assert.Equal(t, connName, c.Name, "Connection name should match")
				break
			}
		}
		assert.True(t, found, "Should find connection when filtering by exact name")

		t.Logf("Successfully tested --name flag filtering: %s", conn.ID)
	})

	t.Run("FilterBySourceID", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping acceptance test in short mode")
		}

		cli := NewCLIRunner(t)
		timestamp := generateTimestamp()

		connName := "test-source-filter-" + timestamp
		sourceName := "test-source-filter-src-" + timestamp
		destName := "test-source-filter-dst-" + timestamp

		// Create a connection
		var conn Connection
		err := cli.RunJSON(&conn,
			"gateway", "connection", "create",
			"--name", connName,
			"--source-name", sourceName,
			"--source-type", "WEBHOOK",
			"--destination-name", destName,
			"--destination-type", "CLI",
			"--destination-cli-path", "/webhooks",
		)
		require.NoError(t, err, "Should create connection")
		require.NotEmpty(t, conn.ID, "Connection should have an ID")

		// Get source ID from the created connection
		var getResp map[string]interface{}
		err = cli.RunJSON(&getResp, "gateway", "connection", "get", conn.ID)
		require.NoError(t, err, "Should get connection details")

		source, ok := getResp["source"].(map[string]interface{})
		require.True(t, ok, "Expected source object")
		sourceID, ok := source["id"].(string)
		require.True(t, ok && sourceID != "", "Expected source ID")

		// Cleanup
		t.Cleanup(func() {
			deleteConnection(t, cli, conn.ID)
		})

		// Filter by source ID
		stdout, stderr, err := cli.Run("gateway", "connection", "list", "--source-id", sourceID, "--output", "json")
		require.NoError(t, err, "Should filter by source ID: stderr=%s", stderr)

		var filteredConnsResp ConnectionListResponse
		err = json.Unmarshal([]byte(stdout), &filteredConnsResp)
		require.NoError(t, err, "Should parse JSON response")

		// Should find our connection
		found := false
		for _, c := range filteredConnsResp.Models {
			if c.ID == conn.ID {
				found = true
				break
			}
		}
		assert.True(t, found, "Should find connection when filtering by source ID")

		t.Logf("Successfully tested --source-id flag filtering: source=%s, conn=%s", sourceID, conn.ID)
	})

	t.Run("FilterByLimit", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping acceptance test in short mode")
		}

		cli := NewCLIRunner(t)

		// List with limit of 5
		stdout, stderr, err := cli.Run("gateway", "connection", "list", "--limit", "5", "--output", "json")
		require.NoError(t, err, "Should list with limit: stderr=%s", stderr)

		var connsResp ConnectionListResponse
		err = json.Unmarshal([]byte(stdout), &connsResp)
		require.NoError(t, err, "Should parse JSON response")

		// Should have at most 5 connections
		assert.LessOrEqual(t, len(connsResp.Models), 5, "Should respect limit parameter")

		t.Logf("Successfully tested --limit flag: returned %d connections (max 5)", len(connsResp.Models))
	})

	t.Run("HumanReadableOutput", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skipping acceptance test in short mode")
		}

		cli := NewCLIRunner(t)
		timestamp := generateTimestamp()

		connName := "test-human-output-" + timestamp
		sourceName := "test-human-src-" + timestamp
		destName := "test-human-dst-" + timestamp

		// Create a connection to test output format
		var conn Connection
		err := cli.RunJSON(&conn,
			"gateway", "connection", "create",
			"--name", connName,
			"--source-name", sourceName,
			"--source-type", "WEBHOOK",
			"--destination-name", destName,
			"--destination-type", "CLI",
			"--destination-cli-path", "/webhooks",
		)
		require.NoError(t, err, "Should create connection")
		require.NotEmpty(t, conn.ID, "Connection should have an ID")

		// Cleanup
		t.Cleanup(func() {
			deleteConnection(t, cli, conn.ID)
		})

		// List without --output json to get human-readable format
		stdout := cli.RunExpectSuccess("gateway", "connection", "list")

		// Should contain human-readable text
		assert.True(t,
			strings.Contains(stdout, "connection") || strings.Contains(stdout, "No connections found"),
			"Should produce human-readable output")

		// Verify source and destination types are displayed
		assert.True(t,
			strings.Contains(stdout, "[WEBHOOK]") || strings.Contains(stdout, "[webhook]"),
			"Should display source type in output")
		assert.True(t,
			strings.Contains(stdout, "[CLI]") || strings.Contains(stdout, "[cli]"),
			"Should display destination type in output")

		t.Logf("Successfully tested human-readable output format with type display")
	})
}
