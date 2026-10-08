import { describe, expect, it } from 'vitest'
import { addPackages, blank, idFor, problems, same, splitPackages, tidy } from './containers'

describe('container editor rules', () => {
  it('makes an id from the name, unlike the others', () => {
    expect(idFor('Web dev (Node 22)', [])).toBe('web-dev-node-22')
    expect(idFor('Sandbox', ['sandbox', 'sandbox-2'])).toBe('sandbox-3')
    expect(idFor('…', [])).toBe('container')
  })

  it('reads package names typed with spaces or commas, once each', () => {
    expect(splitPackages(' git, Go  make ')).toEqual(['git', 'go', 'make'])
    expect(addPackages(['git'], 'git go')).toEqual(['git', 'go'])
  })

  it('says what stops a save', () => {
    const p = { ...blank('web', 'alpine-3.24'), label: 'Web' }
    expect(problems(p)).toEqual([])
    expect(problems({ ...p, label: ' ' })).toEqual(['containerEdit.problem.name'])
    expect(problems({ ...p, packages: ['go;rm'] })).toEqual(['containerEdit.problem.package'])
    expect(problems({ ...p, id: 'Web' })).toEqual(['containerEdit.problem.id'])
    expect(problems({ ...p, setup: ['x'.repeat(4001)] })).toEqual(['containerEdit.problem.step'])
  })

  it('leaves out empty steps, and counts that as no change', () => {
    const p = { ...blank('web', 'alpine-3.24'), label: 'Web', setup: ['npm i -g pnpm'] }
    expect(tidy({ ...p, label: ' Web ', setup: ['npm i -g pnpm', '  '] })).toEqual(p)
    expect(same(p, { ...p, setup: [...p.setup, ''] })).toBe(true)
    expect(same(p, { ...p, packages: ['go'] })).toBe(false)
  })
})
