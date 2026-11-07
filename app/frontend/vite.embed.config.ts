import { resolve } from 'node:path'
import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'
import { deckPlayer } from './packages/chat/build/deck-player'

// <uncli-chat> as a library (decision 0026), built into @uncli/chat's dist
// as its element export: one ES module that registers the elements, with
// Svelte and UNCLI's client inside it. Component styles are injected by Svelte at mount into the root node of the target, which
// is each element's shadow root, so none of them reach the host's page.
// Code highlighting's languages stay separate chunks, loaded when needed.
// `vite build --config vite.embed.config.ts --mode sample` builds the
// sample host page (embed-sample/) on the mock instead.
export default defineConfig(({ mode }) => {
  const plugins = [svelte({ compilerOptions: { css: 'injected' }, emitCss: false }), deckPlayer()]
  const alias = { '@uncli/chat': resolve(__dirname, 'packages/chat/src') }
  if (mode === 'sample') {
    return {
      root: resolve(__dirname, 'embed-sample'),
      plugins,
      resolve: { alias },
      server: { fs: { allow: [resolve(__dirname)] } },
      build: { outDir: resolve(__dirname, 'dist-embed/sample'), emptyOutDir: true, chunkSizeWarningLimit: 3000 },
    }
  }
  return {
    plugins,
    resolve: { alias },
    build: {
      outDir: resolve(__dirname, 'packages/chat/dist'),
      emptyOutDir: true,
      chunkSizeWarningLimit: 3000,
      lib: { entry: resolve(__dirname, 'packages/chat/src/embed/index.ts'), formats: ['es'], fileName: () => 'uncli-chat.js' },
    },
  }
})
