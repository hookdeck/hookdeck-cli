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

// TestMetricsHelp verifies that hookdeck gateway metrics --help lists all 7 subcommands.
func TestMetricsHelp(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess("gateway", "metrics", "--help")
	assert.Contains(t, stdout, "events")
	assert.Contains(t, stdout, "requests")
	assert.Contains(t, stdout, "attempts")
	assert.Contains(t, stdout, "queue-depth")
	assert.Contains(t, stdout, "pending")
	assert.Contains(t, stdout, "events-by-issue")
	assert.Contains(t, stdout, "transformations")
}

// Baseline: one success test per endpoint. API requires at least one measure for most endpoints.
func TestMetricsEvents(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess(append(metricsArgs("events"), "--measures", "count")...)
	assert.NotEmpty(t, stdout)
}

func TestMetricsRequests(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess(append(metricsArgs("requests"), "--measures", "count")...)
	assert.NotEmpty(t, stdout)
}

func TestMetricsAttempts(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess(append(metricsArgs("attempts"), "--measures", "count")...)
	assert.NotEmpty(t, stdout)
}

func TestMetricsQueueDepth(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess(append(metricsArgs("queue-depth"), "--measures", "max_depth")...)
	assert.NotEmpty(t, stdout)
}

func TestMetricsPending(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess(append(metricsArgs("pending"), "--measures", "count")...)
	assert.NotEmpty(t, stdout)
}

func TestMetricsEventsByIssue(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	// events-by-issue requires issue-id as positional argument and --measures
	stdout := cli.RunExpectSuccess("gateway", "metrics", "events-by-issue", "iss_placeholder", "--start", metricsStart, "--end", metricsEnd, "--measures", "count")
	assert.NotEmpty(t, stdout)
}

func TestMetricsTransformations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess(append(metricsArgs("transformations"), "--measures", "count")...)
	assert.NotEmpty(t, stdout)
}

// Common flags: granularity, measures, dimensions, source-id, destination-id, connection-id, output.
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

func TestMetricsQueueDepthWithMeasuresAndDimensions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess(append(metricsArgs("queue-depth"), "--measures", "max_depth,max_age", "--dimensions", "destination_id")...)
	assert.NotEmpty(t, stdout)
}

func TestMetricsEventsWithSourceID(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	// Filter by a placeholder ID; API may return empty data but command should succeed
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

// Output: JSON structure (array of objects with time_bucket, dimensions, metrics).
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
	// Response is an array; may be empty
	assert.NotNil(t, data)
}

func TestMetricsQueueDepthOutputJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	var data []struct {
		TimeBucket *string                `json:"time_bucket"`
		Dimensions map[string]interface{} `json:"dimensions"`
		Metrics    map[string]float64     `json:"metrics"`
	}
	require.NoError(t, cli.RunJSON(&data, append(metricsArgs("queue-depth"), "--measures", "max_depth")...))
	assert.NotNil(t, data)
}

// Validation: missing --start or --end should fail.
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

// Missing --measures: API returns 422 (measures required for all endpoints).
func TestMetricsEventsMissingMeasures(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	_, _, err := cli.Run("gateway", "metrics", "events", "--start", metricsStart, "--end", metricsEnd)
	require.Error(t, err)
}

// events-by-issue without required <issue-id> argument: Cobra rejects (ExactArgs(1)).
func TestMetricsEventsByIssueMissingIssueID(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	_, _, err := cli.Run("gateway", "metrics", "events-by-issue", "--start", metricsStart, "--end", metricsEnd, "--measures", "count")
	require.Error(t, err)
}

// Pending and transformations with minimal flags.
func TestMetricsPendingWithGranularity(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess(append(metricsArgs("pending"), "--granularity", "1h", "--measures", "count")...)
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
