import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  build: {
    // Use esbuild (default) for minification - it's faster than terser
    minify: 'esbuild',
  },
  esbuild: {
    // Keep console logs for debugging (remove in production release)
    // drop: ['console', 'debugger'],
  },
})
