const { shouldPromptUpgrade } = require('../src/version');

describe('Version comparison', () => {
  test('1.10.0-beta.4 vs v1.9.1 should not prompt upgrade', () => {
    expect(shouldPromptUpgrade('1.10.0-beta.4', 'v1.9.1')).toBe(false);
  });
  
  test('1.9.1 vs 1.10.0 should prompt upgrade', () => {
    expect(shouldPromptUpgrade('1.9.1', '1.10.0')).toBe(true);
  });
  
  test('1.10.0-beta.4 vs 1.10.0-beta.5 should prompt upgrade', () => {
    expect(shouldPromptUpgrade('1.10.0-beta.4', '1.10.0-beta.5')).toBe(true);
  });
  
  test('1.10.0-beta.4 vs 1.10.0 should prompt upgrade', () => {
    expect(shouldPromptUpgrade('1.10.0-beta.4', '1.10.0')).toBe(true);
  });
  
  test('1.10.0 vs 1.11.0-beta.1 should not prompt upgrade', () => {
    expect(shouldPromptUpgrade('1.10.0', '1.11.0-beta.1')).toBe(false);
  });
  
  test('handles v prefix correctly', () => {
    expect(shouldPromptUpgrade('v1.9.1', 'v1.10.0')).toBe(true);
  });
  
  test('invalid versions should not prompt', () => {
    expect(shouldPromptUpgrade('invalid', '1.10.0')).toBe(false);
    expect(shouldPromptUpgrade('1.10.0', 'invalid')).toBe(false);
  });
});