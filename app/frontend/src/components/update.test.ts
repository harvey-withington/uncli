import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte'
import { afterEach, describe, expect, it, vi } from 'vitest'
import App from '../App.svelte'
import type { Handlers } from '../lib/api'
import { serverKey, withServer } from '../lib/api/identity'
import { mockBackend } from '../lib/api/mock'
import MobileApp from '../mobile/MobileApp.svelte'

// UNCLI's own updates (decision 0025), on the browser mock: ?update= picks
// the scenario (lib/api/mock-update.ts).
function scenario(s: string) {
  history.replaceState(null, '', `/?update=${s}`)
}

afterEach(() => {
  history.replaceState(null, '', '/')
  localStorage.clear()
})

async function openUpdates() {
  await fireEvent.click(await screen.findByRole('button', { name: /Claude Code 2./ }))
  const dialog = await screen.findByRole('dialog', { name: 'Settings' })
  return within(dialog).getByRole('heading', { name: 'Updates' }).closest('section') as HTMLElement
}

describe('updates', () => {
  it('offer a newer release in the sidebar, ask first, then download and hand it to the installer', async () => {
    scenario('available')
    const backend = mockBackend()
    const install = vi.spyOn(backend, 'installUpdate')
    render(App, { props: { backend } })
    expect(await screen.findByText('UNCLI 0.4.0-alpha is available.')).toBeInTheDocument()
    await fireEvent.click(screen.getByRole('button', { name: 'Update now' }))
    const ask = await screen.findByRole('dialog', { name: 'Update to UNCLI 0.4.0-alpha?' })
    expect(within(ask).getByText(/quits, installs it and starts again/)).toBeInTheDocument()
    await fireEvent.click(within(ask).getByRole('button', { name: 'Update now' }))
    await waitFor(() => expect(install).toHaveBeenCalled())
    expect(await screen.findByText('Installing UNCLI 0.4.0-alpha. UNCLI restarts in a moment.', {}, { timeout: 3000 })).toBeInTheDocument()
  })

  it('show nothing in the sidebar when up to date, and say so in Settings once checked', async () => {
    scenario('none')
    render(App, { props: { backend: mockBackend() } })
    await screen.findByRole('button', { name: /Claude Code 2./ })
    expect(screen.queryByText(/is available/)).toBeNull()
    const section = await openUpdates()
    expect(within(section).getByText(/This is UNCLI 0.3.0-alpha/)).toBeInTheDocument()
    expect(within(section).getByRole('checkbox', { name: 'Check for updates automatically' })).toBeChecked()
    await fireEvent.click(within(section).getByRole('button', { name: 'Check now' }))
    expect(await within(section).findByText(/UNCLI is up to date \(checked/)).toBeInTheDocument()
  })

  it('say a development build is updated another way', async () => {
    scenario('dev')
    render(App, { props: { backend: mockBackend() } })
    const section = await openUpdates()
    expect(within(section).getByText(/This is a development build. It's updated with npm run install-build/)).toBeInTheDocument()
    expect(within(section).getByRole('button', { name: 'Check now' })).toBeDisabled()
    expect(within(section).getByRole('checkbox', { name: 'Check for updates automatically' })).toBeDisabled()
  })
})

describe('the phone’s Settings', () => {
  it('check the PC for updates now, without the PC’s own automatic box', async () => {
    scenario('none')
    const base = mockBackend()
    const check = vi.spyOn(base, 'checkForUpdates')
    const backend = { ...withServer(base, serverKey(location.origin)), kind: 'http' as const }
    render(MobileApp, { props: { backend } })
    await screen.findByRole('list', { name: 'Sessions' })
    await fireEvent.click(screen.getByRole('button', { name: 'Settings' }))
    const section = (await screen.findByRole('heading', { name: 'Updates' })).closest('section') as HTMLElement
    expect(within(section).getByText(/The PC runs UNCLI 0.3.0-alpha/)).toBeInTheDocument()
    expect(within(section).queryByRole('checkbox')).toBeNull()
    await fireEvent.click(within(section).getByRole('button', { name: 'Check now' }))
    await waitFor(() => expect(check).toHaveBeenCalled())
    expect(await within(section).findByText(/UNCLI is up to date \(checked/)).toBeInTheDocument()
  })
})

describe('the phone, once the PC has updated', () => {
  it('asks the PC for its update status again when it reconnects, so Settings shows it done', async () => {
    scenario('available')
    const base = mockBackend()
    let h: Handlers | null = null
    const subscribe = base.subscribe.bind(base)
    base.subscribe = x => { h = x; return subscribe(x) }
    const backend = { ...withServer(base, serverKey(location.origin)), kind: 'http' as const }
    render(MobileApp, { props: { backend } })
    expect(await screen.findByText('UNCLI 0.4.0-alpha is available.')).toBeInTheDocument()
    // The PC restarted on 0.4.0-alpha: it has nothing newer to offer.
    vi.spyOn(backend, 'updateStatus').mockResolvedValue({ current: '0.4.0-alpha', dev: false, state: 'idle', available: false })
    h!.connection?.({ state: 'online', lastOnline: Date.now() })
    await waitFor(() => expect(screen.queryByText('UNCLI 0.4.0-alpha is available.')).toBeNull())
  })

  it('offers Reload after reconnecting to a new build', async () => {
    const base = mockBackend()
    let h: Handlers | null = null
    const subscribe = base.subscribe.bind(base)
    base.subscribe = x => { h = x; return subscribe(x) }
    const backend = { ...withServer(base, serverKey(location.origin)), kind: 'http' as const }
    let commit = 'aaa111'
    const f = vi.spyOn(globalThis, 'fetch').mockImplementation(async () =>
      new Response(JSON.stringify({ app: '0.3.0-alpha', protocol: 1, capabilities: [], commit }), { headers: { 'Content-Type': 'application/json' } }))
    try {
      localStorage.setItem('uncli-remote', JSON.stringify({ token: 't', name: 'Phone', server: location.origin }))
      render(MobileApp, { props: { backend } })
      await screen.findByRole('list', { name: 'Sessions' })
      await waitFor(() => expect(f).toHaveBeenCalledWith('/version', expect.anything()))
      h!.connection?.({ state: 'online', lastOnline: Date.now() })
      await new Promise(r => setTimeout(r, 20))
      expect(screen.queryByText('UNCLI on the PC has been updated.')).toBeNull()
      // The PC restarts on a new build; the phone reconnects.
      commit = 'bbb222'
      h!.connection?.({ state: 'offline', lastOnline: Date.now(), error: 'gone' })
      h!.connection?.({ state: 'online', lastOnline: Date.now() })
      expect(await screen.findByText('UNCLI on the PC has been updated.')).toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Reload' })).toBeInTheDocument()
    } finally {
      f.mockRestore()
    }
  })
})
