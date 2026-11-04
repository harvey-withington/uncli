import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

// element.d.ts is written by hand for TypeScript hosts of the built
// element; this keeps it in step with the source it describes.
const dts = readFileSync('packages/chat/element.d.ts', 'utf8')
const element = readFileSync('packages/chat/src/embed/element.svelte.ts', 'utf8')
const index = readFileSync('packages/chat/src/embed/index.ts', 'utf8')
const controls = readFileSync('packages/chat/src/lib/composercontrols.ts', 'utf8')

const block = (src: string, start: string) => src.slice(src.indexOf(start), src.indexOf('\n}', src.indexOf(start)))
const members = (body: string) => [...body.matchAll(/^\s{2}(\w+)\??[(:]/gm)].map(m => m[1])

describe('element.d.ts', () => {
  it('describes every part of ChatHost', () => {
    const host = members(block(element, 'export interface ChatHost'))
    expect(host.length).toBeGreaterThan(5)
    const typed = members(block(dts, 'export interface ChatHost'))
    for (const m of host) expect(typed, m).toContain(m)
  })

  it('names everything the element entry exports', () => {
    const names = [...index.matchAll(/export (?:type )?\{([^}]+)\}/g)].flatMap(m => m[1]!.split(',').map(s => s.replace(/^\s*type\s+/, '').trim())).filter(Boolean)
    for (const n of names) expect(dts, n).toMatch(new RegExp(`\\b${n}\\b`))
  })

  it('has every composer control field', () => {
    for (const name of ['ButtonControl', 'ToggleControl', 'MenuControl', 'ControlItem']) {
      const src = members(block(controls, `export interface ${name}`))
      const typed = members(block(dts, `export interface ${name}`))
      for (const m of src) expect(typed, `${name}.${m}`).toContain(m)
    }
  })

  it('lists every event the element sends', () => {
    const sent = new Set([...element.matchAll(/'(uncli-[a-z-]+)'/g)].map(m => m[1]).filter(e => e !== 'uncli-root' && e !== 'uncli-chat'))
    for (const e of sent) expect(dts, e).toContain(`'${e}'`)
  })
})
