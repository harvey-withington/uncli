import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte'
import { describe, expect, it } from 'vitest'
import App from '../App.svelte'
import { mockBackend } from '../lib/api/mock'

describe('Background tasks', () => {
  it('says what runs in the background, then shows the turn Claude carries on with by itself', async () => {
    localStorage.clear()
    render(App, { props: { backend: mockBackend() } })
    const box = await screen.findByPlaceholderText(/Ask anything/)
    await fireEvent.input(box, { target: { value: 'Check the other tests with a background agent' } })
    await fireEvent.keyDown(box, { key: 'Enter' })
    expect(await screen.findByText(/Working in the background: Check the other test files/, {}, { timeout: 4000 })).toBeInTheDocument()
    const row = screen.getByRole('navigation', { name: /sessions/i }).querySelector('li[data-id="s-code"]') as HTMLElement
    expect(within(row).getByText('in the background')).toBeInTheDocument()
    // It finishes: Claude carries on, on a page of its own with no question.
    expect(await screen.findByText(/only the parser test was flaky/, {}, { timeout: 4000 })).toBeInTheDocument()
    expect(screen.getAllByText('Claude carried on by itself after a background task finished').length).toBeGreaterThan(0)
    await waitFor(() => expect(screen.queryByText(/Working in the background/)).not.toBeInTheDocument())
  }, 15000)
})
