#!/usr/bin/env -S npx tsx
/**
 * Assess script: read GitHub event, normalize, optionally check redirect, call Claude, output JSON.
 * Output: { action: 'implement' | 'request_info' | 'redirect_to_pr', comment_body?, verification_notes?, pr_url? }
 * Run: GITHUB_EVENT_PATH=... GITHUB_EVENT_NAME=... [ANTHROPIC_API_KEY=...] npx tsx index.ts
 */

import { readFileSync } from 'fs';
import { resolve } from 'path';
import Anthropic from '@anthropic-ai/sdk';
import { normalizeEvent } from './normalize.js';

const EVENT_PATH = process.env.GITHUB_EVENT_PATH || '';
const EVENT_NAME = process.env.GITHUB_EVENT_NAME || '';
const GITHUB_TOKEN = process.env.GITHUB_TOKEN || '';
const ANTHROPIC_API_KEY = process.env.ANTHROPIC_API_KEY || '';
const REPO = process.env.GITHUB_REPOSITORY || '';

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

function buildAssessmentPrompt(payload: unknown, eventName: string, referenceIssue: string): string {
  const p = payload as Record<string, unknown>;
  const issue = p.issue as { title?: string; body?: string; number?: number } | undefined;
  const parts: string[] = [
    'You are assessing a GitHub issue to decide if there is enough information to implement a fix or feature.',
    'Reply with a single JSON object only, no markdown or extra text, with these keys:',
    '- action: either "implement" or "request_info"',
    '- comment_body: (required if action is request_info) a short message to post on the issue asking for the missing information',
    '- verification_notes: (optional if action is implement) free-form notes for the implementer, e.g. "run go test ./pkg/... and ensure build passes"',
    '',
    'Issue:',
    `Title: ${issue?.title ?? 'N/A'}`,
    `Body: ${issue?.body ?? 'N/A'}`,
    '',
    `Reference example of "enough information": GitHub issue #${referenceIssue} (use similar clarity and specificity).`,
  ];

  const comment = p.comment as { body?: string } | undefined;
  if (comment?.body) {
    parts.push('', 'Latest comment:', comment.body);
  }
  if (Array.isArray(p.comments) && p.comments.length) {
    parts.push('', 'Comments:', JSON.stringify(p.comments, null, 2));
  }
  if (eventName === 'pull_request_review' || eventName === 'pull_request_review_comment') {
    const review = (p.review as { body?: string }) ?? {};
    parts.push('', 'PR review (address this feedback):', review.body ?? 'N/A');
  }

  parts.push('', 'Output only the JSON object:');
  return parts.join('\n');
}

async function callClaude(prompt: string, client?: Anthropic): Promise<AssessmentOutput> {
  const api = client ?? new Anthropic({ apiKey: ANTHROPIC_API_KEY });
  if (!client && !ANTHROPIC_API_KEY) {
    throw new Error('ANTHROPIC_API_KEY is not set');
  }
  const response = await api.messages.create({
    model: 'claude-sonnet-4-20250514',
    max_tokens: 1024,
    messages: [{ role: 'user', content: prompt }],
  });
  const text = response.content?.[0]?.type === 'text' ? response.content[0].text : '';
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
  opts: { repo?: string; token?: string; referenceIssue?: string; anthropicClient?: Anthropic }
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
  const prompt = buildAssessmentPrompt(payload, eventName, referenceIssue);
  const result = await callClaude(prompt, opts.anthropicClient);
  result.issue_number = normalized.issueNumber;
  return result;
}

if (process.env.GITHUB_EVENT_PATH) {
  main().catch((err) => {
    console.error(err);
    process.exit(1);
  });
}
