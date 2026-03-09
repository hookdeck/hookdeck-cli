package mcp

import (
	"context"
	"fmt"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

func handleHelp(client *hookdeck.Client) mcpsdk.ToolHandler {
	return func(_ context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		in, err := parseInput(req.Params.Arguments)
		if err != nil {
			return ErrorResult(err.Error()), nil
		}

		topic := in.String("topic")
		if topic == "" {
			return helpOverview(client), nil
		}
		return helpTopic(topic), nil
	}
}

func helpOverview(client *hookdeck.Client) *mcpsdk.CallToolResult {
	projectInfo := "not set"
	if client.ProjectID != "" {
		projectInfo = client.ProjectID
	}

	text := fmt.Sprintf(`Hookdeck MCP Server — Available Tools

Current project: %s

hookdeck_projects        — List or switch projects (actions: list, use)
hookdeck_connections     — Inspect connections and control delivery flow (actions: list, get, pause, unpause)
hookdeck_sources         — Inspect inbound sources (actions: list, get)
hookdeck_destinations    — Inspect delivery destinations (actions: list, get)
hookdeck_transformations — Inspect JavaScript transformations (actions: list, get)
hookdeck_requests        — Query inbound requests (actions: list, get, raw_body, events, ignored_events)
hookdeck_events          — Query processed events (actions: list, get, raw_body)
hookdeck_attempts        — Query delivery attempts (actions: list, get)
hookdeck_issues          — Inspect aggregated failure signals (actions: list, get)
hookdeck_metrics         — Query aggregate metrics (actions: events, requests, attempts, transformations)
hookdeck_help            — This help text

Use hookdeck_help with topic="<tool_name>" for detailed help on a specific tool.`, projectInfo)

	return TextResult(text)
}

var toolHelp = map[string]string{
	"hookdeck_projects": `hookdeck_projects — List or switch the active project

Actions:
  list  — List all projects. Returns id, name, mode, and which is current.
  use   — Switch the active project for this session (in-memory only).

Parameters:
  action      (string, required) — "list" or "use"
  project_id  (string)           — Required for "use" action`,

	"hookdeck_connections": `hookdeck_connections — Inspect connections and control delivery flow

Actions:
  list    — List connections with optional filters
  get     — Get a single connection by ID
  pause   — Pause a connection (stops event delivery)
  unpause — Resume a paused connection

Parameters:
  action         (string, required) — list, get, pause, or unpause
  id             (string)           — Required for get/pause/unpause
  name           (string)           — Filter by name (list)
  source_id      (string)           — Filter by source (list)
  destination_id (string)           — Filter by destination (list)
  disabled       (boolean)          — Filter disabled connections (list)
  limit          (integer)          — Max results (list, default 100)
  next/prev      (string)           — Pagination cursors (list)`,

	"hookdeck_sources": `hookdeck_sources — Inspect inbound sources

Actions:
  list — List sources with optional filters
  get  — Get a single source by ID

Parameters:
  action  (string, required) — list or get
  id      (string)           — Required for get
  name    (string)           — Filter by name (list)
  limit   (integer)          — Max results (list, default 100)
  next/prev (string)         — Pagination cursors (list)`,

	"hookdeck_destinations": `hookdeck_destinations — Inspect delivery destinations

Actions:
  list — List destinations with optional filters
  get  — Get a single destination by ID

Parameters:
  action  (string, required) — list or get
  id      (string)           — Required for get
  name    (string)           — Filter by name (list)
  limit   (integer)          — Max results (list, default 100)
  next/prev (string)         — Pagination cursors (list)`,

	"hookdeck_transformations": `hookdeck_transformations — Inspect JavaScript transformations

Actions:
  list — List transformations with optional filters
  get  — Get a single transformation by ID

Parameters:
  action  (string, required) — list or get
  id      (string)           — Required for get
  name    (string)           — Filter by name (list)
  limit   (integer)          — Max results (list, default 100)
  next/prev (string)         — Pagination cursors (list)`,

	"hookdeck_requests": `hookdeck_requests — Query inbound requests

Actions:
  list           — List requests with optional filters
  get            — Get a single request by ID
  raw_body       — Get the raw body of a request
  events         — List events generated from a request
  ignored_events — List ignored events for a request

Parameters:
  action          (string, required) — list, get, raw_body, events, or ignored_events
  id              (string)           — Required for get/raw_body/events/ignored_events
  source_id       (string)           — Filter by source (list)
  status          (string)           — Filter by status (list)
  rejection_cause (string)           — Filter by rejection cause (list)
  verified        (boolean)          — Filter by verification status (list)
  limit           (integer)          — Max results (list, default 100)
  next/prev       (string)           — Pagination cursors (list)`,

	"hookdeck_events": `hookdeck_events — Query events (processed deliveries)

Actions:
  list     — List events with optional filters
  get      — Get a single event by ID
  raw_body — Get the raw body of an event

Parameters:
  action          (string, required) — list, get, or raw_body
  id              (string)           — Required for get/raw_body
  connection_id   (string)           — Filter by connection (list, maps to webhook_id)
  source_id       (string)           — Filter by source (list)
  destination_id  (string)           — Filter by destination (list)
  status          (string)           — SCHEDULED, QUEUED, HOLD, SUCCESSFUL, FAILED, CANCELLED
  issue_id        (string)           — Filter by issue (list)
  error_code      (string)           — Filter by error code (list)
  response_status (string)           — Filter by HTTP response status (list)
  created_after   (string)           — ISO datetime, lower bound (list)
  created_before  (string)           — ISO datetime, upper bound (list)
  limit           (integer)          — Max results (list, default 100)
  order_by        (string)           — Sort field (list)
  dir             (string)           — "asc" or "desc" (list)
  next/prev       (string)           — Pagination cursors (list)`,

	"hookdeck_attempts": `hookdeck_attempts — Query delivery attempts

Actions:
  list — List attempts (typically filtered by event_id)
  get  — Get a single attempt by ID

Parameters:
  action    (string, required) — list or get
  id        (string)           — Required for get
  event_id  (string)           — Filter by event (list)
  limit     (integer)          — Max results (list, default 100)
  order_by  (string)           — Sort field (list)
  dir       (string)           — "asc" or "desc" (list)
  next/prev (string)           — Pagination cursors (list)`,

	"hookdeck_issues": `hookdeck_issues — Inspect aggregated failure signals

Actions:
  list — List issues with optional filters
  get  — Get a single issue by ID

Parameters:
  action           (string, required) — list or get
  id               (string)           — Required for get
  type             (string)           — Filter: delivery, transformation, backpressure (list)
  filter_status    (string)           — Filter by status (list)
  issue_trigger_id (string)           — Filter by trigger (list)
  order_by         (string)           — Sort: created_at, first_seen_at, last_seen_at, opened_at, status (list)
  dir              (string)           — "asc" or "desc" (list)
  limit            (integer)          — Max results (list, default 100)
  next/prev        (string)           — Pagination cursors (list)`,

	"hookdeck_metrics": `hookdeck_metrics — Query aggregate metrics

Actions:
  events          — Event metrics (auto-routes to queue-depth, pending, or by-issue as needed)
  requests        — Request metrics
  attempts        — Attempt metrics
  transformations — Transformation metrics

Parameters:
  action         (string, required)   — events, requests, attempts, or transformations
  start          (string, required)   — ISO 8601 datetime
  end            (string, required)   — ISO 8601 datetime
  granularity    (string)             — e.g. "1h", "5m", "1d"
  measures       (string[], required)  — Metrics to retrieve. Common: count, successful_count, failed_count, error_count
  dimensions     (string[])           — Grouping dimensions (varies by action)
  source_id      (string)             — Filter by source
  destination_id (string)             — Filter by destination
  connection_id  (string)             — Filter by connection (maps to webhook_id)
  status         (string)             — Filter by status
  issue_id       (string)             — Filter by issue (events only)`,

	"hookdeck_help": `hookdeck_help — Get an overview of available tools or detailed help for a specific tool

Parameters:
  topic  (string) — Tool name for detailed help (e.g. "hookdeck_events"). Omit for overview.`,
}

func helpTopic(topic string) *mcpsdk.CallToolResult {
	// Allow both "hookdeck_events" and "events" forms
	if !strings.HasPrefix(topic, "hookdeck_") {
		topic = "hookdeck_" + topic
	}
	text, ok := toolHelp[topic]
	if ok {
		return TextResult(text)
	}

	// If the topic doesn't match a tool name exactly, it may be a natural
	// language question. List all available tools so the caller can pick.
	var names []string
	for k := range toolHelp {
		names = append(names, k)
	}
	return ErrorResult(fmt.Sprintf(
		"No help found for %q. The topic parameter expects a tool name, not a question.\n\nAvailable tools: %s\n\nOmit the topic parameter for a general overview.",
		topic, strings.Join(names, ", "),
	))
}
