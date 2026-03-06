package acceptance

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// metricsStart and metricsEnd define a fixed date range for metrics acceptance tests.
// Use a past range that the API will accept.
const metricsStart = "2025-01-01T00:00:00Z"
const metricsEnd = "2025-01-02T00:00:00Z"

func metricsArgs(subcmd string, extra ...string) []string {
	args := []string{"gateway", "metrics", subcmd, "--start", metricsStart, "--end", metricsEnd}
	return append(args, extra...)
}

// --- Help ---

// TestMetricsHelp verifies that hookdeck gateway metrics --help lists all 4 subcommands.
func TestMetricsHelp(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess("gateway", "metrics", "--help")
	assert.Contains(t, stdout, "events")
	assert.Contains(t, stdout, "requests")
	assert.Contains(t, stdout, "attempts")
	assert.Contains(t, stdout, "transformations")
	// Removed subcommands should not appear
	assert.NotContains(t, stdout, "queue-depth")
	assert.NotContains(t, stdout, "pending")
	assert.NotContains(t, stdout, "events-by-issue")
}

// --- Events (default) ---

func TestMetricsEvents(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess(append(metricsArgs("events"), "--measures", "count")...)
	assert.NotEmpty(t, stdout)
}

func TestMetricsEventsWithGranularity(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess(append(metricsArgs("events"), "--granularity", "1d", "--measures", "count")...)
	assert.NotEmpty(t, stdout)
}

func TestMetricsEventsWithMeasures(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess(append(metricsArgs("events"), "--measures", "count,failed_count")...)
	assert.NotEmpty(t, stdout)
}

// --- Events (consolidated: queue-depth routing) ---

func TestMetricsEventsQueueDepth(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess(append(metricsArgs("events"), "--measures", "max_depth")...)
	assert.NotEmpty(t, stdout)
}

func TestMetricsEventsQueueDepthWithDimensions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess(append(metricsArgs("events"), "--measures", "max_depth,max_age", "--dimensions", "destination_id")...)
	assert.NotEmpty(t, stdout)
}

// --- Events (consolidated: pending routing) ---

func TestMetricsEventsPending(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess(append(metricsArgs("events"), "--measures", "pending", "--granularity", "1h")...)
	assert.NotEmpty(t, stdout)
}

// --- Events (consolidated: events-by-issue routing) ---

func TestMetricsEventsByIssueID(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess(append(metricsArgs("events"), "--measures", "count", "--issue-id", "iss_placeholder")...)
	assert.NotEmpty(t, stdout)
}

func TestMetricsEventsByIssueDimension(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess(append(metricsArgs("events"), "--measures", "count", "--dimensions", "issue_id")...)
	assert.NotEmpty(t, stdout)
}

// --- Events (filters) ---

func TestMetricsEventsWithSourceID(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess(append(metricsArgs("events"), "--measures", "count", "--source-id", "src_placeholder")...)
	assert.NotEmpty(t, stdout)
}

func TestMetricsEventsWithConnectionID(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess(append(metricsArgs("events"), "--measures", "count", "--connection-id", "web_placeholder")...)
	assert.NotEmpty(t, stdout)
}

func TestMetricsEventsWithDestinationID(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess(append(metricsArgs("events"), "--measures", "count", "--destination-id", "dst_placeholder")...)
	assert.NotEmpty(t, stdout)
}

func TestMetricsEventsWithStatus(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess(append(metricsArgs("events"), "--measures", "count", "--status", "SUCCESSFUL")...)
	assert.NotEmpty(t, stdout)
}

// --- Events (JSON output) ---

func TestMetricsEventsOutputJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	var data []struct {
		TimeBucket *string                `json:"time_bucket"`
		Dimensions map[string]interface{} `json:"dimensions"`
		Metrics    map[string]float64     `json:"metrics"`
	}
	require.NoError(t, cli.RunJSON(&data, append(metricsArgs("events"), "--measures", "count")...))
	assert.NotNil(t, data)
}

func TestMetricsEventsQueueDepthOutputJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	var data []struct {
		TimeBucket *string                `json:"time_bucket"`
		Dimensions map[string]interface{} `json:"dimensions"`
		Metrics    map[string]float64     `json:"metrics"`
	}
	require.NoError(t, cli.RunJSON(&data, append(metricsArgs("events"), "--measures", "max_depth")...))
	assert.NotNil(t, data)
}

// --- Requests ---

func TestMetricsRequests(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess(append(metricsArgs("requests"), "--measures", "count")...)
	assert.NotEmpty(t, stdout)
}

func TestMetricsRequestsWithMeasuresAndDimensions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess(append(metricsArgs("requests"), "--measures", "count,accepted_count", "--dimensions", "source_id")...)
	assert.NotEmpty(t, stdout)
}

// --- Attempts ---

func TestMetricsAttempts(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess(append(metricsArgs("attempts"), "--measures", "count")...)
	assert.NotEmpty(t, stdout)
}

func TestMetricsAttemptsWithMeasuresAndDimensions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess(append(metricsArgs("attempts"), "--measures", "count,error_rate", "--dimensions", "destination_id")...)
	assert.NotEmpty(t, stdout)
}

// --- Transformations ---

func TestMetricsTransformations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess(append(metricsArgs("transformations"), "--measures", "count")...)
	assert.NotEmpty(t, stdout)
}

func TestMetricsTransformationsWithMeasures(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess(append(metricsArgs("transformations"), "--measures", "count,error_rate")...)
	assert.NotEmpty(t, stdout)
}

func TestMetricsTransformationsWithMeasuresAndDimensions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess(append(metricsArgs("transformations"), "--measures", "count,error_rate", "--dimensions", "connection_id")...)
	assert.NotEmpty(t, stdout)
}

// --- Validation ---

func TestMetricsEventsMissingStart(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	_, _, err := cli.Run("gateway", "metrics", "events", "--end", metricsEnd)
	require.Error(t, err)
}

func TestMetricsEventsMissingEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	_, _, err := cli.Run("gateway", "metrics", "events", "--start", metricsStart)
	require.Error(t, err)
}

func TestMetricsRequestsMissingStart(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	_, _, err := cli.Run("gateway", "metrics", "requests", "--end", metricsEnd)
	require.Error(t, err)
}

func TestMetricsAttemptsMissingEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	_, _, err := cli.Run("gateway", "metrics", "attempts", "--start", metricsStart)
	require.Error(t, err)
}

func TestMetricsEventsMissingMeasures(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	_, _, err := cli.Run("gateway", "metrics", "events", "--start", metricsStart, "--end", metricsEnd)
	require.Error(t, err)
}
