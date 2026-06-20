import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import path from 'path'

// https://vite.dev/config/
export default defineConfig(({ mode }) => ({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  // AIDEV-NOTE: Strip console.* and debugger from the PRODUCTION bundle only
  // (mode is 'development' under `vite`/`vite dev`, so dev + e2e keep their logs).
  // Belt-and-suspenders with the no-console ESLint rule: lint blocks stray logs in
  // source, this guarantees none ship even if one slips through (e.g. from a dep).
  esbuild: {
    drop: mode === 'production' ? ['console', 'debugger'] : [],
  },
  server: {
    port: 7827,
    proxy: {
      '/api': {
        // AIDEV-NOTE: No rewrite — the backend serves the REST API under /api (app.py Blueprint), so forward /api/* unchanged.
        target: 'http://localhost:7828',
        changeOrigin: true,
      },
      '/socket.io': {
        target: 'http://localhost:7828',
        changeOrigin: true,
        ws: true,
      },
    },
  },
}))
