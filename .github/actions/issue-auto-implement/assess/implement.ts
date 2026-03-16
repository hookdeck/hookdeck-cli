#!/usr/bin/env -S npx tsx
/**
 * Implement script: fetch issue, call Claude for file edits, apply and write commit message.
 * Env: ISSUE_NUMBER, GITHUB_REPOSITORY, GITHUB_TOKEN, ANTHROPIC_API_KEY, VERIFICATION_NOTES,
 *      GITHUB_WORKSPACE, CONTEXT_FILES. Writes commit message to IMPLEMENT_COMMIT_MSG_FILE.
 */

import { readFileSync, writeFileSync, mkdirSync, existsSync } from 'fs';
import { resolve, dirname } from 'path';
import Anthropic from '@anthropic-ai/sdk';

const REPO_ROOT = process.env.GITHUB_WORKSPACE || resolve(process.cwd(), '../../..');
const COMMIT_MSG_FILE = process.env.IMPLEMENT_COMMIT_MSG_FILE || resolve(REPO_ROOT, '.github/actions/issue-auto-implement/.commit_msg');

type Edit = { path: string; contents: string };
type ImplementOutput = { edits: Edit[]; commit_message: string };

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

function buildPrompt(issueTitle: string, issueBody: string, verificationNotes: string, contextBlock: string): string {
  const parts = [
    'You are implementing a GitHub issue. Produce a single JSON object with no markdown or extra text.',
    'Keys: "edits" (array of { "path": "relative/path/from/repo/root", "contents": "full file content" }), "commit_message" (short conventional commit message).',
    'Only include files you change or create. Paths must be relative to the repo root. Output full file contents for each edited file.',
    '',
    'Issue title:',
    issueTitle,
    '',
    'Issue body:',
    issueBody,
    '',
  ];
  if (verificationNotes) {
    parts.push('Verification notes (e.g. run tests):', verificationNotes, '');
  }
  if (contextBlock) {
    parts.push('', contextBlock);
  }
  parts.push('', 'Output only the JSON object:');
  return parts.join('\n');
}

function safePath(relativePath: string): string {
  const normalized = resolve(REPO_ROOT, relativePath);
  if (!normalized.startsWith(REPO_ROOT)) {
    throw new Error(`Path escapes repo root: ${relativePath}`);
  }
  return normalized;
}

function applyEdits(edits: Edit[]): void {
  for (const { path: rel, contents } of edits) {
    const full = safePath(rel);
    const dir = dirname(full);
    if (!existsSync(dir)) mkdirSync(dir, { recursive: true });
    writeFileSync(full, contents, 'utf-8');
  }
}

async function main(): Promise<void> {
  const issueNumber = process.env.ISSUE_NUMBER;
  const repo = process.env.GITHUB_REPOSITORY;
  const token = process.env.GITHUB_TOKEN;
  const apiKey = process.env.ANTHROPIC_API_KEY;
  const verificationNotes = process.env.VERIFICATION_NOTES || '';

  if (!issueNumber || !repo || !token || !apiKey) {
    throw new Error('Missing required env: ISSUE_NUMBER, GITHUB_REPOSITORY, GITHUB_TOKEN, ANTHROPIC_API_KEY');
  }

  const [owner, repoName] = repo.split('/');
  if (!owner || !repoName) throw new Error('Invalid GITHUB_REPOSITORY');

  const { title, body } = await fetchIssue(owner, repoName, parseInt(issueNumber, 10), token);
  const contextBlock = loadContextFiles();
  const prompt = buildPrompt(title, body, verificationNotes, contextBlock);

  const client = new Anthropic({ apiKey });
  const response = await client.messages.create({
    model: 'claude-sonnet-4-20250514',
    max_tokens: 16384,
    messages: [{ role: 'user', content: prompt }],
  });
  const text = response.content?.[0]?.type === 'text' ? response.content[0].text : '';
  const jsonMatch = text.match(/\{[\s\S]*\}/);
  if (!jsonMatch) throw new Error('Claude did not return valid JSON: ' + text.slice(0, 300));

  const parsed = JSON.parse(jsonMatch[0]) as ImplementOutput;
  if (!Array.isArray(parsed.edits)) throw new Error('Missing or invalid "edits" array');
  const commitMessage = typeof parsed.commit_message === 'string' && parsed.commit_message.trim()
    ? parsed.commit_message.trim()
    : `fix: implement issue #${issueNumber}`;

  applyEdits(parsed.edits);
  writeFileSync(COMMIT_MSG_FILE, commitMessage, 'utf-8');
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
