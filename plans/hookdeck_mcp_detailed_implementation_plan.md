# Hookdeck MCP Server — Detailed Implementation Plan

## Overview

This document maps the high-level MCP build-out plan against the existing hookdeck-cli codebase and provides every implementation detail an engineer needs to build Phase 1 without ambiguity.

**Package location:** `pkg/gateway/mcp`
**Command:** `hookdeck gateway mcp`
**Go MCP SDK:** `github.com/modelcontextprotocol/go-sdk` v1.2.0+
**Transport:** stdio only (Phase 1)
**Auth model:** Inherited from CLI via `Config.GetAPIClient()`

---

## Section 1: Fleshed-Out Implementation Plan

### 1.1 MCP Server Skeleton

#### 1.1.1 Command Registration

The `hookdeck gateway mcp` command must be registered as a subcommand of the existing `gateway` command.

**File to modify:** `pkg/cmd/gateway.go`

Currently, `newGatewayCmd()` (line 13) creates the gateway command and registers subcommands via `addConnectionCmdTo`, `addSourceCmdTo`, etc. Add a new registration call:

```go
addMCPCmdTo(g.cmd)
```

**New file:** `pkg/cmd/mcp.go`

Create a new Cobra command struct following the existing pattern:

```go
type mcpCmd struct {
    cmd *cobra.Command
}

func newMCPCmd() *mcpCmd {
    mc := &mcpCmd{}
    mc.cmd = &cobra.Command{
        Use:   "mcp",
        Args:  validators.NoArgs,
        Short: "Start an MCP server for AI agent access to Hookdeck",
        Long:  `Starts a Model Context Protocol (stdio) server...`,
        RunE:  mc.runMCPCmd,
    }
    return mc
}

func addMCPCmdTo(parent *cobra.Command) {
    parent.AddCommand(newMCPCmd().cmd)
}
```

The `runMCPCmd` method should:
1. Validate the API key via `Config.Profile.ValidateAPIKey()` (pattern used by every command, e.g., `pkg/cmd/event_list.go:93`)
2. Get the API client via `Config.GetAPIClient()` (see `pkg/config/apiclient.go:14`)
3. Initialize the MCP server using `github.com/modelcontextprotocol/go-sdk`
4. Register all 11 tools
5. Start the stdio transport and block until the process is terminated

#### 1.1.2 API Client Wiring

`Config.GetAPIClient()` (`pkg/config/apiclient.go:14-30`) returns a singleton `*hookdeck.Client` with:
- `BaseURL` from `Config.APIBaseURL`
- `APIKey` from `Config.Profile.APIKey`
- `ProjectID` from `Config.Profile.ProjectId`
- `Verbose` enabled when log level is debug

This client is already used by every command. The MCP server should receive this client at initialization and pass it to all tool handlers.

**Important:** The client stores `ProjectID` at construction time. When the `projects.use` action changes the active project, the `Client.ProjectID` field must be mutated in place. Since the same `*hookdeck.Client` pointer is shared by all tool handlers, setting `client.ProjectID = newID` is sufficient to change the project context for all subsequent API calls within the same MCP session.

#### 1.1.3 MCP Server Initialization

Using `github.com/modelcontextprotocol/go-sdk`, create:

```go
// pkg/gateway/mcp/server.go
package mcp

import (
    "github.com/modelcontextprotocol/go-sdk/mcp"
    "github.com/modelcontextprotocol/go-sdk/server"
    "github.com/hookdeck/hookdeck-cli/pkg/hookdeck"
)

type Server struct {
    client    *hookdeck.Client
    mcpServer *server.MCPServer
}

func NewServer(client *hookdeck.Client) *Server {
    s := &Server{client: client}
    s.mcpServer = server.NewMCPServer(
        "hookdeck-gateway",
        "1.0.0",
        server.WithToolCapabilities(true),
    )
    s.registerTools()
    return s
}
```

#### 1.1.4 Stdio Transport

```go
func (s *Server) RunStdio() error {
    transport := server.NewStdioTransport()
    return transport.Run(s.mcpServer)
}
```

The `runMCPCmd` method in `pkg/cmd/mcp.go` calls `server.RunStdio()` and returns its error. This blocks until stdin is closed.

---

### 1.2 Tool Implementations

Each tool is described below with exact source file references, API client methods, request/response types, and MCP input/output schema.

#### Common Patterns

**Pagination parameters** (used by all `list` actions):

All list methods accept `params map[string]string`. The pagination parameters are:
- `limit` (int, default 100) — number of items per page
- `order_by` (string) — field to sort by
- `dir` (string) — "asc" or "desc"
- `next` (string) — opaque cursor for next page
- `prev` (string) — opaque cursor for previous page

All list responses include `Pagination PaginationResponse` with `OrderBy`, `Dir`, `Limit`, `Next *string`, `Prev *string`. Defined in `pkg/hookdeck/connections.go:53-59`.

**MCP tool output format:** All tools should return JSON. For list actions, return `{"models": [...], "pagination": {...}}`. For get/create/update actions, return the resource JSON. This matches the `marshalListResponseWithPagination` pattern in `pkg/cmd/pagination_output.go:33-39`.

**Error translation:** The API client returns `*hookdeck.APIError` (defined in `pkg/hookdeck/client.go:72-85`) with `StatusCode` and `Message`. The MCP layer should translate these into actionable error messages:
- 401 → "Authentication failed. Check your API key."
- 404 → "Resource not found: {id}"
- 422 → Pass through the API message (validation error)
- 429 → "Rate limited. Retry after a brief pause."
- 5xx → "Hookdeck API error: {message}"

Use `errors.As(err, &apiErr)` to extract `*hookdeck.APIError` (pattern in `pkg/hookdeck/client.go:88-91`).

---

#### 1.2.1 Tool: `projects`

**Actions:** `list`, `use`

**Existing CLI implementations:**
- `pkg/cmd/project_list.go` — `runProjectListCmd`
- `pkg/cmd/project_use.go` — `runProjectUseCmd`

**API client methods:**
- `pkg/hookdeck/projects.go:15` — `func (c *Client) ListProjects() ([]Project, error)`
  - Calls `GET /2025-07-01/teams`
  - Returns `[]Project` where `Project` has `Id string`, `Name string`, `Mode string`
  - Note: This method does NOT accept `context.Context` — it uses `context.Background()` internally
  - Note: This method creates its own client WITHOUT `ProjectID` (see `pkg/project/project.go:16-17`) since listing teams/projects is cross-project

**Request/response types:**
- `pkg/hookdeck/projects.go:9-13`:
  ```go
  type Project struct {
      Id   string
      Name string
      Mode string
  }
  ```

**MCP tool schema:**

```
Tool: hookdeck_projects
Input:
  action: string (required) — "list" or "use"
  project_id: string (optional) — required for "use" action

list action:
  - Call ListProjects() (need a separate client without ProjectID, as done in pkg/project/project.go)
  - Return JSON array of projects with current project marked
  - Postprocessing: add `"current": true/false` field based on matching client.ProjectID

use action:
  - Validate project_id exists in the list
  - Mutate client.ProjectID = project_id
  - Do NOT persist to config file (MCP session is ephemeral)
  - Return confirmation with project name
```

**Key difference from CLI:** The CLI's `project use` command persists the project to the config file (`Config.UseProject()` or `Config.UseProjectLocal()`). The MCP server should NOT persist — it should only change the in-memory `client.ProjectID` for the duration of the MCP session. This is session-scoped state.

**Project context persistence:** Since the `hookdeck.Client` is a pointer shared across all tool handlers, setting `client.ProjectID = newProjectID` immediately affects all subsequent API calls in the same session. The `X-Team-ID` and `X-Project-ID` headers are set from `client.ProjectID` in every request (see `pkg/hookdeck/client.go:102-105`).

---

#### 1.2.2 Tool: `connections`

**Actions:** `list`, `get`, `pause`, `unpause`

**Existing CLI implementations:**
- `pkg/cmd/connection_list.go` — list connections
- `pkg/cmd/connection_get.go` — get connection by ID
- `pkg/cmd/connection_pause.go` — pause connection
- `pkg/cmd/connection_unpause.go` — unpause connection

**API client methods:**
- `pkg/hookdeck/connections.go:65` — `ListConnections(ctx, params map[string]string) (*ConnectionListResponse, error)`
  - `GET /2025-07-01/connections?{params}`
- `pkg/hookdeck/connections.go:86` — `GetConnection(ctx, id string) (*Connection, error)`
  - `GET /2025-07-01/connections/{id}`
- `pkg/hookdeck/connections.go:216` — `PauseConnection(ctx, id string) (*Connection, error)`
  - `PUT /2025-07-01/connections/{id}/pause` with body `{}`
- `pkg/hookdeck/connections.go:232` — `UnpauseConnection(ctx, id string) (*Connection, error)`
  - `PUT /2025-07-01/connections/{id}/unpause` with body `{}`

**All methods exist. No gaps.**

**Request/response types:**
- `Connection` (`pkg/hookdeck/connections.go:15-28`): ID, Name, FullName, Description, TeamID, Destination, Source, Rules, DisabledAt, PausedAt, UpdatedAt, CreatedAt
- `ConnectionListResponse` (`pkg/hookdeck/connections.go:42-45`): Models []Connection, Pagination PaginationResponse

**MCP tool schema:**

```
Tool: hookdeck_connections
Input:
  action: string (required) — "list", "get", "pause", "unpause"
  id: string — required for get/pause/unpause
  # list filters:
  name: string (optional)
  source_id: string (optional)
  destination_id: string (optional)
  disabled: boolean (optional, default false)
  limit: integer (optional, default 100)
  next: string (optional) — pagination cursor
  prev: string (optional) — pagination cursor

list action:
  - Build params map from inputs
  - Note: connection_id param maps to "webhook_id" in API (see event_list.go:103)
  - disabled param: when false send "disabled=false", when true send "disabled=true" (see connection_list.go:100-104)
  - Call client.ListConnections(ctx, params)
  - Return ConnectionListResponse as JSON

get action:
  - Call client.GetConnection(ctx, id)
  - Return Connection as JSON

pause action:
  - Call client.PauseConnection(ctx, id)
  - Return updated Connection as JSON

unpause action:
  - Call client.UnpauseConnection(ctx, id)
  - Return updated Connection as JSON
```

---

#### 1.2.3 Tool: `sources`

**Actions:** `list`, `get`

**Existing CLI implementations:**
- `pkg/cmd/source_list.go` — list sources
- `pkg/cmd/source_get.go` — get source by ID

**API client methods:**
- `pkg/hookdeck/sources.go:64` — `ListSources(ctx, params map[string]string) (*SourceListResponse, error)`
  - `GET /2025-07-01/sources?{params}`
- `pkg/hookdeck/sources.go:85` — `GetSource(ctx, id string, params map[string]string) (*Source, error)`
  - `GET /2025-07-01/sources/{id}`

**Request/response types:**
- `Source` (`pkg/hookdeck/sources.go:12-22`): ID, Name, Description, URL, Type, Config, DisabledAt, UpdatedAt, CreatedAt
- `SourceListResponse` (`pkg/hookdeck/sources.go:53-56`): Models []Source, Pagination PaginationResponse

**MCP tool schema:**

```
Tool: hookdeck_sources
Input:
  action: string (required) — "list" or "get"
  id: string — required for get
  # list filters:
  name: string (optional)
  limit: integer (optional, default 100)
  next: string (optional)
  prev: string (optional)

list action:
  - Call client.ListSources(ctx, params)
  - Return SourceListResponse as JSON

get action:
  - Call client.GetSource(ctx, id, nil)
  - Return Source as JSON
```

---

#### 1.2.4 Tool: `destinations`

**Actions:** `list`, `get`

**Existing CLI implementations:**
- `pkg/cmd/destination_list.go` — list destinations
- `pkg/cmd/destination_get.go` — get destination by ID

**API client methods:**
- `pkg/hookdeck/destinations.go:63` — `ListDestinations(ctx, params map[string]string) (*DestinationListResponse, error)`
  - `GET /2025-07-01/destinations?{params}`
- `pkg/hookdeck/destinations.go:84` — `GetDestination(ctx, id string, params map[string]string) (*Destination, error)`
  - `GET /2025-07-01/destinations/{id}`

**Request/response types:**
- `Destination` (`pkg/hookdeck/destinations.go:11-22`): ID, TeamID, Name, Description, Type, Config, DisabledAt, UpdatedAt, CreatedAt
- `DestinationListResponse` (`pkg/hookdeck/destinations.go:267-270`): Models []Destination, Pagination PaginationResponse

**MCP tool schema:**

```
Tool: hookdeck_destinations
Input:
  action: string (required) — "list" or "get"
  id: string — required for get
  # list filters:
  name: string (optional)
  limit: integer (optional, default 100)
  next: string (optional)
  prev: string (optional)

list action:
  - Call client.ListDestinations(ctx, params)
  - Return DestinationListResponse as JSON

get action:
  - Call client.GetDestination(ctx, id, nil)
  - Return Destination as JSON
```

---

#### 1.2.5 Tool: `transformations`

**Actions:** `list`, `get`

**Existing CLI implementations:**
- `pkg/cmd/transformation_list.go` — list transformations
- `pkg/cmd/transformation_get.go` — get transformation by ID

**API client methods:**
- `pkg/hookdeck/transformations.go:90` — `ListTransformations(ctx, params map[string]string) (*TransformationListResponse, error)`
  - `GET /2025-07-01/transformations?{params}`
- `pkg/hookdeck/transformations.go:111` — `GetTransformation(ctx, id string) (*Transformation, error)`
  - `GET /2025-07-01/transformations/{id}`

**Request/response types:**
- `Transformation` (`pkg/hookdeck/transformations.go:12-19`): ID, Name, Code, Env, UpdatedAt, CreatedAt
- `TransformationListResponse` (`pkg/hookdeck/transformations.go:38-41`): Models []Transformation, Pagination PaginationResponse

**MCP tool schema:**

```
Tool: hookdeck_transformations
Input:
  action: string (required) — "list" or "get"
  id: string — required for get
  # list filters:
  name: string (optional)
  limit: integer (optional, default 100)
  next: string (optional)
  prev: string (optional)

list action:
  - Call client.ListTransformations(ctx, params)
  - Return TransformationListResponse as JSON

get action:
  - Call client.GetTransformation(ctx, id)
  - Return Transformation as JSON
```

---

#### 1.2.6 Tool: `requests`

**Actions:** `list`, `get`, `raw_body`, `events`, `ignored_events`, `retry`

**Existing CLI implementations:**
- `pkg/cmd/request_list.go` — list requests
- `pkg/cmd/request_get.go` — get request by ID
- `pkg/cmd/request_raw_body.go` — get raw body
- `pkg/cmd/request_events.go` — get events for a request
- `pkg/cmd/request_ignored_events.go` — get ignored events
- `pkg/cmd/request_retry.go` — retry a request

**API client methods:**
- `pkg/hookdeck/requests.go:49` — `ListRequests(ctx, params map[string]string) (*RequestListResponse, error)`
  - `GET /2025-07-01/requests?{params}`
- `pkg/hookdeck/requests.go:67` — `GetRequest(ctx, id string, params map[string]string) (*Request, error)`
  - `GET /2025-07-01/requests/{id}`
- `pkg/hookdeck/requests.go:150` — `GetRequestRawBody(ctx, requestID string) ([]byte, error)`
  - `GET /2025-07-01/requests/{id}/raw_body`
- `pkg/hookdeck/requests.go:106` — `GetRequestEvents(ctx, requestID string, params map[string]string) (*EventListResponse, error)`
  - `GET /2025-07-01/requests/{id}/events`
- `pkg/hookdeck/requests.go:128` — `GetRequestIgnoredEvents(ctx, requestID string, params map[string]string) (*EventListResponse, error)`
  - `GET /2025-07-01/requests/{id}/ignored_events`
- `pkg/hookdeck/requests.go:89` — `RetryRequest(ctx, requestID string, body *RequestRetryRequest) error`
  - `POST /2025-07-01/requests/{id}/retry`

**Request/response types:**
- `Request` (`pkg/hookdeck/requests.go:13-27`): ID, SourceID, Verified, RejectionCause, EventsCount, CliEventsCount, IgnoredCount, CreatedAt, UpdatedAt, IngestedAt, OriginalEventDataID, Data, TeamID
- `RequestData` (`pkg/hookdeck/requests.go:30-35`): Headers, Body, Path, ParsedQuery
- `RequestListResponse` (`pkg/hookdeck/requests.go:38-41`): Models []Request, Pagination PaginationResponse
- `RequestRetryRequest` (`pkg/hookdeck/requests.go:44-46`): WebhookIDs []string

**MCP tool schema:**

```
Tool: hookdeck_requests
Input:
  action: string (required) — "list", "get", "raw_body", "events", "ignored_events", "retry"
  id: string — required for get/raw_body/events/ignored_events/retry
  # list filters:
  source_id: string (optional)
  status: string (optional)
  rejection_cause: string (optional)
  verified: boolean (optional)
  limit: integer (optional, default 100)
  next: string (optional)
  prev: string (optional)
  # retry options:
  connection_ids: string[] (optional) — limit retry to specific connections

raw_body action:
  - Call client.GetRequestRawBody(ctx, id)
  - Return raw body as string content (may be large; consider truncation)
  - Postprocessing: return as {"raw_body": "<base64 or string>"}

events action:
  - Call client.GetRequestEvents(ctx, id, params)
  - Return EventListResponse as JSON

ignored_events action:
  - Call client.GetRequestIgnoredEvents(ctx, id, params)
  - Return EventListResponse as JSON

retry action:
  - Build RequestRetryRequest with WebhookIDs from connection_ids input
  - Call client.RetryRequest(ctx, id, body)
  - Return success confirmation
```

---

#### 1.2.7 Tool: `events`

**Actions:** `list`, `get`, `raw_body`, `retry`, `cancel`, `mute`

**Existing CLI implementations:**
- `pkg/cmd/event_list.go` — list events
- `pkg/cmd/event_get.go` — get event by ID
- `pkg/cmd/event_raw_body.go` — get raw body
- `pkg/cmd/event_retry.go` — retry event
- `pkg/cmd/event_cancel.go` — cancel event
- `pkg/cmd/event_mute.go` — mute event

**API client methods:**
- `pkg/hookdeck/events.go:48` — `ListEvents(ctx, params map[string]string) (*EventListResponse, error)`
  - `GET /2025-07-01/events?{params}`
- `pkg/hookdeck/events.go:66` — `GetEvent(ctx, id string, params map[string]string) (*Event, error)`
  - `GET /2025-07-01/events/{id}`
- `pkg/hookdeck/events.go:118` — `GetEventRawBody(ctx, eventID string) ([]byte, error)`
  - `GET /2025-07-01/events/{id}/raw_body`
- `pkg/hookdeck/events.go:88` — `RetryEvent(ctx, eventID string) error`
  - `POST /2025-07-01/events/{id}/retry`
- `pkg/hookdeck/events.go:98` — `CancelEvent(ctx, eventID string) error`
  - `PUT /2025-07-01/events/{id}/cancel`
- `pkg/hookdeck/events.go:108` — `MuteEvent(ctx, eventID string) error`
  - `PUT /2025-07-01/events/{id}/mute`

**Request/response types:**
- `Event` (`pkg/hookdeck/events.go:12-31`): ID, Status, WebhookID, SourceID, DestinationID, RequestID, Attempts, ResponseStatus, ErrorCode, CliID, EventDataID, CreatedAt, UpdatedAt, SuccessfulAt, LastAttemptAt, NextAttemptAt, Data, TeamID
- `EventData` (`pkg/hookdeck/events.go:34-39`): Headers, Body, Path, ParsedQuery
- `EventListResponse` (`pkg/hookdeck/events.go:42-45`): Models []Event, Pagination PaginationResponse

**MCP tool schema:**

```
Tool: hookdeck_events
Input:
  action: string (required) — "list", "get", "raw_body", "retry", "cancel", "mute"
  id: string — required for get/raw_body/retry/cancel/mute
  # list filters:
  connection_id: string (optional) — maps to webhook_id in API
  source_id: string (optional)
  destination_id: string (optional)
  status: string (optional) — SCHEDULED, QUEUED, HOLD, SUCCESSFUL, FAILED, CANCELLED
  issue_id: string (optional)
  error_code: string (optional)
  response_status: string (optional)
  created_after: string (optional) — ISO datetime, maps to created_at[gte]
  created_before: string (optional) — ISO datetime, maps to created_at[lte]
  limit: integer (optional, default 100)
  order_by: string (optional)
  dir: string (optional) — "asc" or "desc"
  next: string (optional)
  prev: string (optional)

list action:
  - Build params map; note connection_id → "webhook_id" mapping (pkg/cmd/event_list.go:103)
  - created_after → "created_at[gte]", created_before → "created_at[lte]" (pkg/cmd/event_list.go:129-134)
  - Call client.ListEvents(ctx, params)

raw_body action:
  - Call client.GetEventRawBody(ctx, id)
  - Return as {"raw_body": "<content>"}

retry/cancel/mute actions:
  - Call respective client method
  - Return success confirmation: {"status": "ok", "action": "retry|cancel|mute", "event_id": "..."}
```

---

#### 1.2.8 Tool: `attempts`

**Actions:** `list`, `get`

**Existing CLI implementations:**
- `pkg/cmd/attempt_list.go` — list attempts
- `pkg/cmd/attempt_get.go` — get attempt by ID

**API client methods:**
- `pkg/hookdeck/attempts.go:37` — `ListAttempts(ctx, params map[string]string) (*EventAttemptListResponse, error)`
  - `GET /2025-07-01/attempts?{params}`
- `pkg/hookdeck/attempts.go:55` — `GetAttempt(ctx, id string) (*EventAttempt, error)`
  - `GET /2025-07-01/attempts/{id}`

**Request/response types:**
- `EventAttempt` (`pkg/hookdeck/attempts.go:10-27`): ID, TeamID, EventID, DestinationID, ResponseStatus, AttemptNumber, Trigger, ErrorCode, Body, RequestedURL, HTTPMethod, BulkRetryID, Status, SuccessfulAt, DeliveredAt
- `EventAttemptListResponse` (`pkg/hookdeck/attempts.go:30-34`): Models []EventAttempt, Pagination PaginationResponse, Count *int

**MCP tool schema:**

```
Tool: hookdeck_attempts
Input:
  action: string (required) — "list" or "get"
  id: string — required for get
  # list filters:
  event_id: string (optional but typically required)
  limit: integer (optional, default 100)
  order_by: string (optional)
  dir: string (optional)
  next: string (optional)
  prev: string (optional)

list action:
  - Call client.ListAttempts(ctx, params)
  - Return EventAttemptListResponse as JSON

get action:
  - Call client.GetAttempt(ctx, id)
  - Return EventAttempt as JSON
```

---

#### 1.2.9 Tool: `issues`

**Actions:** `list`, `get`

**Existing CLI implementations:** NONE. There are no issue-specific commands in `pkg/cmd/`. The only reference to issues is as a filter parameter on events (`--issue-id` in `pkg/cmd/event_list.go:71`) and the `metrics events-by-issue` command.

**API client methods:** NONE. There is no `ListIssues()` or `GetIssue()` method in `pkg/hookdeck/`. The API likely has `GET /issues` and `GET /issues/{id}` endpoints, but no client methods exist.

**Gap: Both API client methods and CLI commands must be created.**

**New file required:** `pkg/hookdeck/issues.go`

Based on the Hookdeck API patterns, the implementation should follow the same structure as other resources:

```go
// pkg/hookdeck/issues.go

type Issue struct {
    ID             string    `json:"id"`
    TeamID         string    `json:"team_id"`
    Title          string    `json:"title"`
    Status         string    `json:"status"`
    Type           string    `json:"type"`
    // Reference fields linking to connections/sources/destinations
    Reference      interface{} `json:"reference,omitempty"`
    AggregationKeys interface{} `json:"aggregation_keys,omitempty"`
    FirstSeenAt    time.Time `json:"first_seen_at"`
    LastSeenAt     time.Time `json:"last_seen_at"`
    DismissedAt    *time.Time `json:"dismissed_at,omitempty"`
    OpenedAt       *time.Time `json:"opened_at,omitempty"`
    CreatedAt      time.Time `json:"created_at"`
    UpdatedAt      time.Time `json:"updated_at"`
}

type IssueListResponse struct {
    Models     []Issue            `json:"models"`
    Pagination PaginationResponse `json:"pagination"`
}

func (c *Client) ListIssues(ctx context.Context, params map[string]string) (*IssueListResponse, error) {
    // GET /2025-07-01/issues?{params}
}

func (c *Client) GetIssue(ctx context.Context, id string) (*Issue, error) {
    // GET /2025-07-01/issues/{id}
}
```

**MCP tool schema:**

```
Tool: hookdeck_issues
Input:
  action: string (required) — "list" or "get"
  id: string — required for get
  # list filters:
  status: string (optional) — e.g., OPENED, DISMISSED
  type: string (optional)
  limit: integer (optional, default 100)
  next: string (optional)
  prev: string (optional)
```

---

#### 1.2.10 Tool: `metrics`

**Actions:** `events`, `requests`, `attempts`, `transformations`

The plan abstracts 7 API endpoints into 4 MCP actions. The existing CLI has 7 subcommands:

| CLI Subcommand | API Endpoint | MCP Action |
|---|---|---|
| `metrics events` | `GET /metrics/events` | `events` |
| `metrics requests` | `GET /metrics/requests` | `requests` |
| `metrics attempts` | `GET /metrics/attempts` | `attempts` |
| `metrics transformations` | `GET /metrics/transformations` | `transformations` |
| `metrics queue-depth` | `GET /metrics/queue-depth` | (not directly mapped) |
| `metrics pending` | `GET /metrics/events-pending-timeseries` | (not directly mapped) |
| `metrics events-by-issue` | `GET /metrics/events-by-issue` | (not directly mapped) |

**Three endpoints don't map cleanly to the 4 actions:** queue-depth, events-pending-timeseries, and events-by-issue. See Question #3.

**Existing CLI implementations:**
- `pkg/cmd/metrics.go` — common flags and params
- `pkg/cmd/metrics_events.go` — event metrics
- `pkg/cmd/metrics_requests.go` — request metrics
- `pkg/cmd/metrics_attempts.go` — attempt metrics
- `pkg/cmd/metrics_transformations.go` — transformation metrics
- `pkg/cmd/metrics_pending.go` — pending timeseries
- `pkg/cmd/metrics_queue_depth.go` — queue depth
- `pkg/cmd/metrics_events_by_issue.go` — events by issue

**API client methods:**
- `pkg/hookdeck/metrics.go:102` — `QueryEventMetrics(ctx, params MetricsQueryParams) (MetricsResponse, error)`
- `pkg/hookdeck/metrics.go:107` — `QueryRequestMetrics(ctx, params MetricsQueryParams) (MetricsResponse, error)`
- `pkg/hookdeck/metrics.go:112` — `QueryAttemptMetrics(ctx, params MetricsQueryParams) (MetricsResponse, error)`
- `pkg/hookdeck/metrics.go:117` — `QueryQueueDepth(ctx, params MetricsQueryParams) (MetricsResponse, error)`
- `pkg/hookdeck/metrics.go:122` — `QueryEventsPendingTimeseries(ctx, params MetricsQueryParams) (MetricsResponse, error)`
- `pkg/hookdeck/metrics.go:127` — `QueryEventsByIssue(ctx, params MetricsQueryParams) (MetricsResponse, error)`
- `pkg/hookdeck/metrics.go:132` — `QueryTransformationMetrics(ctx, params MetricsQueryParams) (MetricsResponse, error)`

**Request/response types:**
- `MetricsQueryParams` (`pkg/hookdeck/metrics.go:26-37`): Start, End, Granularity, Measures, Dimensions, SourceID, DestinationID, ConnectionID (maps to webhook_id), Status, IssueID
- `MetricDataPoint` (`pkg/hookdeck/metrics.go:14-18`): TimeBucket, Dimensions, Metrics
- `MetricsResponse` (`pkg/hookdeck/metrics.go:21`): `= []MetricDataPoint`

**Available measures per endpoint (from CLI constants):**
- Events: `count, successful_count, failed_count, scheduled_count, paused_count, error_rate, avg_attempts, scheduled_retry_count` (`pkg/cmd/metrics_events.go:10`)
- Requests: `count, accepted_count, rejected_count, discarded_count, avg_events_per_request, avg_ignored_per_request` (`pkg/cmd/metrics_requests.go:10`)
- Attempts: `count, successful_count, failed_count, delivered_count, error_rate, response_latency_avg, response_latency_max, response_latency_p95, response_latency_p99, delivery_latency_avg` (`pkg/cmd/metrics_attempts.go:10`)
- Transformations: `count, successful_count, failed_count, error_rate, error_count, warn_count, info_count, debug_count` (`pkg/cmd/metrics_transformations.go:10`)
- Queue depth: `max_depth, max_age` (`pkg/cmd/metrics_queue_depth.go:10`)

**Dimension mapping:** The CLI maps `connection_id` / `connection-id` → `webhook_id` for the API (see `pkg/cmd/metrics.go:110-112`). The MCP layer must do the same.

**MCP tool schema:**

```
Tool: hookdeck_metrics
Input:
  action: string (required) — "events", "requests", "attempts", "transformations"
  start: string (required) — ISO 8601 datetime
  end: string (required) — ISO 8601 datetime
  granularity: string (optional) — e.g., "1h", "5m", "1d"
  measures: string[] (optional) — specific measures to return
  dimensions: string[] (optional) — e.g., ["source_id", "connection_id"]
  source_id: string (optional)
  destination_id: string (optional)
  connection_id: string (optional) — maps to webhook_id in API
  status: string (optional)

Preprocessing:
  - Map connection_id → webhook_id in dimensions array
  - Build MetricsQueryParams from inputs
  - Call the appropriate Query*Metrics method based on action

Output:
  - Return MetricsResponse ([]MetricDataPoint) as JSON array
```

---

#### 1.2.11 Tool: `help`

**Actions:** None (single-purpose tool)

**Existing CLI implementations:** No direct equivalent. The CLI uses Cobra's built-in help system.

**Implementation:** This is a static/computed tool that returns contextual help about the available MCP tools. It does not call any API. It should:
1. List all available tools and their actions
2. Provide brief descriptions
3. Include the current project context (from client.ProjectID)

**MCP tool schema:**

```
Tool: hookdeck_help
Input:
  topic: string (optional) — specific tool name for detailed help

Output:
  - If no topic: list all tools with brief descriptions
  - If topic specified: detailed help for that tool including all actions and parameters
```

---

### 1.3 File Structure

```
pkg/gateway/mcp/
├── server.go          # MCP server initialization, tool registration, stdio transport
├── tools.go           # Tool handler dispatch (action routing)
├── tool_projects.go   # projects tool implementation
├── tool_connections.go # connections tool implementation
├── tool_sources.go    # sources tool implementation
├── tool_destinations.go # destinations tool implementation
├── tool_transformations.go # transformations tool implementation
├── tool_requests.go   # requests tool implementation
├── tool_events.go     # events tool implementation
├── tool_attempts.go   # attempts tool implementation
├── tool_issues.go     # issues tool implementation
├── tool_metrics.go    # metrics tool implementation
├── tool_help.go       # help tool implementation
├── errors.go          # Error translation (APIError → MCP error messages)
└── response.go        # Response formatting helpers (JSON marshaling)

pkg/cmd/
├── mcp.go             # Cobra command: hookdeck gateway mcp

pkg/hookdeck/
├── issues.go          # NEW: Issue API client methods (ListIssues, GetIssue)
```

### 1.4 Dependency Addition

**New dependency:** `github.com/modelcontextprotocol/go-sdk` v1.2.0+

Add to `go.mod`:
```
require (
    ...
    github.com/modelcontextprotocol/go-sdk v1.2.0
    ...
)
```

Run `go get github.com/modelcontextprotocol/go-sdk@v1.2.0` and `go mod tidy`.

---

### 1.5 Error Handling

#### 1.5.1 API Error Translation

All API errors flow through `checkAndPrintError` in `pkg/hookdeck/client.go:244-274`, which returns `*APIError` with `StatusCode` and `Message`.

The MCP error layer (`pkg/gateway/mcp/errors.go`) should:

```go
func translateError(err error) *mcp.CallToolError {
    var apiErr *hookdeck.APIError
    if errors.As(err, &apiErr) {
        switch apiErr.StatusCode {
        case 401:
            return &mcp.CallToolError{Message: "Authentication failed. Check your API key."}
        case 403:
            return &mcp.CallToolError{Message: "Permission denied. Your API key may not have access to this resource."}
        case 404:
            return &mcp.CallToolError{Message: "Resource not found."}
        case 422:
            return &mcp.CallToolError{Message: fmt.Sprintf("Validation error: %s", apiErr.Message)}
        case 429:
            return &mcp.CallToolError{Message: "Rate limited. Please retry after a brief pause."}
        default:
            return &mcp.CallToolError{Message: fmt.Sprintf("API error (%d): %s", apiErr.StatusCode, apiErr.Message)}
        }
    }
    return &mcp.CallToolError{Message: fmt.Sprintf("Internal error: %s", err.Error())}
}
```

#### 1.5.2 Rate Limiting

The current API client does NOT implement automatic retry on 429. The `SuppressRateLimitErrors` field (used only for login polling) just changes log level. For the MCP server:

- Option A: Return the 429 error to the MCP client and let the AI agent retry
- Option B: Implement retry with exponential backoff in the MCP layer

Recommendation: Option A is simpler and lets the AI agent manage its own pacing. The error message should include guidance: "Rate limited. Please retry after a brief pause."

The API does not currently parse `Retry-After` headers. The `checkAndPrintError` function reads the response body for the error message but does not inspect headers.

---

### 1.6 ListProjects Client Nuance

The `ListProjects()` method in `pkg/hookdeck/projects.go:15` does NOT accept a context parameter. It also does NOT set `ProjectID` — intentionally, because listing teams/projects is cross-project. The helper in `pkg/project/project.go:10-22` creates a fresh `Client` with only `BaseURL` and `APIKey` (no `ProjectID`).

For the MCP server's `projects.list` action, you should either:
1. Create a temporary client without ProjectID (mirroring `pkg/project/project.go`), or
2. Call `ListProjects()` directly on the shared client — this works because `ListProjects()` hits `GET /teams` which is not project-scoped, and the `X-Team-ID` header is simply ignored for this endpoint

Option 2 is simpler and likely safe, but Option 1 is what the existing codebase does. Follow Option 1 for consistency.

---

## Section 2: Questions and Unknowns

### Functionality Unknown

#### Q1: Issues API client methods do not exist

**What was found:** There are no `ListIssues()` or `GetIssue()` methods in `pkg/hookdeck/`. No `issues.go` file exists. The only issue reference is as a filter parameter (`issue_id`) on events and metrics.

**Why it's unclear:** The plan calls for an `issues` tool with `list` and `get` actions, but the codebase has no implementation to reference.

**Resolution paths:**
1. **Add `pkg/hookdeck/issues.go`** with `ListIssues()` and `GetIssue()` following the existing pattern (same structure as `events.go`, `connections.go`, etc.). The API endpoints are likely `GET /2025-07-01/issues` and `GET /2025-07-01/issues/{id}`.
2. **Verify the Issue model** against the Hookdeck API documentation or OpenAPI spec before implementing, since the exact response fields are unknown from the codebase alone.
3. **Defer the issues tool** to a later phase if the API endpoints are not stable.

#### Q2: Issue struct field definitions are unknown from codebase

**What was found:** There is no Issue struct defined anywhere in the codebase.

**Why it's unclear:** Without the Hookdeck OpenAPI spec or API documentation, the exact fields on the Issue response object are a guess.

**Resolution paths:**
1. Consult the Hookdeck API documentation for the Issue schema
2. Make a test API call to `GET /issues` and inspect the response
3. Start with a minimal struct (`ID`, `Title`, `Status`, `Type`, `CreatedAt`, `UpdatedAt`) and add fields as needed

### Ambiguity in Plan

#### Q3: Three metrics endpoints are not mapped to the 4 MCP actions

**What was found:** The plan specifies 4 metrics actions (events, requests, attempts, transformations), but the CLI and API have 7 endpoints. Three endpoints have no corresponding MCP action:
- `queue-depth` (`GET /metrics/queue-depth`) — measures: max_depth, max_age
- `pending` / `events-pending-timeseries` (`GET /metrics/events-pending-timeseries`) — measures: count
- `events-by-issue` (`GET /metrics/events-by-issue`) — requires issue_id

**Why it's unclear:** The plan says "The API has 7 separate metrics endpoints that the MCP abstracts into 4 actions" but does not specify how the remaining 3 endpoints are handled.

**Resolution paths:**
1. **Expose all 7 as separate actions** — change the MCP tool to have 7 actions instead of 4. This is the most complete.
2. **Fold queue-depth and pending into a broader action** — e.g., add `queue_depth` and `pending` as additional actions. events-by-issue could be folded into `events` with a special parameter.
3. **Omit the 3 endpoints from Phase 1** — accept that agents won't have access to queue depth, pending timeseries, or events-by-issue metrics. These are less commonly needed.

#### Q4: `ListProjects()` does not accept context.Context

**What was found:** `ListProjects()` in `pkg/hookdeck/projects.go:15` uses `context.Background()` internally, unlike all other API methods which accept `ctx context.Context`.

**Why it's unclear:** MCP tool handlers typically receive a context from the MCP framework. Should the MCP layer pass its context, or is it acceptable to use `context.Background()`?

**Resolution paths:**
1. **Use the method as-is** — `context.Background()` is fine since ListProjects is fast and rarely cancelled
2. **Add a `ListProjectsCtx(ctx context.Context)` variant** if context propagation is important for cancellation

#### Q5: Tool naming convention — flat vs namespaced

**What was found:** The plan refers to tools like "projects", "connections", etc. But MCP tools are typically named with a prefix for namespacing.

**Why it's unclear:** Should the tools be named `hookdeck_projects`, `hookdeck_connections`, etc. (namespaced) or just `projects`, `connections` (flat)?

**Resolution paths:**
1. **Namespaced** (e.g., `hookdeck_projects`) — prevents collisions with other MCP servers, recommended practice
2. **Flat** (e.g., `projects`) — simpler for agents, but risks name collisions
3. **Configurable prefix** — overkill for Phase 1

### Implementation Risk

#### Q6: `Config.GetAPIClient()` is a singleton with `sync.Once`

**What was found:** `Config.GetAPIClient()` (`pkg/config/apiclient.go:14-30`) uses `sync.Once` to create a single `*hookdeck.Client`. Once created, the `APIKey` and initial `ProjectID` are baked in.

**Why it's a risk:** The `projects.use` action needs to change `ProjectID`. Since the client is a pointer and `ProjectID` is a public field, setting `client.ProjectID = newID` works. However, the `sync.Once` means the API key cannot be changed after initialization — this is fine for the MCP use case since auth is set before the server starts.

**Impact:** Low. This works as designed. The only concern is thread safety if multiple MCP tool calls execute concurrently and one changes ProjectID while another is mid-request. Since `PerformRequest` reads `c.ProjectID` during header setup (`pkg/hookdeck/client.go:102-105`), there could be a race condition.

**Resolution paths:**
1. **Accept the race** — MCP stdio is inherently sequential (one request at a time), so concurrent mutations should not occur
2. **Add a mutex** around `ProjectID` access if the MCP SDK allows concurrent tool calls
3. **Create a new Client** for each tool call — heavyweight but safe

#### Q7: The `go-sdk` MCP library API surface is unknown from the codebase

**What was found:** The `go.mod` does not include `github.com/modelcontextprotocol/go-sdk`. It's a new dependency.

**Why it's a risk:** The exact API for `server.NewMCPServer()`, tool registration, stdio transport, and error handling in the Go MCP SDK needs to be verified against the actual library version.

**Resolution paths:**
1. **Pin to a specific version** (v1.2.0+) and verify the API before starting implementation
2. **Write a small spike** — create a minimal MCP server with one tool to validate the SDK API before building all 11 tools
3. **Review the SDK's README/examples** for the canonical usage pattern

#### Q8: Raw body responses may be very large

**What was found:** `GetEventRawBody` and `GetRequestRawBody` return `[]byte` of arbitrary size. Webhook payloads can be large.

**Why it's a risk:** MCP tool responses have practical size limits. A multi-megabyte raw body could cause issues for the AI agent processing the response.

**Resolution paths:**
1. **Truncate with indication** — return the first N bytes with a note: "Body truncated at 100KB. Full body is X bytes."
2. **Base64 encode** — return as base64 string (doubles size but is safe for JSON)
3. **Return metadata only** — return content length and content type without the full body, let the agent decide if they need it

#### Q9: The `project use` action's scope within an MCP session

**What was found:** The plan says "use" changes the active project. The CLI persists this to config files. The MCP server should not persist.

**Why it's a risk:** If the MCP server dies and restarts, the project context is lost. The agent would need to call `projects.use` again. This is acceptable behavior but should be documented.

**Resolution paths:**
1. **Session-scoped only** (recommended) — mutate `client.ProjectID` in memory only
2. **Persist to config** — matches CLI behavior but affects other CLI sessions, which is unexpected
3. **Return the project context in every response** — so the agent always knows which project is active
