import { fireEvent, render, screen } from '@testing-library/svelte'
import { afterEach, describe, expect, it } from 'vitest'
import App from '../App.svelte'
import { serverKey, withServer } from '@uncli/chat/lib/api'
import { mockBackend } from '@uncli/client/mock'
import MobileApp from '../mobile/MobileApp.svelte'

// UNCLI running as administrator (AdminNotice): ?admin on the browser mock.
afterEach(() => {
  history.replaceState(null, '', '/')
  localStorage.clear()
})

describe('running as administrator', () => {
  it('is said in the sidebar, with how to start UNCLI normally, until dismissed', async () => {
    history.replaceState(null, '', '/?admin')
    render(App, { props: { backend: mockBackend() } })
    expect(await screen.findByText('UNCLI is running as administrator')).toBeInTheDocument()
    expect(screen.getByText(/without Run as administrator/)).toBeInTheDocument()
    await fireEvent.click(screen.getByRole('button', { name: 'Dismiss' }))
    expect(screen.queryByText('UNCLI is running as administrator')).toBeNull()
  })

  it('is said on the phone as the PC’s', async () => {
    history.replaceState(null, '', '/?admin')
    const backend = { ...withServer(mockBackend(), serverKey(location.origin)), kind: 'http' as const }
    render(MobileApp, { props: { backend } })
    expect(await screen.findByText('UNCLI on the PC is running as administrator')).toBeInTheDocument()
  })

  it('says nothing when UNCLI runs normally', async () => {
    render(App, { props: { backend: mockBackend() } })
    await screen.findByRole('button', { name: /Claude Code 2./ })
    expect(screen.queryByText(/running as administrator/)).toBeNull()
  })
})
