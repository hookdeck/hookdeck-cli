# AGENTS.md — Issue auto-implement action

For agents making changes to this action. This file summarizes flows, design decisions, and implementation details.

## Flows

### 1. Issue to first PR

- **Triggers:** `issues.labeled` (prefixed trigger label), `issue_comment.created` on a labeled issue when **no PR exists yet**.
- **Flow:** Normalize event → assess (enough info?) → if `request_info`: post comment, add needs-info label, exit. If `implement`: implement step (push to branch `auto-implement-issue-<N>`) → verify → on fail retry (cap `max_implement_retries`); on pass create PR with "Closes #N", add pr-created label, optional comment.

### 2. Issue comment when PR already exists

- **Trigger:** `issue_comment.created` on an issue that **already has an open PR** for that issue.
- **Flow:** Post a short reply on the issue directing the user to the PR; exit. No assessment or implement.

### 3. PR review or PR conversation comment → iteration

- **Triggers:** `pull_request_review.submitted`, `pull_request_review_comment.created`, or `issue_comment.created` **on a PR** (when `issue.pull_request` is set) when the PR is from an automation branch or body contains "Closes #N".
- **Flow:** Resolve issue number from PR (body "Closes #N"/"Fixes #N" or head branch `auto-implement-issue-<N>`) → assess with issue + review/comment content → implement ("address review feedback"), push to same branch → verify → on pass: do **not** create PR; post comment on the PR summarizing the new commit(s).

## Event normalization

From the workflow event payload, derive:

- **Issue number:** For `issues` or `issue_comment`: `event.issue.number`. For `pull_request_review` or `pull_request_review_comment`: parse PR body for "Closes #N" or "Fixes #N", or PR head branch for `auto-implement-issue-<N>`.
- **PR exists for issue (issue_comment only):** Check whether an open PR exists for that issue (e.g. head branch `auto-implement-issue-<N>` or body "Closes #<N>").

## Request more info vs comment body from implement

- **Request more info (assess):** The **assess** step decides there is **not enough information** to implement. It returns `action: request_info` and a `comment_body`. The workflow posts that on the issue (or on the PR when the trigger was a PR review/comment), adds the `needs-info` label, and **exits without running implement**.
- **Comment body from implement (no change or need clarification):** The assess step said **implement**. The **implement** step ran (Claude Code CLI with full repo context). When Claude **chose not to make code changes** — e.g. the feedback is a question, the current approach is preferred, or it needs clarification — it **must** write `.comment_body` with the text to post on the PR (one or two sentences). The workflow posts that on the **PR** (review iteration path). If `.comment_body` is missing, the workflow falls back to a generic message. The implement prompt requires Claude to always write `.comment_body` when making no code changes so reviewers get a useful reply. Use for: (a) no-change scenarios (thank the reviewer, briefly explain), or (b) when more information is requested (e.g. "Could you clarify whether you want X or Y?").

If a reviewer's comment is ambiguous, assess might still return **implement** (optimistic). Then implement runs; Claude can either make a best-effort change, or write `.comment_body` asking for clarification. That clarification is posted on the PR.

## Assess script

- **Path:** `assess/src/index.ts` (TypeScript), run with `npx tsx src/index.ts` from the assess directory (no build).
- **Input:** Reads event from `GITHUB_EVENT_PATH`; optional context files from input.
- **Output:** JSON with `action` (`implement` | `request_info`), `comment_body` (if request_info), `verification_notes` (optional). Written to file or GITHUB_OUTPUT.
- **When triggered by PR review:** Include PR review body and review comments in the payload sent to Claude.

## Implement script

- **Path:** `assess/src/implement.ts`, run with `npx tsx src/implement.ts` from the assess directory.
- **Env:** `ISSUE_NUMBER`, `GITHUB_REPOSITORY`, `GITHUB_TOKEN`, `AUTO_IMPLEMENT_ANTHROPIC_API_KEY` (required); `VERIFICATION_NOTES`, `GITHUB_WORKSPACE`, `CONTEXT_FILES`, `IMPLEMENT_COMMIT_MSG_FILE`, `PREVIOUS_VERIFY_OUTPUT` (optional).
- **Flow:** Fetches issue title/body from GitHub API, loads context files, then runs **Claude Code CLI** (`claude` on PATH) in the repo root with a prompt; the script passes `AUTO_IMPLEMENT_ANTHROPIC_API_KEY` to the CLI as `ANTHROPIC_API_KEY`. The CLI implements in-repo (Read/Edit/Bash). When Claude makes code changes it writes `.commit_msg`, `.pr_title`, `.pr_body`; when it makes no code changes it writes `.comment_body` (no-change rationale or request for clarification). The script ensures commit/PR meta files exist only when Claude did not write `.comment_body`. When `PREVIOUS_VERIFY_OUTPUT` is set (e.g. after a failed verify run), it is included in the prompt so the CLI can fix the implementation. In CI, the action installs the CLI (`npm install -g @anthropic-ai/claude-code`) before the implement step when the assess outcome is `implement`.

## Implement–verify loop

- **Single step** `implement_verify_loop`: for each attempt from 1 to `max_implement_retries`, run implement (with `PREVIOUS_VERIFY_OUTPUT` from the previous failure, if any), commit and push, then run `verify_commands`. If verify passes, exit success. If it fails, set the verify output as `PREVIOUS_VERIFY_OUTPUT` and retry. After all attempts, fail. When this step succeeds: if trigger was `pull_request_review`, `pull_request_review_comment`, or `issue_comment` on a PR (`issue.pull_request` set), post a comment on the existing PR (no new PR); otherwise create PR.

## Branch and PR

- Branch name: `auto-implement-issue-<issue_number>`.
- PR title or body must include "Closes #N" (or "Fixes #N") so merging auto-closes the issue.
- On iteration (PR already exists): do not create a new PR; post a comment on the PR summarizing the new commit(s).

## Restricting who can trigger

The first step enforces one of two gates (exactly one must be configured; no bypass):

1. **Permission check** — If `github_allowed_trigger_min_permission` is set (repo variable `AUTO_IMPLEMENT_ALLOWED_TRIGGER_MIN_PERMISSION`: `triage`, `push`, `maintain`, or `admin`), the action calls the repo collaborator permission API and requires the actor to have at least that permission. Works with the default `GITHUB_TOKEN`.
2. **Team check** — Otherwise, if `github_allowed_trigger_team` is set (repo variable `AUTO_IMPLEMENT_ALLOWED_TRIGGER_TEAM`, e.g. `org/team`), the action checks `github.actor` against that team; if unset or not a member, fail. Token must have `read:org` (PAT if `GITHUB_TOKEN` lacks it).

If neither variable is set, the step fails. When both are set, the permission check is used and the team value is ignored.

## Labels

All automation labels use `label_prefix` (default `automation`): `{prefix}/auto-implement`, `{prefix}/needs-info`, `{prefix}/pr-created`. Create via API if missing.

## Local development

Scripts load a `.env` file from the action root or cwd (see README **Local runs**). Key env: `AUTO_IMPLEMENT_ANTHROPIC_API_KEY`, `GITHUB_TOKEN`, `GITHUB_REPOSITORY`, `ISSUE_NUMBER` (implement). Copy `.env.example` to `.env` and fill; optional `./scripts/setup-local-env.sh --with-gh` to set `GITHUB_TOKEN` from `gh auth token`.

## Verification

When changing this action or the assess script:

1. **Run the assess unit tests locally** before committing: `cd .github/actions/issue-auto-implement/assess && npm ci && npm test`. Do not rely on CI alone—catch failures locally first, then push.
2. **CI** runs the same tests in `.github/workflows/issue-auto-implement-test.yml` when you push or open a PR that touches `.github/actions/issue-auto-implement/**`.
3. **Optional:** Run the assess script with a fixture and `AUTO_IMPLEMENT_ANTHROPIC_API_KEY` for end-to-end assessment behavior.
4. **Full workflow:** Trigger manually on a test issue (trigger label or comment). Ensure repo secrets/variables are set. Inspect the Actions run and the issue/PR; re-run after changes to the implement or verify loop.

## Next steps (implementation backlog)

Possible future improvements:

1. **Fetch all issue comments** — For `issues` and `issue_comment` events, optionally call the GitHub API to list all comments on the issue and include them in the assessment payload (not only the single comment from the event).
2. **Optional comment when PR is created** — Input `post_pr_comment` exists; when true, post a short comment on the issue linking to the new PR when one is created.
3. **Local run with fixture and Claude** — `npm run assess:fixture` exists; optional end-to-end testing with real `AUTO_IMPLEMENT_ANTHROPIC_API_KEY`.

Done: context files in assess, implement step (Claude Code CLI), implement–verify loop with re-implement on failure, PR review iteration (comment on PR, no new PR), comment when retries exhausted, secrets/variables and README docs, local assess script.
