#!/usr/bin/env -S npx tsx
/**
 * Commit implement output, push branch, create PR if missing.
 * Caller must ensure repoRoot is the implementation branch (e.g. after checkout or worktree add).
 *
 * Env: GITHUB_TOKEN (for push and gh). IMPLEMENT_COMMIT_MSG_FILE overrides default path.
 * When run as script: ISSUE_NUMBER and GITHUB_WORKSPACE (or cwd = assess/ with repo root 4 levels up).
 */
import { resolve } from 'path';
import { unlinkSync, existsSync, readFileSync } from 'fs';
import { execSync } from 'child_process';

const REL_DIR = '.github/actions/issue-auto-implement';
const REL_COMMIT_MSG = `${REL_DIR}/.commit_msg`;
const REL_PR_TITLE = `${REL_DIR}/.pr_title`;
const REL_PR_BODY = `${REL_DIR}/.pr_body`;

export function pushAndOpenPr(repoRoot: string, issueNumber: number, token?: string): void {
  const commitMsgFile = resolve(repoRoot, REL_COMMIT_MSG);
  if (!existsSync(commitMsgFile)) {
    throw new Error(`Missing ${commitMsgFile}; run implement step first.`);
  }
  const branch = `auto-implement-issue-${issueNumber}`;
  const env = { ...process.env, GH_TOKEN: token || process.env.GITHUB_TOKEN };

  execSync('git add -A', { cwd: repoRoot, stdio: 'inherit' });
  for (const rel of [REL_COMMIT_MSG, REL_PR_TITLE, REL_PR_BODY]) {
    try {
      execSync(`git reset -- ${rel}`, { cwd: repoRoot, stdio: 'pipe' });
    } catch {
      // ignore if path not staged
    }
  }

  let hasStaged: boolean;
  try {
    execSync('git diff --staged --quiet', { cwd: repoRoot, stdio: 'pipe' });
    hasStaged = false;
  } catch {
    hasStaged = true;
  }

  if (hasStaged) {
    execSync(`git commit -F ${REL_COMMIT_MSG}`, { cwd: repoRoot, stdio: 'inherit' });
    unlinkSync(commitMsgFile);
    execSync(`git push -u origin ${branch} --force-with-lease`, { cwd: repoRoot, stdio: 'inherit', env });
  } else {
    console.error('No changes to commit.');
  }

  const prTitleFile = resolve(repoRoot, REL_PR_TITLE);
  const prBodyFile = resolve(repoRoot, REL_PR_BODY);

  let shouldCreatePr = true;
  try {
    const out = execSync('gh pr view --json state', { cwd: repoRoot, encoding: 'utf-8', env });
    const { state } = JSON.parse(out) as { state: string };
    if (state === 'OPEN') {
      shouldCreatePr = false;
      console.error('PR already exists; branch pushed.');
    }
    // If state is CLOSED or MERGED, create a new PR for this branch (GitHub allows that).
  } catch {
    // No PR for this branch; create one.
  }

  if (shouldCreatePr) {
    const title = existsSync(prTitleFile) ? readFileSync(prTitleFile, 'utf-8').trim() : `Implement issue #${issueNumber}`;
    const bodyPath = existsSync(prBodyFile) ? prBodyFile : null;
    const createArgs = bodyPath
      ? `--title ${JSON.stringify(title)} --body-file ${JSON.stringify(prBodyFile)}`
      : `--title ${JSON.stringify(title)} --body ${JSON.stringify(`Closes #${issueNumber}`)}`;
    execSync(`gh pr create ${createArgs}`, {
      cwd: repoRoot,
      stdio: 'inherit',
      env,
    });
    if (existsSync(prTitleFile)) unlinkSync(prTitleFile);
    if (existsSync(prBodyFile)) unlinkSync(prBodyFile);
    console.error('PR created.');
  } else {
    if (existsSync(prTitleFile)) unlinkSync(prTitleFile);
    if (existsSync(prBodyFile)) unlinkSync(prBodyFile);
  }
}

function main(): void {
  const repoRoot = process.env.GITHUB_WORKSPACE || resolve(process.cwd(), '../../../..');
  const issueNumber = process.env.ISSUE_NUMBER;
  if (!issueNumber) {
    console.error('Set ISSUE_NUMBER');
    process.exit(1);
  }
  pushAndOpenPr(repoRoot, parseInt(issueNumber, 10));
}

const isMain =
  typeof process.argv[1] === 'string' &&
  (process.argv[1].endsWith('push-and-open-pr.ts') || process.argv[1].endsWith('push-and-open-pr.js'));
if (isMain) {
  main();
}
