import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte'
import { describe, expect, it, vi } from 'vitest'
import { mockBackend } from '@uncli/client/mock'
import { CHAT_CONTEXT } from '../lib/context'
import { ChatStore } from '../stores/chat.svelte'
import SessionPane from './SessionPane.svelte'

// The chat without the app (decision 0026): a session view with only a
// ChatStore, as an embedded element will have.
describe('the chat on its own', () => {
  it('shows a session, sends, and leaves out what only the app can open', async () => {
    localStorage.clear()
    const backend = mockBackend()
    const chat = new ChatStore(backend)
    await chat.init()
    await chat.select('s-code')
    const send = vi.spyOn(backend, 'send')
    render(SessionPane, { props: { session: chat.current! }, context: new Map([[CHAT_CONTEXT, chat]]) })

    expect(await screen.findByText('Why does TestMultiTurnPartial fail about one run in five?')).toBeInTheDocument()
    // Settings and the usage dashboard are the app's: their buttons don't show.
    expect(screen.queryByRole('button', { name: 'Safe and unsafe' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /remote access/i })).not.toBeInTheDocument()

    const box = screen.getByRole('textbox')
    await fireEvent.input(box, { target: { value: 'One more question' } })
    await fireEvent.keyDown(box, { key: 'Enter' })
    await waitFor(() => expect(send).toHaveBeenCalledWith('s-code', 'One more question', []))
    expect(await screen.findByText('One more question')).toBeInTheDocument()
    expect(within(screen.getByRole('navigation', { name: 'Pages' })).getByText('Page 2 of 2')).toBeInTheDocument()
    chat.destroy()
  })
})
