---
name: MCP read/write tool split and platform tools
overview: "Split the embedded MCP servers so read actions and gated write actions live on separate tool names, and add platform-level tools (organization, projects, API keys) now that the Hookdeck API exposes them. Ships as v3.0.0-beta.2. Tool names change because MCP clients grant permission per tool name and the current compound pattern makes 'allow reads, prompt on writes' inexpressible; platform tools arrive in the same beta so the naming rule is applied once, across everything, before GA."
---

# MCP read/write tool split and platform tools

**Status:** in progress
**Target:** `v3.0.0-beta.2`, branch `release/v3.0.0`
**Baseline:** `v3.0.0-beta.1`
**Blocked on:** merging `main` (v2.6.0) — see checklist step 0

Two workstreams ship together:

1. **Tool naming** — split every resource tool into `_read` / `_write` halves.
2. **Platform tools** — expose the organization, projects and API key endpoints the API
   now offers, named under the same rule from the start.

They ship together because the naming rule should be applied once, to the whole surface. Adding
platform tools after the rename would mean naming them under a convention that had just changed,
or changing them again later.

---

## Problem

MCP clients grant tool permission **per tool name**, with wildcards over names and no
argument matching. The tool name is the only permission boundary that exists.

The Event Gateway and Outpost servers use a compound pattern: one tool per resource, with an
`action` enum whose contents depend on `--allow-write`. `gateway_connections` in write mode
accepts `list`, `get`, `pause`, `unpause`, `create`, `upsert`, `update`, `delete`, `enable`,
`disable`.

So "let the agent read connections freely, but ask before it changes one" cannot be expressed.
A user can allow `gateway_connections` entirely, which permits deleting a live connection
unprompted, or deny it and be prompted on every read.

Two aggravating details in the current code:

- `destructiveHint` is computed per tool, not per action (`pkg/mcpcore/toolspec.go`), so one
  destructive action flags the whole tool and a client gating on the hint prompts on `list`.
- `ReadOnlyHint` is false even in read-only mode for `gateway_connections`, because
  `pause`/`unpause` are `Mutates: true`. Accurate, but it means the read-only tool cannot be
  blanket-allowed either.

Platform tools raise the stakes rather than changing the problem. `POST /organizations/current/api-keys`
mints credentials. There is no version of "allow the agent to read my projects" that should also
permit it to create an organization API key, and under the current pattern those would be the
same grant.

### Why this beta

v3.0.0 already renames every Gateway tool from `hookdeck_*` to `gateway_*`. Per-tool grants and
`allowedTools` entries do not survive a rename, so every user re-grants on upgrade regardless.
Splitting after GA would force a second re-grant. This is the last point where the naming
change, the platform additions, and the existing rename all cost one disruption instead of three.

---

## Decision: naming

**Symmetric `_read` / `_write` suffixes on every resource tool**, with ungated mutations on
their own third tool.

```
gateway_connections_read        list, get                       ReadOnly  not destructive
gateway_connections_pause       pause, unpause                  mutating  not destructive
gateway_connections_write       create, upsert, update,         mutating  destructive
                                delete, enable, disable
```

Rules:

1. Every resource tool carries `_read` or `_write`, with no exceptions — including resources
   that have no write half (`gateway_metrics_read`, `outpost_catalog_read`) and the one that is
   entirely write (`outpost_publish_write`).
2. An action enum must never mix read and write. Each tool's actions are homogeneous.
3. `_write` tools exist only under `--allow-write`. `_read` tools are byte-identical in both
   modes, which is what makes a grant durable.
4. `ReadOnlyHint` is driven by `HasChanging()`, not `HasWrite()`, so a tool carrying an ungated
   mutation still cannot claim to be a pure read.
5. An ungated mutation gets its own tool rather than sitting on either half
   (`gateway_connections_pause`, `hookdeck_projects_use`).

### Why symmetric rather than a bare read name

The asymmetric alternative — reads keep their existing bare names, `_write` is added — was the
initial recommendation because it minimises churn. Evidence moved the decision; see
[Evidence](#evidence). Summarised:

- **It matches the API's own permission vocabulary.** Hookdeck API keys carry scopes shaped
  `gateway.events.read`, `gateway.events.write`, `gateway.sources.read`. The platform already
  draws its permission boundary at read/write, per resource, per product, and names it exactly
  that way. `gateway_events_read` / `gateway_events_write` mirrors `gateway.events.read` /
  `gateway.events.write` character for character apart from the separator. A user reasoning
  about what a key can do and what a tool can do uses one vocabulary. This is the strongest
  argument and it postdates the original recommendation.
- **Permission expressibility.** "Allow all reads" is `*_read` — one rule, on any client. Under
  the asymmetric rule, `gateway_*` also matches the write tools, so an allowlist-only client
  must enumerate every read tool by hand (20 entries, measured).
- **No exception to memorise.** Under the asymmetric rule the name `outpost_publish` would be
  bare *and* write-only, contradicting the "bare means read" rule every other tool teaches, at
  the one tool where a wrong guess has real side effects.
- **No accidental inconsistency.** Under a split-only rule, `gateway_issues_read` and
  `gateway_attempts` are the same shape of tool named differently, purely because
  `gateway_issues_write` happens to exist. That invites hallucinated names like
  `gateway_attempts_read`.

Accepted costs:

- Read tools are renamed a second time in one release. Mitigated by v3.0.0 already forcing a
  re-grant.
- `gateway_metrics_read` carries a suffix distinguishing it from a write half that does not exist.
- Uniform suffixes make `gateway_metrics_read` (aggregate queries) look structurally identical
  to a plain CRUD read. Counter this in tool descriptions, not in names.
- `gateway_connections_pause` and `hookdeck_projects_use` match neither suffix, and are
  conspicuous precisely because everything else is regular.

### Why pause/unpause get their own tool

`pause` and `unpause` mutate but are deliberately not gated: read-only is the mode people
investigate incidents in, pausing a misbehaving connection is usually how that investigation
ends, and pausing buffers rather than drops. Reasoning is in `pkg/gateway/mcp/tool_connections.go`;
`TestWriteGuard_PauseIsNotGated` pins it.

Leaving them on the read tool would keep `gateway_connections_read` un-blanket-allowable, which
defeats most of the point. Moving them to the write tool would gate them, changing what is
allowed. A third tool keeps all three postures honest.

Recorded as considered: GitHub's `pull_request_review_write` carries `resolve_thread` /
`unresolve_thread` — a reversible, low-stakes, paired state change structurally like
`pause`/`unpause` — on the **write** tool, accepting that it disappears in read-only mode. Our
carve-out goes the other way on a deliberate product judgement about incident response.

---

## Decision: platform tools

### What the API now offers

Verified against the live spec at `https://api.hookdeck.com/2026-09-01/openapi` (95 paths),
diffed against the pinned `plans/openapi_2025-07-01.json` (83 paths). New platform endpoints:

| Endpoint | Methods | Notes |
|---|---|---|
| `/organizations/current` | GET, PUT | PUT body: `name` only |
| `/organizations/current/api-keys` | GET, POST | POST: `label`, `type` (`organization`\|`project`), `team_id`, `scopes`, `grants`. Creating organization keys **requires an admin session**. |
| `/organizations/current/api-keys/{id}` | PUT, DELETE | PUT changes scopes/grants, secret unchanged. DELETE stops authenticating **immediately**. |
| `/organizations/current/api-keys/{id}/roll` | POST | `delay_sec` required; old key expires after the delay. |
| `/projects` | GET, POST | POST: `name`, `organization_id`, `private`, `type` (`event_gateway`\|`outpost`) |
| `/projects/{id}` | GET, PUT, DELETE | PUT: `headers_prefix`, `domain`, `name`, `context`, `notification_methods`, `webhook_topics`, `webhook_source_id`, `private` |
| `/projects/{id}/custom_domains` | GET, POST | Replaces `/teams/current/custom_domains` |
| `/projects/{id}/custom_domains/{domain_id}` | DELETE | |

Also new, and **not platform** — Gateway features to triage separately, not in this plan:
`POST /events/{id}/replay`, `POST /requests/{id}/replay`, and the `/bulk/requests/replay` set.

### Prerequisite: API version bump — already done on `main`

`main` merged `feat/api-2026-09-01` (PR #378) and shipped it in **v2.6.0**. On `main`:

- `pkg/hookdeck/client.go` — `const APIPathPrefix = "/2026-09-01"`
- `pkg/hookdeck/projects.go` — lists via `GET /projects`, no longer the undocumented `/teams`

`release/v3.0.0` still pins `/2025-07-01`, so **merging `main` is the first task** and it delivers
this prerequisite. No separate version-bump work is needed, and the "wholesale vs per-call"
question this plan previously carried is moot.

What the merge does *not* bring: there is no client code for organizations or API keys on `main`.
Those endpoints still need `pkg/hookdeck/` methods written here.

Two details the bump carries, both already absorbed on `main`:

- **`/teams` is gone in 2026-09-01**, and was undocumented in 2025-07-01 too — the CLI depended
  on an unspecified endpoint. `GET /projects` replaces it, so this fixed a latent fragility.
- **Custom domains moved** from `/teams/current/custom_domains` to `/projects/{id}/custom_domains`.
  This does **not** affect Outpost's `custom_domain_*` actions, which use the separate Outpost
  path `/config/custom_domain` (`pkg/hookdeck/outpost_config.go`). Confirmed by reading both.

### Proposed platform tools

Platform tools keep the `hookdeck_` prefix (`mcpcore.DefaultPlatformPrefix`), because you log in
to Hookdeck, not to a product. They are registered on **both** servers.

| Tool | Actions | Mode |
|---|---|---|
| `hookdeck_projects_read` | `list`, `get` | both |
| `hookdeck_projects_use` | `use` | both |
| `hookdeck_projects_write` | `create`, `update`, `delete`\* | `--allow-write` |
| `hookdeck_organization_read` | `get` | both |
| `hookdeck_organization_write` | `update` | `--allow-write` |

`*` destructive.

**API key management is deliberately excluded.** `/organizations/current/api-keys` is not exposed
as an MCP tool in any form — not even `list`. Credentials should not flow to agents, and key
management belongs on the CLI, the API, or the dashboard where a human is the one holding them.
This is stricter than the read/write split alone would require: the split would happily permit a
read-only `list`, and the decision is to not offer it regardless. An agent that could mint a
write-scoped key would defeat every other boundary in this plan, and the cheapest way to
guarantee it cannot is to leave the surface off entirely.

This replaces today's single `hookdeck_projects` (`list`, `use`) and resolves the open question
the previous revision of this plan carried: `list` is a read, `use` changes what every
subsequent call targets, and permission is per tool name, so the safe read could not be granted
without the state change. Splitting resolves it under the same rule as everything else.

**`use` gets its own tool**, for the same reason as `pause`. It mutates session state rather
than remote data, an agent needs it in read-only mode to investigate a different project, and
putting it on the read tool would make `hookdeck_projects_read` un-blanket-allowable — the exact
failure this change exists to fix.

**Project type.** `POST /projects` takes `type: event_gateway | outpost`. The Gateway server
should default it to `event_gateway` and the Outpost server to `outpost`, rather than requiring
an agent to supply a value it has no way to infer.

### Tool count

| Server | Today | After split | After platform |
|---|---|---|---|
| Event Gateway | 14 | 22 | **26** |
| Outpost | 11 | 16 | **20** |

Five platform tools replace today's single `hookdeck_projects`, so +4 per server. Inside the
30-50 band section 2 of `hookdeck_mcp_buildout_plan_v2.md` protects, though Gateway at 26 has
limited headroom. The two servers are normally configured separately, so the per-server number
is the relevant one.

---

## Evidence

Four agents were run against the candidate naming schemes. Three saw one scheme each, in
isolation, with no indication that alternatives existed or that naming was under evaluation;
they were handed a tool list and asked to route ten realistic requests and then write permission
rules. A fourth compared all three with the schemes in shuffled order.

### Routing accuracy does not discriminate

All three schemes scored **10/10**. No scheme produced a misroute. The separation is entirely in
permission ergonomics and self-reported friction.

### Permission rules required for "allow all reads, prompt on writes"

| Scheme | allow + deny | allowlist-only |
|---|---|---|
| Asymmetric (bare read names) | 4 rules | **20 enumerated entries** |
| **Symmetric (chosen)** | **3 rules** | **`*_read` + `gateway_help` — 2 entries** |
| Split-only symmetric | **15 rules** | 10 entries |

The split-only agent considered the wildcard and rejected it: `*_write` covered 9 of 15, "so an
explicit list is simplest and safest overall." A partial convention is worse than none, because
it invites a wildcard that silently under-covers.

### Findings independent of scheme

All three blind agents raised these unprompted:

- **`hookdeck_projects` mixes `list` with `use`.** Resolved by the platform split above.
- **`run` on transformations reads as a write.** It persists nothing and correctly stays on the
  read tool, but the name gives no signal; one agent only found it after checking the write tool.
- **`retry` appears on three tools** (`gateway_request_write`, `gateway_event_write`,
  `outpost_events_write`). Only the user's noun disambiguates.
- **`pause` competes with `disable`.** All three picked `pause` for "stop delivery right now"
  and called `disable` defensible. Supports keeping the reversible option ungated.
- **`metrics.events` vs `metrics.attempts`** is ambiguous for "how many were delivered" — an
  attempt is the delivery try, an event the logical unit.

### Alternatives rejected

**A third destructive tier** (`_write` plus `_destructive`, so deletes gate separately). Two
blind agents noted that `_write` bundles `delete` with `disable`. Rejected: the closest analogue
does the same. GitHub's `pull_request_review_write` method enum is
`create, submit_pending, delete_pending, resolve_thread, unresolve_thread` — a real delete beside
a create — and `sub_issue_write` is `add, remove, reprioritize`. Verified by reading the shipped
schemas. No server separates destructive operations onto their own dispatcher name, and the
split would push Gateway past the tool-count band for a distinction nothing else makes.

**Verb-first per-operation tools** (`gateway_create_connection`), which would give read/write
separation structurally and need no action enum at all. This is the fallback
`hookdeck_mcp_buildout_plan_v2.md:130` already names for the compound pattern, with the stated
trigger: "if agents consistently fail to specify an action or confuse action-specific
parameters." The blind test is the evidence against pulling it — 30/30 correct routing, zero
action-selection failures. The compound pattern passes its own criterion. What it fails is
permissions, which was not a consideration when the bet was written, and which a naming change
fixes without abandoning the pattern.

Worth revisiting separately: that plan's "accuracy degrades above 30-50 tools" is the constraint
the compound pattern exists to respect, and it is unverified here. Servers shipping today run
well above it (Riverside 68 tools, n8n 54, GitHub ~90 across toolsets). Observing that vendors
ship those counts is not the same as measuring accuracy at them. Re-test before the next MCP
expansion — and note the platform additions put Gateway at 28, so the next expansion is close.

**Server-level split** (a separate write-only instance, so the server name is the boundary).
Deferred. Write mode is currently additive, so a second instance would expose reads twice and
duplicate the tool list in context. Making it write-only requires exactly the homogeneous tools
this change produces, so it is downstream of this work, not instead of it.

---

## Implementation checklist

### 0. Merge `main` (v2.6.0) into `release/v3.0.0` — do this first

65 commits behind, 77 ahead. 25 conflicted files, 8 of them in the MCP packages this plan
rewrites. Merging after the split would mean resolving those same conflicts twice, the second
time against code that had just been restructured.

- [ ] Merge `origin/main`; brings `APIPathPrefix = "/2026-09-01"` and `GET /projects`
- [ ] Port `d5836cd` (`fix: resolve MCP active project name for project-scoped keys`, +136 lines)
      into `pkg/mcpcore/project_display.go` — this branch moved the file there, `main` fixed it in
      the old `pkg/gateway/mcp/` location, so git reports modify/delete and the fix must be
      carried across by hand or it is silently lost
- [ ] Resolve the MCP conflicts: `tools.go` (197 lines main-side), `tool_help.go` (90),
      `tool_metrics.go` (117), `tool_requests.go` (64), `tool_events.go` (45), `server_test.go`,
      `pkg/mcpcore/tool_projects_errors.go`
- [ ] Resolve the rest: `pkg/config/project_type.go` (121), `pkg/hookdeck/client.go` (65),
      `transformations.go`, `projects_test.go`, `pkg/login/claimed_cli_key.go`,
      `pkg/cmd/{root,event_list,request_list,transformation_run,destination_common,destination_update}.go`
- [ ] Non-code: `package.json` (version), `REFERENCE.md` (regenerate, do not hand-merge),
      `AGENTS.md`, `.github/workflows/acceptance.yml`
- [ ] `go build ./... && go test ./...` green before any split work starts

### 1. `pkg/mcpcore/`

- [ ] `ActionSet`: add write-only and read-only selection alongside the existing additive
      `Available(writeEnabled)`. Keep `ActionSet` the single source of truth — do not duplicate
      action lists across two hand-written specs, they will drift.
- [ ] `ToolSpec.Define`: render the read tool and, under write mode, the write tool.
- [ ] `VisibleProps`: filter props per **tool**, not per mode. The read tool must not advertise
      write-only props (`rules`, `config`); the write tool should not advertise read-only
      filters it cannot use. Check `Prop.Write` handling.
- [ ] Annotations per tool: `ReadOnlyHint` true exactly when no action changes state;
      `DestructiveHint` true exactly when that tool carries a destructive action. Keep
      `HasChanging()` driving `ReadOnlyHint`.
- [ ] `rejectUnknownArgs`: preserve the carve-out that lets a hidden-by-mode argument through
      when the requested action is itself hidden. A write action requested on the read tool in
      read-only mode must still produce the "restart with `--allow-write`" message, not an
      unknown-tool or unknown-arg error.
- [ ] `help.go` / `HelpTopic`: topics for the new tool names.

### 2. Split existing tools

- [ ] `pkg/gateway/mcp/tools.go`, `pkg/outpost/mcp/tools.go`: `resourceSpecs()` / `toolDefs()`.
      Register each `_write` tool immediately after its `_read` counterpart so the pairing is
      visible in the advertised order.
- [ ] `tool_*.go`: `ActionSet` declarations stay the source of truth. Handlers can be shared
      between the pair as long as `DispatchWithDefault` still resolves.
- [ ] Write tools must **not** default an action. Require `action` explicitly. Read tools keep
      their existing defaults.
- [ ] `tool_help.go`: overview must describe the new shape, and say `_write` tools appear under
      `--allow-write` rather than that actions are added.
- [ ] Tool descriptions: drop "only the actions listed above are available; see help for how to
      enable the rest" from read tools. The reads are all that tool ever offers, in either mode.

### 3. Platform tools

- [ ] API client methods for organizations, projects CRUD, API keys
- [ ] `hookdeck_projects_read` (`list`, `get`) — replaces today's `hookdeck_projects`
- [ ] `hookdeck_projects_use` (`use`) — both modes
- [ ] `hookdeck_projects_write` (`create`, `update`, `delete`), `type` defaulted per server
- [ ] `hookdeck_organization_read` / `_write`
- [ ] Register on both Gateway and Outpost servers
- [ ] Help topics for each

### 4. Description fixes (from the blind runs, scheme-independent)

- [ ] `transformations_read.run` — state explicitly that it persists nothing
- [ ] `metrics_read` — distinguish `events` from `attempts` for delivery questions
- [ ] `request_write.retry` / `event_write.retry` — disambiguate from each other
- [ ] `connections_pause.pause` vs `connections_write.disable` — say which is the reversible
      incident-response action
- [ ] `projects_use` — say it changes what every subsequent call targets

### 5. Docs and generated output

- [ ] `README.md`, `REFERENCE.md`
- [ ] `go run ./tools/generate-reference --check`, regenerate if it fails
- [ ] `CHANGELOG.md` — breaking, alongside the `hookdeck_*` to `gateway_*` rename. Say plainly
      that per-tool grants and `allowedTools` entries need updating once. Note the new platform
      tools and the API version bump.
- [ ] Anything in `docs/` listing MCP tools

### 6. Tests

Update:

- [ ] `pkg/gateway/mcp/write_mode_test.go` — `TestListTools_ReadOnlyMode`,
      `TestListTools_WriteMode`, `TestWriteGuard_BlocksWriteActionsInReadOnlyMode`,
      `TestWriteGuard_PauseIsNotGated`, `TestWriteGuard_TransformationRunIsNotGated`,
      `TestWriteGuard_AllowsWriteActionsInWriteMode`, `TestWriteActions_RequireAnID`,
      `TestHelpReportsMode`
- [ ] `pkg/gateway/mcp/write_actions_test.go`
- [ ] `pkg/gateway/mcp/server_test.go` (`connectInMemoryWriteEnabled`)
- [ ] `pkg/gateway/mcp/tool_help_test.go`
- [ ] `pkg/outpost/mcp/tool_actions_test.go`, `pkg/outpost/mcp/projects_test.go`
- [ ] `pkg/mcpcore/tool_projects_test.go`, `pkg/mcpcore/*_test.go`

Add:

- [ ] **No tool mixes read and write actions.** Iterate every registered tool in write mode and
      assert its enum is homogeneous. This is the invariant the change creates and the one that
      will silently regress when someone adds an action later.
- [ ] **Annotations match contents.** `ReadOnlyHint` true exactly when nothing changes state;
      `DestructiveHint` true exactly when a destructive action is present.
- [ ] **Read tools identical in both modes** — name, description, schema, annotations.
- [ ] **Write tools absent in read-only mode**, and a write action still yields the
      "restart with `--allow-write`" guidance rather than an unknown-tool error.
- [ ] `pause`/`unpause` stay ungated
- [ ] `transformations/run` stays ungated
- [ ] `projects use` stays ungated
- [ ] Platform tool coverage: projects CRUD, organization
- [ ] **No API key tool is registered on either server, in either mode**

### 7. Verification

- [ ] `go build ./...`
- [ ] `go test ./...`
- [ ] `gofmt -l .` clean
- [ ] `go run ./tools/generate-reference --check`
- [ ] Start both servers in both modes, diff `tools/list`: read tools byte-identical between
      modes; write tools only under `--allow-write`; no enum mixes read and write; annotations
      match contents
- [ ] Manual QA against a real project per `.agents/skills/` — platform writes especially

---

## Open questions

1. ~~**Should API key management be in MCP at all?**~~ **Resolved: no.** Not exposed in any
   form, including `list`. Credentials should not leak to agents; key management is a CLI, API
   or dashboard action where a human holds the credential. Decided 2026-09-22.

2. ~~**API version bump: wholesale or per-call?**~~ **Resolved** — `main` bumped wholesale to
   `/2026-09-01` in v2.6.0. Merging delivers it.

3. **Do platform tools belong on both servers?**
   Proposed yes, matching `hookdeck_projects` today. It does mean a user running both servers
   sees the platform tools twice and grants them twice. The alternative — platform tools on
   Gateway only — is worse for Outpost-only users.

4. **Does `hookdeck project` (the CLI command tree) grow to match?**
   `pkg/cmd/project.go`, `project_list.go`, `project_use.go` exist. The API now supports create,
   update and delete. Out of scope here, but the gap will be noticed once MCP has it.

## Out of scope

- Which actions are gated. The read/write classification for existing actions is unchanged; this
  is about where actions live, not what is allowed.
- `--allow-write`, `HOOKDECK_MCP_ALLOW_WRITE`, or new flags.
- A write-only server mode.
- API key management in MCP, in any form (see resolved question 1).
- The non-MCP CLI command tree (see open question 4).
- The new Gateway replay endpoints (`/events/{id}/replay`, `/requests/{id}/replay`,
  `/bulk/requests/replay`) — triage separately.
- API client behaviour beyond the version bump, auth, the response envelope.
