# CLI surface — manual QA checklist

Read [../SKILL.md](../SKILL.md) first, including the safety model. Every command
below takes `--hookdeck-config "$HD_CONFIG"`.

Two projects, two credentials:

| Surface | Key in `test/acceptance/.env` |
|---|---|
| Gateway (`hookdeck gateway …`) | `HOOKDECK_CLI_TESTING_API_KEY` |
| Outpost (`hookdeck outpost …`) | `HOOKDECK_CLI_OUTPOST_TESTING_API_KEY` |

This is a checklist of **surfaces**, not a script. The value is in what you try
against each one, so vary it between runs rather than replaying the same calls.

## Gateway resources

`source` · `destination` · `connection` · `event` · `request` · `attempt` ·
`transformation` · `issue` · `metrics`

For each resource that supports them, exercise `list`, `get`, `create`,
`update`, `upsert`, `delete`, and any state verbs (`enable`/`disable`,
`pause`/`unpause`, `retry`/`cancel`/`mute`, `dismiss`).

Beyond the happy path:

- [ ] **State verbs report the resulting state.** After every `enable`,
      `disable`, `pause`, `cancel`, `mute`, run a separate `get` and confirm the
      resource actually changed. Repeat the verb on a resource already in that
      state and check the second call does not claim to have changed anything.
- [ ] **`retry` on a body carrying its own status.** Confirm a declined retry
      exits non-zero rather than printing a tick.
- [ ] **Filters narrow the result set.** For every list filter (`--status`,
      `--source-id`, `--search-term`, date ranges), compare filtered and
      unfiltered counts. A filter the API ignores returns plausible wrong data.
      `--search-term` matches a whole field value, not a substring — confirm
      that is still true and that the help says so.
- [ ] **Pagination.** `--next` / `--prev` across a boundary; confirm no repeats
      and no skips.
- [ ] **`transformation run`.** A handler that returns a request; one that calls
      `console.error` and still returns (must succeed, exit 0, and show the
      console output); one that throws; one that returns nothing (must fail).
- [ ] **Output modes.** `--output json` parses as JSON on both success and
      failure paths, and the exit code matches the outcome in both.

## Outpost resources

`tenant` · `destination` · `destination-type` · `event` · `attempt` · `topic` ·
`config` · `metrics` · `publish` · `status`

- [ ] **Destination types beyond `webhook`.** Create one of each supported type
      (AWS, GCP, Azure, RabbitMQ, Kafka) and confirm the stored config matches
      what the schema declares — particularly any field with a default, which
      must be sent rather than left unset.
- [ ] **Credentials are masked on read, except where the platform generates
      them.** Webhook secrets are produced server-side and must come back;
      caller-supplied credentials must not.
- [ ] **Clearing works, not just setting.** `--filter '{}'`, `--unset` on
      config, removing a topic. Read the resource back to confirm.
- [ ] **`config set`.** Round-trip a value and restore it; confirm `--dry-run`
      changes nothing and that a real set is visible in `config get`.
- [ ] **`tenant portal`** returns a URL, and `--theme light|dark` is accepted.
      Do not test `--open`; it launches a browser.
- [ ] **`publish`** an event and follow it through `event list` → `attempt list`.

## Cross-cutting

- [ ] **Unknown flags and unknown enum values** are rejected with a message that
      names the offending value.
- [ ] **Missing required arguments** fail before any network call.
- [ ] **A resource id containing `/` or `..`** is rejected as an invalid
      identifier rather than addressing a different resource.
- [ ] **`--help` matches behaviour** — documented defaults are actually applied,
      documented flags exist, and `REFERENCE.md` is current
      (`go run ./tools/generate-reference --check`).

## Cleanup

Delete every resource created, in reverse order. Deleting an Outpost tenant
cascades to its destinations. Report anything left behind by name.
