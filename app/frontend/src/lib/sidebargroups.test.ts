import { beforeEach, describe, expect, it } from 'vitest'
import { loadGroups, saveGroups, toggled } from './sidebargroups'

describe('sidebar groups', () => {
  beforeEach(() => localStorage.clear())

  it('flips one flag of one group, leaving the rest', () => {
    let g = toggled({}, 'local', 'collapsed')
    g = toggled(g, 'ripped:9871', 'hideArchived')
    g = toggled(g, 'local', 'archivedOpen', true)
    expect(g).toEqual({ local: { collapsed: true, archivedOpen: true }, 'ripped:9871': { hideArchived: true } })
    expect(toggled(g, 'local', 'collapsed').local).toEqual({ collapsed: false, archivedOpen: true })
  })

  it('is remembered, and anything unreadable is forgotten', () => {
    saveGroups({ local: { collapsed: true } })
    expect(loadGroups()).toEqual({ local: { collapsed: true, hideArchived: false, archivedOpen: false } })
    localStorage.setItem('uncli-sidebar-groups', '[1,2]')
    expect(loadGroups()).toEqual({})
    localStorage.setItem('uncli-sidebar-groups', 'not json')
    expect(loadGroups()).toEqual({})
  })
})
