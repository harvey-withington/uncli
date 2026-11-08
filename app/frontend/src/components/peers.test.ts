import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte'
import { afterEach, beforeEach, describe, expect, it, vi, type MockInstance } from 'vitest'
import App from '../App.svelte'
import { combinedBackend, type Backend } from '@uncli/client'
import { mockBackend } from '@uncli/client/mock'

// The window as the app runs it: this PC's mock, paired (?peers=on) with
// ripped (online, sessions of its own) and studio (offline).
function withPeers(spy?: (b: Backend) => void) {
  const base = mockBackend()
  spy?.(base)
  return combinedBackend(base)
}

describe('Remote servers (decision 0031)', () => {
  beforeEach(() => history.replaceState(null, '', '/?peers=on'))
  afterEach(() => history.replaceState(null, '', '/'))

  it('groups the sidebar by server, and says why one is offline', async () => {
    render(App, { props: { backend: withPeers() } })
    const ripped = await screen.findByRole('region', { name: 'Sessions on ripped' })
    expect(await within(ripped).findByText('Nightly build keeps timing out')).toBeInTheDocument()
    expect(screen.getByText('This PC')).toBeInTheDocument()
    const studio = screen.getByRole('region', { name: 'Sessions on studio' })
    expect(within(studio).getByText("Can't reach studio: connection timed out", { selector: 'p' })).toBeInTheDocument()
  })

  it('disables what needs the session’s own machine, with the reason', async () => {
    render(App, { props: { backend: withPeers() } })
    await fireEvent.click(await screen.findByText('Nightly build keeps timing out'))
    const open = await screen.findByRole('button', { name: /Open folder/ })
    await waitFor(() => expect(open).toHaveAttribute('aria-disabled', 'true'))
    expect(open).toHaveAttribute('title', 'Only on the PC that runs this session')
  })

  it('lists paired servers in Settings, and removes one after asking', async () => {
    let remove: MockInstance<Backend['removeRemoteServer']> | undefined
    render(App, { props: { backend: withPeers(b => (remove = vi.spyOn(b, 'removeRemoteServer'))) } })
    await fireEvent.click(await screen.findByRole('button', { name: /Claude Code 2./ }))
    const dialog = await screen.findByRole('dialog', { name: 'Settings' })
    await fireEvent.click(within(dialog).getByRole('tab', { name: 'Remote access' }))
    const list = await within(dialog).findByRole('list', { name: 'Paired servers' })
    expect(within(list).getByText('Connected')).toBeInTheDocument()
    await fireEvent.click(within(list).getAllByRole('button', { name: 'Remove' })[1]!)
    const ask = await screen.findByRole('dialog', { name: 'Remove studio?' })
    await fireEvent.click(within(ask).getByRole('button', { name: 'Remove' }))
    await waitFor(() => expect(remove).toHaveBeenCalledWith('studio.tail2ebd58.ts.net:9871'))
  })
})
