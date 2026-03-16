import { describe, it, expect, vi } from 'vitest';
import { readFileSync } from 'fs';
import { resolve, dirname } from 'path';
import { fileURLToPath } from 'url';
import { assess } from '../src/index.js';

const __dirname = dirname(fileURLToPath(import.meta.url));
const FIXTURES_DIR = resolve(__dirname, '../fixtures');

function loadFixture(name: string): unknown {
  const raw = readFileSync(resolve(FIXTURES_DIR, name), 'utf-8');
  return JSON.parse(raw);
}

describe('assess', () => {
  it('returns implement or request_info with valid shape when using mock client', async () => {
    const mockClient = {
      messages: {
        create: vi.fn().mockResolvedValue({
          content: [
            {
              type: 'text',
              text: '{"action":"implement","verification_notes":"Run go test ./..."}',
            },
          ],
        }),
      },
    } as unknown as import('@anthropic-ai/sdk').Anthropic;

    const payload = loadFixture('issue-labeled.json');
    const result = await assess('issues', payload, {
      referenceIssue: '192',
      anthropicClient: mockClient,
    });

    expect(result.action).toMatch(/^(implement|request_info)$/);
    if (result.action === 'request_info') {
      expect(typeof result.comment_body).toBe('string');
    }
    if (result.action === 'implement' && result.verification_notes !== undefined) {
      expect(typeof result.verification_notes).toBe('string');
    }
  });

  it('returns valid output shape for issue_comment fixture with mock client', async () => {
    const mockClient = {
      messages: {
        create: vi.fn().mockResolvedValue({
          content: [{ type: 'text', text: '{"action":"request_info","comment_body":"Please add steps to reproduce."}' }],
        }),
      },
    } as unknown as import('@anthropic-ai/sdk').Anthropic;

    const payload = loadFixture('issue-comment.json');
    const result = await assess('issue_comment', payload, {
      referenceIssue: '192',
      anthropicClient: mockClient,
    });

    expect(['implement', 'request_info', 'redirect_to_pr']).toContain(result.action);
    if (result.action === 'request_info') expect(typeof result.comment_body).toBe('string');
  });
});
