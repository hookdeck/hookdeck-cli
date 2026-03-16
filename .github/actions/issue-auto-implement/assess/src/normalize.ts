/**
 * Event normalization: derive issue number (and related) from GitHub workflow event payloads.
 * Used for: issues, issue_comment, pull_request_review, pull_request_review_comment.
 */

export interface NormalizedEvent {
  eventName: string;
  issueNumber: number;
  /** For issue_comment: should we redirect to PR instead of assessing? (PR exists for this issue) */
  redirectToPr?: boolean;
  prUrl?: string;
  /** Pull request head ref (e.g. auto-implement-issue-123) for PR events */
  headRef?: string;
}

/** Match "Closes #123" or "Fixes #456" in text; returns first match or null */
export function parseClosesIssueNumber(text: string): number | null {
  if (!text || typeof text !== 'string') return null;
  const match = text.match(/(?:Closes|Fixes)\s+#(\d+)/i);
  return match ? parseInt(match[1], 10) : null;
}

/** Match branch name auto-implement-issue-<N>; returns N or null */
export function parseIssueNumberFromBranch(branchName: string): number | null {
  if (!branchName || typeof branchName !== 'string') return null;
  const match = branchName.match(/^auto-implement-issue-(\d+)$/);
  return match ? parseInt(match[1], 10) : null;
}

/** Derive issue number from a PR payload (body or head ref) */
export function issueNumberFromPrPayload(pr: { body?: string | null; head?: { ref?: string } }): number | null {
  const fromBody = pr?.body != null ? parseClosesIssueNumber(pr.body) : null;
  if (fromBody != null) return fromBody;
  const ref = pr?.head?.ref;
  return ref != null ? parseIssueNumberFromBranch(ref) : null;
}

/** Minimal payload shapes we need from GitHub Actions event */
export type IssuePayload = { issue: { number: number }; label?: { name: string } };
export type IssueCommentPayload = { issue: { number: number } };
export type PullRequestReviewPayload = { pull_request: { number: number; body?: string | null; head?: { ref?: string }; html_url?: string }; review?: { body?: string | null } };

export function normalizeEvent(eventName: string, payload: unknown): NormalizedEvent | null {
  if (!payload || typeof payload !== 'object') return null;

  const p = payload as Record<string, unknown>;

  if (eventName === 'issues') {
    const issue = (p.issue as { number: number })?.number;
    if (issue == null) return null;
    return { eventName: 'issues', issueNumber: issue };
  }

  if (eventName === 'issue_comment') {
    const issue = (p.issue as { number: number })?.number;
    if (issue == null) return null;
    return {
      eventName: 'issue_comment',
      issueNumber: issue,
      redirectToPr: false, // caller must set from API if PR exists
    };
  }

  if (eventName === 'pull_request_review' || eventName === 'pull_request_review_comment') {
    const pr = p.pull_request as { body?: string | null; head?: { ref?: string }; html_url?: string } | undefined;
    if (!pr) return null;
    const issueNumber = issueNumberFromPrPayload(pr);
    if (issueNumber == null) return null;
    return {
      eventName,
      issueNumber,
      headRef: pr.head?.ref,
    };
  }

  return null;
}
