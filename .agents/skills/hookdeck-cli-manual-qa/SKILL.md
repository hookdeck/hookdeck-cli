---
name: hookdeck-cli-manual-qa
description: >-
  Runs exploratory manual QA of the Hookdeck CLI and its MCP servers against
  real acceptance-test projects, safely. Provides a credential guard that keeps
  destructive commands off any project not named up front and never touches the
  operator's own login, an MCP stdio driver, and per-surface checklists. Use
  when manually testing CLI commands or MCP tools, exercising write paths,
  validating a release candidate, or reproducing a defect against the live API.
---

# Hookdeck CLI — manual QA

Exploratory testing against the live API, in acceptance-test projects, without
putting anything else at risk.

This exists because the automated suites cannot cover everything. They run in
fixed shapes; a QA pass goes looking for the things nobody thought to assert —
wrong success messages, defaults that never get sent, filters that silently do
nothing. Every defect this process has found so far was of one kind: **a wrong
answer that reads like a right one.** Look for those specifically.

## Safety model — read before running anything

Manual QA runs destructive commands against real projects. Two rules make that
survivable, and both are enforced by [scripts/hd-testenv.sh](scripts/hd-testenv.sh):

1. **Never use the default config.** Every command takes
   `--hookdeck-config "$HD_CONFIG"`. The operator's own `hookdeck login` is
   never read or overwritten. This is not hypothetical — a suite run without an
   isolated config has previously written CI credentials over a working login.
2. **The project is confirmed from the credential, not from the argument.** You
   name the project you expect up front; the guard authenticates, resolves the
   project, and refuses to hand back a config if it is anything else or is
   outside the acceptance-test organisation.

Credentials come only from `test/acceptance/.env` (gitignored, CI test keys), so
the reachable blast radius is the test projects those keys can see.

**Before any delete, call `hd_assert "$HD_PROJECT"` and abort on failure.** It is
one cheap line and it catches a config swapped or a project switched mid-run.

If the guard refuses, **stop and read the message**. It fails closed on purpose;
working around it defeats the point.

### Setup

```bash
source .agents/skills/hookdeck-cli-manual-qa/scripts/hd-testenv.sh
hd_testenv HOOKDECK_CLI_TESTING_API_KEY tm_xxxxxxxxxxxx          # gateway project
hd_testenv HOOKDECK_CLI_OUTPOST_TESTING_API_KEY tm_yyyyyyyyyyyy  # outpost project

go build -o hookdeck .            # test the build, not `go run` each time
./hookdeck --hookdeck-config "$HD_CONFIG" whoami
```

**Rebuild before every pass, and after every fix.** A stale binary reports the
old behaviour convincingly: missing tools, missing flags, old names. If
something looks broken in a way that seems too fundamental to be true, check
that first — it has already produced one false finding.

Ask the maintainer which project IDs to use if you do not already know them;
they are deliberately not hardcoded here.

## Running a pass

Two independent surfaces. Run them as separate agents when doing a full sweep —
they do not share state beyond the projects, and one must not wait on the other.

- **CLI surface** — [references/cli-checklist.md](references/cli-checklist.md)
- **MCP surface** — [references/mcp-checklist.md](references/mcp-checklist.md)

For MCP, drive the server with [scripts/mcp-call.py](scripts/mcp-call.py) rather
than wiring up an editor:

```bash
python3 .agents/skills/hookdeck-cli-manual-qa/scripts/mcp-call.py \
  --config "$HD_CONFIG" --server gateway --list
```

## What to look for

Ordinary "does it work" testing finds little at this point. These are the
patterns that have actually yielded defects:

- **Does the message match the state?** Run the command, then read the resource
  back independently. `enable` printing "enabled" proves nothing; a subsequent
  `get` showing it enabled does. Several commands announced the verb they were
  asked to perform rather than the state the API returned.
- **Does an omitted flag mean what the help says?** Documented defaults are not
  necessarily sent. A destination type whose schema says `tls` defaults to on
  was being created with `tls` unset.
- **Can you undo it?** Setting a value is usually tested; clearing it is not.
  `--filter '{}'` reported success and changed nothing because `omitempty`
  dropped it before it reached the wire.
- **Does an error exit non-zero?** An endpoint that answers `200` with
  `{"success": false}`, or a body carrying its own status field, will be
  reported as success by anything that only checks the HTTP status.
- **Does a filter actually filter?** Pass one and confirm the result set
  narrows. A silently ignored filter returns plausible data that is wrong.
- **Do the read and write paths agree?** A fix applied to a CLI command and not
  its MCP sibling leaves the defect open on the surface with no human watching.
  Check both.

When something looks wrong, **verify it against the live API before reporting
it** — construct the minimal call that distinguishes the two explanations. A
report that says "verified: log_level is severity, not a completion flag, here
are the five cases" is actionable; "this looks suspicious" costs someone else
the same investigation.

## Clean up

Acceptance-test projects accumulate resources fast, and a QA pass adds to it.

- Name everything you create with a run-specific prefix (`qa-<date>-<n>`) so it
  can be found and removed later.
- Delete what you created before finishing, in reverse order of creation.
  Deleting a tenant cascades to its destinations.
- If you leave anything behind, say so explicitly in the report with the names
  and the reason — silent residue is indistinguishable from a leak.

## Reporting

Return findings ranked by severity, each with: what you ran, what you expected,
what happened, and the evidence. Separate **confirmed defects** from **things
worth a look** — mixing them makes the confirmed ones cheaper to ignore.

State plainly what you did **not** cover. A pass that reports only successes is
usually a pass that did not go looking.
