/// <reference types="vitest/config" />
import { defineConfig } from 'vite';

export default defineConfig({
  oxc: { jsx: { runtime: 'automatic', importSource: 'preact' } },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    rollupOptions: {
      input: { background: 'src/background.ts', options: 'options.html', popup: 'popup.html' },
      output: { entryFileNames: '[name].js' },
    },
  },
  test: { environment: 'jsdom' },
});
