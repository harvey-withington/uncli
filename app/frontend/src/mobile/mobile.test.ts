import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { serverKey, withServer } from '../lib/api/identity'
import { mockBackend } from '../lib/api/mock'
import type { Backend, Handlers } from '../lib/api'
import MobileApp from './MobileApp.svelte'

const SERVER = serverKey(location.origin) // the app is this page's server's

// The phone app on the mock, as the server's sessions (keys qualified as
// that server's) and, for what only remote clients get, as an HTTP client.
function phoneBackend(): Backend {
  return { ...withServer(mockBackend(), SERVER), kind: 'http' }
}

afterEach(() => {
  history.replaceState(null, '', '/')
  localStorage.clear()
})

describe('the phone app', () => {
  it('explains how to pair when it has no token and no code', () => {
    render(MobileApp)
    expect(screen.getByRole('heading', { name: 'Pair with UNCLI' })).toBeInTheDocument()
    expect(screen.getByText(/Click Pair a device, and scan the QR code/)).toBeInTheDocument()
  })

  it('takes a code typed in when the link lost it', async () => {
    render(MobileApp)
    await fireEvent.input(screen.getByRole('textbox', { name: /type the code/ }), { target: { value: 'bc3yj-64ac7' } })
    await fireEvent.click(screen.getByRole('button', { name: 'Use this code' }))
    expect(await screen.findByRole('button', { name: 'Pair this device' })).toBeEnabled()
  })

  it('takes the code from a pairing link opened in a tab already showing the app', async () => {
    render(MobileApp)
    history.replaceState(null, '', '/m/#pair=K7QWM3XRP2')
    dispatchEvent(new HashChangeEvent('hashchange'))
    expect(await screen.findByRole('button', { name: 'Pair this device' })).toBeEnabled()
  })

  it('carries on waiting for Allow after a reload', async () => {
    localStorage.setItem('uncli-pair-pending', JSON.stringify({ id: 'r1', claim: 'c1', at: Date.now() }))
    const f = vi.spyOn(globalThis, 'fetch').mockImplementation(() => new Promise<Response>(() => {})) // the server is still waiting
    render(MobileApp)
    expect(await screen.findByText(/Waiting for you to allow it/)).toBeInTheDocument()
    await waitFor(() => expect(f).toHaveBeenCalledWith('/pair/claim', expect.anything()))
    f.mockRestore()
  })

  it('asks for a name when opened from a pairing link', () => {
    history.replaceState(null, '', '/m/#pair=K7QWM3XRP2')
    render(MobileApp)
    expect(screen.getByRole('textbox', { name: 'This phone’s name' })).toHaveValue('Phone')
    expect(screen.getByRole('button', { name: 'Pair this device' })).toBeEnabled()
  })

  it('lists the sessions, opens one, and goes back', async () => {
    render(MobileApp, { props: { backend: phoneBackend() } })
    const list = await screen.findByRole('list', { name: 'Sessions' })
    const first = within(list).getAllByRole('button', { name: /Fix the flaky parser test/ })[0]!
    await fireEvent.click(first)
    expect(await screen.findByRole('heading', { name: 'Fix the flaky parser test' })).toBeInTheDocument()
    expect(location.hash).toContain(encodeURIComponent(`${SERVER}~`))
    // Away from the server, a photo or file is sent to it first.
    expect(screen.getByRole('button', { name: 'Attach a photo or file' })).toBeInTheDocument()
    await fireEvent.click(screen.getByRole('button', { name: 'Back' }))
    await waitFor(() => expect(screen.getByRole('list', { name: 'Sessions' })).toBeInTheDocument())
  })

  it('opens the session a notification links to', async () => {
    const b = phoneBackend()
    const id = (await b.bootstrap()).sessions[1]!.id
    history.replaceState(null, '', `/m/#s=${encodeURIComponent(id.split('~')[1]!)}`)
    render(MobileApp, { props: { backend: b } })
    expect(await screen.findByRole('button', { name: 'Back' })).toBeInTheDocument()
  })

  it('says when the server can\u2019t be reached', async () => {
    const b = phoneBackend()
    let h: Handlers | null = null
    const sub = b.subscribe.bind(b)
    b.subscribe = x => { h = x; return sub(x) }
    render(MobileApp, { props: { backend: b } })
    await screen.findByRole('list', { name: 'Sessions' })
    h!.connection?.({ state: 'offline', lastOnline: Date.now() - 5 * 60_000, error: 'Failed to fetch' })
    await waitFor(() => expect(document.querySelector('.banner')?.textContent).toMatch(/Can’t reach .*\(Failed to fetch\).*Last connected 5 min ago\..*Trying again/s))
  })

  it('picks a session folder by browsing the server', async () => {
    const b = phoneBackend()
    const list = vi.spyOn(b, 'listFolder')
    render(MobileApp, { props: { backend: b } })
    await fireEvent.click(await screen.findByRole('button', { name: 'New session' }))
    const dialog = await screen.findByRole('dialog', { name: 'New session' })
    await fireEvent.click(within(dialog).getByRole('radio', { name: /Code/ }))
    await fireEvent.click(within(dialog).getByRole('button', { name: /Choose/ }))
    const browser = await screen.findByRole('dialog', { name: /Choose the repository/ })
    await waitFor(() => expect(list).toHaveBeenCalled())
    await fireEvent.click(await within(browser).findByRole('button', { name: /C:\\Users\\you/ }))
    await fireEvent.click(await within(browser).findByRole('button', { name: /code/ }))
    await waitFor(() => expect(within(browser).getByText('C:\\Users\\you\\code')).toBeInTheDocument())
    await fireEvent.click(within(browser).getByRole('button', { name: 'Use this folder' }))
    await waitFor(() => expect(within(dialog).getByText('C:\\Users\\you\\code')).toBeInTheDocument())
  })
})
