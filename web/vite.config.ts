import { defineConfig } from 'vite';

export default defineConfig({
  build: {
    outDir: '../internal/web/dist',
    emptyOutDir: true,
    target: 'es2022',
  },
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8080',
        ws: true,
      },
    },
  },
});
