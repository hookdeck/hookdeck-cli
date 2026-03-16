import { describe, it, expect } from 'vitest';
import {
  parseClosesIssueNumber,
  parseIssueNumberFromBranch,
  issueNumberFromPrPayload,
  normalizeEvent,
} from './normalize';

describe('parseClosesIssueNumber', () => {
  it('extracts issue number from Closes #123', () => {
    expect(parseClosesIssueNumber('Closes #123')).toBe(123);
    expect(parseClosesIssueNumber('Closes #1')).toBe(1);
  });
  it('extracts issue number from Fixes #456', () => {
    expect(parseClosesIssueNumber('Fixes #456')).toBe(456);
  });
  it('returns first match', () => {
    expect(parseClosesIssueNumber('Closes #10 and Fixes #20')).toBe(10);
  });
  it('returns null for empty or no match', () => {
    expect(parseClosesIssueNumber('')).toBeNull();
    expect(parseClosesIssueNumber('No issue here')).toBeNull();
    expect(parseClosesIssueNumber(null as unknown as string)).toBeNull();
  });
});

describe('parseIssueNumberFromBranch', () => {
  it('extracts issue number from auto-implement-issue-<N>', () => {
    expect(parseIssueNumberFromBranch('auto-implement-issue-42')).toBe(42);
    expect(parseIssueNumberFromBranch('auto-implement-issue-1')).toBe(1);
  });
  it('returns null for non-matching branch', () => {
    expect(parseIssueNumberFromBranch('main')).toBeNull();
    expect(parseIssueNumberFromBranch('feature/foo')).toBeNull();
    expect(parseIssueNumberFromBranch('auto-implement-issue-')).toBeNull();
  });
});

describe('issueNumberFromPrPayload', () => {
  it('prefers body Closes #N over branch', () => {
    expect(
      issueNumberFromPrPayload({
        body: 'Closes #99',
        head: { ref: 'auto-implement-issue-42' },
      })
    ).toBe(99);
  });
  it('uses branch when body has no Closes/Fixes', () => {
    expect(
      issueNumberFromPrPayload({
        body: 'Some description',
        head: { ref: 'auto-implement-issue-42' },
      })
    ).toBe(42);
  });
  it('returns null when neither present', () => {
    expect(issueNumberFromPrPayload({})).toBeNull();
    expect(issueNumberFromPrPayload({ body: '', head: { ref: 'main' } })).toBeNull();
  });
});

describe('normalizeEvent', () => {
  it('extracts issue number from issues payload', () => {
    const r = normalizeEvent('issues', { issue: { number: 192 } });
    expect(r).toEqual({ eventName: 'issues', issueNumber: 192 });
  });

  it('extracts issue number from issue_comment payload', () => {
    const r = normalizeEvent('issue_comment', { issue: { number: 5 } });
    expect(r).toEqual({ eventName: 'issue_comment', issueNumber: 5, redirectToPr: false });
  });

  it('extracts issue number from pull_request_review (body)', () => {
    const r = normalizeEvent('pull_request_review', {
      pull_request: { body: 'Fixes #10', head: { ref: 'auto-implement-issue-10' } },
    });
    expect(r).toEqual({ eventName: 'pull_request_review', issueNumber: 10, headRef: 'auto-implement-issue-10' });
  });

  it('extracts issue number from pull_request_review (branch only)', () => {
    const r = normalizeEvent('pull_request_review', {
      pull_request: { head: { ref: 'auto-implement-issue-7' } },
    });
    expect(r).toEqual({ eventName: 'pull_request_review', issueNumber: 7, headRef: 'auto-implement-issue-7' });
  });

  it('returns null for unknown event or missing data', () => {
    expect(normalizeEvent('push', {})).toBeNull();
    expect(normalizeEvent('issues', {})).toBeNull();
    expect(normalizeEvent('pull_request_review', { pull_request: {} })).toBeNull();
  });
});
