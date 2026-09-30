/// <reference types="vitest/config" />
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import fs from 'node:fs'
import path from 'node:path'

const outDir = path.resolve(import.meta.dirname, '../internal/web/dist')

export default defineConfig({
  plugins: [
    react(),
    tailwindcss(),
    {
      // emptyOutDir deletes the tracked .keep placeholder; put it back.
      name: 'keep-dist-placeholder',
      closeBundle() {
        fs.writeFileSync(path.join(outDir, '.keep'), '')
      },
    },
  ],
  resolve: { alias: { '@': path.resolve(import.meta.dirname, './src') } },
  build: { outDir, emptyOutDir: true },
  server: {
    // Dev: run `devupdater ui -port 8765 -no-browser -workspace <dir>`, open the
    // printed URL once (sets the cookie for 127.0.0.1), then use http://127.0.0.1:5173.
    host: '127.0.0.1',
    proxy: { '/api': 'http://127.0.0.1:8765' },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
    exclude: ['e2e/**', 'node_modules/**'],
  },
})
