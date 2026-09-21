---
name: role-reviewer
description: Project overrides for the role-reviewer role in hookdeck-cli. Delta only; the base role-reviewer skill supplies everything not stated here.
---

# Reviewer overrides for hookdeck-cli

This file is a delta on the base `role-reviewer` skill. It does not repeat it.

Written against `hookdeck-cli` branch `release/v3.0.0` at `3be28c6` on
2026-09-17, and re-verified against `main` at `5c6948c` (v2.6.0) on 2026-09-21.
Everything below is either in `AGENTS.md` or in the code it points at. Sections
that hold on only one of those two lines say so inline. The one section that is
not settled says so.

Canonical location here is `.agents/skills/`. `.claude/skills` and
`.cursor/skills` are committed symlinks to it (both mode `120000`, both
pointing at `../.agents/skills`), so two of the base skill's three lookup
paths — `.agents/skills/` and `.claude/skills/` — resolve to this file. The
third, `.github/skills/`, does not exist here. `.cursor/skills` is a fourth
path the base skill does not look at; it matters for Cursor, not for
resolution.

## Running the checks

Commands are in `AGENTS.md` §5 and §10: `go build -o hookdeck .`,
`go test ./...` from the repo root, `go fmt ./...`, `go vet ./...`,
`golangci-lint run`. A compile failure counts as a failed run, not a skipped
one.

Two things change what a green run means:

- **`go test ./...` does not include `test/acceptance/`.** Those are behind
  per-feature build tags. A green unit run says nothing about CLI-facing
  behaviour, so do not let it stand in for one in your report.
- **Acceptance tests need a real API key** in `test/acceptance/.env`, which is
  gitignored and must never be committed. Without one, acceptance coverage is a
  "not verified" line, not a pass.

`no required module provides package ...` or a TLS error during `go test` is the
sandbox or an empty `GOMODCACHE`, not the diff. Re-run with
`GOMODCACHE="$(go env GOPATH)/pkg/mod"` before drawing a conclusion. `AGENTS.md`
is explicit that an environment failure is never reported as a pass.

## Additional checks

- **Acceptance test build tags.** Every `*_test.go` under `test/acceptance/`
  needs exactly one `//go:build <feature>` tag. An untagged file is built into
  every `-tags=` run, including the telemetry-only job. A new tag also needs a
  home in CI — the slice matrix in `.github/workflows/acceptance.yml` or the
  separate `-tags=telemetry` job in the same file — or the tests are written,
  pass locally, and never run. The same tag lists are hardcoded a second time in
  `test/acceptance/run_parallel.sh` (`SLICE0_TAGS`…`SLICE2_TAGS` on `main`, plus
  its own `-tags=telemetry` run), so a tag added only to the workflow runs in CI
  and silently never runs locally. Check both, and check how many slices the
  branch actually has rather than assuming: `main` has three (0–2), and the
  count has changed before. One existing tag is deliberately outside both CI
  homes, so do not flag it: `manual` (`project_use_manual_test.go`) needs a
  human at a browser. On `release/v3.0.0` there is a second such tag,
  `outpostlive` (`outpost_live_test.go`), which runs from
  `.github/workflows/outpost-live.yml` against a live Outpost host; neither the
  test nor that workflow exists on `main`. A new tag that belongs in neither
  category needs the author to say where it runs.
- **Telemetry carries no user data.** `CLITelemetry` serialises eight fields:
  `source`, `environment`, `command_path`, `invocation_id`, `device_name`,
  `generated_resource` (a bool: was the command generated from the API spec),
  `mcp_client`, and `command_flags`, which is flag *names* only. `mcp_client`
  is the one value rather than a name, and it identifies the calling MCP client,
  not anything the user typed. A diff that adds flag values, URLs, resource IDs
  or payload content is a finding; so is a new value-carrying field whose source
  is user input rather than the client's own identity.
- **Docs that move with the code.** `REFERENCE.md` covers every command with
  examples. Cobra `Example` and `Long`, and the website examples generated from
  them, have to be updated when CLI output, flags or command names change
  (`AGENTS.md` §6, "Keep in sync"). Resource commands use the shared helpers in
  `pkg/cmd/helptext.go` rather than literal Short/Long strings, and positional
  args need `Annotations["cli.arguments"]` or the generator emits no Arguments
  table.
- **One copy of a skill.** `.agents/skills/` is canonical and the harness
  directories are symlinks. `AGENTS.md` § Agent skills puts it as "never copy a
  skill into a second location", so a diff that adds a second copy anywhere is a
  finding. `d0104e8` is the change that set the rule, by deleting the root
  `skills/` that had held a byte-identical copy. `AGENTS.md` cites #336 as the
  precedent; that issue is about a release-skill CI gate that reported `pending`
  on every commit, and what it records is the bug travelling into
  `hookdeck/n8n-nodes-hookdeck` with a verbatim copy of the skill — so it needed
  fixing twice. Cite it that way rather than as two copies drifting apart here.

### MCP checks — `release/v3.0.0` line only

`pkg/mcpcore` does not exist on `main` at v2.6.0. Neither the action-flag struct
nor the tool-prefix scheme below is present there: on `main` every MCP tool name
is a literal `hookdeck_*` string, and there is no read-only server mode to gate
anything against. Confirm which line you are reviewing before applying either
bullet — on `main` they describe code that is not in the tree, and a finding
raised from them is wrong.

- **MCP write gating.** `mcpcore.Action` carries three separate flags and they
  are not interchangeable. `Write` is what keeps an action out of the default
  read-only server, and it covers reads that hand back a credential — a tenant
  token or a portal URL is reusable access. `Destructive` drives the client
  hint. `Mutates` marks an action that changes state but is deliberately left
  available read-only, such as pausing a connection. A new action that changes
  data or mints a credential without `Write: true` is offered in read-only mode:
  that is a finding. Collapsing `Mutates` into `Write` breaks
  `TestWriteGuard_PauseIsNotGated` in `pkg/gateway/mcp/write_mode_test.go`.
- **MCP tool names come from `mcpcore`.** `Server.ToolName` applies the server's
  `ToolPrefix`, so gateway tools are `gateway_*` and Outpost tools `outpost_*`
  without anyone typing the prefix. A literal tool name at a *registration or
  definition* site — where `ToolName`, `ProjectsToolName` or `LoginToolName`
  should have produced it — is a finding. Prose is not: `tool_help.go` keys
  its topics off `srv.ProjectsToolName()` and then spells `hookdeck_projects`
  out in the help text a user reads, and `tool_events.go` maps ID prefixes to
  literal `gateway_*` names in a hint. Those are correct. Platform operations
  — signing in and switching project — keep the `hookdeck_` prefix from
  `DefaultPlatformPrefix` and are identical in both servers on purpose; giving
  one a product prefix is a finding. Because the prefix scheme and `pkg/mcpcore`
  are new in the v3.0.0 line, names are still settling — but renaming one breaks
  every client already configured against it, which belongs in the release notes.

## Do not ask for

- **A changelog entry.** `CHANGELOG.md` says outright that it is no longer
  maintained. Release notes are drafted at release time from
  `git log PREV_TAG..HEAD`, per `.agents/skills/hookdeck-cli-release/SKILL.md`.
  Asking a pull request for an entry asks for something the repo deliberately
  stopped doing. What is worth flagging instead is a user-facing change the
  description does not explain well enough for someone to write that note later.
- **A rewrite of the existing "anonymous telemetry" wording.** Worth knowing
  when you read a telemetry diff: the payload holds no account identifier, but
  it rides an API-key-authenticated request that also carries `X-Team-ID` and
  `X-Project-ID`, so the receiving end can attribute it. The CLI's help text and
  `README.md` nonetheless describe telemetry as anonymous. That is the shipped
  wording and a product question, not a defect to raise on each pull request. A
  diff that extends the anonymity claim to a *new* surface is a finding.

## Scope during the v3.0.0 beta

**Decided 2026-09-21: there is no content-based scope, and that is deliberate.**

This reviewer reviews whatever is moved into the board's Review column, and
nothing else. There is no rule about which kinds of pull request are in or out —
no carve-out for the MCP surface, the config file handling or the release
branch. An earlier draft of this file invented one; nothing in `hookdeck-cli`
supports it.

The reason is that a content rule would not control the thing worth controlling.
This reviewer is advisory: it reports and cannot merge, and its read-only rule is
honoured rather than enforced (see the base skill and the control room's README).
Narrowing what it may read removes signal without removing that. What does
control it is **a human choosing to move a card into Review** — a per-task
decision, made with the diff in front of you, rather than a rule written in
advance.

So: nothing arrives in Review on its own. Feeds may put work in Backlog; only a
person promotes it. If you are reading this because a card reached you, someone
decided it should.
