#!/usr/bin/env -S npx tsx
/**
 * Run assess against a real issue by fetching it from GitHub (no event file).
 * Optionally APPLY=1: post comments to the issue or run implement + push (same as the workflow would).
 *
 * Local runs force the same default context as CI so assess and implement see the same repo guidance.
 * Env: ISSUE_NUMBER (required), GITHUB_REPOSITORY, GITHUB_TOKEN, AUTO_IMPLEMENT_ANTHROPIC_API_KEY.
 * Optional: EVENT_TYPE=issues|issue_comment|pull_request_review; COMMENT_BODY (for issue_comment); REVIEW_BODY (for pull_request_review).
 * Optional: APPLY=1 — post comment on issue (request_info/redirect_to_pr) or run implement then push-and-open-pr (implement).
 * Optional: CONTEXT_FILES — overrides default; default matches action.yml context_files (AGENTS.md,REFERENCE.md).
 *
 * Output: same JSON as index.ts (action, issue_number, comment_body?, verification_notes?, pr_url?).
 */
import './load-dotenv.js';

/** Must match action.yml inputs.context_files default so local and CI use the same repo context. */
const DEFAULT_CONTEXT_FILES = 'AGENTS.md,REFERENCE.md';
import { resolve, dirname } from 'path';
import { fileURLToPath } from 'url';
import { existsSync } from 'fs';
import { execSync } from 'child_process';
import { assess } from './index.js';
import { pushAndOpenPr } from './push-and-open-pr.js';

const __dirname = dirname(fileURLToPath(import.meta.url));

const ISSUE_NUMBER = process.env.ISSUE_NUMBER;
const REPO = process.env.GITHUB_REPOSITORY || '';
const TOKEN = process.env.GITHUB_TOKEN || '';
const EVENT_TYPE = (process.env.EVENT_TYPE || 'issues') as 'issues' | 'issue_comment' | 'pull_request_review';
const COMMENT_BODY = process.env.COMMENT_BODY || '';
const REVIEW_BODY = process.env.REVIEW_BODY || '';
const APPLY = process.env.APPLY === '1' || process.env.APPLY === 'true';
const LABEL_PREFIX = process.env.LABEL_PREFIX || 'automation';

async function fetchIssue(owner: string, repo: string, issueNumber: number): Promise<{ title: string; body: string; number: number; labels: { name: string }[] }> {
  const url = `https://api.github.com/repos/${owner}/${repo}/issues/${issueNumber}`;
  const res = await fetch(url, {
    headers: { Authorization: `Bearer ${TOKEN}`, Accept: 'application/vnd.github+json' },
  });
  if (!res.ok) throw new Error(`Failed to fetch issue: ${res.status} ${await res.text()}`);
  const data = (await res.json()) as { title?: string; body?: string; number?: number; labels?: { name: string }[] };
  return {
    title: data.title ?? '',
    body: data.body ?? '',
    number: data.number ?? issueNumber,
    labels: data.labels ?? [],
  };
}

async function fetchIssueComments(owner: string, repo: string, issueNumber: number): Promise<{ body?: string; user?: { login?: string }; created_at?: string }[]> {
  const url = `https://api.github.com/repos/${owner}/${repo}/issues/${issueNumber}/comments`;
  const res = await fetch(url, {
    headers: { Authorization: `Bearer ${TOKEN}`, Accept: 'application/vnd.github+json' },
  });
  if (!res.ok) return [];
  const data = (await res.json()) as { body?: string; user?: { login?: string }; created_at?: string }[];
  return Array.isArray(data) ? data : [];
}

async function postIssueComment(owner: string, repo: string, issueNumber: number, body: string): Promise<void> {
  const url = `https://api.github.com/repos/${owner}/${repo}/issues/${issueNumber}/comments`;
  const res = await fetch(url, {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${TOKEN}`,
      Accept: 'application/vnd.github+json',
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ body }),
  });
  if (!res.ok) throw new Error(`Failed to post comment: ${res.status} ${await res.text()}`);
}

async function addLabel(owner: string, repo: string, issueNumber: number, label: string): Promise<void> {
  const url = `https://api.github.com/repos/${owner}/${repo}/issues/${issueNumber}/labels`;
  const res = await fetch(url, {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${TOKEN}`,
      Accept: 'application/vnd.github+json',
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ labels: [label] }),
  });
  if (!res.ok) {
    // 422 = label may not exist; ignore
    if (res.status !== 422) throw new Error(`Failed to add label: ${res.status} ${await res.text()}`);
  }
}

async function main(): Promise<void> {
  if (!process.env.CONTEXT_FILES?.trim()) {
    process.env.CONTEXT_FILES = DEFAULT_CONTEXT_FILES;
  }

  const missing: string[] = [];
  if (!ISSUE_NUMBER) missing.push('ISSUE_NUMBER');
  if (!REPO) missing.push('GITHUB_REPOSITORY');
  if (!TOKEN) missing.push('GITHUB_TOKEN');
  if (missing.length) {
    throw new Error(
      `Missing or empty: ${missing.join(', ')}. Set in .env (action root or assess/) or in the environment.`
    );
  }
  const issueNumber = parseInt(ISSUE_NUMBER, 10);
  if (Number.isNaN(issueNumber)) throw new Error('Invalid ISSUE_NUMBER');

  const [owner, repoName] = REPO.split('/');
  if (!owner || !repoName) throw new Error('Invalid GITHUB_REPOSITORY (use owner/repo)');

  if (EVENT_TYPE === 'pull_request_review' && !REVIEW_BODY.trim()) {
    throw new Error('Set REVIEW_BODY when EVENT_TYPE=pull_request_review (paste the review you left on the PR)');
  }

  const issue = await fetchIssue(owner, repoName, issueNumber);
  const comments = await fetchIssueComments(owner, repoName, issueNumber);

  let eventName: string;
  let payload: unknown;

  if (EVENT_TYPE === 'pull_request_review') {
    eventName = 'pull_request_review';
    payload = {
      pull_request: {
        body: `Closes #${issueNumber}`,
        head: { ref: `auto-implement-issue-${issueNumber}` },
      },
      review: { body: REVIEW_BODY },
      issue: { number: issue.number, title: issue.title, body: issue.body, labels: issue.labels },
    };
  } else if (EVENT_TYPE === 'issue_comment' || COMMENT_BODY.trim()) {
    eventName = 'issue_comment';
    payload = {
      action: 'created',
      issue: {
        number: issue.number,
        title: issue.title,
        body: issue.body,
        labels: issue.labels.length ? issue.labels : [{ name: `${LABEL_PREFIX}/auto-implement` }],
      },
      comment: { body: COMMENT_BODY.trim() || '(new comment)' },
      repository: { full_name: REPO },
    };
  } else {
    eventName = 'issues';
    payload = {
      action: 'labeled',
      issue: {
        number: issue.number,
        title: issue.title,
        body: issue.body,
        labels: issue.labels.length ? issue.labels : [{ name: `${LABEL_PREFIX}/auto-implement` }],
      },
      repository: { full_name: REPO },
    };
  }

  const result = await assess(eventName, payload, {
    repo: REPO,
    token: TOKEN,
    referenceIssue: process.env.ASSESSMENT_REFERENCE_ISSUE || '192',
  });

  console.log(JSON.stringify(result));

  if (!APPLY) return;

  if (result.action === 'request_info' && result.comment_body) {
    await postIssueComment(owner, repoName, issueNumber, result.comment_body);
    await addLabel(owner, repoName, issueNumber, `${LABEL_PREFIX}/needs-info`);
    console.error('Posted request for more info on issue #' + issueNumber);
  } else if (result.action === 'redirect_to_pr' && result.pr_url) {
    const body = `A PR is open for this issue. Please review and comment on the PR: ${result.pr_url}`;
    await postIssueComment(owner, repoName, issueNumber, body);
    console.error('Posted redirect to PR on issue #' + issueNumber);
  } else if (result.action === 'implement') {
    const assessDir = process.cwd();
    const repoRoot = resolve(assessDir, '../../../..');
    const branch = `auto-implement-issue-${issueNumber}`;
    const worktreePath = resolve(repoRoot, '.worktrees', branch);

    // Always start from a clean branch: remove existing worktree and local branch, then create fresh from origin/main
    if (existsSync(worktreePath)) {
      try {
        execSync(`git worktree remove "${worktreePath}" --force`, { cwd: repoRoot, stdio: 'inherit' });
      } catch (e) {
        console.error('Failed to remove existing worktree:', e);
        throw e;
      }
    }
    try {
      execSync(`git branch -D ${branch}`, { cwd: repoRoot, stdio: 'pipe' });
    } catch {
      // local branch may not exist
    }
    try {
      execSync('git fetch origin main', { cwd: repoRoot, stdio: 'pipe' });
    } catch {
      // ignore
    }
    execSync(`git worktree add "${worktreePath}" -b "${branch}" origin/main`, {
      cwd: repoRoot,
      stdio: 'inherit',
    });

    const env = {
      ...process.env,
      ISSUE_NUMBER: String(issueNumber),
      VERIFICATION_NOTES: result.verification_notes || '',
      GITHUB_REPOSITORY: REPO,
      GITHUB_TOKEN: TOKEN,
      GITHUB_WORKSPACE: worktreePath,
    };
    execSync('npx tsx src/implement.ts', { cwd: assessDir, env, stdio: 'inherit' });
    pushAndOpenPr(worktreePath, issueNumber, TOKEN);
    console.error('Ran implement in worktree and pushed; PR created or updated.');
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
