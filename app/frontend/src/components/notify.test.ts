import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte'
import { describe, expect, it, vi } from 'vitest'
import App from '../App.svelte'
import { mockBackend } from '../lib/api/mock'
import type { Handlers } from '../lib/api'
import { AppStore } from '../stores/app.svelte'

describe('Notifications', () => {
  it('Settings saves which notifications to show and shows a test', async () => {
    const backend = mockBackend()
    const save = vi.spyOn(backend, 'setPreferences')
    const test = vi.spyOn(backend, 'testNotification')
    render(App, { props: { backend } })
    await fireEvent.click(await screen.findByRole('button', { name: /Claude CLI/ }))
    const dialog = await screen.findByRole('dialog', { name: 'Settings' })
    const select = within(dialog).getByRole('combobox', { name: 'Notifications' })
    expect(select).toHaveValue('all')
    expect(within(select).getAllByRole('option').map(o => o.textContent)).toEqual([
      'When a session finishes or needs approval', 'Only when a session needs approval', 'Off',
    ])
    await fireEvent.click(within(dialog).getByRole('button', { name: 'Show a test' }))
    expect(test).toHaveBeenCalled()
    await fireEvent.change(select, { target: { value: 'off' } })
    await waitFor(() => expect(save).toHaveBeenLastCalledWith(expect.objectContaining({ notifications: 'off' })))
    expect(within(dialog).getByRole('button', { name: 'Show a test' })).toBeDisabled()
  })

  it('a clicked notification opens its session on the latest page', async () => {
    const backend = mockBackend()
    let handlers: Handlers | null = null
    const subscribe = backend.subscribe.bind(backend)
    backend.subscribe = h => { handlers = h; return subscribe(h) }
    const s = new AppStore(backend)
    await s.init()
    const other = s.sessions.find(x => x.id !== s.currentId)
    expect(other).toBeTruthy()
    handlers!.notifyOpen?.(other!.id)
    await waitFor(() => expect(s.currentId).toBe(other!.id))
    expect(s.currentIndex).toBe(Math.max(0, s.currentPages.length - 1))
    handlers!.notifyOpen?.('') // the tray icon: stays where it is
    expect(s.currentId).toBe(other!.id)
  })
})
