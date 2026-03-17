#!/usr/bin/env -S npx tsx
/**
 * Implement script: fetch issue, then run Claude Code CLI in the repo to implement it.
 *
 * Env: ISSUE_NUMBER, GITHUB_REPOSITORY, GITHUB_TOKEN; VERIFICATION_NOTES, GITHUB_WORKSPACE (optional), CONTEXT_FILES.
 *      Claude Code CLI must be on PATH. AUTO_IMPLEMENT_ANTHROPIC_API_KEY is passed to the CLI as ANTHROPIC_API_KEY.
 *      Writes commit message and PR meta to ACTION_DIR for push-and-open-pr.
 */

import { readFileSync, writeFileSync, mkdirSync, existsSync } from 'fs';
import { resolve, dirname } from 'path';
import { spawnSync } from 'child_process';
import { config } from 'dotenv';

// Load .env from action root then cwd (cwd is assess/ when run from there). No-op if files missing.
config({ path: resolve(process.cwd(), '../.env') });
config({ path: resolve(process.cwd(), '.env') });

// Default repo root: in CI GITHUB_WORKSPACE is set; when run from assess/ locally, cwd is assess/ so repo root is 4 levels up
const REPO_ROOT = process.env.GITHUB_WORKSPACE || resolve(process.cwd(), '../../../..');
const ACTION_DIR = '.github/actions/issue-auto-implement';
const COMMIT_MSG_FILE = process.env.IMPLEMENT_COMMIT_MSG_FILE || resolve(REPO_ROOT, ACTION_DIR + '/.commit_msg');
const PR_TITLE_FILE = resolve(REPO_ROOT, ACTION_DIR + '/.pr_title');
const PR_BODY_FILE = resolve(REPO_ROOT, ACTION_DIR + '/.pr_body');
/** When implement makes no code changes, Claude writes the PR comment body here (no-change rationale or request for clarification). */
const COMMENT_BODY_FILE = resolve(REPO_ROOT, ACTION_DIR + '/.comment_body');

async function fetchIssue(owner: string, repo: string, issueNumber: number, token: string): Promise<{ title: string; body: string }> {
  const url = `https://api.github.com/repos/${owner}/${repo}/issues/${issueNumber}`;
  const res = await fetch(url, {
    headers: { Authorization: `Bearer ${token}`, Accept: 'application/vnd.github+json' },
  });
  if (!res.ok) throw new Error(`Failed to fetch issue: ${res.status}`);
  const data = (await res.json()) as { title?: string; body?: string };
  return { title: data.title ?? '', body: data.body ?? '' };
}

function loadContextFiles(): string {
  const contextFiles = process.env.CONTEXT_FILES || '';
  if (!contextFiles.trim()) return '';
  const paths = contextFiles.split(',').map((s) => s.trim()).filter(Boolean);
  const chunks: string[] = [];
  for (const rel of paths) {
    try {
      const full = resolve(REPO_ROOT, rel);
      const content = readFileSync(full, 'utf-8');
      chunks.push(`--- ${rel} ---\n${content}`);
    } catch {
      // Skip missing files
    }
  }
  return chunks.length ? ['Repository context:', '', ...chunks].join('\n') : '';
}

/**
 * Prompt for Claude Code CLI: implement in-repo with Read/Edit/Bash; write meta files when done.
 */
function buildClaudeCliPrompt(
  issueTitle: string,
  issueBody: string,
  verificationNotes: string,
  contextBlock: string,
  previousVerifyOutput: string,
  issueNumber: number
): string {
  const metaDir = ACTION_DIR;
  const parts = [
    'Implement this GitHub issue in the current repository. You have full access to read and edit files and run commands.',
    '',
    'Rules:',
    '- Only change what is necessary to implement the issue. Preserve existing exported symbols and call sites unless the issue explicitly asks to remove or replace them.',
    '- Consider the broader codebase—other code may depend on the files you edit; make minimal, targeted edits and keep the public API intact.',
    '- When you MAKE code changes, you MUST write three files (create the directory if needed):',
    `  1. ${metaDir}/.commit_msg — one line, conventional commit message (e.g. "fix: correct version comparison for beta").`,
    `  2. ${metaDir}/.pr_title — one-line PR title.`,
    `  3. ${metaDir}/.pr_body — markdown body: brief problem summary, then "How it was solved" or "Solution". Do NOT include "Closes #N" (it will be appended).`,
    `  These files are workflow-only inputs (consumed by the action to create the commit and PR). Do NOT add or commit them to the repository.`,
    '',
    `- When you decide NOT to make any code changes, you MUST NOT write .commit_msg, .pr_title, or .pr_body. You MUST write ${metaDir}/.comment_body instead — one or two sentences that will be posted on the PR. Required for: (a) no-change scenarios (e.g. the feedback is a question or the current approach is preferred; thank the reviewer and briefly explain why no change), or (b) when more information is needed (e.g. "Could you clarify whether you want X or Y?"). Without .comment_body the workflow posts a generic fallback; always write it when you make no code changes so the reviewer gets a useful reply.`,
    '',
    'Issue title:',
    issueTitle,
    '',
    'Issue body:',
    issueBody,
    '',
  ];
  if (verificationNotes) {
    parts.push('Verification (run these to confirm):', verificationNotes, '');
  }
  if (previousVerifyOutput.trim()) {
    parts.push(
      '',
      'The previous implementation was applied but verification failed. Fix based on:',
      '',
      '--- Verification output ---',
      previousVerifyOutput.trim(),
      '--- End ---',
      ''
    );
  }
  if (contextBlock) {
    parts.push('', contextBlock);
  }
  parts.push('', 'After implementing, write exactly one of: (A) .commit_msg, .pr_title, and .pr_body if you made code changes; or (B) .comment_body only if you made no code changes. When you make no code changes, writing .comment_body is required so the PR gets a specific reply instead of a generic one.');
  return parts.join('\n');
}

/**
 * Run Claude Code CLI in REPO_ROOT with prompt on stdin. Throws if CLI is not found or exits non-zero.
 * Uses AUTO_IMPLEMENT_ANTHROPIC_API_KEY (passed to the CLI as ANTHROPIC_API_KEY).
 */
function runClaudeCli(prompt: string): void {
  const apiKey = process.env.AUTO_IMPLEMENT_ANTHROPIC_API_KEY;
  if (!apiKey) {
    throw new Error('AUTO_IMPLEMENT_ANTHROPIC_API_KEY must be set for the Claude Code CLI.');
  }
  const env = { ...process.env, ANTHROPIC_API_KEY: apiKey };

  const result = spawnSync(
    'claude',
    ['-p', '--allowedTools', 'Read,Edit,Bash'],
    {
      cwd: REPO_ROOT,
      input: prompt,
      stdio: ['pipe', 'inherit', 'inherit'],
      encoding: 'utf-8',
      timeout: 25 * 60 * 1000, // 25 minutes
      env,
    }
  );
  if (result.error && (result.error as NodeJS.ErrnoException).code === 'ENOENT') {
    throw new Error('Claude Code CLI not found (claude not on PATH). Install it and set AUTO_IMPLEMENT_ANTHROPIC_API_KEY.');
  }
  if (result.error) throw result.error;
  if (result.status !== 0) {
    process.exit(result.status ?? 1);
  }
}

/** Ensure commit message and PR meta files exist when Claude made code changes; skip defaults if Claude wrote .comment_body. */
function ensureMetaFiles(issueNumber: number): void {
  const metaDir = dirname(COMMIT_MSG_FILE);
  if (!existsSync(metaDir)) mkdirSync(metaDir, { recursive: true });
  if (existsSync(COMMENT_BODY_FILE)) {
    return;
  }
  if (!existsSync(COMMIT_MSG_FILE)) {
    writeFileSync(COMMIT_MSG_FILE, `fix: implement issue #${issueNumber}`, 'utf-8');
  }
  if (!existsSync(PR_TITLE_FILE)) {
    writeFileSync(PR_TITLE_FILE, `Implement issue #${issueNumber}`, 'utf-8');
  }
  if (!existsSync(PR_BODY_FILE)) {
    writeFileSync(PR_BODY_FILE, `Closes #${issueNumber}`, 'utf-8');
  }
}

async function main(): Promise<void> {
  const issueNumber = process.env.ISSUE_NUMBER;
  const repo = process.env.GITHUB_REPOSITORY;
  const token = process.env.GITHUB_TOKEN;
  const verificationNotes = process.env.VERIFICATION_NOTES || '';
  const previousVerifyOutput = process.env.PREVIOUS_VERIFY_OUTPUT || '';

  if (!issueNumber || !repo || !token) {
    throw new Error('Missing required env: ISSUE_NUMBER, GITHUB_REPOSITORY, GITHUB_TOKEN');
  }

  const [owner, repoName] = repo.split('/');
  if (!owner || !repoName) throw new Error('Invalid GITHUB_REPOSITORY');

  const issueNum = parseInt(issueNumber, 10);
  const { title, body } = await fetchIssue(owner, repoName, issueNum, token);
  const contextBlock = loadContextFiles();

  const prompt = buildClaudeCliPrompt(title, body, verificationNotes, contextBlock, previousVerifyOutput, issueNum);
  runClaudeCli(prompt);
  ensureMetaFiles(issueNum);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
