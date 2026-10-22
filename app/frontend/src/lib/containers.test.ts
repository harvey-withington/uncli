import { describe, expect, it } from 'vitest'
import { addPackages, blank, folderNameFor, folderProblem, idFor, problems, same, splitPackages, tidy } from './containers'

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

describe('a container’s folders', () => {
  const f = (source: string, name: string, access: 'ro' | 'rw' = 'ro') => ({ source, name, access })

  it('takes full paths and the three variables, and refuses what is too wide', () => {
    expect(folderProblem(f('%USERPROFILE%\\notes', 'notes'))).toBe('')
    expect(folderProblem(f('S:\\Source\\arc', 'arc', 'rw'))).toBe('')
    expect(folderProblem(f('%TEMP%\\x', 'x'))).toBe('containerEdit.folderBadVar')
    expect(folderProblem(f('notes', 'notes'))).toBe('containerEdit.folderNotFull')
    expect(folderProblem(f('C:\\', 'c'))).toBe('containerEdit.folderTooWide')
    expect(folderProblem(f('%USERPROFILE%\\', 'me'))).toBe('containerEdit.folderTooWide')
    expect(folderProblem(f('C:\\Program Files (x86)', 'pf'))).toBe('containerEdit.folderTooWide')
    expect(folderProblem(f('S:\\Source', '.ssh'))).toBe('containerEdit.folderBadName')
  })

  it('names a folder from its last part', () => {
    expect(folderNameFor('%USERPROFILE%\\My Notes\\')).toBe('my-notes')
    expect(folderNameFor('%APPDATA%')).toBe('appdata')
  })

  it('stops a save on a bad or repeated folder, and leaves out empty rows', () => {
    const p = { ...blank('dev', 'alpine-3.24'), label: 'Dev' }
    expect(problems({ ...p, folders: [f('S:\\A', 'a'), f('S:\\B', 'a')] })).toContain('containerEdit.problem.folderTwice')
    expect(problems({ ...p, folders: [f('C:\\', 'c')] })).toContain('containerEdit.problem.folder')
    expect(problems({ ...p, folders: [f('', '')] })).toEqual([])
    expect(tidy({ ...p, folders: [f(' S:\\A ', ' a '), f('', '')] }).folders).toEqual([f('S:\\A', 'a')])
  })
})
