import { describe, expect, it } from 'vitest'
import type { Capabilities, Profile } from './api'
import { shareableFor, sharesNote, startingShares, toggled } from './shares'

const chat = { id: 'chat', isolated: true, shares: ['tools', 'skills'] } as Profile
const code = { id: 'code', shares: [] } as unknown as Profile
const caps = { sharedTools: true, sharedSkills: true } as Capabilities

describe('what a new session can share of the user’s own', () => {
  it('is only an isolated type’s, on this machine, on a CLI that can', () => {
    expect(shareableFor(chat, caps, '')).toEqual(['tools', 'skills'])
    expect(shareableFor(code, caps, '')).toEqual([])
    expect(shareableFor(chat, caps, 'sandbox')).toEqual([])
    expect(shareableFor(chat, { sharedTools: true } as Capabilities, '')).toEqual(['tools'])
    expect(shareableFor(chat, undefined, '')).toEqual([])
  })

  it('starts from what the type’s last session shared, else its defaults', () => {
    const both = shareableFor(chat, caps, '')
    expect(startingShares(chat, undefined, both)).toEqual(['tools', 'skills'])
    expect(startingShares(chat, { models: {}, folders: {}, shares: { chat: [] } }, both)).toEqual([])
    expect(startingShares(chat, { models: {}, folders: {}, shares: { chat: ['skills'] } }, both)).toEqual(['skills'])
    expect(startingShares(chat, undefined, ['tools'])).toEqual(['tools'])
  })

  it('switches one at a time, keeping the order', () => {
    expect(toggled(['skills'], 'tools', true)).toEqual(['tools', 'skills'])
    expect(toggled(['tools', 'skills'], 'tools', false)).toEqual(['skills'])
  })

  it('says what a session has', () => {
    expect(sharesNote(['tools', 'skills'])).toBe('shares.note.both')
    expect(sharesNote(['skills'])).toBe('shares.note.skills')
    expect(sharesNote([])).toBe('shares.note.none')
  })
})
