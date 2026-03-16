# Issue auto-implement action

Reusable composite action for label-triggered issue automation: assess (request more info or implement), implement-verify loop, then create PR or iterate on an existing PR after review.

## How to use (quick start)

1. **Workflow** — Ensure `.github/workflows/issue-auto-implement.yml` exists and calls this action (see the workflow in this repo for the exact `on:` and `uses:`).
2. **Secrets and variable** — In the repo: Settings → Secrets and variables → Actions. Add secret **`AUTO_IMPLEMENT_ANTHROPIC_API_KEY`** (Anthropic API key). Add variable **`AUTO_IMPLEMENT_ALLOWED_TRIGGER_TEAM`** (e.g. `your-org/your-team`); only members of this team can trigger the action.
3. **Trigger label** — Create a label **`automation/auto-implement`** (or `{your-prefix}/auto-implement` if you set `label_prefix`) in the repo so you can add it to issues. The action will create the other labels (`needs-info`, `pr-created`) when it runs.
4. **Trigger** — On an issue, add the label `automation/auto-implement`. The workflow runs: it assesses the issue (request more info vs implement), and if implement, runs the Claude Code CLI and opens a PR. You can also comment on the issue (to add context and re-trigger) or review the PR (to iterate).

## Usage (reference)

Used by `.github/workflows/issue-auto-implement.yml`. Requires `anthropic_api_key` (e.g. from repo secret `AUTO_IMPLEMENT_ANTHROPIC_API_KEY`), `github_allowed_trigger_team` (e.g. from repo variable `AUTO_IMPLEMENT_ALLOWED_TRIGGER_TEAM`), and `github_token` from the workflow.

## Inputs

| Input | Required | Default | Description |
|-------|----------|---------|-------------|
| `anthropic_api_key` | Yes | - | Claude API key. Set via repo secret `AUTO_IMPLEMENT_ANTHROPIC_API_KEY` so multiple actions can use different keys. |
| `github_token` | Yes | - | Token with contents, issues, pull-requests, read:org |
| `context_files` | No | AGENTS.md,REFERENCE.md | Comma-separated paths for assessment context |
| `assessment_reference_issue` | No | 192 | Reference issue number for "enough information" |
| `label_prefix` | No | automation | Prefix for labels (e.g. automation/auto-implement) |
| `verify_commands` | No | go test ./... | Commands run for verification |
| `max_implement_retries` | No | 3 | Max retries on verify failure (cap 5) |
| `github_allowed_trigger_team` | Yes | - | GitHub Team slug (e.g. org/team); only members can trigger. Set via repo variable `AUTO_IMPLEMENT_ALLOWED_TRIGGER_TEAM`. |
| `post_pr_comment` | No | false | When true, post a comment on the issue linking to the new PR when one is created. |

Secrets and variables use an action-specific prefix (e.g. `AUTO_IMPLEMENT_`) so each action can have its own keys/variables and it's clear which workflow uses which. This also avoids clashing with platform-reserved names (e.g. `GITHUB_*`).

## CI/CD: what you need to run this workflow

To use this action in GitHub Actions:

1. **Workflow** — Call the action from a workflow (e.g. `.github/workflows/issue-auto-implement.yml`) on `issues.labeled`, `issue_comment`, `pull_request_review`, and/or `pull_request_review_comment`. The job needs `contents: write`, `issues: write`, `pull-requests: write`.
2. **Secrets** — Add **`AUTO_IMPLEMENT_ANTHROPIC_API_KEY`** (repo secret). Used for the assess step and passed to the Claude Code CLI in the implement step.
3. **Variables** — Add **`AUTO_IMPLEMENT_ALLOWED_TRIGGER_TEAM`** (repo variable, required): GitHub Team slug (e.g. `org/team-name`) whose members may trigger the run.
4. **Token** — Pass `github_token` (e.g. `secrets.GITHUB_TOKEN`) to the action. The team check needs `read:org`; if `GITHUB_TOKEN` lacks it, use a PAT with `read:org` and pass it as the token.
5. **Implement in CI** — The action installs the Claude Code CLI (`@anthropic-ai/claude-code`) when the assess outcome is `implement`, so the workflow does not need to install it. Implement runs in the repo with Read/Edit/Bash; the CLI uses `AUTO_IMPLEMENT_ANTHROPIC_API_KEY`.

No other setup is required. Optionally set `verify_commands` (default `go test ./...`) and `context_files` (default `AGENTS.md,REFERENCE.md`) to match your repo.

## Secrets and variables (repo setup)

- **`AUTO_IMPLEMENT_ANTHROPIC_API_KEY`** (repo secret) — Claude API key for the assess and implement steps. Add under Settings → Secrets and variables → Actions.
- **`AUTO_IMPLEMENT_ALLOWED_TRIGGER_TEAM`** (repo variable, required) — GitHub Team slug (e.g. `org/team-name`) whose members may trigger the workflow. Add under Settings → Secrets and variables → Actions. The first step checks `github.actor` against this team; if unset or not a member, the run fails.
- **Token for team check** — The workflow passes `github_token` (usually `secrets.GITHUB_TOKEN`) to the action. The team check needs `read:org`. If your default `GITHUB_TOKEN` does not have `read:org`, use a PAT with that scope and pass it as the token (e.g. a repo secret) instead.

## Triggers

- **issues.labeled** — prefixed trigger label (e.g. `automation/auto-implement`)
- **issue_comment.created** — on an issue with that label (redirect to PR if PR exists)
- **pull_request_review.submitted** / **pull_request_review_comment.created** — PR from automation branch or with "Closes #N"

## Labels

The action ensures these labels exist (creates them if missing): `{prefix}/auto-implement`, `{prefix}/needs-info`, `{prefix}/pr-created`.

## Testing

From the repo root, run the assess script tests:

```bash
cd .github/actions/issue-auto-implement/assess && npm ci && npm test
```

CI runs these in `.github/workflows/issue-auto-implement-test.yml` when you push or open a PR that touches this action.

**Integration tests (Claude API):** Tests in `assess/test/integration/` call the real Anthropic API. They do not run with `npm test`. From the assess directory, run `npm run test:integration` (requires `AUTO_IMPLEMENT_ANTHROPIC_API_KEY` in `.env` or env). Unit tests live in `assess/test/unit/`; shared fixtures in `assess/test/fixtures/`. You can add integration tests to CI later with the secret configured.

## Local runs (Claude)

Scripts load a **local `.env`** file so you don't have to pass secrets on the command line. They look for `.env` in (1) the action root (`.github/actions/issue-auto-implement/.env`) and (2) the current working directory (e.g. `assess/.env`). Later overrides earlier; shell env still wins.

### Env vars (local)

| Variable | Required for | How to get it |
|----------|--------------|----------------|
| `AUTO_IMPLEMENT_ANTHROPIC_API_KEY` | Assess, Implement | [Anthropic console](https://console.anthropic.com/) → API keys. Assess uses it directly; implement passes it to Claude Code CLI (`claude` on PATH). |
| `GITHUB_TOKEN` | Implement; optional for Assess | `gh auth token`, or a PAT with `repo`, `read:org`. |
| `GITHUB_REPOSITORY` | Implement | `owner/repo` (e.g. `hookdeck/hookdeck-cli`). |
| `ISSUE_NUMBER` | Implement | The issue number to implement. |
| `GITHUB_EVENT_PATH` | Assess (when not using fixture) | Path to event JSON; for fixture: `./fixtures/issue-labeled.json`. |
| `GITHUB_EVENT_NAME` | Assess | e.g. `issues` or `issue_comment`. |
| `GITHUB_WORKSPACE` | Optional | Repo root; default inferred from cwd when run from `assess/`. |
| `CONTEXT_FILES` | Optional | Comma-separated paths (relative to repo root) for Claude context. |
| `VERIFICATION_NOTES` | Optional (Implement) | Notes from assess step. |
| `PREVIOUS_VERIFY_OUTPUT` | Optional (Implement retries) | Previous verify failure output. |

### One-time setup: `.env`

1. From the action root: `cp .env.example .env`
2. Edit `.env` and set at least `AUTO_IMPLEMENT_ANTHROPIC_API_KEY` and (for implement) `GITHUB_TOKEN`. Optionally run `./scripts/setup-local-env.sh --with-gh` to fill `GITHUB_TOKEN` from `gh auth token`; use `--template-only` to only create `.env` from `.env.example`.

### Assess (issue → implement vs request_info)

Uses a fixture as the GitHub event. Claude decides whether to `implement` or `request_info`; output is JSON to stdout.

```bash
cd .github/actions/issue-auto-implement/assess
npm run assess:fixture
```

With `.env` in place, no need to pass the key on the command line. Optional: set `GITHUB_TOKEN` and `GITHUB_REPOSITORY` to exercise redirect-to-PR and fetch-comments. Set `ASSESS_DEBUG=1` to log the prompt sent to Claude and the raw response to stderr. Other fixtures: `GITHUB_EVENT_PATH=./test/fixtures/issue-comment.json GITHUB_EVENT_NAME=issue_comment npx tsx src/index.ts`.

### Implement (issue → Claude Code CLI → files on disk)

Fetches the issue from the GitHub API, then runs **Claude Code CLI** in the repo (`claude` on PATH with Read/Edit/Bash). The CLI implements the issue in-repo and writes commit/PR meta files. Use a branch you can discard or reset. Requires Claude Code CLI installed and `AUTO_IMPLEMENT_ANTHROPIC_API_KEY` set (passed to the CLI).

```bash
cd .github/actions/issue-auto-implement/assess
npm run implement:issue
```

With `.env` set (e.g. `ISSUE_NUMBER`, `GITHUB_REPOSITORY`, `GITHUB_TOKEN`, `AUTO_IMPLEMENT_ANTHROPIC_API_KEY`), no need to pass them inline. Override any var on the command line if needed (e.g. `ISSUE_NUMBER=42 npm run implement:issue`). Then from the repo root inspect `git status` and the commit message at `.github/actions/issue-auto-implement/.commit_msg`. Optionally set `VERIFICATION_NOTES` and `CONTEXT_FILES`.

For implementation details and verification steps, see `AGENTS.md`.

### Local run against a real issue (no workflow events)

To test the full flow locally against a real GitHub issue and create a PR, use the **local assess** script (fetches the issue from the API and runs the same assess logic). With **APPLY=1** the script applies the outcome: posts the request-for-more-info or redirect comment on the issue, or runs implement and push (creates/updates the PR). When the outcome is implement, the script creates or reuses a **worktree** at `.worktrees/auto-implement-issue-<N>` so your current branch is left untouched; implement runs there, then commit/push/PR is done in TypeScript (`assess/src/push-and-open-pr.ts`). The workflow does not need to run; you trigger each step locally and optionally pass **COMMENT_BODY** (after you add a comment on the issue) or **REVIEW_BODY** (after you review the PR).
