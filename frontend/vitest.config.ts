import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    // Node 25+ exposes its own global localStorage, which shadows jsdom's and
    // is an empty stub without --localstorage-file. Turn it off in the test
    // workers so jsdom's storage is the one tests see.
    execArgv: ['--no-experimental-webstorage'],
    coverage: {
      thresholds: {
        statements: 60,
        branches: 60,
      },
    },
  },
});
