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
				Description: "List available Hookdeck projects or switch the active project for this session.",
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
				Description: "Manage connections (webhook routes) that link sources to destinations.",
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
				Description: "Manage inbound webhook sources.",
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
				Description: "Manage webhook delivery destinations.",
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
				Description: "Manage JavaScript transformations applied to webhook payloads.",
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
				Description: "Query inbound webhook requests received by Hookdeck.",
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
				Description: "Query events (processed webhook deliveries).",
				InputSchema: schema(map[string]prop{
					"action":          {Type: "string", Desc: "Action: list, get, or raw_body", Enum: []string{"list", "get", "raw_body"}},
					"id":              {Type: "string", Desc: "Event ID (required for get/raw_body)"},
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
				Description: "Query delivery attempts for webhook events.",
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
				Description: "List and inspect Hookdeck issues (delivery failures, transformation errors, etc.).",
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
				Description: "Query metrics for events, requests, attempts, and transformations.",
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
				Description: "Describe available tools and their actions.",
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
