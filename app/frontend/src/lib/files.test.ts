import { describe, expect, it } from 'vitest'
import type { Editor } from './api'
import { canOpenDefault, editorLabel, relPath, splitPath } from './files'

describe('relPath', () => {
  it('shows files inside the session folder relative to it', () => {
    expect(relPath('C:\\Users\\you\\code', 'C:\\Users\\you\\code\\internal\\x.go')).toBe('internal/x.go')
    expect(relPath('C:\\Users\\you\\code', 'c:/users/YOU/code/a.txt')).toBe('a.txt') // case and slashes
    expect(relPath('/home/me/proj', '/home/me/proj/src/main.rs')).toBe('src/main.rs')
  })
  it('leaves other paths as they are', () => {
    expect(relPath('C:\\w\\code', 'C:\\w\\code2\\x.go')).toBe('C:\\w\\code2\\x.go')
    expect(relPath('/home/me/proj', '/home/me/Proj/x')).toBe('/home/me/Proj/x') // Unix paths keep case
    expect(relPath('', '/x')).toBe('/x')
  })
})

describe('splitPath', () => {
  it('splits folder and name', () => {
    expect(splitPath('internal/adapter/parser.go')).toEqual({ dir: 'internal/adapter/', name: 'parser.go' })
    expect(splitPath('README.md')).toEqual({ dir: '', name: 'README.md' })
    expect(splitPath('C:\\x\\y.txt')).toEqual({ dir: 'C:\\x\\', name: 'y.txt' })
  })
})

describe('editorLabel', () => {
  const eds: Editor[] = [
    { id: 'vscode', label: 'VS Code', command: 'code', found: false },
    { id: 'cursor', label: 'Cursor', command: 'cursor', found: true },
  ]
  it('follows the choice', () => {
    expect(editorLabel(undefined, eds)).toBe('Cursor')
    expect(editorLabel({ editor: 'auto' }, eds)).toBe('Cursor')
    expect(editorLabel({ editor: 'vscode' }, eds)).toBe('VS Code')
    expect(editorLabel({ editor: 'custom', editorCommand: '"subl" {file}:{line}' }, eds)).toBe('subl')
    expect(editorLabel({ editor: 'auto' }, [])).toBe('')
  })
})

describe('canOpenDefault', () => {
  it('opens documents, never programs', () => {
    expect(canOpenDefault('C:\\a\\report.HTML')).toBe(true)
    expect(canOpenDefault('notes.md')).toBe(true)
    expect(canOpenDefault('setup.exe')).toBe(false)
    expect(canOpenDefault('run.cmd')).toBe(false)
    expect(canOpenDefault('.bashrc')).toBe(false)
  })
})
