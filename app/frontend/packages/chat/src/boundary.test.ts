import { readdirSync, readFileSync } from 'node:fs'
import { join, relative } from 'node:path'
import { describe, expect, it } from 'vitest'

// @uncli/chat stands on its own (decision 0026): it imports nothing from
// UNCLI's app (the window's shell, the Wails backend), so any host can
// use it. Within it, Svelte and @uncli/client are what it builds on.
const ROOT = join(import.meta.dirname)

function sources(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap(e =>
    e.isDirectory() ? sources(join(dir, e.name)) : /\.(ts|svelte)$/.test(e.name) && !e.name.endsWith('.test.ts') ? [join(dir, e.name)] : [],
  )
}

describe('@uncli/chat', () => {
  it('imports nothing from outside the package but its dependencies', () => {
    const bad: string[] = []
    for (const file of sources(ROOT)) {
      for (const [, spec] of readFileSync(file, 'utf8').matchAll(/(?:from|import)\s*\(?\s*'([^']+)'/g)) {
        if (!spec) continue
        const outside = spec.startsWith('.') && relative(ROOT, join(file, '..', spec.replace(/\?.*$/, ''))).startsWith('..')
        if (outside || spec.startsWith('@uncli/chat')) bad.push(`${relative(ROOT, file)}: ${spec}`)
      }
    }
    expect(bad).toEqual([])
  })
})
