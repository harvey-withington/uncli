import { describe, expect, it } from 'vitest'
import { complete, matchCommands, slashQuery } from './slash'

const list = [
  { name: 'compact', description: 'Compress the conversation' },
  { name: 'context', description: 'Show context usage' },
  { name: 'code-review', description: 'Review the diff' },
  { name: 'clear', description: 'Start a new conversation' },
  { name: 'my:compactor', description: 'A plugin command' },
]

describe('the slash-command menu', () => {
  it('opens while a command is being typed, and not otherwise', () => {
    expect(slashQuery('/', 1)).toBe('')
    expect(slashQuery('/com', 4)).toBe('com')
    expect(slashQuery('/compact now', 12)).toBeNull() // past the command
    expect(slashQuery('/compact now', 3)).toBeNull() // already has more after it
    expect(slashQuery('hello /com', 10)).toBeNull()
    expect(slashQuery('/usr/bin', 8)).toBeNull()
  })

  it('puts names that start with the query first, then the ones that mention it', () => {
    expect(matchCommands(list, 'co').map(c => c.name)).toEqual(['code-review', 'compact', 'context', 'clear', 'my:compactor'])
    expect(matchCommands(list, 'compact').map(c => c.name)).toEqual(['compact', 'my:compactor'])
    expect(matchCommands(list, 'review').map(c => c.name)).toEqual(['code-review'])
    expect(matchCommands(list, '').length).toBe(5)
    expect(matchCommands(list, '', 2).length).toBe(2)
  })

  it('completes the command and keeps what follows', () => {
    expect(complete('/com', 'compact')).toBe('/compact ')
    expect(complete('/', 'clear')).toBe('/clear ')
  })
})
