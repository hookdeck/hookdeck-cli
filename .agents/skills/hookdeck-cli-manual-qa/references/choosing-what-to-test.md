# Choosing what to test

Read [../SKILL.md](../SKILL.md) first. This file answers the question the rest
of the skill does not: **what do you point a pass at?**

Both v3.0.0 passes were handed a list of recently-changed areas by a human. That
works once. It does not scale, and it aims the pass at what somebody already
suspected — the defects below were found *around* those areas, not in them.

Everything here is derived from what those two passes actually found (issues
#423–#436) and the fixes that came out of them. The evidence is attached to each
point so you can tell when it stops applying.

---

## The shapes that have yielded defects

Ordered by what they have returned per hour spent. None of them require knowing
what changed recently.

### 1. The same capability on two surfaces, diverging

**Evidence.** `gateway_connections_write` could not `create` without an explicit
`rules` array, while `hookdeck gateway connection create` worked without one
(#425). `outpost tenant portal --theme purple` was rejected by the CLI and
accepted over MCP (489085a) — and chasing that turned up the larger defect
underneath it: declared enums were never enforced over MCP *at all*, so
`status: "bogus"` had always gone out to the API.

**How to hunt it.** Take one capability and perform it both ways with identical
inputs, then diff the stored resource *and* the error. Validation is the richest
seam: every CLI flag with a fixed set of values, a documented default, or a
mutual exclusion is a candidate, because the CLI's validation lives in the
command and the MCP schema is a second, independent declaration of the same
rule. When you find one divergence, ask the wider question the way the theme bug
demanded: is the mechanism missing for this one property, or missing entirely?

### 2. Prose that promises what the code does not do

**Evidence.** Help output, tool descriptions and error hints named
`hookdeck_projects`, `outpost_tenants` and `outpost_destinations` — names the
read/write split had removed, so an agent following them got `unknown tool`
(#426). Thirteen Outpost properties said "accepts an array of strings" over a
`"type": "string"` schema the validator refuses (471ba21). Ninety-one property
descriptions named actions their tool does not have (3f3ae69). `listen --help`
gave the default CLI destination name backwards (#423). The one-line smoke test
printed in `gateway mcp --help` exited non-zero and produced no output (#428).

**How to hunt it.** Read the text as if you were about to act on it, then act on
it: run every example out of `--help` verbatim and check the exit code; call the
tool the error hint tells you to call; send the shape the description says is
accepted. Much of this is now automated — see *Already guarded* below — and what
is left to you is the **semantic** half: claims about reversibility, safety,
ordering, defaults and API behaviour, which no schema check can evaluate.
`gateway_help` saying pause/unpause are "reversible and drop nothing" is false
for a disabled connection (#431) and no guard can know that.

### 3. The failure path and the empty path

**Evidence.** `--output json` emits nothing parseable when the command fails —
valid JSON on success, plain text on stderr and an empty stdout on failure
(#433). An unsupported `--output yaml` returns the human table and exit 0
(#432). Twenty-three group commands answered an unknown subcommand by printing
help and exiting 0 (131a4e9) — so a script calling a command v3.0.0 *removed*
got silence and success.

**How to hunt it.** Every time a command succeeds, immediately run three more:
the same call against an id that does not exist, the same call with one argument
set to an unsupported value, and a list call whose filter matches nothing. Check
the *shape* of stdout and the exit code, not the wording.

### 4. The default, and the argument you did not pass

**Evidence.** #425 again: the omitted `rules` key marshalled as `"rules": null`.
It is the most obvious first call anyone makes against a new write tool, and it
was the one call no test made — every existing test spelled `rules` out. Earlier
passes found the same shape in a destination type whose schema declares `tls`
on by default and which was created with `tls` unset.

**How to hunt it.** Call every write action with the required arguments and
nothing else. Then add optional arguments one at a time. Then read the resource
back and compare it against what the schema says the defaults are — a default
that is documented is not necessarily sent.

### 5. An input accepted and then dropped

**Evidence.** `--filter '{}'` reported success and changed nothing, because
`omitempty` removed it before the wire. List filters the API ignores return
plausible, wrong data. On MCP, `{"action":"get","order_by":"created_at"}` was
accepted and `order_by` never read, so the caller got one record back that read
as though it had been sorted.

**How to hunt it.** Never accept a success message as evidence that an input
arrived. For a filter, compare filtered and unfiltered counts. For a value, read
the resource back. For a clear (`--filter '{}'`, `--unset`, removing a topic),
confirm the field is actually gone — setting is usually tested, clearing is not.
The MCP half of this is now guarded (see below); the CLI and API halves are not.

### 6. Machine-readable output polluted by human-readable text

**Evidence.** Source validation warned with `fmt.Printf` when its OpenAPI spec
fetch failed, so the warning landed on stdout ahead of the JSON and consumers of
`--output json` got `invalid character 'W'` (a00396e). The spec fetch fails
*intermittently*, so this broke scripts at random and passed CI on every run
where the fetch happened to work.

**How to hunt it.** Parse the output, do not read it: pipe `--output json`
through `python3 -m json.tool` and check both exit codes. Aim it at commands
whose work includes a network call that can fail independently of the command
itself — that is where conditional pollution lives. This class is now guarded
inside `pkg/cmd`; a diagnostic printed from anywhere else is still yours to find.

### 7. An exit code that does not match what happened

**Evidence.** #432 and 131a4e9 above; #428 (a documented example exiting
non-zero); and from earlier passes, `transformation run` reporting success for a
thrown error (#410) and `event mute`/`cancel` succeeding on events they cannot
affect (#413) — endpoints that answer `200` with their own status field in the
body.

**How to hunt it.** `echo $?` after everything, including help output and
anything you copied out of the docs. Where a response carries its own status
field, decide which one the exit code is meant to follow, and check it does.

### 8. State verbs: the message, the state, and the side effect

**Evidence.** Pausing a *disabled* connection clears `disabled_at`, so
pause/unpause leaves it **enabled** — and because `gateway_connections_pause` is
deliberately available without `--allow-write`, a read-only MCP session can
re-enable a connection somebody deliberately disabled (#431). Earlier passes
found several commands announcing the verb they were asked to perform rather
than the state the API returned.

**How to hunt it.** Take a full `get` before and after, and diff the **whole**
resource, not the field the verb is named after. Start from the non-default
state — a disabled connection, an already-paused one — because the interesting
transitions are the ones nobody demonstrates. Repeat the verb and check the
second call does not claim to have changed anything.

---

## A routine that produces targets without a list

1. **Start from the surface, not the changelog.** Walk the `--help` tree and
   `tools/list` in both modes. Every command, action and property is a
   candidate, and the listing is a checklist you did not need to be given.
2. **Diff the two surfaces.** The set of things the CLI can do and the set the
   MCP servers advertise are never identical; the differences and the overlaps
   are both interesting. Shape 1 lives here.
3. **Take the first call.** Minimum arguments, no optionals, on the tool a
   newcomer would reach for first (shape 4).
4. **Take the second call: the same one, wrong.** Bad id, bad enum value,
   unsupported `--output`, misspelled subcommand (shapes 3, 7).
5. **Then read the prose and act on it** (shape 2).
6. **Then exercise behaviour** — state verbs from a non-default starting state,
   filters against counts, clears (shapes 5, 8).
7. **Prefer the paths with nobody watching.** MCP over CLI, `--output json` over
   the table, failure over success. A wrong answer on the human path gets
   noticed by a human eventually; a wrong answer to an agent does not.
8. **Record what you did not reach**, by name.

---

## What manual QA catches that the acceptance suite structurally cannot

This is the honest boundary. Anything on the *other* side of it belongs in a
test, not in a QA pass.

- **Whether the words are true.** A test asserts behaviour. It can assert that
  `theme=purple` is rejected; it cannot notice that the help calls an operation
  "reversible" when it is not (#431). Names, actions and promised types are now
  checked mechanically — semantics are not, and cannot be.
- **Cross-surface consistency.** Both surfaces have thorough tests; they were
  written separately, and each encodes its own author's understanding. Nothing
  compares them. Creating the same resource both ways and diffing the stored
  result is a human job (#425, 489085a).
- **A test that shares the code's wrong assumption.** #425 survived because
  every create test spelled `rules` out — the same blind spot in both. The same
  pattern appears in the metrics acceptance tests, which encode two known bugs
  as expected behaviour (#397). A suite cannot find what it assumes.
- **Anything needing a real browser.** The tenant portal URL, `listen --open`,
  the login browser flow, OSC-8 hyperlinks and `--color off` in the interactive
  listen TUI (#403, #404). A test can assert a URL comes back; only a person can
  open it and look.
- **Timing under real load.** Three propagation windows on this branch were each
  set from a quiet machine and each proved too short: 20s for events after a
  trigger, which 48 tests depended on (6c629de); 90s for a tenant portal that
  took 195s (a18f715); and an `events_count` read immediately after its event
  existed (373033b). A quiet local run hides all three. If you wait on something
  and the wait is "usually enough", that is a finding — report the measurement.
- **Packaging.** Version, architecture, the npm shim's signal handling (#429).
  See *Smoke-testing a published release* in SKILL.md.

## Already guarded — do not spend a pass re-finding these

These were findings once. They are now tests, and a pass that rediscovers them
has spent its time proving CI works.

| Behaviour | Guard | Where |
|---|---|---|
| The advertised tool surface in both modes — names, actions, annotations | `TestToolSurfaceIsWhatWeThinkItIs`, `TestEveryToolNameCarriesItsPosture` | `pkg/gateway/mcp/surface_test.go`; `TestListTools_*`, `TestReadOnlyPropSurface` in `pkg/outpost/mcp/tools_test.go` |
| Agent-facing text naming tools or actions that do not exist | `TestAgentFacingTextNamesOnlyRealTools` / `…RealActions` / `…RealToolActions` | `pkg/gateway/mcp/surface_test.go`, `pkg/outpost/mcp/agent_text_test.go`, detector in `internal/toolprose` |
| A description promising an array over a scalar property | `TestToolPropertiesDoNotPromiseArrays` | both servers |
| A write tool advertising read-only parameters | `TestWriteToolsDoNotAdvertiseReadOnlyParameters` | `pkg/gateway/mcp/surface_test.go` |
| A property accepted by an action whose handler ignores it | action-scope tests | `action_scopes_test.go` in both servers |
| A declared enum not enforced over MCP | `TestCheckArgumentTypes_Enum` | `pkg/mcpcore/enum_test.go` |
| A diagnostic printed to stdout, corrupting `--output json` | `TestDiagnosticsDoNotGoToStdout` | `pkg/cmd/stdout_purity_test.go` |
| An unknown subcommand of a group command exiting 0 | `TestUnknownSubcommandOfGroupCommandFails`, `TestGroupCommandsAreMarked` | `pkg/cmd/group_test.go` |
| `--hookdeck-config` honoured for writes as well as reads | `TestProjectUseHookdeckConfigFlagBeatsCwdLocalConfig` and siblings | `pkg/cmd/project_use_config_target_test.go` |
| Hand-written command examples in `REFERENCE.md` naming commands that do not exist | `TestReferenceExamplesNameRealCommands` | `pkg/cmd/reference_examples_doc_test.go`, plus `go run ./tools/generate-reference --check` |

If you find a defect of a guarded class anyway, that is worth reporting loudly —
it means the guard has a hole, and the hole is the more valuable finding.
