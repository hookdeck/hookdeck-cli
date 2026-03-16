#!/usr/bin/env -S npx tsx
/**
 * Assess script: read GitHub event, normalize, optionally check redirect, call Claude, output JSON.
 * Output: { action: 'implement' | 'request_info' | 'redirect_to_pr', comment_body?, verification_notes?, pr_url? }
 * Run: GITHUB_EVENT_PATH=... GITHUB_EVENT_NAME=... [AUTO_IMPLEMENT_ANTHROPIC_API_KEY=...] npx tsx src/index.ts
 */

import { readFileSync } from 'fs';
import { resolve } from 'path';
import { config } from 'dotenv';
import Anthropic from '@anthropic-ai/sdk';
import { normalizeEvent } from './normalize.js';

// Load .env from action root then cwd (cwd is assess/ when run from there). No-op if files missing.
config({ path: resolve(process.cwd(), '../.env') });
config({ path: resolve(process.cwd(), '.env') });

const EVENT_PATH = process.env.GITHUB_EVENT_PATH || '';
const EVENT_NAME = process.env.GITHUB_EVENT_NAME || '';
const GITHUB_TOKEN = process.env.GITHUB_TOKEN || '';
const ANTHROPIC_API_KEY = process.env.AUTO_IMPLEMENT_ANTHROPIC_API_KEY || '';
const REPO = process.env.GITHUB_REPOSITORY || '';
const CONTEXT_FILES = process.env.CONTEXT_FILES || '';
const REPO_ROOT = process.env.GITHUB_WORKSPACE || resolve(process.cwd(), '../../..');

export type AssessmentOutput = {
  action: 'implement' | 'request_info' | 'redirect_to_pr';
  issue_number?: number;
  comment_body?: string;
  verification_notes?: string;
  pr_url?: string;
};

async function checkExistingPr(owner: string, repo: string, issueNumber: number): Promise<{ pr_url: string } | null> {
  if (!GITHUB_TOKEN) return null;
  const branch = `auto-implement-issue-${issueNumber}`;
  const url = `https://api.github.com/repos/${owner}/${repo}/pulls?head=${owner}:${branch}&state=open`;
  const res = await fetch(url, {
    headers: { Authorization: `Bearer ${GITHUB_TOKEN}`, Accept: 'application/vnd.github+json' },
  });
  if (!res.ok) return null;
  const data = (await res.json()) as { html_url?: string }[];
  const pr = data?.[0];
  return pr?.html_url ? { pr_url: pr.html_url } : null;
}

type IssueComment = { body?: string; user?: { login?: string }; created_at?: string };

async function fetchIssueComments(
  owner: string,
  repo: string,
  issueNumber: number,
  token: string
): Promise<IssueComment[]> {
  const url = `https://api.github.com/repos/${owner}/${repo}/issues/${issueNumber}/comments`;
  const res = await fetch(url, {
    headers: { Authorization: `Bearer ${token}`, Accept: 'application/vnd.github+json' },
  });
  if (!res.ok) return [];
  const data = (await res.json()) as IssueComment[];
  return Array.isArray(data) ? data : [];
}

function loadPayload(): { eventName: string; payload: unknown } {
  if (!EVENT_PATH) {
    throw new Error('GITHUB_EVENT_PATH is not set');
  }
  const raw = readFileSync(EVENT_PATH, 'utf-8');
  const payload = JSON.parse(raw) as unknown;
  const eventName = EVENT_NAME || (payload as Record<string, string>)?.action ? inferEventName(payload) : '';
  if (!eventName) throw new Error('Could not determine event name (set GITHUB_EVENT_NAME or use a standard payload)');
  return { eventName, payload };
}

function inferEventName(payload: unknown): string {
  const p = payload as Record<string, unknown>;
  if (p.issue && p.comment) return 'issue_comment';
  if (p.pull_request && p.review) return 'pull_request_review';
  if (p.pull_request && (p as { comment?: unknown }).comment) return 'pull_request_review_comment';
  if (p.issue && p.label) return 'issues';
  return '';
}

function loadContextFiles(): string {
  if (!CONTEXT_FILES.trim()) return '';
  const paths = CONTEXT_FILES.split(',').map((s) => s.trim()).filter(Boolean);
  const chunks: string[] = [];
  for (const rel of paths) {
    try {
      const full = resolve(REPO_ROOT, rel);
      const content = readFileSync(full, 'utf-8');
      chunks.push(`--- ${rel} ---\n${content}`);
    } catch {
      // Skip missing files (e.g. REFERENCE.md may not exist in all repos)
    }
  }
  return chunks.length ? ['Repository context:', '', ...chunks].join('\n') : '';
}

function buildAssessmentPrompt(
  payload: unknown,
  eventName: string,
  referenceIssue: string,
  contextBlock: string,
  issueComments: IssueComment[] = []
): string {
  const p = payload as Record<string, unknown>;
  const issue = p.issue as { title?: string; body?: string; number?: number } | undefined;
  const parts: string[] = [
    'You are assessing a GitHub issue to decide if there is enough information to implement a fix or feature.',
    'Reply with a single JSON object only, no markdown or extra text, with these keys:',
    '- action: either "implement" or "request_info"',
    '- comment_body: (required if action is request_info) a short message to post on the issue asking for the missing information',
    '- verification_notes: (optional if action is implement) free-form notes for the implementer. Include running the test suite and ensuring the application builds; infer the repo\'s usual test and build commands from the repository context (e.g. go test ./... && go build ., npm test && npm run build, make test, etc.).',
    '',
    'Issue:',
    `Title: ${issue?.title ?? 'N/A'}`,
    `Body: ${issue?.body ?? 'N/A'}`,
    '',
    `Reference example of "enough information": GitHub issue #${referenceIssue} (use similar clarity and specificity).`,
  ];
  if (contextBlock) {
    parts.push('', contextBlock);
  }
  if (issueComments.length > 0) {
    parts.push(
      '',
      'All issue comments (from API):',
      issueComments
        .map((c) => `[${c.user?.login ?? 'unknown'} @ ${c.created_at ?? 'N/A'}]: ${c.body ?? ''}`)
        .join('\n\n')
    );
  }
  const comment = p.comment as { body?: string } | undefined;
  if (comment?.body && !issueComments.some((c) => c.body === comment.body)) {
    parts.push('', 'Latest event comment:', comment.body);
  }
  if (Array.isArray(p.comments) && p.comments.length && issueComments.length === 0) {
    parts.push('', 'Comments:', JSON.stringify(p.comments, null, 2));
  }
  if (eventName === 'pull_request_review' || eventName === 'pull_request_review_comment') {
    const review = (p.review as { body?: string }) ?? {};
    parts.push('', 'PR review (address this feedback):', review.body ?? 'N/A');
  }

  parts.push('', 'Output only the JSON object:');
  return parts.join('\n');
}

const DEBUG = process.env.ASSESS_DEBUG === '1' || process.env.ASSESS_DEBUG === 'true';

async function callClaude(prompt: string, client?: Anthropic): Promise<AssessmentOutput> {
  const api = client ?? new Anthropic({ apiKey: ANTHROPIC_API_KEY });
  if (!client && !ANTHROPIC_API_KEY) {
    throw new Error('AUTO_IMPLEMENT_ANTHROPIC_API_KEY is not set');
  }
  if (DEBUG) {
    process.stderr.write('--- ASSESS PROMPT (sent to Claude) ---\n');
    process.stderr.write(prompt);
    process.stderr.write('\n--- END PROMPT ---\n');
  }
  const response = await api.messages.create({
    model: 'claude-sonnet-4-20250514',
    max_tokens: 1024,
    messages: [{ role: 'user', content: prompt }],
  });
  const text = response.content?.[0]?.type === 'text' ? response.content[0].text : '';
  if (DEBUG) {
    process.stderr.write('--- CLAUDE RAW RESPONSE ---\n');
    process.stderr.write(text);
    process.stderr.write('\n--- END RESPONSE ---\n');
  }
  const jsonMatch = text.match(/\{[\s\S]*\}/);
  if (!jsonMatch) {
    throw new Error('Claude did not return valid JSON: ' + text.slice(0, 200));
  }
  const parsed = JSON.parse(jsonMatch[0]) as AssessmentOutput;
  if (parsed.action !== 'implement' && parsed.action !== 'request_info') {
    parsed.action = 'request_info';
  }
  return parsed;
}

async function main(): Promise<void> {
  const { eventName, payload } = loadPayload();
  const result = await assess(eventName, payload, {
    repo: REPO,
    token: GITHUB_TOKEN,
    referenceIssue: process.env.ASSESSMENT_REFERENCE_ISSUE || '192',
  });
  console.log(JSON.stringify(result));
}

/** Exported for tests: run assessment with given payload and optional mock client */
export async function assess(
  eventName: string,
  payload: unknown,
  opts: {
    repo?: string;
    token?: string;
    referenceIssue?: string;
    anthropicClient?: Anthropic;
    contextFilesContent?: string;
  }
): Promise<AssessmentOutput> {
  const normalized = normalizeEvent(eventName, payload);
  if (!normalized) throw new Error('Could not normalize event');

  if (eventName === 'issue_comment' && opts.repo && opts.token) {
    const [owner, repo] = opts.repo.split('/');
    if (owner && repo) {
      const existing = await checkExistingPr(owner, repo, normalized.issueNumber);
      if (existing) return { action: 'redirect_to_pr', issue_number: normalized.issueNumber, pr_url: existing.pr_url };
    }
  }

  const referenceIssue = opts.referenceIssue ?? '192';
  const contextBlock = opts.contextFilesContent ?? loadContextFiles();
  let issueComments: IssueComment[] = [];
  if ((eventName === 'issues' || eventName === 'issue_comment') && opts.repo && opts.token) {
    const [owner, repo] = opts.repo.split('/');
    if (owner && repo) {
      issueComments = await fetchIssueComments(owner, repo, normalized.issueNumber, opts.token);
    }
  }
  const prompt = buildAssessmentPrompt(payload, eventName, referenceIssue, contextBlock, issueComments);
  const result = await callClaude(prompt, opts.anthropicClient);
  result.issue_number = normalized.issueNumber;
  return result;
}

// Only run main when invoked as script (not when imported by tests). In CI, the runner sets
// GITHUB_EVENT_PATH, so we must skip main() when Vitest is running to avoid unhandled process.exit.
if (process.env.GITHUB_EVENT_PATH && !process.env.VITEST) {
  main().catch((err) => {
    console.error(err);
    process.exit(1);
  });
}
