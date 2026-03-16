# Remote test plan (throwaway)

Use this to verify the issue-auto-implement action works on GitHub after merging PR #238. Delete this file once you're done.

## Prerequisites (after merge)

1. **Merge** [PR #238](https://github.com/hookdeck/hookdeck-cli/pull/238) (`feature/issue-auto-implement` → `main`).
2. **Delete** the implement branch [auto-implement-issue-232](https://github.com/hookdeck/hookdeck-cli/tree/auto-implement-issue-232) from remote (it was from a local implement run; no longer needed):
   - GitHub: repo → Branches → find `auto-implement-issue-232` → delete.
   - Or: `git push origin --delete auto-implement-issue-232`
3. **Secrets/variable** already set: `AUTO_IMPLEMENT_ANTHROPIC_API_KEY` (secret), `AUTO_IMPLEMENT_ALLOWED_TRIGGER_TEAM` (variable).
4. **Labels** created: run **Issue auto-implement setup** (Actions → Issue auto-implement setup → Run workflow) once to create `automation/auto-implement`, `automation/needs-info`, `automation/pr-created`. Or create `automation/auto-implement` manually in the repo.

---

## Test 1: Label triggers workflow

1. Open or create a **test issue** (e.g. "Test: auto-implement workflow" with a short, clear description).
2. Add the label **`automation/auto-implement`** to the issue.
3. **Expect:** Actions tab shows a run for "Issue auto-implement" (workflow `issue-auto-implement.yml`).
4. **Check:** Run completes; either "request_info" (comment posted, needs-info label) or "implement" (implement step runs, then verify, then PR created). If implement runs, confirm the "Install Claude Code CLI" step and the implement step succeed.

**Pass:** Workflow runs when label is added; no "workflow not triggered" or permission errors.

---

## Test 2: Implement path (full flow on GitHub)

Use an issue with **enough context** so assess returns `implement` (e.g. clear title + body, or reference a small, well-defined task).

1. Add `automation/auto-implement` to that issue.
2. **Expect:** Assess → implement → verify loop → create PR (or comment on issue if request_info).
3. **Check:** Actions run shows: Checkout → Install assess deps → Run assess → Checkout branch for implement → **Install Claude Code CLI** → Implement and verify (loop) → Create PR step (if verify passed).
4. **Check:** A PR is created from branch `auto-implement-issue-<N>` with "Closes #<N>".
5. **Check:** PR is open and points at the correct issue.

**Pass:** Implement runs in CI, CLI installs, PR is created.

---

## Test 3: PR review iteration (optional)

1. On the PR from Test 2, submit a **review** (e.g. "Please add a unit test for the new function").
2. **Expect:** Workflow runs again (pull_request_review trigger).
3. **Check:** Assess runs with review context → implement runs → push to same branch → verify → **no new PR**; instead a comment on the existing PR (e.g. "Addressed review feedback...").

**Pass:** Review triggers run; same PR updated; comment posted on PR.

---

## Test 4: Team check (optional)

1. Use an account that is **not** in `AUTO_IMPLEMENT_ALLOWED_TRIGGER_TEAM`.
2. Add the label to an issue (or trigger in another way).
3. **Expect:** Workflow run fails early with an error that the actor is not in the team.

**Pass:** Non-members cannot run the action.

---

## Quick checklist

- [ ] PR #238 merged to main
- [ ] Branch `auto-implement-issue-232` deleted from remote
- [ ] Label `automation/auto-implement` exists
- [ ] Test 1: Adding label triggers workflow
- [ ] Test 2: Implement path runs in CI and creates PR (if issue has enough context)
- [ ] Test 3 (optional): PR review triggers iteration
- [ ] Test 4 (optional): Team check blocks non-members
- [ ] Delete this file when done
