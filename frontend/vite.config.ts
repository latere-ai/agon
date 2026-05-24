import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';

export default defineConfig({
  plugins: [vue()],
  // latere-ui ships source SFCs; compile them for the SSG build and keep one Vue copy.
  resolve: { dedupe: ['vue'] },
  ssr: { noExternal: ['latere-ui'] },
  ssgOptions: {
    concurrency: 1,
    includedRoutes(paths: string[]) {
      return paths.filter(p => !p.includes(':') && !p.includes('*'));
    },
  },
  server: {
    port: 5173,
    proxy: {
      '/healthz': 'http://localhost:8080',
      '/readyz': 'http://localhost:8080',
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    sourcemap: false,
    target: 'es2022',
  },
  test: {
    environment: 'happy-dom',
    globals: false,
    include: ['src/**/*.test.ts'],
  },
});
