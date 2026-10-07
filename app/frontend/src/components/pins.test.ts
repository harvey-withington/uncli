import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte'
import { describe, expect, it } from 'vitest'
import App from '../App.svelte'
import { mockBackend } from '../lib/api/mock'

async function app() {
  localStorage.clear()
  render(App, { props: { backend: mockBackend() } })
  await screen.findByRole('list', { name: 'Pinned' })
}

describe('Pins', () => {
  it('lists pinned pages across sessions, opens one, and pins with P', async () => {
    await app()
    const pinned = screen.getByRole('list', { name: 'Pinned' })
    // Newest first: the Q3 notes page, then a page of an archived session.
    const items = within(pinned).getAllByRole('listitem')
    expect(items.map(li => li.querySelector('.q')?.textContent)).toEqual([
      'What are the three decisions in these notes?', 'Compare these three standing desks for a small flat.',
    ])
    expect(items[1]?.querySelector('.where')?.textContent).toBe('Compare three standing desks · archived')

    await fireEvent.click(within(pinned).getByText('What are the three decisions in these notes?'))
    expect(await screen.findByRole('heading', { name: 'Summarise the Q3 planning notes' })).toBeInTheDocument()
    await waitFor(() => expect(screen.getByRole('button', { name: 'Pin this page (P)' })).toHaveAttribute('aria-pressed', 'true'))
    expect(screen.getByText('What are the three decisions in these notes?', { selector: '.q-text' })).toBeInTheDocument() // page 1, the pinned one

    // P unpins the page shown; it leaves the group.
    await fireEvent.keyDown(window, { key: 'p' })
    await waitFor(() => expect(within(screen.getByRole('list', { name: 'Pinned' })).getAllByRole('listitem')).toHaveLength(1))
    expect(screen.getByRole('button', { name: 'Pin this page (P)' })).toHaveAttribute('aria-pressed', 'false')
  })

  it('unpins from the sidebar', async () => {
    await app()
    await fireEvent.click(screen.getByRole('button', { name: /^Unpin “Compare these three/ }))
    await waitFor(() => expect(within(screen.getByRole('list', { name: 'Pinned' })).getAllByRole('listitem')).toHaveLength(1))
  })
})

describe('Archive', () => {
  it('archives a session out of the list, reads it read-only, and restores it', async () => {
    await app()
    const sessions = screen.getByRole('navigation', { name: /sessions/i })
    const row = within(sessions).getByText('Plan a weekend in Lisbon').closest('li') as HTMLElement
    await fireEvent.click(within(row).getByRole('button', { name: 'Archive' }))
    expect(await screen.findByText(/Archived “Plan a weekend in Lisbon”/)).toBeInTheDocument()
    const toggle = screen.getByRole('button', { name: 'Archived (2)' })
    expect(toggle).toHaveAttribute('aria-expanded', 'false')
    expect(within(sessions).queryByText('Plan a weekend in Lisbon')).not.toBeInTheDocument()

    await fireEvent.click(toggle)
    const archived = screen.getByRole('list', { name: 'Archived sessions' })
    await fireEvent.click(within(archived).getByText('Plan a weekend in Lisbon'))
    const note = (await screen.findByText('This session is archived. Restore it to carry on the conversation.')).closest('[role=status]') as HTMLElement
    expect(screen.queryByPlaceholderText(/Ask anything/)).not.toBeInTheDocument()

    await fireEvent.click(within(note).getByRole('button', { name: 'Restore' }))
    expect(await screen.findByPlaceholderText(/Ask anything/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Archived (1)' })).toBeInTheDocument()
  })
})
