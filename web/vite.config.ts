import preact from '@preact/preset-vite';
import { defineConfig } from 'vitest/config';

const backend = 'http://127.0.0.1:8080';

export default defineConfig({
  plugins: [preact()],
  build: {
    outDir: '../internal/webui/dist',
    emptyOutDir: true,
  },
  server: {
    proxy: { '/api': backend, '/dav': backend, '/healthz': backend },
  },
  test: {
    environment: 'node',
    // A zone with daylight saving time, so the date helpers are tested across its transitions.
    env: { TZ: 'America/New_York' },
  },
});
