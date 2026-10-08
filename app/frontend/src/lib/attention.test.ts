import { describe, expect, it } from 'vitest'
import type { ContainerInfo, ContainersInfo } from './api'
import { needsAttention, settingsAttention } from './attention'

const box = (over: Partial<ContainerInfo> = {}): ContainerInfo => ({
  id: 'sandbox', label: 'Sandbox', base: 'alpine-3.24', baseLabel: 'Alpine', packages: [], setup: [], builtin: true, edited: false, built: true, ...over,
})
const info = (over: Partial<ContainersInfo> = {}): ContainersInfo => ({
  wsl: { installed: true, distros: ['uncli-sandbox'] }, signedIn: true, accountSignedIn: true, containers: [box()], bases: [], ...over,
})

describe('settings attention', () => {
  it('is quiet when all is well, or there is nothing to know yet', () => {
    expect(needsAttention(settingsAttention(info()))).toBe(false)
    expect(needsAttention(settingsAttention(null))).toBe(false)
    // WSL not installed is a choice, not a problem
    expect(needsAttention(settingsAttention(info({ wsl: { installed: false, distros: [] }, containers: [box({ built: false })] })))).toBe(false)
  })

  it('points at the tab that needs the user', () => {
    expect(settingsAttention(info({ containers: [box({ changes: ['unknown'] })] }))).toEqual({ containers: ['attention.rebuild'] })
    expect(settingsAttention(info({ containers: [box({ built: false, error: 'apk: network down' })] }))).toEqual({ containers: ['attention.buildFailed'] })
    expect(settingsAttention(info({ wsl: { installed: true, distros: [], unresponsive: true } }))).toEqual({ containers: ['attention.wslStuck'] })
    expect(settingsAttention(info({ signedIn: false, accountSignedIn: false }))).toEqual({ providers: ['attention.containerSignIn'] })
    expect(settingsAttention(info({ accountSignedIn: false, containers: [box({ connectors: 'shared' })] }))).toEqual({ providers: ['attention.accountSignIn'] })
  })

  it('waits while a container is being built', () => {
    expect(needsAttention(settingsAttention(info({ containers: [box({ step: 'packages', changes: ['packages'] })] })))).toBe(false)
  })
})
