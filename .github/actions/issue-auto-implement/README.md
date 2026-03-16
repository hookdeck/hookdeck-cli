# Issue auto-implement action

Reusable composite action for label-triggered issue automation: assess (request more info or implement), implement-verify loop, then create PR or iterate on an existing PR after review.

## Usage

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

Secrets and variables use an action-specific prefix (e.g. `AUTO_IMPLEMENT_`) so each action can have its own keys/variables and it's clear which workflow uses which. This also avoids clashing with platform-reserved names (e.g. `GITHUB_*`).

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

CI runs these in `.github/workflows/issue-auto-implement-test.yml` when you push or open a PR that touches this action. To run the assess script with a real Claude call, set `ANTHROPIC_API_KEY` and use a fixture:

```bash
cd .github/actions/issue-auto-implement/assess
GITHUB_EVENT_PATH=./fixtures/issue-labeled.json GITHUB_EVENT_NAME=issues npx tsx index.ts
```

For implementation details, verification steps, and the implementation backlog, see `AGENTS.md`.
