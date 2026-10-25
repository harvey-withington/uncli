import { resolve } from 'node:path'
import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'

// The phone app (decision 0021): its own page, built into dist/m and served
// by UNCLI's remote access server at /m/, with its manifest, icons and
// service worker (m/public).
export default defineConfig({
  root: resolve(__dirname, 'm'),
  base: '/m/',
  plugins: [svelte()],
  // The build's id: the page registers the service worker with it, so a new
  // build replaces the old cache (m/public/sw.js).
  define: { __BUILD_ID__: JSON.stringify(Date.now().toString(36)) },
  server: { fs: { allow: [resolve(__dirname)] } },
  build: {
    outDir: resolve(__dirname, 'dist/m'),
    emptyOutDir: true,
    chunkSizeWarningLimit: 1500,
  },
})
