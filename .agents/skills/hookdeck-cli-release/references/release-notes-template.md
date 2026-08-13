# Release notes template (Hookdeck CLI)

Copy and fill in for the GitHub release description (e.g. write into the temp file used with `gh release create --notes-file`). Adjust heading levels (`##` vs `###`) to match recent GA style (v2.0.0 used `###` subsections) or smaller patch style (v1.9.x used `##`).

**Do not include a section if there is nothing to say** — omit the heading entirely (do not add “Breaking changes” with “None”, empty “Fixes”, etc.). Typical releases only need a subset of the sections below.

## Summary

<!-- 2–4 sentences: who benefits, scope since previous tag (e.g. since `v1.9.1`). -->

## Breaking changes / migration

<!-- Include only when users must change scripts, configs, or habits. For each item: what changed, why, what to update (scripts, flags, CI). Omit this whole section when there are no breaking changes. -->

## New features

<!-- Include when shipping user-visible capability. Group by area/command; mention important flags. Omit if nothing applies. -->

## Fixes

<!-- Omit if no fixes, or merge into Improvements for tiny releases. -->

## Improvements / behavior changes

<!-- UX, defaults, performance, reliability visible to users. Omit if nothing applies. -->

## Internal / reliability / infrastructure

<!-- Refactors, test/CI, dependency bumps with low user-visible impact. Omit if nothing worth mentioning. -->

## Contributors (optional)

<!-- Omit this entire section for most releases.

Include ONLY when:
- There are **new contributors** whose first merged work ships in this release — welcome them by name / @handle, OR
- Someone made an **exceptionally large** contribution worth a specific shout-out for this release.

Do NOT add generic "thanks to everyone who contributed" for regular maintainers or repeat contributors. -->

**Full Changelog**: https://github.com/hookdeck/hookdeck-cli/compare/PREV_TAG...NEW_TAG
