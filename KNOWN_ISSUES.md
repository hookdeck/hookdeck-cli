# Known Issues

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
