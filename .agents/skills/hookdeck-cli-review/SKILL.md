---
name: hookdeck-cli-review
description: >-
  Reviews changes to the Hookdeck CLI: why a green pull request still needs its
  acceptance run read, where an acceptance-test build tag must be registered,
  which generated files and wire contracts break silently, which docs move with
  the code, and what not to raise. Use when reviewing a pull request, a diff or
  a branch in hookdeck-cli, or when deciding whether a change is safe to merge.
---

# Reviewing changes to the Hookdeck CLI

`AGENTS.md` is the reference for building, testing and linting this repository,
and for the rules it already states. Read it first. This file is the review
delta: what to look at in a diff, and the traps that a green pull request
hides. Where a rule already lives in `AGENTS.md`, this points at it rather than
restating it.

## Acceptance runs on the pull request, but nothing enforces it

`.github/workflows/test-acceptance.yml` triggers on every `pull_request`
targeting `main` or `next`, with no path filter. So the acceptance suite — the
three matrix slices plus the separate telemetry job — **does run on the pull
request in front of you**. Assuming it did not is the wrong call.

What it is not is *required*. Branch protection on `main` requires exactly:

```
unit-test  build-linux  build-mac  build-windows
```

Acceptance is not in that list. A pull request with a **red or skipped**
acceptance run is still mergeable, and the merge button will not tell you.

**So read `gh pr checks` and flag a red or skipped acceptance run**, because
nothing else in the process will. That is the single most useful thing this
review does that the required checks do not.

### The one case where it genuinely does not run

`pull_request` does not fire when a pull request's head branch is updated by
**merging another pull request into it**. `test-acceptance.yml` states this in
a comment above its triggers. A stacked branch — a release branch that collects
other pull requests — can therefore reach `main` having never run acceptance at
all, and `gh pr checks` shows that as an absence rather than as a failure.

The remedy is `workflow_dispatch`, which the same workflow accepts against any
ref. One catch, also in that comment: GitHub reads the workflow definition from
the ref you dispatch, so a branch can only be dispatched once it already
carries the `workflow_dispatch` block. A branch cut before that block landed
has to be rebased first.

Dispatch it on its own, though. `test/acceptance/README.md` § Rate limits
explains why: a push to the branch also triggers the `pull_request` run, and
two concurrent runs against the same projects exhaust the API rate limit and
fail at the timeout with no assertion failures at all — which reads exactly
like a code failure.

So "acceptance never ran" is a finding with an action attached, not just an
observation.

### Why a reviewer should care, not just the release engineer

`.github/workflows/release.yml` calls the same acceptance workflow as a gate:
`build-mac`, `build-linux` and `build-windows` each declare
`needs: [acceptance]`, and `publish-npm` needs those three builds. A red
acceptance therefore blocks the whole release pipeline.

Merging a red acceptance does not fail loudly at merge time. It fails at the
next release tag, on someone else's change, and the bisect starts from the
wrong commit.

### Running it yourself

`go test ./...` does not include the acceptance suite — see `AGENTS.md`
§ Running `go test` after code changes. A local run needs real credentials in
`test/acceptance/.env`, which is gitignored. Without them, "not verified" is the
honest report, not a pass.

## A new acceptance tag has to be registered in five places

Every `*_test.go` under `test/acceptance/` carries exactly one feature build
tag, and untagged files leak into every tag build — `AGENTS.md` § Acceptance
tests and feature tags states that rule; do not repeat it in a review, check it.

`test/acceptance/README.md` § Parallelisation covers the rest of the mechanism:
which tags are in which slice, that each slice has its own test account, and
that telemetry runs separately. Read it rather than relying on this file.

What no file states is that the same set of tags is maintained **by hand in
five places**, and they drift independently:

- `test/acceptance/run_parallel.sh` — `SLICE0_TAGS`, `SLICE1_TAGS`,
  `SLICE2_TAGS`, which is how the suite runs locally
- `.github/workflows/acceptance.yml` — the `matrix.include` `tags` strings,
  which is how it runs in CI
- `test/acceptance/README.md` § Running Tests — the "Run all automated tests"
  command, a single `-tags="..."` string that has to carry every tag
- `test/acceptance/README.md` § Running Tests — the three per-slice commands
- `test/acceptance/README.md` § Parallelisation — the `Slice N features` bullets

Registered in CI but not in the script, the tag never runs locally. In the
script but not in CI, it never runs on a pull request. Left out of the "run all"
command, the documented way to run everything quietly stops running everything —
and that one is the worst of the five, because the command still exits 0.

All five agree today, on 21 distinct tags. Check all five whenever a diff adds
one.

One thing the README gets wrong, before you trust it on this: it attributes the
matrix to `.github/workflows/test-acceptance.yml`. That file only declares the
triggers and calls `.github/workflows/acceptance.yml`, which is where the slices
and their tags actually live.

### The `manual` tag is a deliberate exception

`test/acceptance/project_use_manual_test.go` is `//go:build manual`, and
`manual` appears in **none** of those five lists. That is intentional: the test
needs an interactive browser login, which no CI job can perform, and
`test/acceptance/README.md` gives it its own `-tags=manual` command instead.
Applying the registration rule literally will report it as drift. It is not.

### Which slice a tag lands in is a decision, not bookkeeping

The README describes the slices; what it does not say is that choosing between
them is a judgement call. They are not interchangeable buckets:

- a different test account per slice means a test that assumes resources
  created by another slice will not find them
- slice 0 already carries 12 tags against a 12-minute timeout, while slice 1
  carries 2

Adding a slow tag to slice 0 is how the suite starts timing out. If a diff adds
a tag, the slice it went into is worth a sentence.

The telemetry job is separate for a reason worth checking against a diff: it
forces `HOOKDECK_CLI_TELEMETRY_DISABLED=0`, where the matrix jobs set it to `1`.
A telemetry-tagged test added to a slice will not behave as intended.

## Credentials, and logs anyone can read

This repository is public, so **every acceptance log is world-readable**.
`.github/workflows/acceptance.yml` says so itself, which is why the test
credentials have to belong to a test-only account rather than to a person: a key
that reaches every org its owner belongs to must never be one a failing test can
print.

The question worth asking of a diff is therefore: **does this print or commit a
credential?**

- a new `t.Logf`, `fmt.Print` or error string that interpolates a key, a config
  file or a whole HTTP request
- a test fixture or `.env`-shaped file added to the tree
- a redaction helper whose pattern is narrower than the data it guards

That last one has a concrete edge here. **The CLI writes single-quoted TOML.**
Viper's writer emits:

```toml
profile = 'default'

[default]
  api_key = 'cli_secret_abc123'
```

A redaction regex written against `api_key = "..."` — double quotes, as in the
`pkg/config/testdata/*.toml` fixtures — matches nothing against a config the CLI
actually wrote, and the key goes to the log verbatim. Check redaction patterns
against real output, not against a fixture.

## Things that compile, pass, and are still broken

Three surfaces in this repository have no test or CI job guarding them, so a
green run means nothing about them.

### `REFERENCE.md` is generated

It is produced by `tools/generate-reference`, and the generated regions are
fenced by `<!-- GENERATE... -->` / `<!-- GENERATE_END -->` markers.

- `go run ./tools/generate-reference --check` regenerates to a temp file, diffs,
  and **exits 1 when `REFERENCE.md` is stale**
- **no CI job runs it.** Nothing catches a command-surface change that leaves
  the file behind

So run it yourself when a diff touches commands, flags or examples. And flag a
hand-edit *inside* a `GENERATE` block: it is not wrong today, it is deleted on
the next regeneration, silently.

### The website is a second consumer

The same generator feeds the docs website, and it emits positional arguments
only when `Annotations["cli.arguments"]` is set on the command (`AGENTS.md`
§ Cobra Example and output for website docs). A new command that takes
positional arguments without that annotation documents them nowhere.

The review consequence: a command-surface change is not finished inside this
repository. Say so, so the docs change is not discovered after release.

### `CLITelemetry` JSON tags are a wire contract

`pkg/hookdeck/telemetry.go` defines `CLITelemetry`, sent on every API request
in the `X-Hookdeck-CLI-Telemetry` header. Its `json:"..."` tags —
`command_path`, `invocation_id`, `command_flags`, `mcp_client` and the rest —
are field names that Hookdeck's usage analytics query by name.

Renaming a field compiles, passes unit tests, passes acceptance, and silently
empties the dashboards built on the old name. Treat a change to those tags as a
breaking change to a consumer outside this repository, and say so explicitly.
Adding a field is safe; renaming or removing one is not.

## Docs that move with the code

A change to a command's surface is incomplete without:

- **`REFERENCE.md`** — regenerated, per above
- **Cobra `Short` and `Long`**, via the shared helpers in `pkg/cmd/helptext.go`
  rather than per-command literals (`AGENTS.md` § Command help text)
- **`KNOWN_ISSUES.md`**, when the change *fixes* user-visible behaviour. The
  file documents each issue by symptom, cause, affected versions and
  recommended fix. A fix that ships while its entry still says "Affected:
  v2.5.0 and earlier" with no resolution tells users to keep working around
  something that no longer exists.

## What not to raise

- **A `CHANGELOG.md` entry.** The file states plainly that it is no longer
  maintained; release notes come from the GitHub Releases page and are drafted
  at release time. Asking for an entry asks for something the repository
  deliberately stopped doing.
- **The `version` field in `package.json`.** It is machine-written:
  `release.yml` sets it from the tag and commits it during `publish-npm`. It is
  expected to lag the tag in the tree, and a human editing it by hand is noise,
  not a finding.
- **Style the repository does not enforce**, or a refactor nobody asked for. A
  review that returns twenty comments buries the one that mattered.
