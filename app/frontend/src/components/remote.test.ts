import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte'
import { describe, expect, it, vi } from 'vitest'
import App from '../App.svelte'
import { mockBackend } from '../lib/api/mock'

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
})
