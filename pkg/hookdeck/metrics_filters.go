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
