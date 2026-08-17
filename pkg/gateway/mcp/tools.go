package mcp

import (
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/config"
	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
	"github.com/hookdeck/hookdeck-cli/pkg/mcpcore"
)

// Tool names. The gateway server namespaces its tools with "hookdeck_".
const (
	toolPrefix       = "hookdeck"
	loginToolName    = toolPrefix + "_login"
	helpToolName     = toolPrefix + "_help"
	helpTopicPrefix  = toolPrefix + "_"
	loginToolDesc    = "Authenticate the Hookdeck CLI or sign in again. Without arguments, returns a URL for browser login when not yet authenticated, or confirms if already signed in. Set reauth: true to clear the current session and start a new browser login (use when hookdeck_projects list fails and the stored key may be a single-project or dashboard API key)."
	projectsToolDesc = "Always call this first when the user references a specific project by name. List available projects to find the matching project ID, then use the `use` action to switch to it before calling any other tools. All queries (events, issues, connections, metrics, requests) are scoped to the active project — if the wrong project is active, all results will be wrong. Also use this when unsure which project is currently active. If list or use fails (especially 401/403), the error may suggest hookdeck_login with reauth: true. JSON successes use a standard data/meta envelope; see hookdeck_help (overview or any tool topic)."
)

// NewServer creates an MCP server exposing the Event Gateway tools.
//
// The supplied client is shared across all tool handlers; changing its
// ProjectID (e.g. via the projects tool's use action) affects subsequent calls
// within the same session.
//
// hookdeck_login is always registered: it signs in when unauthenticated, or
// with reauth: true clears stored credentials and starts a fresh browser login.
func NewServer(client *hookdeck.Client, cfg *config.Config) *mcpcore.Server {
	return mcpcore.NewServer(mcpcore.Options{
		Name:       "hookdeck-gateway",
		ToolPrefix: toolPrefix,
		Client:     client,
		Config:     cfg,
		ToolDefs:   toolDefs,
	})
}

// toolDefs lists every tool the MCP server exposes. Each entry pairs a Tool
// definition (with a proper JSON Schema) with a handler that calls the
// Hookdeck API.
func toolDefs(srv *mcpcore.Server) []mcpcore.ToolDef {
	client := srv.Client()
	return []mcpcore.ToolDef{
		srv.ProjectsToolDef(projectsToolDesc),
		{
			Tool: &mcpsdk.Tool{
				Name:        "hookdeck_connections",
				Description: "Inspect connections (routes linking sources to destinations). List connections with filters, get details by ID or name, or pause/unpause a connection's delivery pipeline. Results are scoped to the active project — call `hookdeck_projects` first if the user has specified a project.",
				InputSchema: mcpcore.Schema(map[string]mcpcore.Prop{
					"action":         {Type: "string", Desc: "Action: list, get, pause, or unpause", Enum: []string{"list", "get", "pause", "unpause"}},
					"id":             {Type: "string", Desc: "Connection ID or name (required for get/pause/unpause)"},
					"name":           {Type: "string", Desc: "Filter by name (list)"},
					"source_id":      {Type: "string", Desc: "Filter by source ID (list)"},
					"destination_id": {Type: "string", Desc: "Filter by destination ID (list)"},
					"disabled":       {Type: "boolean", Desc: "Filter disabled connections (list)"},
					"limit":          {Type: "integer", Desc: "Max results (list)"},
					"next":           {Type: "string", Desc: "Next page cursor"},
					"prev":           {Type: "string", Desc: "Previous page cursor"},
				}, "action"),
			},
			Handler: handleConnections(client),
		},
		{
			Tool: &mcpsdk.Tool{
				Name:        "hookdeck_sources",
				Description: "List and inspect inbound sources (HTTP endpoints that receive events). Returns source configuration including URL, verification settings, and allowed HTTP methods.",
				InputSchema: mcpcore.Schema(map[string]mcpcore.Prop{
					"action": {Type: "string", Desc: "Action: list or get", Enum: []string{"list", "get"}},
					"id":     {Type: "string", Desc: "Source ID (required for get)"},
					"name":   {Type: "string", Desc: "Filter by name (list)"},
					"limit":  {Type: "integer", Desc: "Max results (list)"},
					"next":   {Type: "string", Desc: "Next page cursor"},
					"prev":   {Type: "string", Desc: "Previous page cursor"},
				}, "action"),
			},
			Handler: handleSources(client),
		},
		{
			Tool: &mcpsdk.Tool{
				Name:        "hookdeck_destinations",
				Description: "List and inspect delivery destinations where events are sent. Destination types include HTTP endpoints, CLI (local development), and MOCK (testing). Returns destination configuration including URL, authentication, and rate limiting settings.",
				InputSchema: mcpcore.Schema(map[string]mcpcore.Prop{
					"action": {Type: "string", Desc: "Action: list or get", Enum: []string{"list", "get"}},
					"id":     {Type: "string", Desc: "Destination ID (required for get)"},
					"name":   {Type: "string", Desc: "Filter by name (list)"},
					"limit":  {Type: "integer", Desc: "Max results (list)"},
					"next":   {Type: "string", Desc: "Next page cursor"},
					"prev":   {Type: "string", Desc: "Previous page cursor"},
				}, "action"),
			},
			Handler: handleDestinations(client),
		},
		{
			Tool: &mcpsdk.Tool{
				Name:        "hookdeck_transformations",
				Description: "List and inspect JavaScript transformations applied to event payloads. Returns transformation code and configuration for debugging payload processing.",
				InputSchema: mcpcore.Schema(map[string]mcpcore.Prop{
					"action": {Type: "string", Desc: "Action: list or get", Enum: []string{"list", "get"}},
					"id":     {Type: "string", Desc: "Transformation ID (required for get)"},
					"name":   {Type: "string", Desc: "Filter by name (list)"},
					"limit":  {Type: "integer", Desc: "Max results (list)"},
					"next":   {Type: "string", Desc: "Next page cursor"},
					"prev":   {Type: "string", Desc: "Previous page cursor"},
				}, "action"),
			},
			Handler: handleTransformations(client),
		},
		{
			Tool: &mcpsdk.Tool{
				Name:        "hookdeck_requests",
				Description: "Query inbound requests (raw HTTP data received by Hookdeck before routing). List supports the same filters as `hookdeck gateway request list` (metadata, date range, payload search, sort). Get details, inspect raw body, or view events and ignored events from a request. Results are scoped to the active project — call `hookdeck_projects` first if the user has specified a project.",
				InputSchema: mcpcore.Schema(map[string]mcpcore.Prop{
					"action":          {Type: "string", Desc: "Action: list, get, raw_body, events, or ignored_events", Enum: []string{"list", "get", "raw_body", "events", "ignored_events"}},
					"id":              {Type: "string", Desc: "Request ID: filter by ID(s) on list (comma-separated), or required for get/raw_body/events/ignored_events"},
					"source_id":       {Type: "string", Desc: "Filter by source (list)"},
					"status":          {Type: "string", Desc: "Filter by status: accepted or rejected (list)"},
					"rejection_cause": {Type: "string", Desc: "Filter by rejection cause (list)"},
					"verified":        {Type: "boolean", Desc: "Filter by verification status (list)"},
					"created_after":   {Type: "string", Desc: "created_at lower bound. " + descDateAfter},
					"created_before":  {Type: "string", Desc: "created_at upper bound. " + descDateBefore},
					"ingested_after":  {Type: "string", Desc: "ingested_at lower bound. " + descDateAfter},
					"ingested_before": {Type: "string", Desc: "ingested_at upper bound. " + descDateBefore},
					"body":            {Type: "string", Desc: "Filter by request body. " + descJSONFilter},
					"headers":         {Type: "string", Desc: "Filter by request headers. " + descJSONFilter},
					"parsed_query":    {Type: "string", Desc: "Filter by parsed query string as JSON. " + descJSONFilter},
					"path":            {Type: "string", Desc: descPathFilter},
					"order_by":        {Type: "string", Desc: "Sort field (list), e.g. created_at"},
					"dir":             {Type: "string", Desc: "Sort direction: asc or desc (list)"},
					"limit":           {Type: "integer", Desc: "Max results (list)"},
					"next":            {Type: "string", Desc: "Next page cursor"},
					"prev":            {Type: "string", Desc: "Previous page cursor"},
				}, "action"),
			},
			Handler: handleRequests(client),
		},
		{
			Tool: &mcpsdk.Tool{
				Name:        "hookdeck_events",
				Description: "Query events (processed deliveries routed through connections to destinations). List supports the same filters as `hookdeck gateway event list` (metadata, date range, payload search, sort). Get event details (get) or the event payload (raw_body). Use action raw_body with the event id to get the payload directly — do not use hookdeck_requests for the payload when you already have an event id. Results are scoped to the active project — call `hookdeck_projects` first if the user has specified a project.",
				InputSchema: mcpcore.Schema(map[string]mcpcore.Prop{
					"action":              {Type: "string", Desc: "Action: list, get, or raw_body. Use raw_body to get the event payload (body); get returns metadata and headers only.", Enum: []string{"list", "get", "raw_body"}},
					"id":                  {Type: "string", Desc: "Event ID: filter by ID(s) on list (comma-separated), or required for get/raw_body"},
					"connection_id":       {Type: "string", Desc: "Filter by connection (list, maps to webhook_id)"},
					"source_id":           {Type: "string", Desc: "Filter by source (list)"},
					"destination_id":      {Type: "string", Desc: "Filter by destination (list)"},
					"status":              {Type: "string", Desc: "Event status: SCHEDULED, QUEUED, HOLD, SUCCESSFUL, FAILED, CANCELLED"},
					"attempts":            {Type: "string", Desc: "Filter by attempt count (list). Integer or API operator syntax; pass through as string."},
					"issue_id":            {Type: "string", Desc: "Filter by issue (list)"},
					"error_code":          {Type: "string", Desc: "Filter by error code (list)"},
					"response_status":     {Type: "string", Desc: "Filter by HTTP response status (list)"},
					"cli_id":              {Type: "string", Desc: "Filter by CLI listen session ID (list)"},
					"created_after":       {Type: "string", Desc: "created_at lower bound. " + descDateAfter},
					"created_before":      {Type: "string", Desc: "created_at upper bound. " + descDateBefore},
					"successful_after":    {Type: "string", Desc: "successful_at lower bound. " + descDateAfter},
					"successful_before":   {Type: "string", Desc: "successful_at upper bound. " + descDateBefore},
					"last_attempt_after":  {Type: "string", Desc: "last_attempt_at lower bound. " + descDateAfter},
					"last_attempt_before": {Type: "string", Desc: "last_attempt_at upper bound. " + descDateBefore},
					"body":                {Type: "string", Desc: "Filter by event payload body. " + descJSONFilter},
					"headers":             {Type: "string", Desc: "Filter by event headers. " + descJSONFilter},
					"parsed_query":        {Type: "string", Desc: "Filter by parsed query as JSON. " + descJSONFilter},
					"path":                {Type: "string", Desc: descPathFilter},
					"limit":               {Type: "integer", Desc: "Max results (list)"},
					"order_by":            {Type: "string", Desc: "Sort field (list)"},
					"dir":                 {Type: "string", Desc: "Sort direction: asc or desc (list)"},
					"next":                {Type: "string", Desc: "Next page cursor"},
					"prev":                {Type: "string", Desc: "Previous page cursor"},
				}, "action"),
			},
			Handler: handleEvents(client),
		},
		{
			Tool: &mcpsdk.Tool{
				Name:        "hookdeck_attempts",
				Description: "Query delivery attempts (each HTTP request made to deliver an event to its destination). Filter by event to see retry history, response status codes, and error details.",
				InputSchema: mcpcore.Schema(map[string]mcpcore.Prop{
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
			Handler: handleAttempts(client),
		},
		{
			Tool: &mcpsdk.Tool{
				Name:        "hookdeck_issues",
				Description: "List and inspect Hookdeck issues — aggregated failure signals such as repeated delivery failures, transformation errors, and backpressure alerts. Use this to identify systemic problems across your event pipeline. Results are scoped to the active project — call `hookdeck_projects` first if the user has specified a project.",
				InputSchema: mcpcore.Schema(map[string]mcpcore.Prop{
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
			Handler: handleIssues(client),
		},
		{
			Tool: &mcpsdk.Tool{
				Name:        "hookdeck_metrics",
				Description: "Query aggregate metrics over a time range. Get counts, failure rates, error rates, queue depth, and pending event data for events, requests, attempts, and transformations. Supports grouping by dimensions like source, destination, or connection. Results are scoped to the active project — call `hookdeck_projects` first if the user has specified a project.",
				InputSchema: mcpcore.Schema(map[string]mcpcore.Prop{
					"action":         {Type: "string", Desc: "Metric type: events, requests, attempts, or transformations", Enum: []string{"events", "requests", "attempts", "transformations"}},
					"start":          {Type: "string", Desc: "Start datetime (ISO 8601, required)"},
					"end":            {Type: "string", Desc: "End datetime (ISO 8601, required)"},
					"granularity":    {Type: "string", Desc: "Time bucket size, e.g. 1h, 5m, 1d"},
					"measures":       {Type: "array", Desc: "Metrics to retrieve (required). Common: count, successful_count, failed_count, error_count", Items: &mcpcore.Prop{Type: "string"}},
					"dimensions":     {Type: "array", Desc: "Grouping dimensions", Items: &mcpcore.Prop{Type: "string"}},
					"source_id":      {Type: "string", Desc: "Filter by source"},
					"destination_id": {Type: "string", Desc: "Filter by destination"},
					"connection_id":  {Type: "string", Desc: "Filter by connection (maps to webhook_id)"},
					"status":         {Type: "string", Desc: "Filter by status"},
					"issue_id":       {Type: "string", Desc: "Filter by issue (events only)"},
				}, "action", "start", "end", "measures"),
			},
			Handler: handleMetrics(client),
		},
		{
			Tool: &mcpsdk.Tool{
				Name:        helpToolName,
				Description: "Get an overview of all available Hookdeck tools or detailed help for a specific tool. Use this when unsure which tool to use for a task. The overview and each tool topic document the common JSON response shape (data + meta). Note: all tools operate on the active project — use `hookdeck_projects` to verify or switch project context before querying.",
				InputSchema: mcpcore.Schema(map[string]mcpcore.Prop{
					"topic": {Type: "string", Desc: "Tool name for detailed help (e.g. hookdeck_events). Omit for overview."},
				}),
			},
			Handler: handleHelp(client),
		},
		srv.LoginToolDef(loginToolDesc),
	}
}

const (
	descDateAfter  = "ISO 8601 datetime lower bound (list). Maps to API field[gte]; do not pass bracket keys in MCP args. Combinable with the matching *_before param."
	descDateBefore = "ISO 8601 datetime upper bound (list). Maps to API field[lte]; do not pass bracket keys in MCP args."
	descJSONFilter = "Hookdeck JSON filter (object or string). Same syntax as hookdeck listen --filter-body."
	descPathFilter = "Partial URL path match (string)."
)
