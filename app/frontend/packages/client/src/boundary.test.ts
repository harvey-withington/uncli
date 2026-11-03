import { readdirSync, readFileSync } from 'node:fs'
import { join, relative } from 'node:path'
import { describe, expect, it } from 'vitest'

// @uncli/client stands alone (decision 0026): nothing from the app, and no
// Svelte, so any stack can use it.
const ROOT = join(import.meta.dirname)

function sources(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap(e =>
    e.isDirectory() ? sources(join(dir, e.name)) : e.name.endsWith('.ts') && !e.name.endsWith('.test.ts') ? [join(dir, e.name)] : [],
  )
}

describe('@uncli/client', () => {
  it('imports nothing from outside the package, and no Svelte', () => {
    const bad: string[] = []
    for (const file of sources(ROOT)) {
      for (const [, spec] of readFileSync(file, 'utf8').matchAll(/(?:from|import)\s*\(?\s*'([^']+)'/g)) {
        if (!spec) continue
        const outside = spec.startsWith('.') && relative(ROOT, join(file, '..', spec)).startsWith('..')
        if (outside || spec === 'svelte' || spec.startsWith('svelte/') || spec.endsWith('.svelte')) bad.push(`${relative(ROOT, file)}: ${spec}`)
      }
    }
    expect(bad).toEqual([])
  })
})
