import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte'
import { describe, expect, it } from 'vitest'
import { mockBackend } from '../lib/api/mock'
import { APP_CONTEXT } from '../lib/context'
import { AppStore } from '../stores/app.svelte'
import SessionPane from './SessionPane.svelte'

async function chat() {
  localStorage.clear()
  const s = new AppStore(mockBackend())
  await s.init()
  await s.select('s-chat')
  return s
}

// The viewer's iframe (its name label carries the same title).
const frameIn = (pane: HTMLElement) => waitFor(() => {
  const f = pane.querySelector('iframe')
  if (!f) throw new Error('no iframe yet')
  return f
})

const ctx = (s: AppStore) => new Map([[APP_CONTEXT, s]])

describe('Artifact pane', () => {
  it('lists the artifacts as of the page, and paging back pages them back', async () => {
    const s = await chat()
    render(SessionPane, { props: { session: s.current! }, context: ctx(s) })
    await fireEvent.click(screen.getByRole('tab', { name: /^Artifacts/ }))
    const pane = screen.getByRole('complementary', { name: 'Artifacts' })
    expect(screen.getByRole('tab', { name: /^Artifacts/ })).toHaveAttribute('aria-selected', 'true')
    expect(within(screen.getByRole('tab', { name: /^Artifacts/ })).getByLabelText('4 artifacts')).toHaveTextContent('4')
    const list = within(pane).getByRole('list', { name: 'Artifacts as of this page' })
    await waitFor(() => expect(within(list).getAllByRole('button').map(b => b.querySelector('.path')?.textContent))
      .toEqual(['budget.svg', 'itinerary.html', 'notes.md', 'route.mmd']))
    expect(within(pane).getByText('after page 2')).toBeInTheDocument()
    // Page 2 changed two of them; notes.md is only in the folder.
    expect(within(list).getAllByText('this page')).toHaveLength(2)

    // HTML runs in a sandbox with no network.
    await fireEvent.click(within(list).getByText('itinerary.html'))
    const frame = await frameIn(pane)
    expect(frame).toHaveAttribute('sandbox', 'allow-scripts')
    expect(frame.getAttribute('srcdoc')).toContain("default-src 'none'")
    expect(frame.getAttribute('srcdoc')).toContain('printable')
    expect(within(pane).getByText('as page 2 left it')).toBeInTheDocument()

    // Back to page 1: its own version, and only what existed then.
    s.goTo(0)
    await waitFor(() => expect(within(pane).getByText('after page 1')).toBeInTheDocument())
    await waitFor(() => expect(pane.querySelector('iframe')?.getAttribute('srcdoc')).toContain('Scripts run in the sandbox'))
    expect(within(list).queryByText('budget.svg')).not.toBeInTheDocument()
  })

  it('shows Markdown through the app renderer, and opens from the page’s file list', async () => {
    const s = await chat()
    render(SessionPane, { props: { session: s.current! }, context: ctx(s) })
    // Clicking an artifact's name in the page's file list opens it on the Artifacts tab.
    await fireEvent.click(screen.getByTitle(/^Show in the artifact pane: .*budget\.svg$/))
    const pane = screen.getByRole('complementary', { name: 'Artifacts' })
    expect(await frameIn(pane)).toHaveAttribute('sandbox', '')
    await fireEvent.click(within(pane).getByText('notes.md'))
    expect(await within(pane).findByText('Comfortable shoes')).toBeInTheDocument()
    expect(within(pane).getByText('in the folder now')).toBeInTheDocument()
    // The On this page tab shares the panel, with its section count.
    const outlineTab = screen.getByRole('tab', { name: /^On this page/ })
    expect(within(outlineTab).getByLabelText('2 sections')).toHaveTextContent('2')
    await fireEvent.click(outlineTab)
    expect(screen.getByRole('complementary', { name: 'On this page' })).toBeInTheDocument()
    // Arrow keys move between the tabs.
    await fireEvent.keyDown(outlineTab, { key: 'ArrowRight' })
    expect(screen.getByRole('complementary', { name: 'Artifacts' })).toBeInTheDocument()
  })

  it('isn’t offered in a Code session', async () => {
    localStorage.clear()
    const s = new AppStore(mockBackend())
    await s.init()
    render(SessionPane, { props: { session: s.current! }, context: ctx(s) })
    expect(screen.getAllByRole('tab').map(t => t.querySelector('.name')?.textContent)).toEqual(['On this page'])
  })
})
