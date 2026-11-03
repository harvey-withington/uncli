import { resolve } from 'node:path'
import { defineConfig } from 'vitest/config'
import { svelte } from '@sveltejs/vite-plugin-svelte'

export default defineConfig({
  plugins: [svelte({ hot: false })],
  // Use Svelte's browser build in jsdom, or mount() picks the SSR entry.
  resolve: { conditions: ['browser'], alias: { '@uncli/chat': resolve(__dirname, 'packages/chat/src') } },
  // Tests read the UI contract (docs/UI-CONVENTIONS.md) to check it against the code.
  server: { fs: { allow: ['.', '../../docs'] } },
  test: {
    environment: 'jsdom',
    include: ['src/**/*.test.ts', 'packages/*/src/**/*.test.ts'],
    setupFiles: ['src/test-setup.ts'],
  },
})
