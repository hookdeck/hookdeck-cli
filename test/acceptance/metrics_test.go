//go:build metrics

package acceptance

import (
	"strings"
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

// TestMetricsHelp verifies that hookdeck gateway metrics --help lists all 4 subcommands and required flags.
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
	assert.Contains(t, stdout, "--start")
	assert.Contains(t, stdout, "--end")
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

// TestMetricsEventsQueueDepthMeasure covers the advertised spelling of the
// measure. `queue_depth` is what --help tells the user to pass, but the
// endpoint's enum accepts max_depth and max_age only, so the flag could never
// succeed until the CLI started translating it. TestMetricsEventsQueueDepth
// above passes max_depth, the wire spelling, so it never exercised the name
// the CLI documents.
func TestMetricsEventsQueueDepthMeasure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess(append(metricsArgs("events"), "--measures", "queue_depth")...)
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

// TestMetricsEventsPendingWithoutGranularity covers --measures pending on its
// own. Routing to the pending-timeseries endpoint used to be gated on
// --granularity also being set; without it the call fell through to the default
// events route, which rejects the measure. Granularity is optional on that
// endpoint, so TestMetricsEventsPending above — which always passes
// --granularity 1h — could never have caught it.
func TestMetricsEventsPendingWithoutGranularity(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess(append(metricsArgs("events"), "--measures", "pending")...)
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
	stdout := cli.RunExpectSuccess(append(metricsArgs("events"), "--measures", "count", "--dimensions", "issue_id", "--issue-id", "iss_placeholder")...)
	assert.NotEmpty(t, stdout)
}

func TestMetricsEventsPerIssueRequiresIssueID(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout, stderr, err := cli.Run(append(metricsArgs("events"), "--measures", "count", "--dimensions", "issue_id")...)
	require.Error(t, err)
	combined := stdout + stderr
	assert.True(t, strings.Contains(combined, "per-issue") && strings.Contains(combined, "--issue-id"),
		"expected per-issue/--issue-id error message in output; got stdout: %q stderr: %q", stdout, stderr)
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

func TestMetricsRequestsWithSourceID(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess(append(metricsArgs("requests"), "--measures", "count", "--source-id", "src_placeholder")...)
	assert.NotEmpty(t, stdout)
}

func TestMetricsRequestsWithGranularity(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess(append(metricsArgs("requests"), "--measures", "count", "--granularity", "1d")...)
	assert.NotEmpty(t, stdout)
}

func TestMetricsRequestsOutputJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	var data []struct {
		TimeBucket *string                `json:"time_bucket"`
		Dimensions map[string]interface{} `json:"dimensions"`
		Metrics    map[string]float64     `json:"metrics"`
	}
	require.NoError(t, cli.RunJSON(&data, append(metricsArgs("requests"), "--measures", "count")...))
	assert.NotNil(t, data)
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

// TestMetricsAttemptsRejectsConnectionID replaces a test that asserted
// `metrics attempts --connection-id` succeeds. It did succeed, but only because
// the attempts endpoint has no webhook_id filter and the API drops keys it does
// not recognize: the call returned unfiltered totals under a flag that said
// otherwise. Measured against production over 14 days - attempts returned 150
// with and without a bogus --connection-id, while events returned 0 against 145
// for the same flag, which that endpoint does honour.
//
// The flag is no longer offered there, so the contract to hold is that it is
// refused rather than silently ignored.
func TestMetricsAttemptsRejectsConnectionID(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout, stderr, err := cli.Run(append(metricsArgs("attempts"), "--measures", "count", "--connection-id", "web_placeholder")...)
	require.Error(t, err, "attempts ignores connection-id, so the flag must not be accepted")
	// cobra writes the flag error to stdout, not stderr.
	assert.Contains(t, stdout+stderr, "unknown flag")
}

func TestMetricsAttemptsWithGranularity(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess(append(metricsArgs("attempts"), "--measures", "count", "--granularity", "1d")...)
	assert.NotEmpty(t, stdout)
}

func TestMetricsAttemptsOutputJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	var data []struct {
		TimeBucket *string                `json:"time_bucket"`
		Dimensions map[string]interface{} `json:"dimensions"`
		Metrics    map[string]float64     `json:"metrics"`
	}
	require.NoError(t, cli.RunJSON(&data, append(metricsArgs("attempts"), "--measures", "count")...))
	assert.NotNil(t, data)
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

func TestMetricsTransformationsWithConnectionID(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess(append(metricsArgs("transformations"), "--measures", "count", "--connection-id", "web_placeholder")...)
	assert.NotEmpty(t, stdout)
}

func TestMetricsTransformationsWithGranularity(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	stdout := cli.RunExpectSuccess(append(metricsArgs("transformations"), "--measures", "count", "--granularity", "1d")...)
	assert.NotEmpty(t, stdout)
}

func TestMetricsTransformationsOutputJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	var data []struct {
		TimeBucket *string                `json:"time_bucket"`
		Dimensions map[string]interface{} `json:"dimensions"`
		Metrics    map[string]float64     `json:"metrics"`
	}
	require.NoError(t, cli.RunJSON(&data, append(metricsArgs("transformations"), "--measures", "count")...))
	assert.NotNil(t, data)
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

func TestMetricsTransformationsMissingStart(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	_, _, err := cli.Run("gateway", "metrics", "transformations", "--end", metricsEnd, "--measures", "count")
	require.Error(t, err)
}

func TestMetricsTransformationsMissingEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping acceptance test in short mode")
	}
	cli := NewCLIRunner(t)
	_, _, err := cli.Run("gateway", "metrics", "transformations", "--start", metricsStart, "--measures", "count")
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
