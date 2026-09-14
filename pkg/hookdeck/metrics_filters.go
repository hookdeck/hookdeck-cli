package hookdeck

import "fmt"

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

// Dimension and status vocabularies per metrics route. These differ sharply
// between endpoints, so --help must not advertise one generic list: naming a
// dimension the route does not accept sends the user into an API 422.
const (
	RequestMetricsDimensions        = "source_id, rejection_cause, status, bulk_retry_ids, events_count, ignored_count"
	AttemptMetricsDimensions        = "destination_id, delivery_group, event_id, status, error_code, bulk_retry_id, trigger"
	TransformationMetricsDimensions = "transformation_id, webhook_id, log_level, issue_id"
	EventMetricsDimensions          = "source_id, destination_id, connection_id, delivery_group, status, issue_id"
)

// Status vocabularies. Request events are accepted or rejected at the edge;
// events and attempts carry a delivery status.
const (
	RequestStatusValues = "ACCEPTED, REJECTED"
	EventStatusValues   = "SCHEDULED, QUEUED, HOLD, SUCCESSFUL, FAILED, CANCELLED"
	AttemptStatusValues = "SUCCESSFUL, FAILED"
)

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

	"queue_depth": EventRouteQueueDepth,
	"max_depth":   EventRouteQueueDepth,
	"max_age":     EventRouteQueueDepth,

	"pending": EventRoutePending,
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
