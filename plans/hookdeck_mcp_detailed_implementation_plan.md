# Hookdeck MCP Server — Detailed Implementation Plan

## Overview

This document maps the high-level MCP build-out plan against the existing hookdeck-cli codebase and provides every implementation detail an engineer needs to build Phase 1 without ambiguity.

**Package location:** `pkg/gateway/mcp`
**Command:** `hookdeck gateway mcp`
**Go MCP SDK:** `github.com/modelcontextprotocol/go-sdk` v1.2.0+
**Transport:** stdio only (Phase 1)
**Auth model:** Inherited from CLI via `Config.GetAPIClient()`

---

## Phase 1 Progress

### Part 1: Issues CLI Backfill (prerequisite) -- COMPLETE

- [x] `pkg/hookdeck/issues.go` — Issue types and API client methods (ListIssues, GetIssue, UpdateIssue, DismissIssue, CountIssues)
- [x] `pkg/cmd/helptext.go` — Add `ResourceIssue = "issue"`
- [x] `pkg/cmd/issue.go` — Issue group command (`issue` / `issues`)
- [x] `pkg/cmd/issue_list.go` — `hookdeck gateway issue list`
- [x] `pkg/cmd/issue_get.go` — `hookdeck gateway issue get <id>`
- [x] `pkg/cmd/issue_update.go` — `hookdeck gateway issue update <id> --status <status>`
- [x] `pkg/cmd/issue_dismiss.go` — `hookdeck gateway issue dismiss <id>`
- [x] `pkg/cmd/issue_count.go` — `hookdeck gateway issue count`
- [x] `pkg/cmd/gateway.go` — Register issue commands via `addIssueCmdTo(g.cmd)`
- [x] Build and verify compilation

### Part 2: Metrics CLI Consolidation (prerequisite)

- [ ] Expand `pkg/cmd/metrics_events.go` to handle queue-depth, pending, and events-by-issue routing
- [ ] Remove `pkg/cmd/metrics_pending.go` (folded into metrics_events)
- [ ] Remove `pkg/cmd/metrics_queue_depth.go` (folded into metrics_events)
- [ ] Remove `pkg/cmd/metrics_events_by_issue.go` (folded into metrics_events)
- [ ] Update `pkg/cmd/metrics.go` — remove deprecated subcommand registrations

### Part 3: MCP Server Skeleton

- [ ] Add `github.com/modelcontextprotocol/go-sdk` dependency
- [ ] `pkg/gateway/mcp/server.go` — MCP server init, tool registration, stdio transport
- [ ] `pkg/gateway/mcp/tools.go` — Tool handler dispatch (action routing)
- [ ] `pkg/gateway/mcp/errors.go` — API error → MCP error translation
- [ ] `pkg/gateway/mcp/response.go` — Response formatting helpers
- [ ] `pkg/cmd/mcp.go` — Cobra command: `hookdeck gateway mcp`
- [ ] `pkg/cmd/gateway.go` — Register MCP command via `addMCPCmdTo(g.cmd)`

### Part 4: MCP Tool Implementations

- [ ] `pkg/gateway/mcp/tool_projects.go` — projects (list, use)
- [ ] `pkg/gateway/mcp/tool_connections.go` — connections (list, get, create, update, delete, upsert)
- [ ] `pkg/gateway/mcp/tool_sources.go` — sources (list, get, create, update, delete, upsert)
- [ ] `pkg/gateway/mcp/tool_destinations.go` — destinations (list, get, create, update, delete, upsert)
- [ ] `pkg/gateway/mcp/tool_transformations.go` — transformations (list, get, create, update, upsert)
- [ ] `pkg/gateway/mcp/tool_requests.go` — requests (list, get, get_body, retry)
- [ ] `pkg/gateway/mcp/tool_events.go` — events (list, get, get_body, retry, mute)
- [ ] `pkg/gateway/mcp/tool_attempts.go` — attempts (list, get, get_body)
- [ ] `pkg/gateway/mcp/tool_issues.go` — issues (list, get, update, dismiss, count)
- [ ] `pkg/gateway/mcp/tool_metrics.go` — metrics (requests, events, attempts, transformations)
- [ ] `pkg/gateway/mcp/tool_help.go` — help (list_tools, tool_detail)

### Part 5: Integration Testing & Polish

- [ ] End-to-end test: start MCP server, send tool calls, verify responses
- [ ] Verify all 11 tools return well-formed JSON
- [ ] Test error scenarios (auth failure, 404, 422, rate limiting)
- [ ] Test project switching within an MCP session

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

**Actions:** `list`, `get`, `update`, `dismiss`

**Existing CLI implementations:** NONE. There are no issue-specific commands in `pkg/cmd/`. The only reference to issues is as a filter parameter on events (`--issue-id` in `pkg/cmd/event_list.go:71`) and the `metrics events-by-issue` command.

**API client methods:** NONE. No `issues.go` file exists in `pkg/hookdeck/`.

**Gap: API client methods, CLI commands, AND MCP tool all must be created.**

This is a Phase 1 prerequisite: backfill CLI commands for issues following the same conventions as other resources, then wire them into the MCP tool.

##### 1.2.9.1 API Endpoints (from OpenAPI spec at `plans/openapi_2025-07-01.json`)

| Method | Path | Operation | Description |
|--------|------|-----------|-------------|
| GET | `/issues` | `getIssues` | List issues with filters and pagination |
| GET | `/issues/count` | `getIssueCount` | Count issues matching filters |
| GET | `/issues/{id}` | `getIssue` | Get a single issue by ID |
| PUT | `/issues/{id}` | `updateIssue` | Update issue status |
| DELETE | `/issues/{id}` | `dismissIssue` | Dismiss an issue |

##### 1.2.9.2 Issue Object Schema (from OpenAPI)

The `Issue` type is a union (`anyOf`) of `DeliveryIssue` and `TransformationIssue`. Both share the same base fields but differ in `type`, `aggregation_keys`, and `reference`.

**Shared fields (both delivery and transformation issues):**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | string | yes | Issue ID (e.g., `iss_YXKv5OdJXCiVwkPhGy`) |
| `team_id` | string | yes | Project ID |
| `status` | IssueStatus enum | yes | `OPENED`, `IGNORED`, `ACKNOWLEDGED`, `RESOLVED` |
| `type` | string enum | yes | `delivery` or `transformation` |
| `opened_at` | datetime | yes | When issue was last opened |
| `first_seen_at` | datetime | yes | When issue was first opened |
| `last_seen_at` | datetime | yes | When issue last occurred |
| `dismissed_at` | datetime, nullable | no | When dismissed |
| `auto_resolved_at` | datetime, nullable | no | When auto-resolved (hidden in docs) |
| `merged_with` | string, nullable | no | Merged issue ID (hidden in docs) |
| `updated_at` | string | yes | Last updated |
| `created_at` | string | yes | Created |
| `last_updated_by` | string, nullable | no | Deprecated, always null |
| `aggregation_keys` | object | yes | Type-specific (see below) |
| `reference` | object | yes | Type-specific (see below) |

**DeliveryIssue-specific:**
- `aggregation_keys`: `{webhook_id: string[], response_status: number[], error_code: AttemptErrorCodes[]}`
- `reference`: `{event_id: string, attempt_id: string}`

**TransformationIssue-specific:**
- `aggregation_keys`: `{transformation_id: string[], log_level: TransformationExecutionLogLevel[]}`
- `reference`: `{transformation_execution_id: string, trigger_event_request_transformation_id: string|null}`

**IssueWithData** extends Issue with a `data` field:
- Delivery: `data: {trigger_event: Event, trigger_attempt: EventAttempt}`
- Transformation: `data: {transformation_execution: TransformationExecution, trigger_attempt?: EventAttempt}`

**GET /issues list response:** `IssueWithDataPaginatedResult` — `{pagination, count, models: IssueWithData[]}`

##### 1.2.9.3 GET /issues Query Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `id` | string or string[] | Filter by Issue IDs |
| `issue_trigger_id` | string or string[] | Filter by Issue trigger IDs |
| `type` | IssueType or IssueType[] | `delivery`, `transformation`, `backpressure` |
| `status` | IssueStatus or IssueStatus[] | `OPENED`, `IGNORED`, `ACKNOWLEDGED`, `RESOLVED` |
| `merged_with` | string or string[] | Filter by merged issue IDs |
| `aggregation_keys` | JSON object | Filter by aggregation keys (webhook_id, response_status, error_code) |
| `created_at` | datetime or Operators | Filter by created date |
| `first_seen_at` | datetime or Operators | Filter by first seen date |
| `last_seen_at` | datetime or Operators | Filter by last seen date |
| `dismissed_at` | datetime or Operators | Filter by dismissed date |
| `order_by` | enum | `created_at`, `first_seen_at`, `last_seen_at`, `opened_at`, `status` |
| `dir` | enum | `asc`, `desc` |
| `limit` | integer (0-255) | Result set size |
| `next` | string | Pagination cursor |
| `prev` | string | Pagination cursor |

##### 1.2.9.4 PUT /issues/{id} Request Body

```json
{
  "status": "OPENED" | "IGNORED" | "ACKNOWLEDGED" | "RESOLVED"  // required
}
```

Returns the updated `Issue` object.

##### 1.2.9.5 New API Client Implementation

**New file:** `pkg/hookdeck/issues.go`

```go
package hookdeck

import (
    "context"
    "encoding/json"
    "fmt"
    "net/url"
    "time"
)

// Issue represents a Hookdeck issue (union of DeliveryIssue and TransformationIssue).
// Uses interface{} for type-specific fields (aggregation_keys, reference, data)
// since the shape varies by issue type.
type Issue struct {
    ID              string                 `json:"id"`
    TeamID          string                 `json:"team_id"`
    Status          string                 `json:"status"`
    Type            string                 `json:"type"`
    OpenedAt        time.Time              `json:"opened_at"`
    FirstSeenAt     time.Time              `json:"first_seen_at"`
    LastSeenAt      time.Time              `json:"last_seen_at"`
    DismissedAt     *time.Time             `json:"dismissed_at,omitempty"`
    AutoResolvedAt  *time.Time             `json:"auto_resolved_at,omitempty"`
    MergedWith      *string                `json:"merged_with,omitempty"`
    UpdatedAt       time.Time              `json:"updated_at"`
    CreatedAt       time.Time              `json:"created_at"`
    AggregationKeys map[string]interface{} `json:"aggregation_keys"`
    Reference       map[string]interface{} `json:"reference"`
    Data            map[string]interface{} `json:"data,omitempty"`
}

// IssueListResponse represents the paginated response from listing issues
type IssueListResponse struct {
    Models     []Issue            `json:"models"`
    Pagination PaginationResponse `json:"pagination"`
    Count      *int               `json:"count,omitempty"`
}

// IssueCountResponse represents the response from counting issues
type IssueCountResponse struct {
    Count int `json:"count"`
}

// IssueUpdateRequest is the request body for PUT /issues/{id}
type IssueUpdateRequest struct {
    Status string `json:"status"`
}

// ListIssues retrieves issues with optional filters
func (c *Client) ListIssues(ctx context.Context, params map[string]string) (*IssueListResponse, error) {
    queryParams := url.Values{}
    for k, v := range params {
        queryParams.Add(k, v)
    }
    resp, err := c.Get(ctx, APIPathPrefix+"/issues", queryParams.Encode(), nil)
    if err != nil {
        return nil, err
    }
    var result IssueListResponse
    _, err = postprocessJsonResponse(resp, &result)
    if err != nil {
        return nil, fmt.Errorf("failed to parse issue list response: %w", err)
    }
    return &result, nil
}

// GetIssue retrieves a single issue by ID
func (c *Client) GetIssue(ctx context.Context, id string) (*Issue, error) {
    resp, err := c.Get(ctx, APIPathPrefix+"/issues/"+id, "", nil)
    if err != nil {
        return nil, err
    }
    var issue Issue
    _, err = postprocessJsonResponse(resp, &issue)
    if err != nil {
        return nil, fmt.Errorf("failed to parse issue response: %w", err)
    }
    return &issue, nil
}

// UpdateIssue updates an issue's status
func (c *Client) UpdateIssue(ctx context.Context, id string, req *IssueUpdateRequest) (*Issue, error) {
    data, err := json.Marshal(req)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal issue update request: %w", err)
    }
    resp, err := c.Put(ctx, APIPathPrefix+"/issues/"+id, data, nil)
    if err != nil {
        return nil, err
    }
    var issue Issue
    _, err = postprocessJsonResponse(resp, &issue)
    if err != nil {
        return nil, fmt.Errorf("failed to parse issue response: %w", err)
    }
    return &issue, nil
}

// DismissIssue dismisses an issue (DELETE /issues/{id})
func (c *Client) DismissIssue(ctx context.Context, id string) error {
    urlPath := APIPathPrefix + "/issues/" + id
    req, err := c.newRequest(ctx, "DELETE", urlPath, nil)
    if err != nil {
        return err
    }
    resp, err := c.PerformRequest(ctx, req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    return nil
}

// CountIssues counts issues matching the given filters
func (c *Client) CountIssues(ctx context.Context, params map[string]string) (*IssueCountResponse, error) {
    queryParams := url.Values{}
    for k, v := range params {
        queryParams.Add(k, v)
    }
    resp, err := c.Get(ctx, APIPathPrefix+"/issues/count", queryParams.Encode(), nil)
    if err != nil {
        return nil, err
    }
    var result IssueCountResponse
    _, err = postprocessJsonResponse(resp, &result)
    if err != nil {
        return nil, fmt.Errorf("failed to parse issue count response: %w", err)
    }
    return &result, nil
}
```

##### 1.2.9.6 New CLI Commands

Following the existing resource command conventions, create these files:

**`pkg/cmd/helptext.go` — add:**
```go
ResourceIssue = "issue"
```

**`pkg/cmd/issue.go`** — group command:
```go
// Pattern: same as source.go
// Use: "issue", Aliases: []string{"issues"}
// Short: ShortBeta("Manage your issues")
// Subcommands: list, get, update, dismiss, count
```

**`pkg/cmd/issue_list.go`** — list issues:
```go
// Flags: --type (delivery,transformation,backpressure), --status (OPENED,IGNORED,ACKNOWLEDGED,RESOLVED),
//        --issue-trigger-id, --order-by, --dir, --limit, --next, --prev, --output
// Pattern: same as source_list.go, event_list.go
```

**`pkg/cmd/issue_get.go`** — get issue by ID:
```go
// Args: ExactArgs(1) — issue ID
// Pattern: same as source_get.go (but no name resolution needed — issues are ID-only)
```

**`pkg/cmd/issue_update.go`** — update issue status:
```go
// Args: ExactArgs(1) — issue ID
// Flags: --status (required) — OPENED, IGNORED, ACKNOWLEDGED, RESOLVED
// Calls client.UpdateIssue(ctx, id, &IssueUpdateRequest{Status: status})
```

**`pkg/cmd/issue_dismiss.go`** — dismiss an issue:
```go
// Args: ExactArgs(1) — issue ID
// Calls client.DismissIssue(ctx, id)
// Pattern: same as connection_delete.go / source_delete.go
```

**`pkg/cmd/issue_count.go`** — count issues:
```go
// Flags: same filters as list (--type, --status, --issue-trigger-id)
// Calls client.CountIssues(ctx, params)
// Pattern: same as source_count.go
```

**Registration in `pkg/cmd/gateway.go`:**
```go
addIssueCmdTo(g.cmd)
```

##### 1.2.9.7 MCP Tool Schema

```
Tool: hookdeck_issues
Input:
  action: string (required) — "list", "get", "update", "dismiss"
  id: string — required for get/update/dismiss
  # update parameters:
  status: string — required for update; OPENED, IGNORED, ACKNOWLEDGED, RESOLVED
  # list filters:
  type: string (optional) — delivery, transformation, backpressure
  filter_status: string (optional) — OPENED, IGNORED, ACKNOWLEDGED, RESOLVED
  issue_trigger_id: string (optional)
  order_by: string (optional) — created_at, first_seen_at, last_seen_at, opened_at, status
  dir: string (optional) — asc, desc
  limit: integer (optional, default 100)
  next: string (optional)
  prev: string (optional)

list action:
  - Build params map from inputs
  - Call client.ListIssues(ctx, params)
  - Return IssueListResponse as JSON

get action:
  - Call client.GetIssue(ctx, id)
  - Return Issue as JSON

update action:
  - Call client.UpdateIssue(ctx, id, &IssueUpdateRequest{Status: status})
  - Return updated Issue as JSON

dismiss action:
  - Call client.DismissIssue(ctx, id)
  - Return success confirmation
```

---

#### 1.2.10 Tool: `metrics`

**Actions:** `events`, `requests`, `attempts`, `transformations`

##### 1.2.10.1 Metrics Consolidation

The current API has 7 endpoints, but the correct domain model has 4 resource-aligned metrics endpoints. Three of the current endpoints (`queue-depth`, `events-pending-timeseries`, `events-by-issue`) are views of the events resource and should be exposed as measures and dimensions on a single `events` action, not as separate actions.

**Consolidation mapping:**

| Current API Endpoint | Target MCP Action | How It Maps |
|---|---|---|
| `GET /metrics/events` | `events` | Direct |
| `GET /metrics/queue-depth` | `events` | Measures: `pending`, `queue_depth`; Dimensions: `destination_id` |
| `GET /metrics/events-pending-timeseries` | `events` | Measures: `pending`; with granularity |
| `GET /metrics/events-by-issue` | `events` | Dimensions: `issue_id` |
| `GET /metrics/requests` | `requests` | Direct |
| `GET /metrics/attempts` | `attempts` | Direct |
| `GET /metrics/transformations` | `transformations` | Direct |

##### 1.2.10.2 CLI Metrics Refactoring (Phase 1 prerequisite)

The CLI should also be updated from 7 subcommands to 4 resource-aligned subcommands. The CLI client layer handles routing to the correct underlying API endpoint(s) based on the measures/dimensions requested.

**Current files to refactor:**
- `pkg/cmd/metrics.go` — keep common flags; update subcommand registration
- `pkg/cmd/metrics_events.go` — expand to handle queue-depth, pending, and events-by-issue
- `pkg/cmd/metrics_requests.go` — keep as-is
- `pkg/cmd/metrics_attempts.go` — keep as-is
- `pkg/cmd/metrics_transformations.go` — keep as-is
- `pkg/cmd/metrics_pending.go` — **remove** (folded into events)
- `pkg/cmd/metrics_queue_depth.go` — **remove** (folded into events)
- `pkg/cmd/metrics_events_by_issue.go` — **remove** (folded into events)

**CLI routing logic for `hookdeck metrics events`:**

When the user requests measures like `pending`, `queue_depth`, `max_depth`, `max_age` or dimensions like `issue_id`, the CLI client must route to the correct underlying API endpoint:

```go
func queryEventMetricsConsolidated(ctx context.Context, client *hookdeck.Client, params hookdeck.MetricsQueryParams) (hookdeck.MetricsResponse, error) {
    // Route based on measures/dimensions requested:
    // If measures include "queue_depth", "max_depth", "max_age" → QueryQueueDepth
    // If measures include "pending" with granularity → QueryEventsPendingTimeseries
    // If dimensions include "issue_id" or IssueID is set → QueryEventsByIssue
    // Otherwise → QueryEventMetrics (default)
}
```

This routing is an implementation detail of the CLI client layer. Both MCP tools and CLI commands use the same routing.

**Updated measures per action (consolidated):**

- **Events:** `count, successful_count, failed_count, scheduled_count, paused_count, error_rate, avg_attempts, scheduled_retry_count, pending, queue_depth, max_depth, max_age`
- **Requests:** `count, accepted_count, rejected_count, discarded_count, avg_events_per_request, avg_ignored_per_request`
- **Attempts:** `count, successful_count, failed_count, delivered_count, error_rate, response_latency_avg, response_latency_max, response_latency_p95, response_latency_p99, delivery_latency_avg`
- **Transformations:** `count, successful_count, failed_count, error_rate, error_count, warn_count, info_count, debug_count`

**Updated dimensions per action:**

- **Events:** `connection_id`, `source_id`, `destination_id`, `issue_id`
- **Requests:** `source_id`
- **Attempts:** `destination_id`
- **Transformations:** `transformation_id`, `connection_id`

##### 1.2.10.3 Existing API Client Methods (unchanged)

The underlying API client methods remain unchanged — the routing logic is added in a new helper layer:

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

**Dimension mapping:** The CLI maps `connection_id` / `connection-id` → `webhook_id` for the API (see `pkg/cmd/metrics.go:110-112`). Both CLI and MCP must do this.

##### 1.2.10.4 MCP Tool Schema

```
Tool: hookdeck_metrics
Input:
  action: string (required) — "events", "requests", "attempts", "transformations"
  start: string (required) — ISO 8601 datetime
  end: string (required) — ISO 8601 datetime
  granularity: string (optional) — e.g., "1h", "5m", "1d"
  measures: string[] (optional) — specific measures to return
  dimensions: string[] (optional) — e.g., ["source_id", "connection_id", "issue_id"]
  source_id: string (optional)
  destination_id: string (optional)
  connection_id: string (optional) — maps to webhook_id in API
  status: string (optional)
  issue_id: string (optional) — for events action, triggers events-by-issue routing

Preprocessing:
  - Map connection_id → webhook_id in dimensions array
  - Build MetricsQueryParams from inputs
  - For "events" action: use consolidated routing to pick correct API endpoint
  - For other actions: call respective Query*Metrics method

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
# Phase 1 prerequisite: Issues CLI backfill
pkg/hookdeck/
├── issues.go              # NEW: Issue API client (ListIssues, GetIssue, UpdateIssue, DismissIssue, CountIssues)

pkg/cmd/
├── issue.go               # NEW: Issue group command (issue/issues)
├── issue_list.go           # NEW: hookdeck gateway issue list
├── issue_get.go            # NEW: hookdeck gateway issue get
├── issue_update.go         # NEW: hookdeck gateway issue update
├── issue_dismiss.go        # NEW: hookdeck gateway issue dismiss
├── issue_count.go          # NEW: hookdeck gateway issue count
├── helptext.go             # MODIFY: add ResourceIssue = "issue"
├── gateway.go              # MODIFY: add addIssueCmdTo(g.cmd)

# Phase 1 prerequisite: Metrics CLI consolidation
pkg/cmd/
├── metrics.go              # MODIFY: remove 3 deprecated subcommand registrations
├── metrics_events.go       # MODIFY: expand to handle queue-depth, pending, events-by-issue routing
├── metrics_requests.go     # KEEP: unchanged
├── metrics_attempts.go     # KEEP: unchanged
├── metrics_transformations.go # KEEP: unchanged
├── metrics_pending.go      # REMOVE: folded into metrics_events.go
├── metrics_queue_depth.go  # REMOVE: folded into metrics_events.go
├── metrics_events_by_issue.go # REMOVE: folded into metrics_events.go

# MCP server
pkg/gateway/mcp/
├── server.go              # MCP server initialization, tool registration, stdio transport
├── tools.go               # Tool handler dispatch (action routing)
├── tool_projects.go       # projects tool implementation
├── tool_connections.go    # connections tool implementation
├── tool_sources.go        # sources tool implementation
├── tool_destinations.go   # destinations tool implementation
├── tool_transformations.go # transformations tool implementation
├── tool_requests.go       # requests tool implementation
├── tool_events.go         # events tool implementation
├── tool_attempts.go       # attempts tool implementation
├── tool_issues.go         # issues tool implementation
├── tool_metrics.go        # metrics tool implementation
├── tool_help.go           # help tool implementation
├── errors.go              # Error translation (APIError → MCP error messages)
└── response.go            # Response formatting helpers (JSON marshaling)

pkg/cmd/
├── mcp.go                 # Cobra command: hookdeck gateway mcp

# Reference
plans/
├── openapi_2025-07-01.json # OpenAPI spec for Hookdeck API (reference)
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

### Resolved

The following questions from the initial analysis have been resolved:

- **Q1–Q2 (Issues API/struct unknown):** Resolved. The OpenAPI spec (`plans/openapi_2025-07-01.json`) provides full Issue schema and endpoint definitions. Section 1.2.9 now contains complete API client code, CLI commands, and MCP tool schema derived from the spec.
- **Q3 (Metrics endpoint mapping):** Resolved. Metrics will be consolidated from 7 CLI subcommands to 4 resource-aligned ones (requests, events, attempts, transformations). The CLI client handles routing to the underlying 7 API endpoints. See Section 1.2.10 for full details.

### Open Questions

#### Q1: `ListProjects()` does not accept context.Context

**What was found:** `ListProjects()` in `pkg/hookdeck/projects.go:15` uses `context.Background()` internally, unlike all other API methods which accept `ctx context.Context`.

**Recommendation:** Use the method as-is for Phase 1. MCP stdio is sequential, and ListProjects is fast. A `ListProjectsCtx` variant can be added later if needed.

#### Q2: Tool naming convention — flat vs namespaced

**What was found:** MCP tools are typically named with a prefix for namespacing (e.g., `hookdeck_projects`) to prevent collisions with other MCP servers.

**Recommendation:** Use `hookdeck_` prefix (e.g., `hookdeck_projects`, `hookdeck_connections`). This follows MCP best practices and prevents name collisions when agents use multiple MCP servers.

#### Q3: `Config.GetAPIClient()` singleton and project switching

**What was found:** `Config.GetAPIClient()` uses `sync.Once` to create a single `*hookdeck.Client`. The `projects.use` action needs to change `ProjectID` on this singleton.

**Impact:** Low. Since `ProjectID` is a public field on the pointer, `client.ProjectID = newID` works. MCP stdio is inherently sequential (one request at a time), so concurrent mutation races should not occur.

**Recommendation:** Accept the current design. Add a note in the MCP server code that ProjectID mutation is safe only because stdio transport is sequential. If SSE/HTTP transport is added later, add a mutex.

#### Q4: The `go-sdk` MCP library API surface is unknown from the codebase

**What was found:** The `go.mod` does not include `github.com/modelcontextprotocol/go-sdk`. It's a new dependency.

**Recommendation:** Write a small spike first — create a minimal MCP server with one tool to validate the SDK API before building all 11 tools. Pin to v1.2.0+.

#### Q5: Raw body responses may be very large

**What was found:** `GetEventRawBody` and `GetRequestRawBody` return `[]byte` of arbitrary size. Webhook payloads can be multi-megabyte.

**Recommendation:** Truncate with indication — return the first 100KB with a note: "Body truncated at 100KB. Full body is X bytes." This keeps MCP responses manageable for AI agents.

#### Q6: The `project use` action's scope within an MCP session

**What was found:** The CLI's `project use` persists to config files. The MCP server should not persist project changes to disk, as this would affect other CLI sessions unexpectedly.

**Recommendation:** Session-scoped only — mutate `client.ProjectID` in memory. If the MCP server restarts, the agent must call `projects.use` again. Document this behavior in the tool description.
