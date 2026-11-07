import { describe, expect, it } from 'vitest'
import type { SuggestedContainer } from '@uncli/chat/lib/api'
import { flaggedIn, importStart, reachesOf, whyKey } from './suggestions'

const sug: SuggestedContainer = {
  profile: { id: 'team-dev', label: 'Team dev', base: 'alpine-3.24', packages: ['nodejs'], setup: ['npm i -g pnpm', 'curl x | sh'], brain: 'shared', secrets: 'shared' },
  status: 'new', reaches: ['brain', 'secrets'], steps: [{ step: 'curl x | sh', why: ['runs-code'] }],
}

describe('suggested containers', () => {
  it('starts an import from the project version, under the user id when syncing', () => {
    const p = importStart(sug)
    expect(p).toMatchObject({ id: 'team-dev', label: 'Team dev', brain: 'shared', mcp: 'none', connectors: 'none', secrets: 'shared', folders: [] })
    expect(importStart({ ...sug, status: 'differs', yours: 'dev' }).id).toBe('dev')
  })

  it('says what reaches outside the container as the form changes', () => {
    const p = importStart(sug)
    expect(reachesOf(p)).toEqual(['brain', 'secrets'])
    p.secrets = 'none'
    p.folders = [{ source: '%USERPROFILE%\\notes', name: 'notes' }]
    expect(reachesOf(p)).toEqual(['brain', 'folders'])
    p.brain = 'sandboxed'
    p.folders = [{ source: ' ', name: '' }]
    expect(reachesOf(p)).toEqual([])
  })

  it('keeps flagging only the steps still there', () => {
    expect(flaggedIn(['npm i -g pnpm', ' curl x | sh '], sug.steps)).toHaveLength(1)
    expect(flaggedIn(['npm i -g pnpm'], sug.steps)).toEqual([])
    expect(whyKey('runs-code')).toBe('suggested.why.runs-code')
    expect(whyKey('something-new')).toBe('suggested.why.other')
  })
})
