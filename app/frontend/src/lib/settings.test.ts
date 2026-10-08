import { afterEach, describe, expect, it } from 'vitest'
import { loadSettingsTab, nextTab, saveSettingsTab, tabOf, tabsFor } from './settings'

describe('settings tabs', () => {
  afterEach(() => localStorage.clear())

  it('opens each section on its tab', () => {
    expect(tabOf('safe')).toBe('approvals')
    expect(tabOf('decider')).toBe('approvals')
    expect(tabOf('notify')).toBe('general')
    expect(tabOf('cli')).toBe('providers')
    expect(tabOf('containers')).toBe('containers')
    expect(tabOf('provider-antigravity')).toBe('providers')
    expect(tabOf('nope')).toBeNull()
  })

  it('shows Containers only on Windows', () => {
    expect(tabsFor('windows')).toEqual(['general', 'approvals', 'providers', 'containers'])
    expect(tabsFor('darwin')).toEqual(['general', 'approvals', 'providers'])
  })

  it('remembers the last tab, if it is shown', () => {
    expect(loadSettingsTab(tabsFor('windows'))).toBe('general')
    saveSettingsTab('containers')
    expect(loadSettingsTab(tabsFor('windows'))).toBe('containers')
    expect(loadSettingsTab(tabsFor('darwin'))).toBe('general')
  })

  it('moves with the arrow keys, Home and End, wrapping', () => {
    const tabs = tabsFor('windows')
    expect(nextTab(tabs, 'general', 'ArrowDown')).toBe('approvals')
    expect(nextTab(tabs, 'general', 'ArrowUp')).toBe('containers')
    expect(nextTab(tabs, 'approvals', 'End')).toBe('containers')
    expect(nextTab(tabs, 'containers', 'Home')).toBe('general')
    expect(nextTab(tabs, 'general', 'ArrowLeft')).toBeNull()
  })
})
