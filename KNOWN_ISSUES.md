# Known Issues

## `hookdeck listen` never prints "Connected" when output is piped or `--color off` is set

**Symptom:** `hookdeck listen` prints the connection banner and then nothing. The documented
`Connected. Waiting for events...` line never appears, so scripts, CI jobs and agent harnesses
waiting for it time out — even though the tunnel is connected and forwarding events.

**Why:** The readiness line was printed only when a spinner was drawn, and no spinner is drawn
when the output stream is not a terminal or when colors are disabled.

**Affected:** v2.5.0 and earlier.

**Recommended fix:** Update to **v2.5.1 or later**.

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
