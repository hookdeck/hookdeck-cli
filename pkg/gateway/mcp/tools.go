package mcp

import (
	"encoding/json"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

// toolDefs lists every tool the MCP server exposes. Each entry pairs a Tool
// definition (with a proper JSON Schema) with a handler that calls the
// Hookdeck API.
func toolDefs(client *hookdeck.Client) []struct {
	tool    *mcpsdk.Tool
	handler mcpsdk.ToolHandler
} {
	return []struct {
		tool    *mcpsdk.Tool
		handler mcpsdk.ToolHandler
	}{
		{
			tool: &mcpsdk.Tool{
				Name:        "hookdeck_projects",
				Description: "Always call this first when the user references a specific project by name. List available projects to find the matching project ID, then use the `use` action to switch to it before calling any other tools. All queries (events, issues, connections, metrics, requests) are scoped to the active project — if the wrong project is active, all results will be wrong. Also use this when unsure which project is currently active.",
				InputSchema: schema(map[string]prop{
					"action":     {Type: "string", Desc: "Action to perform: list or use", Enum: []string{"list", "use"}},
					"project_id": {Type: "string", Desc: "Project ID (required for use action)"},
				}, "action"),
			},
			handler: handleProjects(client),
		},
		{
			tool: &mcpsdk.Tool{
				Name:        "hookdeck_connections",
				Description: "Inspect connections (routes linking sources to destinations). List connections with filters, get details by ID, or pause/unpause a connection's delivery pipeline. Results are scoped to the active project — call `hookdeck_projects` first if the user has specified a project.",
				InputSchema: schema(map[string]prop{
					"action":         {Type: "string", Desc: "Action: list, get, pause, or unpause", Enum: []string{"list", "get", "pause", "unpause"}},
					"id":             {Type: "string", Desc: "Connection ID (required for get/pause/unpause)"},
					"name":           {Type: "string", Desc: "Filter by name (list)"},
					"source_id":      {Type: "string", Desc: "Filter by source ID (list)"},
					"destination_id": {Type: "string", Desc: "Filter by destination ID (list)"},
					"disabled":       {Type: "boolean", Desc: "Filter disabled connections (list)"},
					"limit":          {Type: "integer", Desc: "Max results (list)"},
					"next":           {Type: "string", Desc: "Next page cursor"},
					"prev":           {Type: "string", Desc: "Previous page cursor"},
				}, "action"),
			},
			handler: handleConnections(client),
		},
		{
			tool: &mcpsdk.Tool{
				Name:        "hookdeck_sources",
				Description: "List and inspect inbound sources (HTTP endpoints that receive events). Returns source configuration including URL, verification settings, and allowed HTTP methods.",
				InputSchema: schema(map[string]prop{
					"action": {Type: "string", Desc: "Action: list or get", Enum: []string{"list", "get"}},
					"id":     {Type: "string", Desc: "Source ID (required for get)"},
					"name":   {Type: "string", Desc: "Filter by name (list)"},
					"limit":  {Type: "integer", Desc: "Max results (list)"},
					"next":   {Type: "string", Desc: "Next page cursor"},
					"prev":   {Type: "string", Desc: "Previous page cursor"},
				}, "action"),
			},
			handler: handleSources(client),
		},
		{
			tool: &mcpsdk.Tool{
				Name:        "hookdeck_destinations",
				Description: "List and inspect delivery destinations where events are sent. Destination types include HTTP endpoints, CLI (local development), and MOCK (testing). Returns destination configuration including URL, authentication, and rate limiting settings.",
				InputSchema: schema(map[string]prop{
					"action": {Type: "string", Desc: "Action: list or get", Enum: []string{"list", "get"}},
					"id":     {Type: "string", Desc: "Destination ID (required for get)"},
					"name":   {Type: "string", Desc: "Filter by name (list)"},
					"limit":  {Type: "integer", Desc: "Max results (list)"},
					"next":   {Type: "string", Desc: "Next page cursor"},
					"prev":   {Type: "string", Desc: "Previous page cursor"},
				}, "action"),
			},
			handler: handleDestinations(client),
		},
		{
			tool: &mcpsdk.Tool{
				Name:        "hookdeck_transformations",
				Description: "List and inspect JavaScript transformations applied to event payloads. Returns transformation code and configuration for debugging payload processing.",
				InputSchema: schema(map[string]prop{
					"action": {Type: "string", Desc: "Action: list or get", Enum: []string{"list", "get"}},
					"id":     {Type: "string", Desc: "Transformation ID (required for get)"},
					"name":   {Type: "string", Desc: "Filter by name (list)"},
					"limit":  {Type: "integer", Desc: "Max results (list)"},
					"next":   {Type: "string", Desc: "Next page cursor"},
					"prev":   {Type: "string", Desc: "Previous page cursor"},
				}, "action"),
			},
			handler: handleTransformations(client),
		},
		{
			tool: &mcpsdk.Tool{
				Name:        "hookdeck_requests",
				Description: "Query inbound requests (raw HTTP data received by Hookdeck before routing). List with filters, get details, inspect the raw body, or view the events and ignored events generated from a request. Results are scoped to the active project — call `hookdeck_projects` first if the user has specified a project.",
				InputSchema: schema(map[string]prop{
					"action":         {Type: "string", Desc: "Action: list, get, raw_body, events, or ignored_events", Enum: []string{"list", "get", "raw_body", "events", "ignored_events"}},
					"id":             {Type: "string", Desc: "Request ID (required for get/raw_body/events/ignored_events)"},
					"source_id":      {Type: "string", Desc: "Filter by source (list)"},
					"status":         {Type: "string", Desc: "Filter by status (list)"},
					"rejection_cause": {Type: "string", Desc: "Filter by rejection cause (list)"},
					"verified":       {Type: "boolean", Desc: "Filter by verification status (list)"},
					"limit":          {Type: "integer", Desc: "Max results (list)"},
					"next":           {Type: "string", Desc: "Next page cursor"},
					"prev":           {Type: "string", Desc: "Previous page cursor"},
				}, "action"),
			},
			handler: handleRequests(client),
		},
		{
			tool: &mcpsdk.Tool{
				Name:        "hookdeck_events",
				Description: "Query events (processed deliveries routed through connections to destinations). List with filters by status, source, destination, or date range. Get event details (get) or the event payload (raw_body). Use action raw_body with the event id to get the payload directly — do not use hookdeck_requests for the payload when you already have an event id. Results are scoped to the active project — call `hookdeck_projects` first if the user has specified a project.",
				InputSchema: schema(map[string]prop{
					"action":          {Type: "string", Desc: "Action: list, get, or raw_body. Use raw_body to get the event payload (body); get returns metadata and headers only.", Enum: []string{"list", "get", "raw_body"}},
					"id":              {Type: "string", Desc: "Event ID (required for get/raw_body). Use with raw_body to fetch the event payload without querying the request."},
					"connection_id":   {Type: "string", Desc: "Filter by connection (list, maps to webhook_id)"},
					"source_id":       {Type: "string", Desc: "Filter by source (list)"},
					"destination_id":  {Type: "string", Desc: "Filter by destination (list)"},
					"status":          {Type: "string", Desc: "Event status: SCHEDULED, QUEUED, HOLD, SUCCESSFUL, FAILED, CANCELLED"},
					"issue_id":        {Type: "string", Desc: "Filter by issue (list)"},
					"error_code":      {Type: "string", Desc: "Filter by error code (list)"},
					"response_status": {Type: "string", Desc: "Filter by HTTP response status (list)"},
					"created_after":   {Type: "string", Desc: "ISO datetime lower bound (list)"},
					"created_before":  {Type: "string", Desc: "ISO datetime upper bound (list)"},
					"limit":           {Type: "integer", Desc: "Max results (list)"},
					"order_by":        {Type: "string", Desc: "Sort field (list)"},
					"dir":             {Type: "string", Desc: "Sort direction: asc or desc (list)"},
					"next":            {Type: "string", Desc: "Next page cursor"},
					"prev":            {Type: "string", Desc: "Previous page cursor"},
				}, "action"),
			},
			handler: handleEvents(client),
		},
		{
			tool: &mcpsdk.Tool{
				Name:        "hookdeck_attempts",
				Description: "Query delivery attempts (each HTTP request made to deliver an event to its destination). Filter by event to see retry history, response status codes, and error details.",
				InputSchema: schema(map[string]prop{
					"action":   {Type: "string", Desc: "Action: list or get", Enum: []string{"list", "get"}},
					"id":       {Type: "string", Desc: "Attempt ID (required for get)"},
					"event_id": {Type: "string", Desc: "Filter by event (list)"},
					"limit":    {Type: "integer", Desc: "Max results (list)"},
					"order_by": {Type: "string", Desc: "Sort field (list)"},
					"dir":      {Type: "string", Desc: "Sort direction: asc or desc (list)"},
					"next":     {Type: "string", Desc: "Next page cursor"},
					"prev":     {Type: "string", Desc: "Previous page cursor"},
				}, "action"),
			},
			handler: handleAttempts(client),
		},
		{
			tool: &mcpsdk.Tool{
				Name:        "hookdeck_issues",
				Description: "List and inspect Hookdeck issues — aggregated failure signals such as repeated delivery failures, transformation errors, and backpressure alerts. Use this to identify systemic problems across your event pipeline. Results are scoped to the active project — call `hookdeck_projects` first if the user has specified a project.",
				InputSchema: schema(map[string]prop{
					"action":           {Type: "string", Desc: "Action: list or get", Enum: []string{"list", "get"}},
					"id":               {Type: "string", Desc: "Issue ID (required for get)"},
					"type":             {Type: "string", Desc: "Filter: delivery, transformation, or backpressure (list)"},
					"filter_status":    {Type: "string", Desc: "Filter by status (list)"},
					"issue_trigger_id": {Type: "string", Desc: "Filter by trigger (list)"},
					"order_by":         {Type: "string", Desc: "Sort field (list)"},
					"dir":              {Type: "string", Desc: "Sort direction: asc or desc (list)"},
					"limit":            {Type: "integer", Desc: "Max results (list)"},
					"next":             {Type: "string", Desc: "Next page cursor"},
					"prev":             {Type: "string", Desc: "Previous page cursor"},
				}, "action"),
			},
			handler: handleIssues(client),
		},
		{
			tool: &mcpsdk.Tool{
				Name:        "hookdeck_metrics",
				Description: "Query aggregate metrics over a time range. Get counts, failure rates, error rates, queue depth, and pending event data for events, requests, attempts, and transformations. Supports grouping by dimensions like source, destination, or connection. Results are scoped to the active project — call `hookdeck_projects` first if the user has specified a project.",
				InputSchema: schema(map[string]prop{
					"action":         {Type: "string", Desc: "Metric type: events, requests, attempts, or transformations", Enum: []string{"events", "requests", "attempts", "transformations"}},
					"start":          {Type: "string", Desc: "Start datetime (ISO 8601, required)"},
					"end":            {Type: "string", Desc: "End datetime (ISO 8601, required)"},
					"granularity":    {Type: "string", Desc: "Time bucket size, e.g. 1h, 5m, 1d"},
					"measures":       {Type: "array", Desc: "Metrics to retrieve (required). Common: count, successful_count, failed_count, error_count", Items: &prop{Type: "string"}},
					"dimensions":     {Type: "array", Desc: "Grouping dimensions", Items: &prop{Type: "string"}},
					"source_id":      {Type: "string", Desc: "Filter by source"},
					"destination_id": {Type: "string", Desc: "Filter by destination"},
					"connection_id":  {Type: "string", Desc: "Filter by connection (maps to webhook_id)"},
					"status":         {Type: "string", Desc: "Filter by status"},
					"issue_id":       {Type: "string", Desc: "Filter by issue (events only)"},
				}, "action", "start", "end", "measures"),
			},
			handler: handleMetrics(client),
		},
		{
			tool: &mcpsdk.Tool{
				Name:        "hookdeck_help",
				Description: "Get an overview of all available Hookdeck tools or detailed help for a specific tool. Use this when unsure which tool to use for a task. Note: all tools operate on the active project — use `hookdeck_projects` to verify or switch project context before querying.",
				InputSchema: schema(map[string]prop{
					"topic": {Type: "string", Desc: "Tool name for detailed help (e.g. hookdeck_events). Omit for overview."},
				}),
			},
			handler: handleHelp(client),
		},
	}
}

// prop describes a single JSON Schema property.
type prop struct {
	Type  string   `json:"type"`
	Desc  string   `json:"description,omitempty"`
	Enum  []string `json:"enum,omitempty"`
	Items *prop    `json:"items,omitempty"`
}

// schema builds a JSON Schema object with the given properties and required fields.
func schema(properties map[string]prop, required ...string) json.RawMessage {
	s := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		s["required"] = required
	}
	data, _ := json.Marshal(s)
	return data
}
