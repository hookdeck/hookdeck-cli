# Diagnosis: PR #261 — Bot replied "Verification passed" instead of adding tests

## What happened

- **PR #261** added `--local` to `login` and `ci` (auto-implement from issue #215).
- **Reviewer comment:** "Could we add some unit and acceptance test coverage for this change?"
- **Bot response:** "The --local flag has already been implemented... Verification passed." — no new tests were added.

## Root cause: review feedback never reaches the implement step

The pipeline has two steps that use Claude:

1. **Assess** (`assess/src/index.ts`) — decides `implement` vs `request_info`, outputs `verification_notes`.
2. **Implement** (`assess/src/implement.ts`) — runs Claude Code CLI in the repo to make changes.

**Assess** receives the full event payload. For a comment on the PR (`issue_comment` on the PR, or `pull_request_review`), the prompt includes:
- Issue (for a PR comment: the PR title and body),
- "All issue comments" (PR comments from the API),
- "Latest event comment" or "PR review (address this feedback):" with the reviewer's text.

So assess **does** see "Could we add some unit and acceptance test coverage for this change?" and correctly returns `action: implement`.

**Implement** only receives:
- `ISSUE_NUMBER` (215),
- Issue title/body **fetched from the GitHub API for issue #215** (the original issue about `--local`),
- `VERIFICATION_NOTES` (from assess: generic "run test suite" style notes),
- Context files, `PREVIOUS_VERIFY_OUTPUT`.

So implement **never** sees the PR comment or review body. The Claude Code CLI prompt is built from the **original issue** only. It has no instruction to "add unit and acceptance tests" — so it reasonably treats the issue as already done and writes `.comment_body` ("Verification passed") instead of making code changes.

## Why it wasn't "enough coverage"

The bot didn't decide there was "enough coverage." It never had the reviewer's ask in the implement prompt. So it didn't consider coverage at all; it only had the original issue text.

## Design gap

AGENTS.md says: *"assess with issue + review/comment content → implement ('address review feedback')"*. The **assess** step does get review/comment content, but that content is **not** passed through to **implement**. So "address review feedback" is not possible with the current wiring.

## Fix (implemented in this worktree)

1. **Assess** — When the trigger is PR review or a comment on a PR, derive the review/comment text from the payload and add it to the assessment JSON output as `review_feedback` (so the workflow can pass it on).
2. **Action** — Expose `review_feedback` from the assess step and pass it to the implement step as `REVIEW_FEEDBACK`.
3. **Implement** — Read `REVIEW_FEEDBACK`; when set, add a clear section to the Claude Code CLI prompt: **"Review feedback to address (you must implement this):"** so the CLI actually implements the reviewer's request (e.g. add unit and acceptance tests).

After this change, a comment like "Could we add some unit and acceptance test coverage for this change?" will be passed into the implement prompt, and the implement step will be instructed to address it, so the bot should add tests instead of replying with "Verification passed."
