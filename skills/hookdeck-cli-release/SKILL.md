---
name: hookdeck-cli-release
description: >-
  Guides maintainers through Hookdeck CLI releases (stable GA, beta from main,
  beta from feature branches) and user-centric GitHub release notes. Validates
  proposed versions against SemVer from the actual change set (e.g. breaking
  changes require a major bump). Use when cutting a release, publishing a tag,
  drafting release notes, choosing vMAJOR.MINOR.PATCH, GoReleaser, npm publish,
  pre-releases, following the release checklist, or `gh release create`.
---

# Hookdeck CLI — release workflow

## Canonical documentation

Follow **[README.md](../../README.md) § Releasing** for human-oriented steps (GitHub UI alternative, install commands for beta/stable).

**Agents:** perform the **publish** step with the **GitHub CLI** (`gh`) — see **Publish with GitHub CLI (`gh`)** below (temp notes file → `gh release create` → remove temp file).

This skill adds **how automation works**, **release note expectations**, and a **research loop** for drafting notes.

## Agent checklist (end-to-end)

Follow **in order**. Treat items with **gate** as blocking unless the maintainer explicitly overrides.

- [ ] **Release shape:** GA from **`main`** vs beta from **`main`** vs beta from **feature branch** — matches [README § Releasing](../../README.md) and maintainer intent.
- [ ] **`PREV_TAG` / `NEW_TAG`:** Confirmed (or proposed and agreed); baseline tag is correct for the line of development (e.g. last GA vs beta series).
- [ ] **Change set:** Reviewed `git log PREV_TAG..HEAD` (and diff if needed); changes grouped for **user-facing** release notes (see **Research loop**).
- [ ] **SemVer gate:** Proposed `NEW_TAG` matches **minimum** MAJOR/MINOR/PATCH for the delta (see **SemVer: validate the proposed version**). Stop and realign if under-bumped.
- [ ] **Release notes:** Draft complete (see **Drafting release notes** and [references/release-notes-template.md](references/release-notes-template.md)); includes **Full Changelog** compare link; **contributor shout-outs only when warranted** (see that section).
- [ ] **CI gate:** Latest commit on the **target branch** has **green** GitHub checks (mandatory for GA on `main`; required for betas on the branch being tagged).
- [ ] **Approval:** Maintainer signed off on tag name, notes, and branch — no unilateral surprise tags.
- [ ] **Publish:** Write notes to a **temporary file**, run **`gh release create`** (see **Publish with GitHub CLI (`gh`)**), then **`rm`** the temp file. Use `--prerelease` for betas. (Humans may still use the GitHub UI per README.)
- [ ] **Post-publish (optional):** Confirm the **`release`** workflow in Actions completed successfully for the new tag.

For commit-level detail while working through the checklist, use the **Research loop** below.

## What triggers a release?

- **[.github/workflows/release.yml](../../.github/workflows/release.yml)** runs on **`push` of tags** matching `v*` (not on ordinary branch pushes).
- Publishing a release in the GitHub UI (with a new tag) or `git push origin vX.Y.Z` both create that tag push and start the workflow.

## What the workflow does (high level)

1. **GoReleaser** (macOS, Linux, Windows jobs): builds binaries/archives, updates distribution channels per [.goreleaser/*.yml](../../.goreleaser/mac.yml) (Homebrew, Scoop, Docker, GitHub release artifacts). Config uses `release.mode: append` and `changelog.disable: true` — **GoReleaser does not write the release note body from git**; maintainers supply or edit the GitHub release description.
2. **`publish-npm` job**: Determines which branch contains the tag (prefers `main` / `master`, else first matching remote branch), checks out that branch, sets `package.json` version from the tag, builds npm binaries via GoReleaser, runs **`npm publish`** with `latest` for stable semver or a derived tag (e.g. `beta`) for pre-releases (see workflow `npm_tag` step).

## Stable (GA) release

- **Humans (README):** GitHub Releases → Draft → new tag `vM.m.p` → target **`main`** → notes → Publish.
- **Agents:** After gates pass, use **`gh release create`** with `--target main` and `--notes-file` (see **Publish with GitHub CLI (`gh`)**).
- **Do not publish a GA release until CI is green for `main`:** the **latest commit on `main`** must show successful checks in GitHub (same bar as README: ensure tests pass on `main`). Verify on the **Actions** tab (filter branch `main`, confirm the run for the tip of `main` succeeded) or via the commit’s status on github.com.
- **CLI check (optional):** after `git fetch origin main`, confirm combined status is `success` for `origin/main` (replace owner/repo if forked):

  ```bash
  SHA=$(git rev-parse origin/main)
  gh api "repos/hookdeck/hookdeck-cli/commits/${SHA}/status" --jq .state
  ```

  Do **not** tag or publish GA if this is `failure` or still `pending` for required work.

- Stable tags drive **`latest`** on npm, stable Homebrew/Scoop formulas, Docker `latest`.

## Pre-release (beta)

- **From `main`:** Tag like `v1.3.0-beta.1`, target `main`, mark **pre-release** (`gh release create ... --prerelease`). Good for broad beta testing. **Still verify `main` is green** (same CI check as GA) before tagging.
- **From a feature branch:** Same tag pattern; **`--target <feature-branch>`** so the workflow builds that tip. **Verify CI is green for that branch’s latest commit** before tagging. Add notes on **what to test** (betas often ship with minimal notes; still document intent).
- Install paths for beta: see README (npm `@beta`, `hookdeck-beta` brew/scoop, Docker image tag — **not** `latest` for beta).

## Publish with GitHub CLI (`gh`)

**Agents should create the release with `gh`**, not only push a bare tag. That creates the GitHub Release (with notes) and the tag together, which matches how maintainers expect the **`release`** workflow to run.

1. **Create a temp file for notes** (never commit it). Register cleanup so the file is removed even if `gh` fails:

   ```bash
   NOTES_FILE="$(mktemp "${TMPDIR:-/tmp}/hookdeck-cli-release-notes.XXXXXX.md")"
   trap 'rm -f "$NOTES_FILE"' EXIT
   ```

2. **Write** the final markdown body to `"$NOTES_FILE"` (same content you would paste in the GitHub UI).

3. **Create the release** (run from a clone of `hookdeck/hookdeck-cli`, or use `--repo` as below):

   **Stable GA from `main`:**

   ```bash
   gh release create "vM.m.p" \
     --repo hookdeck/hookdeck-cli \
     --target main \
     --title "vM.m.p" \
     --notes-file "$NOTES_FILE"
   ```

   **Pre-release (beta):** add `--prerelease`. **Feature branch:** set `--target <branch>` instead of `main`.

4. **Cleanup:** With `trap` above, the temp file is deleted on shell exit. If you did not use `trap`, run `rm -f "$NOTES_FILE"` after `gh` succeeds.

**Requirements:** `gh` installed and authenticated (`gh auth login`). Do not put secrets in the notes file.

### Fallback: tag without `gh`

If `gh` is unavailable, a maintainer may use **README** flow (UI) or:

```bash
git checkout <branch>
git tag vX.Y.Z[-beta.N]
git push origin vX.Y.Z[-beta.N]
```

Then **edit the GitHub release** to add notes, or create the release in the UI so assets and changelog align with team practice.

## SemVer: validate the proposed version

The user may suggest a tag (e.g. `v2.0.1`). **Always sanity-check it** against what actually changed since **`PREV_TAG`** (usually the last **GA** tag on that line of development—confirm with the maintainer for long beta series).

**Interpret SemVer for this CLI (user-facing contract):**

| Change since `PREV_TAG` | Bump | Examples |
|-------------------------|------|----------|
| **Breaking** — requires users to change scripts, configs, or habits | **MAJOR** | Removed or renamed commands/flags; different defaults that break automation; dropped or incompatible config file fields; incompatible change to documented machine-readable output |
| **New capability**, backward compatible | **MINOR** | New commands or flags; new subcommands; additive behavior; deprecations **announced** but old path still works |
| **Fixes / internal / docs-only** (no new user-facing capability, no break) | **PATCH** | Bug fixes; telemetry/CI; dependency bumps with no CLI contract change; help text clarifications |

**Signals (hints only):** Conventional commits with `BREAKING CHANGE:` / `feat!:` / `fix!:` suggest severity—still **read the diff and release notes**; commits can be mis-tagged.

**Pre-releases** (`v2.1.0-beta.1`): the **base version** (`2.1.0`) must still follow the table above relative to the last GA. A beta for a **major** rewrite should be `v3.0.0-beta.1`, not `v2.5.0-beta.1`, if the delta includes breaking changes vs `v2.x` GA.

**Agent behavior:**

1. After categorizing changes for release notes, state the **minimum** SemVer bump required.
2. Compare to the user’s proposed `NEW_TAG`. If they conflict (e.g. patch tag but breaking changes), **do not treat the user’s version as authoritative**—explain the mismatch and recommend the correct `vMAJOR.MINOR.PATCH` (and pre-release suffix if applicable).
3. If ambiguous (unclear whether a change breaks callers), **ask the maintainer** before tagging.

## Drafting release notes (user-centric)

Use **[references/release-notes-template.md](references/release-notes-template.md)** as a starting skeleton.

**Sections:** Include only headings that have real content — **omit** empty sections (e.g. do not add “Breaking changes” with “None”).

**Patterns observed in this repo:**

- **Large GA (e.g. v2.0.0):** `Summary`, then as needed: `Breaking changes / migration`, `New features` (subsections per area), `Improvements / behavior changes`, `Internal` — skip any block with nothing to say.
- **Feature release (e.g. v1.9.0):** `## Features` with detailed bullets + **Full Changelog** compare link.
- **Patch (e.g. v1.9.1):** `## Fixes`, `## Updates`, PR links with authors + **Full Changelog** — omit unused sections.

Always include a **Full Changelog** line:

`https://github.com/hookdeck/hookdeck-cli/compare/<prev_tag>...<new_tag>`

**Contributors / shout-outs:** Do **not** add a generic “thanks to all contributors” block every release. **Regular maintainers and repeat contributors do not need a call-out.** Only include a contributor section when:

- There are **new contributors** first shipping in this release (welcome them by name/GitHub handle), and/or
- Someone made an **exceptionally large** contribution worth highlighting for this specific release.

Otherwise omit the **Contributors** section entirely.

## Research loop (agent or maintainer)

1. **Tags:** Confirm `PREV_TAG` and `NEW_TAG` with the user (or `git describe --tags --abbrev=0` on the release branch). For beta series, baseline may be last **GA** tag.
2. **Commits:** `git log PREV_TAG..HEAD --oneline` and read full messages. Treat **Conventional Commits** (`feat:`, `fix:`, `BREAKING CHANGE:`) as hints only — rewrite for **user impact** (commands, flags, migrations).
3. **Group:** Merge related commits; call out breaking changes and required user actions explicitly.
4. **SemVer check:** Using **SemVer: validate the proposed version**, classify the delta since `PREV_TAG` and verify the proposed `NEW_TAG` matches the required **MAJOR / MINOR / PATCH** bump. Flag mismatches before any tag or release.
5. **PRs / links:** Map commits to PRs (`gh pr list`, GitHub compare UI) for **PR links in the notes**. Use **Contributors** shout-outs only per **Drafting release notes** (new contributors or exceptional contribution—not every author every time).
6. **Sanity:** Skim diff or `REFERENCE.md` / user-facing help if commits are unclear.
7. **CI on GitHub (gate):** Before tagging, confirm the **branch you will release** (`main` for typical GA, or the feature branch for a branch beta) has **green checks on the latest commit** in GitHub Actions / commit status. For GA from `main`, treat this as **mandatory**; do not proceed on red or unknown pending required checks.

## Safety and governance

- **CI:** Do not cut a **stable GA** release unless **`main`’s latest run of checks** in GitHub is green (see **Stable (GA) release** and research step 7). For betas, require green CI for the **target branch** you are tagging.
- **SemVer:** Do not publish a tag that **under-bumps** the version for the change set (e.g. patch release that includes breaking CLI changes); resolve with the maintainer first.
- Do not push surprise tags; respect branch protection and team process.
- Never put secrets or tokens in release notes or skill content.

## Related files

| Topic | Location |
|--------|-----------|
| Maintainer steps, install commands | [README.md § Releasing](../../README.md) |
| CI entrypoint | [.github/workflows/release.yml](../../.github/workflows/release.yml) |
| Artifacts / brew / scoop / docker | [.goreleaser/](../../.goreleaser/) |
