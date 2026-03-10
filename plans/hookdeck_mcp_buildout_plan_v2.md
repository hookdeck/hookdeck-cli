---
name: Hookdeck MCP build-out plan v2 — Investigation and Operations First
overview: "Build plan for the Hookdeck MCP server reflecting the revised scope: 11 tools (10 investigation/operational + 1 help catch-all), read-focused with lightweight operational actions (pause, unpause, retry), no CRUD or listen, CLI-local via stdio, Go SDK, actionable errors, three-layer testing. Incorporates all architectural decisions from the scope decision document and technical implementation details from the original build-out."
---

# Hookdeck MCP — Build-Out Plan v2: Investigation and Operations First

**Who this is for:** Implementers building the MCP server, PMs reviewing scope, and reviewers giving feedback. **MCP (Model Context Protocol)** is a protocol that lets AI tools call servers for tools and resources. This plan defines a server that runs as `hookdeck gateway mcp` (stdio) inside the Hookdeck CLI so agents can query production event data, investigate failures, and monitor pipeline health without opening the Dashboard.

---

## Goal

The Hookdeck MCP is the **investigation and operations layer** of Hookdeck's agentic developer experience. It sits alongside the **skills + CLI development path** (setup, scaffolding, transformation authoring, listen/tunnel lifecycle) which is already distributed through `hookdeck/agent-skills` and the Cursor plugin marketplace.

The MCP and skills + CLI are purpose-built for different contexts:

- **Building:** Developer in IDE with terminal access → skills + CLI (already available)
- **Investigating:** Someone querying production data or triaging an incident → MCP (this plan)

The MCP answers questions like "what's failing and why?" It surfaces production event data, connection health, delivery attempts, and aggregate metrics through natural language queries in any MCP-connected client (Claude UI, Cursor, ChatGPT with MCP support, Claude Code). It does not replace skills + CLI for setup workflows. When agents make action-oriented requests (create a source, start a tunnel), the `help` tool redirects them there.

---

## Scope overview

| Phase | Goal | Status |
|-------|------|--------|
| Phase 1 | 11 investigation and operational tools + `help` catch-all. Stdio only. | **This plan** |
| Phase 2 | `search_docs` tool — skills and documentation content served through MCP. Streamable HTTP transport. | Contingent on Phase 1 |
| Phase 3 | `use_hookdeck_cli` tool — CLI execution from within the MCP for environments with terminal access. | Contingent on Phase 2 |

Each phase is contingent on learnings from the previous one. If Phase 1 feedback redirects us (e.g. strong demand for write operations), the ordering changes.

**Phase 1 success criteria:** All 11 tools implemented and wired to the Hookdeck API; Layer 1 (protocol) and Layer 2 (tool integration with mock API) tests pass; and the end-to-end investigation example below runs correctly in at least one MCP client (e.g. Claude Desktop or Cursor).

**Explicitly out of scope for Phase 1:**

- CRUD/write tools (source, destination, connection create/update/delete) — handled by skills + CLI
- Listen/tunnel lifecycle — handled by skills + CLI
- `send_test_request` — handled by skills + CLI
- MCP resources (`resources/list`, `resources/read`) — Phase 2
- `search_docs` tool — Phase 2
- Streamable HTTP transport — Phase 2
- `use_hookdeck_cli` tool — Phase 3
- Hosted/cloud MCP — future

---

## End-to-end example: production investigation

This example is based on a real Claude Code session investigating event failures using the Hookdeck CLI. It's included here, translated to MCP tool calls, because it directly motivated the Phase 1 tool set. The investigation path was: metrics first to establish there's a problem, then progressively narrowing scope to find the root cause.

**Setup:** User installs Hookdeck CLI, runs `hookdeck login`, runs `hookdeck gateway mcp`, and adds the MCP to their client (e.g. Claude Desktop or Cursor settings). No additional skills installation is required for this path.

### Step 1 — Check project context

Agent calls `projects` (action: `list`) to confirm which project is active. If the correct project isn't set, it calls `projects` (action: `use`) to switch. This mirrors `hookdeck whoami` in the original CLI trace — every investigation starts from knowing what you're looking at.

### Step 2 — Get overall metrics

Agent calls `metrics` (action: `events`) with measures `count`, `failed_count`, `error_rate` over a recent window (e.g. last 24 hours). This surfaces the failure rate across the whole project. In the real trace, this revealed an elevated error rate that warranted further investigation.

### Step 3 — List failed events

Agent calls `events` (action: `list`) with `status: FAILED` to pull the recent failure set. This answers "which events are failing?" and gives the agent connection IDs and event IDs to work with in subsequent steps.

### Step 4 — Resolve the connection

From the failed events, the agent picks a `connection_id` and calls `connections` (action: `get`) to identify what that connection is: source name, destination name, rules in place, current state. In the real trace this resolved to `pagerduty-prod → api-pagerduty-prod` — which told the agent the failure was on the PagerDuty integration specifically, not a broad platform issue.

### Step 5 — Scope metrics to the connection

Agent calls `metrics` (action: `events`) again, this time with `connection_id` set to the PagerDuty connection. This confirms whether the failure rate is specific to this connection and whether it's ongoing or historical. In the real trace, the per-connection metrics showed a high and sustained failure rate since February 24th — confirming the problem was localized and not self-resolving.

### Step 6 — Inspect an event

Agent calls `events` (action: `get`) on a specific failed event ID. Returns the event body and headers. This answers "what is actually in these events?" — in the real trace, this revealed that PagerDuty was sending a variety of webhook event types, not just the ones the integration was designed to handle.

### Step 7 — Inspect the delivery attempts

Agent calls `attempts` (action: `list`) filtered by `event_id`, then `attempts` (action: `get`) on the relevant attempt. Returns the full outbound request sent to the destination and the destination's response verbatim. In the real trace, this showed the destination returning 400 errors — the internal API was rejecting unrecognized PagerDuty event types rather than silently accepting them.

### Step 8 — Root cause identified

The investigation used five tools: `projects`, `metrics`, `connections`, `events`, and `attempts`. The finding: the PagerDuty connection had a high failure rate not because the connection was broken, but because PagerDuty sends many webhook event types beyond what the integration needed, and the internal API was rejecting those unrelated events with 400s instead of responding 200 and discarding them. The fix was to acknowledge all incoming events with 200 and only process the relevant ones.

Metrics was essential at two points — discovering the overall failure rate, then confirming the per-connection rate was ongoing. Without the `metrics` tool, the agent would have had to list all events and calculate rates manually. Without `attempts`, the agent would have had the failed events but not the destination response needed to understand why they were failing.

**Note on `help`:** If at any point during this investigation the user had asked "can you update the connection to filter out the unwanted PagerDuty event types?", the agent would not find a write tool and would call `help` with that topic. The response redirects to skills + CLI: install the Hookdeck agent skills (`npx skills add hookdeck`), then use `hookdeck gateway connection upsert` to update the connection rules. The MCP surfaces the problem; skills + CLI fixes the configuration.

**Note on retry:** Replaying failed events is explicitly out of scope for Phase 1. Retry creates a new attempt, which is a data-generating write operation — unlike pause/unpause, which only affect flow control without augmenting data. Retry is a natural Phase 2 candidate if usage data shows it's needed alongside investigation.

---

# Phase 1: Investigation and operational tools

## 1. Implementation approach

This plan is intentionally high-level on API details. An agent with access to the CLI codebase should derive the exact request/response shapes, parameter names, and API mappings directly from the existing gateway command implementations (`pkg/cmd/event_*.go`, `request_*.go`, `attempt_*.go`, `metrics*.go`, etc.) — the MCP tools are wrappers over the same API client already used there, not new functionality. The codebase is the ground truth for anything not explicitly specified here.

The MCP server uses the **same internal API client** the CLI uses (`Config.GetAPIClient()`), not shelling out to CLI subprocesses. One auth story (`hookdeck login` or a CI API key); no subprocess management or stdout parsing. Tool calls use the same project/workspace context as the CLI. The agent can list and switch projects via the `projects` tool (action: `use`).

**Authentication:** Two paths, both zero-config for the agent:

1. **Pre-authenticated (typical):** User has already run `hookdeck login`. The MCP server inherits the CLI's API key and project context. All resource tools work immediately.
2. **In-band login (unauthenticated start):** If the CLI has no API key, the server registers a `hookdeck_login` tool that initiates browser-based device auth (polls for completion, persists credentials, then removes itself via `notifications/tools/list_changed`). All other tools return: `"Not authenticated. Please call the hookdeck_login tool to authenticate with Hookdeck."` until login completes. No tool succeeds silently with missing credentials.

**Suggested implementation order:**
1. MCP server skeleton and transport setup — the `initialize` handshake is handled automatically by the SDK, not something to implement as a tool; validate with `tools/list`
2. `login` tool (conditional registration when unauthenticated; enables zero-config agent onboarding)
3. `projects` tool (sets project context for all subsequent calls)
4. `connections`, `sources`, `destinations` (orientation tools)
5. `transformations` tool
6. `events` tool (list, get, raw_body)
7. `requests` tool (list, get, raw_body, events, ignored_events)
8. `attempts` tool
9. `issues` tool
10. `metrics` tool
11. `help` tool (stub early; enrich once other tools exist so responses can reference what's available)

Layer 1 and 2 tests can follow each slice. `projects` first because project context is required for all subsequent calls to be meaningful.

---

## 2. Tool surface area (12 tools)

LLM tool-calling accuracy degrades above 30-50 tools. Phase 1 ships **12 tools** — 10 investigation and operational tools, a catch-all guidance tool, and a conditional login tool. All resource tools use the **compound pattern**: a single tool name with an `action` parameter. This keeps the selection surface small while preserving per-tool capability.

The compound pattern is a testable bet. If agents consistently fail to specify an action or confuse action-specific parameters, the fallback is to expand compound tools into single-action tools (e.g. `connections_list`, `connections_get`, `connections_pause`). Layer 3 behavioral testing validates this early.

### 2.1 Tool definitions

| Tool | Actions | Primary purpose |
|------|---------|----------------|
| `projects` | `list`, `use` | Project context and org scoping |
| `connections` | `list`, `get`, `pause`, `unpause` | Primary orientation point for infrastructure |
| `sources` | `list`, `get` | Source details, URLs (e.g. "what's the URL I gave to Stripe?") |
| `destinations` | `list`, `get` | Destination details, URLs, auth config |
| `transformations` | `list`, `get` | "What does this transformation do?" |
| `requests` | `list`, `get`, `raw_body`, `events`, `ignored_events` | Inbound requests with filters, raw payload inspection, and downstream event tracing |
| `events` | `list`, `get`, `raw_body` | Events with body search, status filters, and raw payload inspection |
| `attempts` | `list`, `get` | Delivery attempts and destination responses |
| `issues` | `list`, `get` | Open issues, quick pipeline health check |
| `metrics` | `events`, `requests`, `attempts`, `transformations` | Aggregate stats over time with measures and dimensions |
| `help` | topic (string) | Catch-all guidance; redirects action-oriented requests to skills + CLI |
| `login` | *(none — single action)* | Conditional: only registered when unauthenticated; removed after successful login |

### 2.2 Tool detail: `projects`

Actions: `list` | `use`

Projects exist in the context of organizations. `list` returns all projects with org information (call `ListProjects()`, GET `/teams`; each project's `Name` is formatted as `[Organization] ProjectName`). `use` sets the active project for the MCP session via `Config.UseProject(projectId, projectMode)` (or `UseProjectLocal` for directory-scoped context).

Parameters for `use`: `project_id` (and `mode`), or `organization_name` + `project_name` resolved from list. Optional `persist_scope` (global vs local, maps to CLI's `--local` flag).

CLI reference: `hookdeck project list`, `hookdeck project use`.

### 2.3 Tool detail: `connections`

Actions: `list` | `get` | `pause` | `unpause`

`list` returns connections with name, source, destination, and status. Supports filter parameters: `source_id`, `destination_id`, `archived`, `archived_at`. `get` takes `connection_id` or `connection_name` and returns full connection details including rules and transformation reference. `pause` and `unpause` are lightweight operational actions — natural responses to what you find during investigation rather than setup operations.

**Rationale for pause/unpause:** "This connection is hammering a down destination" → pause it. "Destination is recovered" → unpause. Cutting off operational response at read-only would force a context switch to the dashboard or CLI at exactly the moment investigation concludes.

CLI reference: `hookdeck gateway connection list/get`. API: GET `/connections`, GET `/connections/{id}`, PUT `/connections/{id}/pause`, PUT `/connections/{id}/unpause`.

### 2.4 Tool detail: `sources`

Actions: `list` | `get`

`list` returns all sources with name, URL, and allowed HTTP methods. `get` takes `source_id` or `source_name` and returns full source details. The source URL is frequently what users need ("what URL do I give to Stripe?").

CLI reference: `hookdeck gateway source list/get`. API: GET `/sources`, GET `/sources/{id}`.

### 2.5 Tool detail: `destinations`

Actions: `list` | `get`

`list` returns destinations with name, URL, and auth config summary. `get` takes `destination_id` or `destination_name` and returns full destination details including auth type and rate limit config.

CLI reference: `hookdeck gateway destination list/get`. API: GET `/destinations`, GET `/destinations/{id}`.

### 2.6 Tool detail: `transformations`

Actions: `list` | `get`

`list` returns transformations with name and environment. `get` takes `transformation_id` or `transformation_name` and returns full transformation details including the JavaScript code. Used when investigating whether a transformation is dropping or mutating events.

CLI reference: `hookdeck gateway transformation list/get`. API: GET `/transformations`, GET `/transformations/{id}`.

### 2.7 Tool detail: `requests`

Actions: `list` | `get` | `raw_body` | `events` | `ignored_events`

`list` returns inbound requests with filters: `source_id`, `status`, `rejection_cause`, `verified`; plus `limit`, `next`, `prev`. `get` takes `request_id` and returns full request details including headers and parsed body. `raw_body` returns the unparsed inbound payload for a request — useful when the parsed body loses fidelity or you need the exact bytes. `events` lists the events generated from a request (the fan-out after routing). `ignored_events` lists events that were received but not routed (e.g. filtered by rules). Together these answer "did the provider send this?" and "what happened to it after ingestion?"

CLI reference: `hookdeck gateway request list/get`. API: GET `/requests`, GET `/requests/{id}`, GET `/requests/{id}/raw_body`, GET `/requests/{id}/events`, GET `/requests/{id}/ignored_events`.

### 2.8 Tool detail: `events`

Actions: `list` | `get` | `raw_body`

`list` returns events with filters: `connection_id`, `source_id`, `destination_id`, `status` (SCHEDULED, QUEUED, HOLD, SUCCESSFUL, FAILED, CANCELLED), `response_status`, `error_code`, `issue_id`, `created_after` / `created_before`; plus `order_by`, `dir`, `limit`, `next`, `prev`.

`get` takes `event_id` and returns event details including parsed body and headers. `raw_body` returns the unparsed event payload — same pattern as `requests` `raw_body`, useful when the parsed body loses fidelity.

**Enrichment decision (resolved):** `get` returns the event as-is from the API without inlining the latest attempt. The simpler approach was chosen — agents call `attempts` (action: `list`, filtered by `event_id`) when they need delivery details. This keeps the tool straightforward and avoids coupling event retrieval to attempt data.

CLI reference: `hookdeck gateway event list/get`. API: GET `/events`, GET `/events/{id}`, GET `/events/{id}/raw_body`.

### 2.9 Tool detail: `attempts`

Actions: `list` | `get`

`list` returns delivery attempts with filters: `event_id`, `status`, `response_status`, `created_after` / `created_before`; plus `order_by`, `dir`, `limit`. `get` takes `attempt_id` and returns full attempt details including the outbound request (method, URL, headers, body sent to the destination) and the destination response (status code, headers, body). This is the deepest level of delivery investigation.

CLI reference: `hookdeck gateway attempt list/get`. API: GET `/attempts`, GET `/attempts/{id}`.

### 2.10 Tool detail: `issues`

Actions: `list` | `get`

`list` returns open issues with filters: `type` (e.g. delivery, transformation), `status`, `connection_id`, `created_after` / `created_before`. Returns aggregated failure signals rather than individual events. Quick health check: "are there active issues on my pipeline?" `get` takes `issue_id` and returns issue detail including related events and timeline.

API: GET `/issues`, GET `/issues/{id}`.

### 2.11 Tool detail: `metrics`

Actions: `events` | `requests` | `attempts` | `transformations`

Returns aggregate stats over time. The current Hookdeck API has 7 separate metrics endpoints that conflate measures, dimensions, and resource types. The MCP tool abstracts over this with a clean 4-action interface, mapping internally to whatever endpoints exist. The API design inconsistency does not need to block Phase 1.

Each action supports: `connection_id` (or `source_id` / `destination_id` / `transformation_id` for scoping), time range (`period`, `from`, `to`), `measures` (count, failed_count, error_rate, etc.), and `dimensions` for grouping. The agent used metrics at two points in the real investigation trace: once for overall pipeline health, once scoped to a specific connection to confirm an ongoing failure rate.

CLI reference: `hookdeck gateway metrics events/requests/attempts`. API: GET `/metrics/events`, GET `/metrics/requests`, GET `/metrics/attempts`, GET `/metrics/transformations`.

### 2.12 Tool detail: `login`

Action: none (single-action tool, no `action` parameter)

**Conditional registration:** Only added to the tool list when the MCP server starts without an API key (`client.APIKey == ""`). After successful login, the tool is removed via `mcpServer.RemoveTools("hookdeck_login")`, which sends `notifications/tools/list_changed` to clients that support dynamic tool updates.

**Flow:** Calls `StartLogin()` to initiate browser-based device auth → returns the browser URL for the user to open → polls `WaitForAPIKey()` at 2-second intervals (up to ~4 minutes) → on success, persists credentials to the CLI profile, updates the shared client's `APIKey` and `ProjectID`, and removes itself from the tool list. On timeout, returns the browser URL again so the user can retry.

This tool bridges the gap between "user installed the CLI but hasn't logged in yet" and "all MCP tools require auth." Without it, an agent encountering the auth error would have no way to resolve the situation within the MCP session.

### 2.13 Tool detail: `help`

Action: `topic` (string parameter)

The catch-all entry point when no other tool fits the request. Two behaviors:

1. **Action-oriented requests (create, update, delete, listen, scaffold):** Returns installation and workflow guidance pointing to skills + CLI. Example response for topic `"create connection"`: "Creating and managing connections isn't available through the MCP — that's handled by the Hookdeck CLI with agent skills. Install the skills: `npx skills add hookdeck`. Then follow the setup workflow at `hookdeck://event-gateway/references/01-setup`. The CLI command is `hookdeck gateway connection upsert`."

2. **Ambiguous or unknown operational queries:** Returns pointers to the relevant MCP tool. Example for topic `"filter events by payload field"`: "Use the `events` tool (action: `list`) with a `body` filter parameter. Hookdeck's event search supports JSON path filtering on the event body."

The `help` tool generates the signal that tells us what Phase 2 should be. Calls to `help`, and the topics passed to it, are the primary feedback mechanism for understanding unmet needs.

**Server-level description:** Set a clear MCP server description so clients display it correctly: "Hookdeck MCP — investigation and operational tools for querying event data, inspecting delivery attempts, checking pipeline health, and performing lightweight operational actions (pause, unpause). For setup, scaffolding, and development workflows, use skills + CLI: `npx skills add hookdeck`."

---

## 3. Error handling

Every tool call returns a **clear, actionable error message** the agent can reason about. No generic errors.

| Failure | Tool response |
|---------|--------------|
| Auth missing | `"Not authenticated. Please call the hookdeck_login tool to authenticate with Hookdeck."` (When `hookdeck_login` is available, the agent can resolve this in-band.) |
| Auth failed (401) | `"Authentication failed. Check your API key."` |
| Resource not found (404) | `"Resource not found: {API message}"` |
| Validation error (422) | API error message passed through verbatim. |
| Rate limit (429) | `"Rate limited. Retry after a brief pause."` |
| API 5xx | `"Hookdeck API error: {API message}"` |

**Note:** The error translation layer (`TranslateAPIError`) maps `*hookdeck.APIError` status codes to these messages. Non-API errors are returned unchanged.

**Rate limiting:** Rely on the API's 429 responses. Surface the `Retry-After` header to the agent. No client-side rate limiting or queuing in Phase 1.

---

## 4. Logging

Structured logging to **stderr** via Go's `slog` package (stdout is reserved for JSON-RPC in stdio mode).

- **INFO:** Lifecycle events (startup, shutdown, project context changes).
- **WARN:** Recoverable issues (API timeouts with retry).
- **ERROR:** Failures.

`--verbose` flag on `hookdeck gateway mcp` enables **DEBUG** level: individual tool calls, API requests and responses. Use for development and support.

---

## 5. Testing strategy

Three-layer approach. The CLI and API client are already well-tested; focus is on the MCP-specific layer.

**Layer 1 — Protocol compliance:** Use the official Go SDK's client (`mcp.NewClient` + `mcp.CommandTransport`) to test: `initialize` handshake, `tools/list` schema correctness, and proper error responses for invalid parameters.

**Layer 2 — Tool integration (mock API client):** Mock the API client at the interface boundary. Test that each tool maps inputs to the right API call, maps responses back correctly, and surfaces errors (4xx, 5xx, 429) with actionable messages. Target 3-5 tests per tool (33-55 tests for 11 tools). Priority tools for early coverage: `projects` (project context is a prerequisite for other tools), `events` (most complex filter set), `metrics` (API abstraction complexity), `help` (output correctness for different topic categories).

**Layer 3 — Behavioral (manual / semi-automated):** Test with real LLM agents: measure tool hit rate (does the agent pick the right tools?), compound action accuracy (does the agent correctly specify action parameters?), and unnecessary call rate. The PagerDuty investigation trace (8 steps using `projects`, `metrics`, `connections`, `events`, `attempts`) is a concrete behavioral test scenario to run end-to-end. Informs whether the compound tool design works or whether specific tools need to be split into single-action tools.

Use **MCP Inspector** (`npx @modelcontextprotocol/inspector`) for manual validation during development.

---

## 6. Package location and Go MCP stack

- **Package location:** `pkg/gateway/mcp`. Gateway-scoped, consistent with the original plan. Outpost MCP is out of scope; reuse (e.g. transport, error handling) may be considered later.
- **Command:** `hookdeck gateway mcp`. Gateway-scoped for the same reason — all Event Gateway resources live under this namespace, and scoping here preserves the option to restrict the MCP to Event Gateway projects in future.
- **Go MCP library:** Use the official `modelcontextprotocol/go-sdk` (v1.2.0+). Stable with a formal backward-compatibility guarantee, maintained by the MCP organization and Google, supports the 2025-11-25 spec, first-class stdio and streamable HTTP transports sharing the same server implementation.
- **Transport:** **Phase 1 is stdio only.** Stdio covers Claude Desktop, Cursor, Claude Code, Windsurf, Cline, and current AI coding tools. The SDK makes adding HTTP straightforward later since the server is transport-agnostic.

---

---

## 7. Missing features for consideration

The following items are specified in the plan but not yet implemented, or are gaps discovered during implementation. Each is listed with context to help decide whether to implement now (Phase 1), defer to Phase 2, or skip.

### 7.1 Help tool: skills + CLI redirect for action-oriented requests

**Plan reference:** Section 2.13 — when someone asks about create/update/delete/listen/scaffold, help should return guidance like *"Creating and managing connections isn't available through the MCP — that's handled by the Hookdeck CLI with agent skills."*

**Current state:** The help tool only returns tool reference documentation. If you ask about a topic that doesn't match a tool name, it returns an error saying the topic parameter expects a tool name. There is no natural-language routing and no skills/CLI redirect.

**Impact:** This is the primary mechanism for bridging MCP users to write operations. Without it, agents hitting the boundary of what's available get a dead-end error instead of actionable guidance. The plan also identifies `help` call topics as the feedback signal for Phase 2 priorities.

**Effort:** Medium — needs a category-matching layer (keyword or pattern-based) and a set of redirect response templates.

### 7.2 Server-level description

**Plan reference:** Section 2.13 — set a description so MCP clients display context about the server's purpose.

**Current state:** The server only sets `Name: "hookdeck-gateway"` and `Version`. No description field.

**Impact:** Low-medium. Clients like Claude Desktop show the server description to help users understand what's available. Without it, users see just the name.

**Effort:** Trivial — one field on `mcpsdk.Implementation` or server options.

### 7.3 Structured logging via slog

**Plan reference:** Section 4 — structured logging to stderr via `slog`, INFO/WARN/ERROR levels, `--verbose` flag for DEBUG.

**Current state:** No logging in the MCP server code at all.

**Impact:** Medium for debugging and support. Without it, diagnosing issues in production or during development requires adding ad-hoc prints. The `--verbose` flag is particularly useful for MCP Inspector workflows.

**Effort:** Medium — add slog setup in `NewServer`, pass logger through to handlers, add `--verbose` flag to the `hookdeck gateway mcp` command.

### 7.4 "No project selected" validation

**Plan reference:** Section 3 error table — *"No project selected. Use projects (action: use) to set the active project."*

**Current state:** Tools call the API without checking if `client.ProjectID` is set. The API may return confusing errors or default project data.

**Impact:** Medium. Without this guard, agents get opaque API errors when project context is missing instead of clear guidance to call `projects (action: use)`.

**Effort:** Low — add a `requireProject()` check similar to `requireAuth()`, call it from each tool handler.

### 7.5 Projects tool: `organization_name` + `project_name` resolution and `persist_scope`

**Plan reference:** Section 2.2 — resolve project by org + name, optional `persist_scope` (global vs local).

**Current state:** Only supports `project_id` for the `use` action.

**Impact:** Low-medium. Agents can work around this by calling `list` first to find the ID. `persist_scope` is mostly relevant for multi-directory workflows.

**Effort:** Low for name resolution (lookup from list results). Low for `persist_scope` (maps to existing `UseProject` vs `UseProjectLocal`).

### 7.6 Richer error messages for not-found and bad-request

**Plan reference:** Section 3 — *"Connection web_G79G7nNUYWTa not found. Use connections (action: list) to see available connections."* and *"Invalid filter parameter 'statuss'. Valid values for status: ..."*

**Current state:** Not-found returns `"Resource not found: {API message}"`. Validation errors pass through the API message verbatim. Neither includes next-step guidance (e.g. suggesting the `list` action).

**Impact:** Low-medium. The current errors are functional but not as agent-friendly. Adding "try X instead" guidance helps agents self-correct.

**Effort:** Low — enhance `TranslateAPIError` to include tool-aware suggestions for 404 and 422.

### 7.7 Rate limit: surface `Retry-After` header value

**Plan reference:** Section 3 — *"Rate limited. Retry after {N} seconds."*

**Current state:** Returns `"Rate limited. Retry after a brief pause."` — no specific duration from the API response.

**Impact:** Low. The generic message works, but surfacing the actual value lets agents wait precisely.

**Effort:** Low — extract `Retry-After` from `APIError` (if the API client exposes it) and include in the message.

---

# Phase 2: Documentation and skills content

*Contingent on Phase 1 usage data and feedback from the `help` tool.*

## 7. `search_docs` tool

Add a dedicated tool for searching Hookdeck documentation and skills content. This makes skills content available in MCP-connected contexts without requiring separate installation — an agent connected to the Hookdeck MCP can read setup and workflow knowledge directly, and if it has terminal access, execute CLI commands based on that knowledge.

Two design patterns to evaluate before building:

- **Single-tool (Vercel, Supabase pattern):** One `search_docs` tool takes a `query` and optional `tokens` limit; returns relevant content inline. Simpler, lower tool count.
- **Multi-tool (Inngest, AWS pattern):** Separate tools for `list_docs` (browse structure), `grep_docs` (search by pattern), `read_doc` (load by path). More agent control over content loading, but adds 2-3 tools to the count.

The `help` tool in Phase 1 is specifically designed to surface which topics users actually ask about. That signal informs which pattern and which content to prioritize in Phase 2.

Phase 2 also adds **streamable HTTP transport** (`hookdeck gateway mcp serve`), enabling hosted or remote MCP connections. This is the prerequisite for a future cloud-hosted MCP that works in environments where the CLI can't be installed.

## 8. Repository structures and URI mapping

The URI schemes and resolution rules below are the implementation spec for Phase 2. Phase 1 references these URIs only as strings inside `help` tool responses — no fetching or resolution happens in Phase 1.

### 8.1 hookdeck/agent-skills

Repo: https://github.com/hookdeck/agent-skills — staged workflow (01-setup through 04-iterate) and reference material for Event Gateway and Outpost.

**URI scheme `hookdeck://`** — path after the host = path under `skills/` in agent-skills.

| URI | Repo path |
|-----|-----------|
| `hookdeck://event-gateway/SKILL` | `skills/event-gateway/SKILL.md` |
| `hookdeck://event-gateway/references/01-setup` | `skills/event-gateway/references/01-setup.md` |
| `hookdeck://event-gateway/references/02-scaffold` | `skills/event-gateway/references/02-scaffold.md` |
| `hookdeck://event-gateway/references/03-listen` | `skills/event-gateway/references/03-listen.md` |
| `hookdeck://event-gateway/references/04-iterate` | `skills/event-gateway/references/04-iterate.md` |

### 8.2 hookdeck/webhook-skills

Repo: https://github.com/hookdeck/webhook-skills — provider-specific webhook skills (Stripe, Shopify, GitHub, etc.) and webhook-handler-patterns.

**URI scheme `webhooks://`** — path after the host = path under `skills/` in webhook-skills.

| URI | Repo path |
|-----|-----------|
| `webhooks://stripe-webhooks/references/overview` | `skills/stripe-webhooks/references/overview.md` |
| `webhooks://stripe-webhooks/references/verification` | `skills/stripe-webhooks/references/verification.md` |

### 8.3 Resolver

- `hookdeck://` + path → `https://raw.githubusercontent.com/hookdeck/agent-skills/main/skills/` + path (append `.md` when no file extension)
- `webhooks://` + path → `https://raw.githubusercontent.com/hookdeck/webhook-skills/main/skills/` + path (same rule)

### 8.4 Content delivery and caching

Skill content is not bundled into the CLI binary. Content evolves independently of CLI releases, so it must be fetched and kept fresh.

**Index source for `resources/list`:**
- For `webhooks://`: fetch `providers.yaml` from webhook-skills at startup. Derive resource list from it.
- For `hookdeck://`: fetch `skills.yaml` from agent-skills at startup (create this manifest as part of Phase 2 work in agent-skills). Derive resource URIs from it.

Both schemes use the same startup pattern: fetch manifest, derive resource list, cache. This avoids hardcoding paths in the CLI.

**Content fetching:** Lazy on first `resources/read` for a given URI, then cached for the session. Use ETags or `If-Modified-Since`; fall back to last cache on fetch failure so the MCP can still serve if GitHub is unavailable.

**Cache location:** `~/.hookdeck/mcp/cache/`. Set `Annotations.LastModified` on resources so agents can see when content was last refreshed.

**Open:** Evaluate a Hookdeck-controlled endpoint (e.g. `skills.hookdeck.com`) instead of GitHub raw URLs to decouple from GitHub availability.

---

# Phase 3: CLI execution via MCP

*Contingent on Phase 2. Most speculative phase.*

## 9. `use_hookdeck_cli` tool

For environments where the agent has both MCP and terminal access (Claude Code, Cursor), this tool delegates execution to the CLI. The MCP provides investigation tools (Phase 1), procedural knowledge (Phase 2), and execution capability (Phase 3). At this point, the MCP becomes a single agent integration for both investigation and development.

This phase depends on Phase 2 proving that serving skills content through MCP is valuable, and on seeing demand from users who have both MCP and terminal access and want a unified agent integration rather than separately installed skills.

If Phase 1 shows that users immediately want write operations through the MCP rather than a CLI delegation tool, write tools are added before Phase 3. The phasing reflects current hypothesis, not a fixed plan.

---

# Summary of decisions

| Topic | Decision |
|-------|----------|
| **Scope** | 12 tools: 10 investigation/operational (read + pause/unpause) + 1 `help` catch-all + 1 conditional `login`. No CRUD, no listen, no retry. |
| **Primary use case** | Investigation and production monitoring. Not development/setup. |
| **Development path** | Skills + CLI (already available). MCP does not duplicate this. |
| **Compound tools** | Single tool with action parameter. Testable bet; fallback to split tools if agents struggle with action selection. |
| **`help` tool** | Catch-all for out-of-scope requests. Returns skills + CLI redirect for write/setup operations. Generates signal for Phase 2 priorities. |
| **MCP resources** | Not served in Phase 1. URI schemes (`hookdeck://`, `webhooks://`) and content delivery infrastructure are Phase 2. |
| **Tools implementation** | Same API client as gateway commands (`Config.GetAPIClient()`); no CLI subprocess. |
| **Package** | `pkg/gateway/mcp` (gateway-scoped). |
| **Command** | `hookdeck gateway mcp`. |
| **Go MCP** | Official `modelcontextprotocol/go-sdk` (v1.2.0+). |
| **Transport** | Phase 1: stdio only. Phase 2: streamable HTTP. |
| **Auth** | Two paths: inherited from CLI (pre-authenticated), or in-band `hookdeck_login` tool (browser-based device auth, self-removing after success). Clear error if missing. |
| **Error handling** | Actionable messages for every failure. Rate limit: surface API Retry-After. Write tool requests: redirect to skills + CLI. |
| **Logging** | Structured stderr via `slog`; INFO/WARN/ERROR; `--verbose` for DEBUG. |
| **Testing** | Three layers: protocol compliance, tool integration (mock API), behavioral (manual/semi-automated). MCP Inspector for manual validation. |
| **Hosted MCP** | Deferred. Starting CLI-local to avoid hosting infrastructure and because auth is trivially inherited from CLI login. |
| **Phase 2** | `search_docs` tool + URI/resource infrastructure + streamable HTTP. Contingent on Phase 1. |
| **Phase 3** | `use_hookdeck_cli` tool. Most speculative; contingent on Phase 2. |

---

# References

- **Scope decision:** Hookdeck MCP Scope Decision: Investigation and Operations First (internal Notion doc)
- **CLI gateway:** `pkg/cmd/gateway.go`; event/request/attempt/metrics in `pkg/cmd/event_*.go`, `request_*.go`, `attempt_*.go`, `metrics*.go`. All use API client (`Config.GetAPIClient()`).
- **Go MCP SDK:** https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp; HTTP example: https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/examples/http
- **Agent skills:** https://github.com/hookdeck/agent-skills
- **Webhook skills:** https://github.com/hookdeck/webhook-skills
- **Hookdeck OpenAPI spec:** `https://api.hookdeck.com/2025-07-01/openapi` or CLI's cached spec
- **MCP Inspector:** `npx @modelcontextprotocol/inspector`
