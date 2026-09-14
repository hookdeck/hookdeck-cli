package hookdeck

import (
	"fmt"
	"strings"
)

// MetricsFilters names the filters a metrics endpoint actually honours.
//
// The API drops a filter its schema does not declare and returns unfiltered
// totals, so offering one where it has no effect is worse than omitting it.
// Shared by the CLI and MCP layers so they cannot drift apart.
type MetricsFilters struct {
	SourceID      bool
	DestinationID bool
	ConnectionID  bool
	Status        bool
	IssueID       bool
	DeliveryGroup bool
}

// Filters honoured by each metrics endpoint. Keep in step with the API.
var (
	RequestMetricsFilters        = MetricsFilters{SourceID: true, Status: true}
	AttemptMetricsFilters        = MetricsFilters{DestinationID: true, Status: true, DeliveryGroup: true}
	TransformationMetricsFilters = MetricsFilters{ConnectionID: true, IssueID: true}

	// The four endpoints `events` can route to, depending on measures and
	// dimensions. The caller offers the union and narrows per route.
	EventMetricsFilters           = MetricsFilters{SourceID: true, DestinationID: true, ConnectionID: true, Status: true, IssueID: true, DeliveryGroup: true}
	DefaultEventRouteFilters      = MetricsFilters{SourceID: true, DestinationID: true, ConnectionID: true, Status: true, DeliveryGroup: true}
	QueueDepthRouteFilters        = MetricsFilters{DestinationID: true, DeliveryGroup: true}
	PendingTimeseriesRouteFilters = MetricsFilters{DestinationID: true}
	EventsByIssueRouteFilters     = MetricsFilters{SourceID: true, DestinationID: true, ConnectionID: true, IssueID: true}
)

// RejectUnsupportedFilters reports the first filter that was set but is not
// honoured by the endpoint the call routes to. names supplies the caller's own
// spelling for each filter, so a CLI user reads "--source-id" and an MCP client
// reads "source_id".
func RejectUnsupportedFilters(params MetricsQueryParams, allowed MetricsFilters, route string, names MetricsFilterNames) error {
	checks := []struct {
		set  bool
		ok   bool
		name string
	}{
		{params.SourceID != "", allowed.SourceID, names.SourceID},
		{params.DestinationID != "", allowed.DestinationID, names.DestinationID},
		{params.ConnectionID != "", allowed.ConnectionID, names.ConnectionID},
		{params.Status != "", allowed.Status, names.Status},
		{params.IssueID != "", allowed.IssueID, names.IssueID},
		{params.DeliveryGroup != "", allowed.DeliveryGroup, names.DeliveryGroup},
	}
	for _, c := range checks {
		if c.set && !c.ok {
			return fmt.Errorf("%s is not supported by %s; the API would ignore it and return unfiltered results", c.name, route)
		}
	}
	return nil
}

// MetricsFilterNames is how each filter is spelled to the caller.
type MetricsFilterNames struct {
	SourceID      string
	DestinationID string
	ConnectionID  string
	Status        string
	IssueID       string
	DeliveryGroup string
}

// CLIFilterNames spells filters as command-line flags.
var CLIFilterNames = MetricsFilterNames{
	SourceID:      "--source-id",
	DestinationID: "--destination-id",
	ConnectionID:  "--connection-id",
	Status:        "--status",
	IssueID:       "--issue-id",
	DeliveryGroup: "--delivery-group",
}

// MCPFilterNames spells filters as tool arguments.
var MCPFilterNames = MetricsFilterNames{
	SourceID:      "source_id",
	DestinationID: "destination_id",
	ConnectionID:  "connection_id",
	Status:        "status",
	IssueID:       "issue_id",
	DeliveryGroup: "delivery_group",
}

// Dimensions honoured by each metrics endpoint, taken from the API's OpenAPI
// document (the `dimensions` enum of each GET /metrics/* operation).
//
// These differ sharply between endpoints, so neither --help nor the MCP tool
// schema may advertise one generic list: naming a dimension the route does not
// accept sends the caller into an API 422. Filters have been gated against a
// shared matrix for a while; dimensions were not gated at all, which is how
// `dimensions: ["delivery_group"]` and `dimensions: ["status"]` on pending
// event metrics reached the API as raw 422s.
//
// Spelled as the API spells them: the connection dimension is webhook_id here,
// and both callers map their own connection_id onto it before validating.
var (
	RequestMetricsDimensionValues        = []string{"source_id", "rejection_cause", "status", "bulk_retry_ids", "events_count", "ignored_count"}
	AttemptMetricsDimensionValues        = []string{"destination_id", "delivery_group", "event_id", "status", "error_code", "bulk_retry_id", "trigger"}
	TransformationMetricsDimensionValues = []string{"transformation_id", "webhook_id", "log_level", "issue_id"}

	// The four endpoints `events` can route to. As with filters, the caller
	// advertises the union and narrows per route.
	DefaultEventRouteDimensions      = []string{"source_id", "destination_id", "webhook_id", "delivery_group", "status", "error_code", "event_data_id", "cli_id", "cli_user_id", "attempts", "response_status"}
	QueueDepthRouteDimensions        = []string{"destination_id", "delivery_group"}
	PendingTimeseriesRouteDimensions = []string{"destination_id"}
	EventsByIssueRouteDimensions     = []string{"issue_id", "source_id", "destination_id", "webhook_id"}

	EventMetricsDimensionValues = unionValues(
		DefaultEventRouteDimensions,
		QueueDepthRouteDimensions,
		PendingTimeseriesRouteDimensions,
		EventsByIssueRouteDimensions,
	)
)

// Dimension vocabularies rendered for a caller, in the caller's spelling.
var (
	RequestMetricsDimensions        = DimensionList(RequestMetricsDimensionValues)
	AttemptMetricsDimensions        = DimensionList(AttemptMetricsDimensionValues)
	TransformationMetricsDimensions = DimensionList(TransformationMetricsDimensionValues)
	EventMetricsDimensions          = DimensionList(EventMetricsDimensionValues)
)

// Measures honoured by each metrics action, from the same OpenAPI document.
//
// `events` additionally carries the route-selecting spellings the CLI and MCP
// invented - "pending" and "queue_depth" - which are translated before the
// request is sent. Advertised, not enforced: the API owns the enum, so a new
// measure it gains still reaches it rather than being refused here.
var (
	EventMetricsMeasureValues          = []string{"count", "successful_count", "failed_count", "scheduled_count", "paused_count", "error_rate", "avg_attempts", "scheduled_retry_count", "max_count_per_second", "pending", "queue_depth", "max_depth", "max_age"}
	RequestMetricsMeasureValues        = []string{"count", "accepted_count", "rejected_count", "discarded_count", "avg_events_per_request", "avg_ignored_per_request"}
	AttemptMetricsMeasureValues        = []string{"count", "successful_count", "failed_count", "delivered_count", "error_rate", "response_latency_avg", "response_latency_max", "response_latency_p95", "response_latency_p99", "delivery_latency_avg"}
	TransformationMetricsMeasureValues = []string{"count", "successful_count", "failed_count", "error_rate", "error_count", "warn_count", "info_count", "debug_count"}
)

// Measure vocabularies rendered for a caller.
var (
	EventMetricsMeasures          = ValueList(EventMetricsMeasureValues)
	RequestMetricsMeasures        = ValueList(RequestMetricsMeasureValues)
	AttemptMetricsMeasures        = ValueList(AttemptMetricsMeasureValues)
	TransformationMetricsMeasures = ValueList(TransformationMetricsMeasureValues)
)

// Status vocabularies. Requests are accepted or rejected at the edge; events
// and attempts carry a delivery status. Transformation metrics have no status
// filter at all.
//
// EventStatusValues is rendered from EventStatusValueList (status.go), the list
// the log routes validate a caller's value against, so what is advertised and
// what is accepted cannot drift. The metrics route spells the request statuses
// upper case; the request log spells them lower case, as RequestLogStatusValues.
var (
	RequestStatusValues        = "ACCEPTED, REJECTED"
	EventStatusValues          = ValueList(EventStatusValueList)
	AttemptStatusValues        = "SUCCESSFUL, FAILED"
	TransformationStatusValues = ""
)

// ValueList renders a vocabulary for display in --help or a tool schema.
func ValueList(values []string) string {
	return strings.Join(values, ", ")
}

// DimensionList renders a dimension vocabulary in the caller's spelling: the
// API's webhook_id is connection_id to both the CLI and MCP, which map it on
// the way in.
func DimensionList(values []string) string {
	out := make([]string, len(values))
	for i, v := range values {
		if v == "webhook_id" {
			v = "connection_id"
		}
		out[i] = v
	}
	return strings.Join(out, ", ")
}

// unionValues concatenates vocabularies, keeping first-seen order and dropping
// duplicates, so a union list cannot drift from the routes it is built from.
func unionValues(lists ...[]string) []string {
	var out []string
	seen := map[string]bool{}
	for _, list := range lists {
		for _, v := range list {
			if seen[v] {
				continue
			}
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

func containsValue(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

// RejectUnsupportedDimensions reports the first dimension the endpoint does not
// define, plus the API's one cross-field rule: grouping by delivery_group needs
// a destination_id filter.
//
// Without this the caller sees a raw 422 for something the tool appeared to
// offer - and on `dimensions: ["delivery_group"]` that is the release's
// headline feature looking broken. dimensionsName is the caller's own spelling
// of the argument, so a CLI user reads "--dimensions" and an MCP client reads
// "dimensions".
func RejectUnsupportedDimensions(params MetricsQueryParams, allowed []string, route string, names MetricsFilterNames, dimensionsName string) error {
	for _, d := range params.Dimensions {
		if d == "connection_id" {
			d = "webhook_id"
		}
		if !containsValue(allowed, d) {
			return fmt.Errorf("%s %q is not supported by %s; that route groups by: %s",
				dimensionsName, d, route, DimensionList(allowed))
		}
	}
	if containsValue(params.Dimensions, "delivery_group") && params.DestinationID == "" {
		return fmt.Errorf("%s delivery_group requires %s; the API rejects grouping by delivery group without a destination filter",
			dimensionsName, names.DestinationID)
	}
	return nil
}

// TranslateQueueDepthMeasures maps the CLI's and MCP's "queue_depth" spelling
// onto the API's "max_depth", dropping a duplicate if both were requested. The
// queue-depth endpoint accepts max_depth and max_age only.
func TranslateQueueDepthMeasures(measures []string) []string {
	out := make([]string, 0, len(measures))
	seen := make(map[string]bool, len(measures))
	for _, m := range measures {
		if m == "queue_depth" {
			m = "max_depth"
		}
		if seen[m] {
			continue
		}
		seen[m] = true
		out = append(out, m)
	}
	return out
}

// Names of the events-metrics routes, as they appear to the caller in errors.
const (
	EventRouteDefault    = "event metrics"
	EventRouteQueueDepth = "queue depth metrics"
	EventRoutePending    = "pending event metrics"
	EventRouteByIssue    = "per-issue event metrics"
)

// eventMeasureRoutes maps every measure `metrics events` advertises onto the API
// endpoint that measure selects. The by-issue route is chosen by dimension
// rather than by measure, so it has no entry here.
//
// A measure that is absent from this map does not influence routing: the
// request goes to the default endpoint and the API rejects the measure itself,
// which is a better error than one this package could invent.
var eventMeasureRoutes = map[string]string{
	"count":                 EventRouteDefault,
	"successful_count":      EventRouteDefault,
	"failed_count":          EventRouteDefault,
	"scheduled_count":       EventRouteDefault,
	"paused_count":          EventRouteDefault,
	"error_rate":            EventRouteDefault,
	"avg_attempts":          EventRouteDefault,
	"scheduled_retry_count": EventRouteDefault,
	"max_count_per_second":  EventRouteDefault,

	"queue_depth": EventRouteQueueDepth,
	"max_depth":   EventRouteQueueDepth,
	"max_age":     EventRouteQueueDepth,

	"pending": EventRoutePending,
}

// eventDimensionRoutes maps a dimension onto the endpoint it selects. Only
// issue_id selects a route of its own; every other dimension is grouped by
// whichever endpoint the measures choose.
var eventDimensionRoutes = map[string]string{
	"issue_id": EventRouteByIssue,
}

// firstRoutedMeasure returns the first measure that selects an endpoint, and the
// endpoint it selects. ("", "") means the measures do not decide the route — the
// request falls through to whatever the dimensions select, or to the default.
func firstRoutedMeasure(measures []string) (string, string) {
	for _, m := range measures {
		if route, ok := eventMeasureRoutes[m]; ok {
			return m, route
		}
	}
	return "", ""
}

// RejectMixedMeasureRoutes refuses a measure list that spans more than one
// events-metrics endpoint.
//
// Routing picks a single endpoint from the measures, so a mixed list is not a
// combined query: the extra measures are either silently dropped (the pending
// route replaces the whole list with "count") or rewritten into something the
// endpoint rejects with a 422. Neither is what the caller asked for, so say so
// here instead. measuresName is the caller's own spelling of the argument, so a
// CLI user reads "--measures" and an MCP client reads "measures".
func RejectMixedMeasureRoutes(measures []string, measuresName string) error {
	firstMeasure := ""
	firstRoute := ""
	for _, m := range measures {
		route, known := eventMeasureRoutes[m]
		if !known {
			continue
		}
		if firstRoute == "" {
			firstMeasure, firstRoute = m, route
			continue
		}
		if route != firstRoute {
			return fmt.Errorf("%s cannot mix %q (%s) with %q (%s): these are separate API endpoints, so ask for one route's measures at a time",
				measuresName, firstMeasure, firstRoute, m, route)
		}
	}
	return nil
}

// RejectCrossRouteEventQuery refuses an events query whose parts select more
// than one API endpoint.
//
// `metrics events` fans out over four endpoints and calls exactly one of them,
// choosing it from the measures, then the issue_id dimension, then the issue
// filter. First match wins, so a request naming parts of two routes is answered
// from one of them and the rest of the question is dropped without a word: a
// queue-depth measure shadowed the issue_id dimension entirely (#407), and
// "pending" shadows it the same way. One route's numbers returned under another
// route's question are worse than no answer, so name both routes and refuse.
//
// It subsumes RejectMixedMeasureRoutes, which is the same rule applied within
// the measure list. Callers should use this and not both.
//
// measuresName and dimensionsName are the caller's own spellings of the
// arguments, and names supplies the same for the filters, so a CLI user reads
// "--measures" and an MCP client reads "measures".
func RejectCrossRouteEventQuery(params MetricsQueryParams, measuresName, dimensionsName string, names MetricsFilterNames) error {
	if err := RejectMixedMeasureRoutes(params.Measures, measuresName); err != nil {
		return err
	}

	measure, measureRoute := firstRoutedMeasure(params.Measures)
	// The default route is the one every dimension refines rather than
	// contradicts: `--measures count --dimensions issue_id` is a per-issue count,
	// which is exactly what the by-issue endpoint answers.
	if measureRoute == "" || measureRoute == EventRouteDefault {
		return nil
	}

	conflict := func(selector, route string) error {
		return fmt.Errorf("%s %q (%s) cannot be combined with %s (%s): these are separate API endpoints, so ask for one route at a time",
			measuresName, measure, measureRoute, selector, route)
	}

	for _, d := range params.Dimensions {
		route, selects := eventDimensionRoutes[d]
		if selects && route != measureRoute {
			return conflict(fmt.Sprintf("%s %q", dimensionsName, d), route)
		}
	}
	// The filter selects the by-issue route on its own, so it conflicts on its
	// own too — and saying which two routes were asked for is more use than
	// reporting it as a filter the endpoint happens to ignore.
	if params.IssueID != "" && measureRoute != EventRouteByIssue {
		return conflict(names.IssueID, EventRouteByIssue)
	}
	return nil
}
