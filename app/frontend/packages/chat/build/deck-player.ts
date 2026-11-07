// The walkthrough deck's script as a file of its own (decision 0029):
// reveal.js (its UMD build, a plain script that defines Reveal) and then
// the deck's player (src/lib/deck/player.js). The deck's frame loads it by
// URL from the app's origin, since a page's content security policy, which
// a srcdoc frame inherits, may refuse inline scripts.
//
// `import url from 'virtual:uncli-deck-player'` is that URL: an emitted
// asset in a build (the app's, the phone's, or the element's, where it's
// relative to the element's own module), served from memory by the dev
// server and in tests.
import { readFileSync } from 'node:fs'
import { createRequire } from 'node:module'
import { resolve } from 'node:path'
import type { Plugin } from 'vite'

const ID = 'virtual:uncli-deck-player'
const DEV_PATH = '/@uncli/deck-player.js'

export function deckPlayerSource(): string {
  const require = createRequire(resolve(__dirname, '../package.json'))
  const reveal = readFileSync(require.resolve('reveal.js'), 'utf8') // the "require" export: the UMD build
  const player = readFileSync(resolve(__dirname, '../src/lib/deck/player.js'), 'utf8')
  return `${reveal}\n${player}`
}

export function deckPlayer(): Plugin {
  let serve = false
  return {
    name: 'uncli-deck-player',
    configResolved(c) {
      serve = c.command === 'serve'
    },
    resolveId: id => (id === ID ? `\0${ID}` : null),
    load(id) {
      if (id !== `\0${ID}`) return null
      if (serve) return `export default ${JSON.stringify(DEV_PATH)}`
      const ref = this.emitFile({ type: 'asset', name: 'deck-player.js', source: deckPlayerSource() })
      return `export default import.meta.ROLLUP_FILE_URL_${ref}`
    },
    configureServer(server) {
      server.middlewares.use(DEV_PATH, (_req, res) => {
        res.setHeader('Content-Type', 'text/javascript; charset=utf-8')
        res.end(deckPlayerSource())
      })
    },
  }
}
