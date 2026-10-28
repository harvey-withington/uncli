import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte'
import { afterEach, describe, expect, it } from 'vitest'
import { serverKey, withServer } from '../lib/api/identity'
import { mockBackend } from '../lib/api/mock'
import MobileApp from './MobileApp.svelte'
import { wide } from './wide.svelte'

// On a tablet (900 px or wider) the phone app shows the session list beside
// the open session, and the side panel beside the answer, not over it. The
// tests have no screen, so it's switched on by hand (wide.svelte.ts).
afterEach(() => {
  wide.on = false
  history.replaceState(null, '', '/')
  localStorage.clear()
})

describe('the phone app on a tablet', () => {
  it('keeps the list beside the open session, and the side panel beside the answer', async () => {
    wide.on = true
    const backend = { ...withServer(mockBackend(), serverKey(location.origin)), kind: 'http' as const }
    render(MobileApp, { props: { backend } })
    const list = await screen.findByRole('list', { name: 'Sessions' })
    expect(screen.getByText('Choose a session from the list, or start a new one.')).toBeInTheDocument()
    await fireEvent.click(within(list).getAllByRole('button', { name: /Fix the flaky parser test/ })[0]!)
    expect(await screen.findByRole('heading', { name: 'Fix the flaky parser test' })).toBeInTheDocument()
    // The list is still there, beside the session.
    expect(screen.getByRole('list', { name: 'Sessions' })).toBeInTheDocument()
    await fireEvent.click(screen.getByRole('button', { name: /side panel/i }))
    await waitFor(() => expect(screen.getByRole('tab', { name: /On this page/ })).toBeInTheDocument())
    // Beside, not over: no dialog, no Close button of its own.
    expect(screen.queryByRole('dialog', { name: /side panel/i })).toBeNull()
    expect(document.querySelector('.panel .close')).toBeNull()
  })
})
