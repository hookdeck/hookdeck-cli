# AGENTS.md — Issue auto-implement action

For agents making changes to this action. This file summarizes flows, design decisions, and implementation details.

## Flows

### 1. Issue to first PR

- **Triggers:** `issues.labeled` (prefixed trigger label), `issue_comment.created` on a labeled issue when **no PR exists yet**.
- **Flow:** Normalize event → assess (enough info?) → if `request_info`: post comment, add needs-info label, exit. If `implement`: implement step (push to branch `auto-implement-issue-<N>`) → verify → on fail retry (cap `max_implement_retries`); on pass create PR with "Closes #N", add pr-created label, optional comment.

### 2. Issue comment when PR already exists

- **Trigger:** `issue_comment.created` on an issue that **already has an open PR** for that issue.
- **Flow:** Post a short reply on the issue directing the user to the PR; exit. No assessment or implement.

### 3. PR review → iteration

- **Triggers:** `pull_request_review.submitted`, `pull_request_review_comment.created` when the PR is from an automation branch or body contains "Closes #N".
- **Flow:** Resolve issue number from PR (body "Closes #N"/"Fixes #N" or head branch `auto-implement-issue-<N>`) → assess with issue + review content → implement ("address review feedback"), push to same branch → verify → on pass: do **not** create PR; post comment on the PR summarizing the new commit(s).

## Event normalization

From the workflow event payload, derive:

- **Issue number:** For `issues` or `issue_comment`: `event.issue.number`. For `pull_request_review` or `pull_request_review_comment`: parse PR body for "Closes #N" or "Fixes #N", or PR head branch for `auto-implement-issue-<N>`.
- **PR exists for issue (issue_comment only):** Check whether an open PR exists for that issue (e.g. head branch `auto-implement-issue-<N>` or body "Closes #<N>").

## Assess script

- **Path:** `assess/index.ts` (TypeScript), run with `npx tsx assess/index.ts` (no build).
- **Input:** Reads event from `GITHUB_EVENT_PATH`; optional context files from input.
- **Output:** JSON with `action` (`implement` | `request_info`), `comment_body` (if request_info), `verification_notes` (optional). Written to file or GITHUB_OUTPUT.
- **When triggered by PR review:** Include PR review body and review comments in the payload sent to Claude.

## Implement script

- **Path:** `assess/implement.ts`, run with `npx tsx implement.ts` from the assess directory.
- **Env:** `ISSUE_NUMBER`, `GITHUB_REPOSITORY`, `GITHUB_TOKEN`, `ANTHROPIC_API_KEY` (required); `VERIFICATION_NOTES`, `GITHUB_WORKSPACE`, `CONTEXT_FILES`, `IMPLEMENT_COMMIT_MSG_FILE`, `PREVIOUS_VERIFY_OUTPUT` (optional).
- **Flow:** Fetches issue title/body from GitHub API, loads context files, calls Claude for JSON `{ edits: [{ path, contents }], commit_message }`, applies edits under repo root, writes commit message to `IMPLEMENT_COMMIT_MSG_FILE`. Paths in edits must be relative; script validates they stay inside repo root. When `PREVIOUS_VERIFY_OUTPUT` is set (e.g. after a failed verify run), it is included in the prompt so Claude can fix the implementation.

## Implement–verify loop

- **Single step** `implement_verify_loop`: for each attempt from 1 to `max_implement_retries`, run implement (with `PREVIOUS_VERIFY_OUTPUT` from the previous failure, if any), commit and push, then run `verify_commands`. If verify passes, exit success. If it fails, set the verify output as `PREVIOUS_VERIFY_OUTPUT` and retry. After all attempts, fail. Create PR only when this step succeeds.

## Branch and PR

- Branch name: `auto-implement-issue-<issue_number>`.
- PR title or body must include "Closes #N" (or "Fixes #N") so merging auto-closes the issue.
- On iteration (PR already exists): do not create a new PR; post a comment on the PR summarizing the new commit(s).

## Restricting who can trigger

Only members of the `github_allowed_trigger_team` (input; set via repo variable `AUTO_IMPLEMENT_ALLOWED_TRIGGER_TEAM`) may trigger the flow. First step checks `github.actor` against that team; if the variable is unset or the actor is not a member, fail immediately. Token must have `read:org`.

## Labels

All automation labels use `label_prefix` (default `automation`): `{prefix}/auto-implement`, `{prefix}/needs-info`, `{prefix}/pr-created`. Create via API if missing.

## Verification

When changing this action or the assess script:

1. **Run the assess unit tests locally** before committing: `cd .github/actions/issue-auto-implement/assess && npm ci && npm test`. Do not rely on CI alone—catch failures locally first, then push.
2. **CI** runs the same tests in `.github/workflows/issue-auto-implement-test.yml` when you push or open a PR that touches `.github/actions/issue-auto-implement/**`.
3. **Optional:** Run the assess script with a fixture and `ANTHROPIC_API_KEY` for end-to-end assessment behavior.
4. **Full workflow:** Trigger manually on a test issue (trigger label or comment). Ensure repo secrets/variables are set. Inspect the Actions run and the issue/PR; re-run after changes to the implement or verify loop.

## Next steps (implementation backlog)

Recommended order:

1. **Context files in assess** — Pass the `context_files` input into the assess script (e.g. via env) and include those file contents (AGENTS.md, REFERENCE.md, etc.) in the Claude assessment prompt so the model has full repo guidance.
2. **Fetch all issue comments** — For `issues` and `issue_comment` events, optionally call the GitHub API to list all comments on the issue and include them in the assessment payload (not only the single comment from the event).
3. **Implement step (real)** — Replace the placeholder with Claude generating and applying code changes. Options: call Anthropic API to produce a patch or file edits from the issue body + verification_notes (and repo context), then apply, commit, and push; or integrate a Claude Code Action / external tool. Branch is already checked out; implement must make commits and push.
4. **True implement–verify loop** — On verify failure, re-run the **implement** step with the verify failure output in the prompt (then verify again), up to `max_implement_retries`. Currently only the verify command is retried; the plan requires re-implementing with failure context.
5. **PR review iteration path** — When the trigger is `pull_request_review` or `pull_request_review_comment`: after verify passes, do **not** create a new PR; instead post a comment on the existing PR summarizing the changes in the new commit(s). Detect "PR already exists" (e.g. from event type or by checking for an open PR for this branch) and branch the flow: create PR vs. comment on PR.
6. **Optional comment when PR is created** — Add an input (e.g. `post_pr_comment`) and, when creating a PR, optionally post a short comment on the issue linking to the new PR.
7. **Comment when retries exhausted** — When the verify loop fails after all retries, post a comment on the issue so the run is visible and explainable from the issue thread. Add a step that runs on failure (e.g. `if: failure() && steps.assess.outputs.action == 'implement'`) and posts a comment on the issue: e.g. "The auto-implement run could not complete: verification failed after N attempts. See the [workflow run](link) for logs. You can address the failure and re-trigger by adding a comment or re-applying the label."
8. **Secrets and variables** — Document in README: add `AUTO_IMPLEMENT_ANTHROPIC_API_KEY` as a repo secret; set `AUTO_IMPLEMENT_ALLOWED_TRIGGER_TEAM` (required) as a repo variable; note that the default `GITHUB_TOKEN` may need to be replaced with a PAT that has `read:org` for the team check.
9. **Local run with fixture and Claude** — Add an npm script or small wrapper (e.g. `npm run assess:fixture issue-labeled`) to run the assess script with a fixture and real `ANTHROPIC_API_KEY` for manual end-to-end assessment testing.
