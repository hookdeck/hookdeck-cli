/**
 * Integration tests: call real Claude API. Require AUTO_IMPLEMENT_ANTHROPIC_API_KEY (or ANTHROPIC_API_KEY).
 * Run with: npm run test:integration
 * Not run with: npm test (unit tests only)
 */
import { config } from 'dotenv';
import { resolve, dirname } from 'path';
import { fileURLToPath } from 'url';
import { readFileSync } from 'fs';
import { describe, it, expect } from 'vitest';
import { assess } from '../../src/index.js';

const __dirname = dirname(fileURLToPath(import.meta.url));

// Load .env from action root then cwd so integration tests see the same env as local runs
config({ path: resolve(process.cwd(), '../.env') });
config({ path: resolve(process.cwd(), '.env') });

const hasApiKey = !!(
  process.env.AUTO_IMPLEMENT_ANTHROPIC_API_KEY || process.env.ANTHROPIC_API_KEY
);

const FIXTURES_DIR = resolve(__dirname, '../fixtures');

function loadFixture(name: string): unknown {
  const raw = readFileSync(resolve(FIXTURES_DIR, name), 'utf-8');
  return JSON.parse(raw);
}

describe.skipIf(!hasApiKey)('assess (integration with Claude)', () => {
  it('returns valid assessment shape for issue-labeled fixture (real API)', async () => {
    const payload = loadFixture('issue-labeled.json');
    const result = await assess('issues', payload, {
      referenceIssue: '192',
      // No anthropicClient: use real API
    });

    expect(result).toBeDefined();
    expect(['implement', 'request_info']).toContain(result.action);
    expect(typeof result.issue_number).toBe('number');
    expect(result.issue_number).toBe(192);

    if (result.action === 'request_info') {
      expect(typeof result.comment_body).toBe('string');
      expect(result.comment_body.length).toBeGreaterThan(0);
    }
    if (result.action === 'implement' && result.verification_notes !== undefined) {
      expect(typeof result.verification_notes).toBe('string');
    }
  }, 45_000);

  it('returns valid assessment shape for issue-comment fixture (real API)', async () => {
    const payload = loadFixture('issue-comment.json');
    const result = await assess('issue_comment', payload, {
      referenceIssue: '192',
    });

    expect(result).toBeDefined();
    expect(['implement', 'request_info', 'redirect_to_pr']).toContain(result.action);
    expect(typeof result.issue_number).toBe('number');

    if (result.action === 'request_info') {
      expect(typeof result.comment_body).toBe('string');
    }
  }, 45_000);

  it('returns valid assessment shape for pull_request_review fixture (real API)', async () => {
    const payload = loadFixture('pull_request_review.json');
    const result = await assess('pull_request_review', payload, {
      referenceIssue: '192',
    });

    expect(result).toBeDefined();
    expect(['implement', 'request_info']).toContain(result.action);
    expect(typeof result.issue_number).toBe('number');
  }, 45_000);
});
