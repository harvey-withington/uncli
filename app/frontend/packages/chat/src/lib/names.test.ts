import { readdirSync, readFileSync } from 'node:fs'
import { join, relative } from 'node:path'
import { describe, expect, it } from 'vitest'
import en from '../locales/en.json'

// Two chats on one page may be on servers with different AI providers
// (decision 0026), so the chat never leans on the page-wide default names:
// every string that names the provider ({agent}, {cli}, {account}) is
// given the session's names.
const ROOT = join(import.meta.dirname, '..')
const NAMED = new Set(Object.entries(en as Record<string, string>).filter(([, v]) => /\{(agent|cli|account)\}/.test(v)).map(([k]) => k))

function sources(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap(e =>
    e.isDirectory() ? sources(join(dir, e.name)) : /\.(ts|svelte)$/.test(e.name) && !e.name.endsWith('.test.ts') ? [join(dir, e.name)] : [],
  )
}

describe('provider names in the chat', () => {
  it('are always the session’s, never the page-wide default', () => {
    const bare: string[] = []
    for (const file of sources(ROOT)) {
      const src = readFileSync(file, 'utf8')
      const at = relative(ROOT, file)
      // t('key') with no params at all.
      for (const m of src.matchAll(/\bt\(\s*'([a-zA-Z0-9_.]+)'\s*\)/g)) if (NAMED.has(m[1]!)) bare.push(`${at}: ${m[1]}`)
      // t(`prefix${…}`) with no params: any named key it could build.
      for (const m of src.matchAll(/\bt\(\s*`([a-zA-Z0-9_.]*)\$\{[^`]*`\s*\)/g)) {
        const could = [...NAMED].filter(k => k.startsWith(m[1]!) && !k.slice(m[1]!.length).includes('.'))
        if (could.length) bare.push(`${at}: \`${m[1]}…\` (${could.join(', ')})`)
      }
    }
    expect(bare).toEqual([])
  })
})
