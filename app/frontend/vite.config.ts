import { writeFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { defineConfig, type Plugin } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'

// go:embed needs frontend/dist to exist on a clean checkout, so the build
// puts back the .gitkeep that emptying the folder removes.
const keepDist = (): Plugin => ({
  name: 'keep-dist',
  closeBundle() {
    writeFileSync(resolve(__dirname, 'dist/.gitkeep'), '')
  },
})

export default defineConfig({
  plugins: [svelte(), keepDist()],
  server: {
    watch: { ignored: ['**/wailsjs/**'] },
  },
  build: {
    chunkSizeWarningLimit: 1500,
  },
})
