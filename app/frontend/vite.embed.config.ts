import { resolve } from 'node:path'
import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'

// <uncli-chat> as a library (decision 0026): one ES module that registers
// the element, with Svelte and UNCLI's client inside it. Component styles
// are injected by Svelte at mount into the root node of the target, which
// is each element's shadow root, so none of them reach the host's page.
// Code highlighting's languages stay separate chunks, loaded when needed.
// `vite build --config vite.embed.config.ts --mode sample` builds the
// sample host page (embed-sample/) on the mock instead.
export default defineConfig(({ mode }) => {
  const plugins = [svelte({ compilerOptions: { css: 'injected' }, emitCss: false })]
  if (mode === 'sample') {
    return {
      root: resolve(__dirname, 'embed-sample'),
      plugins,
      server: { fs: { allow: [resolve(__dirname)] } },
      build: { outDir: resolve(__dirname, 'dist-embed/sample'), emptyOutDir: true, chunkSizeWarningLimit: 3000 },
    }
  }
  return {
    plugins,
    build: {
      outDir: resolve(__dirname, 'dist-embed/lib'),
      emptyOutDir: true,
      chunkSizeWarningLimit: 3000,
      lib: { entry: resolve(__dirname, 'src/embed/index.ts'), formats: ['es'], fileName: () => 'uncli-chat.js' },
    },
  }
})
