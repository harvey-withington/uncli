import { beforeEach, describe, expect, it } from 'vitest'
import { atEnd, loadDraft, loadFollow, loadLastSession, saveDraft, saveFollow, saveLastSession } from './keep'

describe('what is kept between runs', () => {
  beforeEach(() => localStorage.clear())

  it('keeps each session’s draft apart, and forgets an empty one', () => {
    saveDraft('a', 'half a thought')
    saveDraft('b', 'another')
    expect(loadDraft('a')).toBe('half a thought')
    expect(loadDraft('b')).toBe('another')
    saveDraft('a', '')
    expect(loadDraft('a')).toBe('')
    expect(localStorage.getItem('uncli-draft:a')).toBeNull()
  })

  it('remembers the open session and the follow choice', () => {
    expect(loadLastSession()).toBe('')
    saveLastSession('s1')
    expect(loadLastSession()).toBe('s1')
    expect(loadFollow()).toBe(false)
    saveFollow(true)
    expect(loadFollow()).toBe(true)
  })

  it('knows when the end is showing', () => {
    expect(atEnd({ scrollHeight: 1000, scrollTop: 600, clientHeight: 400 })).toBe(true)
    expect(atEnd({ scrollHeight: 1000, scrollTop: 590, clientHeight: 400 })).toBe(false)
  })
})
