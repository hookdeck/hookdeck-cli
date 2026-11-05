# Known Issues

## `--rule-transform-name` with an ID or a mistyped name attaches an empty transformation

**Symptom:** `hookdeck gateway connection create`, `update` or `upsert` with `--rule-transform-name`
set to a transformation ID, or to a name that doesn't exist, succeeds, but every event on the
connection then fails its transformation with a `TypeError`.

**Why:** The CLI always sent the value as a transformation name. When no transformation has that
name, the API creates one, even with no code, so the connection got a new, empty transformation
instead of the one you meant.

**Affected:** v3.1.0 and earlier.

**Recommended fix:** Update to **v3.2.0 or later**. The value is then resolved as an ID or an exact
name, and an unknown value without `--rule-transform-code` is an error.

- npm: `npm install -g hookdeck-cli@latest`
- Homebrew: `brew upgrade hookdeck`

**Workaround (until you update):** Pass the rule as JSON with `--rules` or `--rules-file`,
referencing the transformation by `transformation_id`.

**Cleanup:** If you hit this, you have empty transformations named after an ID (`trs_...`) or a
typo. Find them with `hookdeck gateway transformation list`. Point any connection that uses one at
the right transformation, then delete it with `hookdeck gateway transformation delete <id>`.

**Tracking:** #468

## Connection rules built from `--rule-*` flags run in a fixed order, not the order you wrote them

**Symptom:** A connection created, updated or upserted with `--rule-*` flags runs its rules in a
different order from the flags. For example, `--rule-filter-body … --rule-transform-name …`
produces a transform rule followed by a filter rule, so the filter sees the transformed payload
instead of the original one.

**Why:** The CLI always built rules as deduplicate, transform, filter, delay, retry, whatever the
order of the flags. Filter, transform and deduplicate rules run in the order they appear in the
connection's `rules` array.

**Affected:** v3.1.0 and earlier. `--rules` and `--rules-file` were not affected: they send the
array exactly as given.

**Recommended fix:** Update to **v3.2.0 or later**. Rules then follow the position of each rule
type's first flag. If you put filter flags before transform flags and relied on transform running
first, put the transform flags first.

- npm: `npm install -g hookdeck-cli@latest`
- Homebrew: `brew upgrade hookdeck`

**Workaround (until you update):** Pass the rules as JSON with `--rules` or `--rules-file`, in the
order you want them to run.

**Tracking:** #464

## `hookdeck listen` never prints "Connected" when output is piped or `--color off` is set

**Symptom:** `hookdeck listen` prints the connection banner and then nothing. The documented
`Connected. Waiting for events...` line never appears, so scripts, CI jobs and agent harnesses
waiting for it time out — even though the tunnel is connected and forwarding events.

**Why:** The readiness line was printed only when a spinner was drawn, and no spinner is drawn
when the output stream is not a terminal or when colors are disabled.

**Affected:** v2.5.0 and earlier.

**Recommended fix:** Update to **v2.6.0 or later**.

- npm: `npm install -g hookdeck-cli@latest`
- Homebrew: `brew upgrade hookdeck`

**Workaround (until you update):** Drop `--color off` and run on a terminal, or treat the first
forwarded event rather than the readiness line as your ready signal. To confirm the tunnel is up
without waiting for traffic, run with `--log-level debug` and look for `Connected!`.

**Tracking:** #376

## `hookdeck listen` can stop delivering events after an extended disconnect

**Symptom:** `hookdeck listen` appears connected but events stop arriving; the Hookdeck
dashboard shows delivery attempts failing with `CLI_UNAVAILABLE`.

**Why:** A CLI session can expire server-side (for example after your machine has been asleep
or offline for a while). Affected CLI versions don't automatically re-establish it.

**Recommended fix:** Update to **v2.3.2 or later** — newer versions recover automatically.

- npm: `npm install -g hookdeck-cli@latest`
- Homebrew: `brew upgrade hookdeck`

**Workaround (until you update):** Stop (`Ctrl+C`) and restart `hookdeck listen`. This creates
a fresh session and delivery resumes. Logging out / logging in is not required.

**Tracking:** #323
