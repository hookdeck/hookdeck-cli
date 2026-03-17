import { resolve } from 'path';
import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    include: ['test/integration/**/*.test.ts'],
    globals: false,
    testTimeout: 60_000,
    hookTimeout: 10_000,
  },
});
