import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import { codecovVitePlugin } from '@codecov/vite-plugin';

export default defineConfig({
  plugins: [
    react(),
    codecovVitePlugin({
      enableBundleAnalysis: process.env.CODECOV_TOKEN !== undefined,
      bundleName: 'dockgraph-frontend',
      uploadToken: process.env.CODECOV_TOKEN,
    }),
  ],
  server: {
    proxy: {
      '/ws': {
        target: 'ws://localhost:7800',
        ws: true,
      },
      '/healthz': {
        target: 'http://localhost:7800',
      },
    },
  },
});
