import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte'
import { describe, expect, it, vi } from 'vitest'
import App from '../App.svelte'
import { mockBackend } from '../lib/api/mock'
import type { Handlers } from '../lib/api'

async function openRemote(backend = mockBackend()) {
  render(App, { props: { backend } })
  await fireEvent.click(await screen.findByRole('button', { name: /Claude Code 2./ }))
  const dialog = await screen.findByRole('dialog', { name: 'Settings' })
  await fireEvent.click(within(dialog).getByRole('tab', { name: 'Remote access' }))
  return dialog
}

describe('Settings → Remote access', () => {
  it('turns serving on and shows the address to open', async () => {
    const backend = mockBackend()
    const set = vi.spyOn(backend, 'setRemoteAccess')
    const dialog = await openRemote(backend)
    expect(await within(dialog).findByText('Off. Nothing listens.')).toBeInTheDocument()
    expect(within(dialog).getByRole('textbox', { name: 'Port' })).toHaveValue('9871')
    await fireEvent.click(within(dialog).getByRole('checkbox', { name: 'Serve sessions to paired devices' }))
    await waitFor(() => expect(set).toHaveBeenCalledWith(true, 9871))
    expect(await within(dialog).findByText('On. Open this address on a paired device:')).toBeInTheDocument()
    expect(within(dialog).getByText('https://deviant.tail2ebd58.ts.net:9871')).toBeInTheDocument()
    expect(within(dialog).getByText('On this PC it’s also at http://127.0.0.1:9871.')).toBeInTheDocument()
  })

  it('refuses a port that cannot be used', async () => {
    const dialog = await openRemote()
    const port = await within(dialog).findByRole('textbox', { name: 'Port' })
    await waitFor(() => expect(port).toHaveValue('9871'))
    await fireEvent.input(port, { target: { value: '70000' } })
    expect(within(dialog).getByText('Choose a port from 1 to 65535.')).toBeInTheDocument()
    expect(within(dialog).getByRole('checkbox', { name: 'Serve sessions to paired devices' })).toBeDisabled()
  })

  it('pairs a device only with Allow here', async () => {
    const backend = mockBackend()
    let handlers: Handlers | null = null
    const subscribe = backend.subscribe.bind(backend)
    backend.subscribe = h => { handlers = h; return subscribe(h) }
    const answer = vi.spyOn(backend, 'answerPairing').mockResolvedValue()
    const dialog = await openRemote(backend)
    const pair = await within(dialog).findByRole('button', { name: 'Pair a device' })
    expect(pair).toBeDisabled()
    expect(within(dialog).getByText('Turn Remote access on, with Tailscale connected, to pair a device.')).toBeInTheDocument()
    await fireEvent.click(within(dialog).getByRole('checkbox', { name: 'Serve sessions to paired devices' }))
    await waitFor(() => expect(pair).toBeEnabled())
    await fireEvent.click(pair)
    const pd = await screen.findByRole('dialog', { name: 'Pair a device' })
    expect(await within(pd).findByText('https://deviant.tail2ebd58.ts.net:9871/m/#pair=K7QWM3XRP2')).toBeInTheDocument()
    expect(within(pd).getByRole('img', { name: 'QR code of the pairing link' })).toBeInTheDocument()
    expect(within(pd).getByText(/The code expires in [45]:\d\d\./)).toBeInTheDocument()

    handlers!.pairRequest?.({ id: 'r1', name: 'Pixel 9' })
    const ask = await screen.findByRole('dialog', { name: 'Allow Pixel 9?' })
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Pair a device' })).toBeNull())
    expect(within(ask).getByRole('button', { name: 'Deny' })).toHaveFocus()
    await fireEvent.click(within(ask).getByRole('button', { name: 'Allow' }))
    await waitFor(() => expect(answer).toHaveBeenCalledWith('r1', true))
  })

  it('lists paired devices and revokes one after confirming', async () => {
    const backend = mockBackend()
    vi.spyOn(backend, 'pairedDevices').mockResolvedValue([
      { id: 'd1', name: 'Pixel 9', paired: '2026-10-20T10:00:00Z', lastSeen: new Date(Date.now() - 2 * 3600_000).toISOString() },
    ])
    const revoke = vi.spyOn(backend, 'revokeDevice')
    const dialog = await openRemote(backend)
    expect(await within(dialog).findByText('Pixel 9')).toBeInTheDocument()
    expect(within(dialog).getByText(/last seen 2 h/)).toBeInTheDocument()
    await fireEvent.click(within(dialog).getByRole('button', { name: 'Revoke' }))
    const sure = await screen.findByRole('dialog', { name: 'Revoke Pixel 9?' })
    await fireEvent.click(within(sure).getByRole('button', { name: 'Revoke' }))
    await waitFor(() => expect(revoke).toHaveBeenCalledWith('d1'))
    expect(await within(dialog).findByText('No devices yet. Pair one to reach your sessions from it.')).toBeInTheDocument()
  })
})
