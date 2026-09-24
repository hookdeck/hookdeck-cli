---
name: hookdeck-cli-manual-qa
description: >-
  Runs exploratory manual QA of the Hookdeck CLI and its MCP servers against
  real acceptance-test projects, safely. Provides a credential guard that keeps
  destructive commands off any project not named up front and never touches the
  operator's own login, an MCP stdio driver, per-surface checklists, and how to
  choose what to test when nobody hands you a list. Use when manually testing
  CLI commands or MCP tools, exercising write paths, validating a release
  candidate, or reproducing a defect against the live API.
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

### Record every config file before you start, and check them at the end

The two rules above protect the config the guard hands you. They cannot protect
a config the guard does not know about, and in pass 1 that gap cost the operator
a working config: `hookdeck project use` honoured `--hookdeck-config` for reads
but re-derived its own write target, so it overwrote the `.hookdeck/config.toml`
that happened to be in the working directory and lost its contents (#424).

Take the before state, and compare it after:

```bash
BEFORE="${TMPDIR:-/tmp}/hd-qa-configs.before"
for c in .hookdeck/config.toml "$HOME/.config/hookdeck/config.toml"; do
  [ -f "$c" ] && printf '%s  %s\n' "$(md5 -q "$c")" "$c"
done | tee "$BEFORE"
ls -l .hookdeck/config.toml "$HOME/.config/hookdeck/config.toml" 2>/dev/null
```

Re-run the same block at the end and diff it against `$BEFORE`; that is what let
pass 2 state "nothing outside the guarded config was touched" as evidence rather
than as a hope. **Compare hashes; never `cat` a config** — the CLI writes
single-quoted TOML, so a redaction pattern matching only double quotes prints
the key verbatim.

The underlying bug is fixed on this branch (#424, commit `3fc8124`), so
`--hookdeck-config` can now be trusted for writes as well as reads and
`pkg/cmd/project_use_config_target_test.go` keeps it that way. Do the check
anyway: two lines, and the next such bug will not announce itself either.

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

## What to look for, and how to choose it

Ordinary "does it work" testing finds little at this point, and a pass steered
by a list of recently-changed areas only looks where somebody already suspected
a problem. [references/choosing-what-to-test.md](references/choosing-what-to-test.md)
is how to pick targets from the product itself. It carries, with the evidence
from every pass so far:

- the eight defect **shapes** that have actually yielded findings — a capability
  diverging across the CLI and MCP; prose promising what the code does not do;
  the failure and empty paths; the argument you did not pass; an input accepted
  and then dropped; machine-readable output polluted by human text; an exit code
  that does not match what happened; state verbs with unrecorded side effects —
  and how to go looking for each;
- a **routine** that produces targets with nobody handing you a list;
- **what manual QA catches that the acceptance suite structurally cannot**, and
  where that boundary honestly sits;
- **what is now guarded by a test**, so a pass does not spend itself
  rediscovering findings that have already become CI failures.

Read it before planning a pass, not after.

## Smoke-testing a published release

Checking a published artifact is a different job from the QA above — it tests
packaging, not behaviour — and it needs its own guard, because the documented
install command is destructive to the operator's machine:

```bash
npm install -g hookdeck-cli@beta     # DO NOT run this to test a release
```

`-g` overwrites whatever `hookdeck` the operator already has. On a machine where
the CLI came from Homebrew it writes into the same `bin` directory and shadows or
replaces the working install, and the operator is left on a pre-release build
without having asked to be.

Install into an isolated prefix instead, and run the binary by path:

```bash
SMOKE="${TMPDIR:-/tmp}/hookdeck-smoke"
rm -rf "$SMOKE" && mkdir -p "$SMOKE"
npm install --prefix "$SMOKE" hookdeck-cli@beta
HD="$SMOKE/node_modules/.bin/hookdeck"
```

Then use `$HD` for every command, still with `--hookdeck-config "$HD_CONFIG"`
from the guard above, so neither the operator's binary nor their login is touched.

**Record the before state and check it afterwards.** Claiming "nothing was
changed" is worth nothing without evidence:

```bash
which hookdeck; hookdeck version | head -1      # before and after
npm ls -g hookdeck-cli --depth=0                 # must stay unchanged
md5 -q ~/.config/hookdeck/config.toml            # must be identical after
```

What is worth checking on a published artifact, none of which a local build can tell you:

- the version reported matches the tag
- a native binary is shipped for the host architecture, and is the one selected
  (`file` the binary; an `arm64` host silently running an `amd64` build is a
  packaging defect that still "works")
- the command tree is present, and a real read reaches the API
- for a release that renames or adds MCP tools, that `tools/list` shows the new
  names from the *installed* package rather than from your working tree

Remove the prefix when finished — it is around 90 MB, because the package ships
binaries for every platform.

### One complete journey beats twenty surface checks

The v3.0.0 pass checked tool counts, `--allow-write` gating, exit codes and
`[BETA]` markers. All real, all passed, and it shipped with `hookdeck login`
broken for every new user. The pass had verified that the parts existed, never
that a person could get a job done with them.

**Finish a pass by doing one whole task, start to finish, on the installed
artifact**, and follow it wherever it goes:

1. **Sign in** — not with a key you already have. See below.
2. **Find out where you are** — list projects, select one.
3. **Ask the product a real question** — "how much traffic did this project take
   this fortnight, and did anything fail?" Use the tools an agent would.
4. **Follow one answer down a level** — a spike, a failure, an open issue.

Every one of those steps is a place a release can be broken while every
individual check still passes. The v3.0.0 metrics bug (#440) surfaced within
minutes of the first time anyone asked the product a question rather than
inventorying it.

### Do not skip login. It is the most-run command in the product

A QA brief that says "prefer read-only operations" or "do not mint credentials"
reads as prudent and excludes the single path every new user takes. That
exclusion is exactly how #438 shipped.

Sign in for real, with the isolation the guard above already gives you:

```bash
"$HD" login --hookdeck-config "$HD_CONFIG"     # browser flow, needs a real TTY
```

Three things make this safe rather than reckless, and they are the point:

- **`--hookdeck-config` decides where the credential is written** (v3.0.0
  onwards — before that the flag was honoured for reads and ignored for writes).
  The operator's `~/.config/hookdeck/config.toml` is never touched, and you
  verify that with the checksum you already recorded.
- **Start from a config that does not exist.** With a key already in the file,
  `login` validates it instead of signing in, and a valid-but-wrong-class key
  dead-ends without falling back to the browser.
- **`hookdeck_login` over MCP is the friendlier route and tests the same code
  path.** It returns the browser URL immediately and polls in the background, so
  it needs no TTY. Prefer it when driving the MCP server — but the server's
  stdin must stay open across the user's click, which means a fifo with its own
  holder process, not a batched heredoc:

  ```bash
  mkfifo "$F"; nohup sleep 3600 > "$F" &        # holder keeps the write end open
  nohup "$HD" gateway mcp --hookdeck-config "$HD_CONFIG" < "$F" > "$OUT" &
  ```

  Without the holder, the session ends when your shell exits and the background
  poll is cancelled with `login cancelled: MCP session closed` — after the user
  has already clicked the link.

If the account you can sign into is one you must not disturb, ask for a
throwaway. "We could not test login safely" is a reason to get a safer account,
not a reason to ship login untested.

## Clean up

Acceptance-test projects accumulate resources fast, and a QA pass adds to it.

- Name everything you create with a run-specific prefix (`qa-<date>-<n>`) so it
  can be found and removed later.
- Delete what you created before finishing, in reverse order of creation.
  Deleting a tenant cascades to its destinations.
- If you leave anything behind, say so explicitly in the report with the names
  and the reason — silent residue is indistinguishable from a leak.

## Reporting

Return findings ranked by severity. Separate **confirmed defects** from **things
worth a look** — mixing them makes the confirmed ones cheaper to ignore.

Every finding carries five things, and is not worth filing without them:

1. **The exact command or tool call**, arguments included, as run — not a
   paraphrase of it.
2. **The exact output**, including the **exit code**. `echo $?`.
3. **What you expected instead, and why**: the help text, the schema, the
   sibling surface, the documented default. A finding with no stated expectation
   is an opinion.
4. **The minimal reproduction** — the shortest call that still shows it, with
   the incidental arguments removed one at a time.
5. **Whether you ruled out the environment**, and how. Say which explanation you
   eliminated, not just that you are confident.

### Rule out the environment before you call it a defect

Two things were nearly reported as product defects in one day: a CI slice
failing with 429s, where the rate limiting was noise and the real cause was
eleven acceptance tests still calling pre-split tool names (an earlier pass read
the 429s as the cause and moved on — `plans/mcp_read_write_tool_split.md` §7);
and a project-list test that fails intermittently on list ordering (#409), which
is a test defect, not a product one. What separates the two, cheapest first:

- **Rebuild.** A stale binary reports the old behaviour convincingly.
- **Run it again, and run it alone.** Anything that passes on a second run, or
  passes outside its suite, is a flake or an ordering dependency until proven
  otherwise.
- **Read the status code.** A 429 or a 502 is the environment unless a single
  call reproduces it deterministically.
- **Find where the behaviour originates.** #431 was confirmed as server-side by
  noticing the CLI calls the dedicated pause endpoint rather than sending the
  field — which changed the report from "our bug" into "confirm the intent with
  the API team", a different and more useful ask.
- **Construct the minimal call that distinguishes the two explanations.** A
  report that says "verified: `log_level` is severity, not a completion flag,
  here are the five cases" is actionable; "this looks suspicious" costs someone
  else the same investigation.

If you cannot decide, file it as **worth a look**, and say in one line exactly
what you could not rule out.

State plainly what you did **not** cover. A pass that reports only successes is
usually a pass that did not go looking.
