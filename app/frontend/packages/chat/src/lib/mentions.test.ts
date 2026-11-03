import { describe, expect, it } from 'vitest'
import { insertMention, mentionQuery } from './mentions'

describe('mentionQuery', () => {
  it('finds an @ and its word at the cursor, at the start or after a space', () => {
    expect(mentionQuery('@off', 4)).toEqual({ start: 0, query: 'off' })
    expect(mentionQuery('see @Off', 8)).toEqual({ start: 4, query: 'Off' })
    expect(mentionQuery('see @', 5)).toEqual({ start: 4, query: '' })
  })

  it('isn’t a mention in an address, after a space, or before the cursor’s word', () => {
    expect(mentionQuery('me@example.com', 14)).toBeNull()
    expect(mentionQuery('@off line', 9)).toBeNull()
    expect(mentionQuery('@off and more', 2)).toEqual({ start: 0, query: 'o' })
  })
})

describe('insertMention', () => {
  it('puts the host’s text where the mention was, with a space, and the cursor after it', () => {
    const at = mentionQuery('see @Off please', 8)!
    expect(insertMention('see @Off please', at, '[Offline cards](bruv:card:42)')).toEqual({
      text: 'see [Offline cards](bruv:card:42) please',
      caret: 'see [Offline cards](bruv:card:42)'.length,
    })
    const end = mentionQuery('@Off', 4)!
    expect(insertMention('@Off', end, 'X')).toEqual({ text: 'X ', caret: 2 })
  })
})
