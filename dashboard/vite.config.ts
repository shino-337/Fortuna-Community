import path from 'path';
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig(() => {
    return {
      server: {
        port: 3000,
        host: '0.0.0.0',
      },
      plugins: [react()],
      define: {
        '__BUILD_TIME__': JSON.stringify(new Date().toISOString()),
      },
      resolve: {
        alias: {
          '@': path.resolve(__dirname, '.'),
        },
      },
      build: {
        chunkSizeWarningLimit: 600,
        rollupOptions: {
          output: {
            manualChunks(id) {
              if (id.includes('node_modules/d3')) return 'd3';
              if (id.includes('node_modules/recharts')) return 'recharts';
              if (id.includes('node_modules/react-dom') || id.includes('node_modules/react/')) {
                return 'react-vendor';
              }
            },
          },
        },
      },
    };
});
