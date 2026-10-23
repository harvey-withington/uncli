import type { WatchEntry, WatchFile } from './types'

// The mock's session folders for the File tab: a few text files a session
// might watch, newest first by when they changed. writeWatched changes one
// (tests use it to see the tab update live).

const MIN = 60_000

function seed(now: number): Record<string, { text: string; modified: number }> {
  return {
    'TODO.md': { text: '# TODO\n\n- [x] Find the flaky parser test\n- [ ] Fix the off-by-one in `scan()`\n- [ ] Add a test for empty input\n', modified: now - 2 * MIN },
    'docs/TODO-later.md': { text: '# Later\n\n- Split the parser into a lexer and a parser\n', modified: now - 3 * 24 * 60 * MIN },
    'docs/notes.md': { text: '# Notes\n\nThe parser reads one token ahead.\n', modified: now - 40 * MIN },
    'src/parser.ts': { text: 'export function scan(input: string): string[] {\n  return input.split(/\\s+/).filter(Boolean)\n}\n', modified: now - 10 * MIN },
    'build.log': { text: 'building…\nok\n', modified: now - 90 * MIN },
    'logo.bin': { text: '\0', modified: now - 5 * 24 * 60 * MIN },
  }
}

export function mockWatch() {
  const folders = new Map<string, ReturnType<typeof seed>>()
  const folder = (sid: string) => {
    let f = folders.get(sid)
    if (!f) folders.set(sid, (f = seed(Date.now())))
    return f
  }
  return {
    async watchFiles(sid: string, query: string): Promise<WatchEntry[]> {
      const q = query.trim().toLowerCase()
      return Object.entries(folder(sid))
        .filter(([p]) => !p.endsWith('.bin') && p.toLowerCase().includes(q))
        .map(([path, f]) => ({ path, size: f.text.length, modified: f.modified }))
        .sort((a, b) => b.modified - a.modified || a.path.localeCompare(b.path))
        .slice(0, 50)
    },
    async watchRead(sid: string, path: string, since: number): Promise<WatchFile | null> {
      const f = folder(sid)[path]
      if (!f) return since === -1 ? null : { path, text: '', size: 0, modified: 0, missing: true }
      if (f.modified === since) return null
      const binary = f.text.includes('\0')
      return { path, text: binary ? '' : f.text, size: f.text.length, modified: f.modified, binary: binary || undefined }
    },
    // Not part of Backend: changes (or deletes, with null) a mock file.
    writeWatched(sid: string, path: string, text: string | null) {
      const f = folder(sid)
      if (text === null) delete f[path]
      else f[path] = { text, modified: Math.max(Date.now(), (f[path]?.modified ?? 0) + 1) }
    },
  }
}
