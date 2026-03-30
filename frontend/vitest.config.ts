import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    coverage: {
      thresholds: {
        statements: 60,
        branches: 60,
      },
    },
  },
});
