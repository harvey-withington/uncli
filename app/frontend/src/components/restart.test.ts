import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte'
import { describe, expect, it, vi } from 'vitest'
import App from '../App.svelte'
import { mockBackend } from '../lib/api/mock'

async function app() {
  localStorage.clear()
  const backend = mockBackend()
  render(App, { props: { backend } })
  await screen.findByPlaceholderText(/Ask anything/)
  return backend
}

describe('Restart', () => {
  it('restarts the CLI of an idle session at once', async () => {
    const backend = await app()
    const restart = vi.spyOn(backend, 'restart')
    await fireEvent.click(screen.getByRole('button', { name: 'Restart' }))
    await waitFor(() => expect(restart).toHaveBeenCalledTimes(1))
    expect(await screen.findByText(/restarted\. The conversation carries on\./)).toBeInTheDocument()
  })

  it('asks first while an answer is in progress, and stops it', async () => {
    const backend = await app()
    const restart = vi.spyOn(backend, 'restart')
    const box = screen.getByPlaceholderText(/Ask anything/)
    await fireEvent.input(box, { target: { value: 'Tell me a long story' } })
    await fireEvent.keyDown(box, { key: 'Enter' })
    await fireEvent.click(await screen.findByRole('button', { name: 'Restart' }))
    const dialog = await screen.findByRole('dialog', { name: "Restart while it's answering?" })
    expect(restart).not.toHaveBeenCalled()
    await fireEvent.click(within(dialog).getByRole('button', { name: 'Restart' }))
    await waitFor(() => expect(restart).toHaveBeenCalledTimes(1))
  })
})
