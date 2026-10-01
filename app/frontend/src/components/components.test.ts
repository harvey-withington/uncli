import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte'
import { describe, expect, it, vi } from 'vitest'
import App from '../App.svelte'
import { mockBackend } from '../lib/api/mock'
import type { Page } from '../lib/api'
import { APP_CONTEXT } from '../lib/context'
import { AppStore } from '../stores/app.svelte'
import NavBar from './NavBar.svelte'
import PageView from './PageView.svelte'
import Sidebar from './Sidebar.svelte'

async function store() {
  const s = new AppStore(mockBackend())
  await s.init()
  return s
}

const ctx = (s: AppStore) => new Map([[APP_CONTEXT, s]])

describe('PageView', () => {
  it('shows the question, answer, chips, copy buttons and trace', async () => {
    const s = await store()
    const page = s.currentPage as Page
    render(PageView, { props: { page }, context: ctx(s) })
    expect(screen.getByText('Why does TestMultiTurnPartial fail about one run in five?')).toBeInTheDocument()
    expect(screen.getByText(/each API call/)).toBeInTheDocument()
    expect(screen.getByText('Opus 5.5')).toBeInTheDocument()
    expect(screen.getByText('Thorough')).toBeInTheDocument()
    expect(screen.getByText('23k in · 640 out')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Copy question' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Copy code' })).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: 'Copy markdown' }).length).toBeGreaterThanOrEqual(2)
    expect(screen.getByText('4 tool calls')).toBeInTheDocument()
    expect(screen.getByText('1 denied')).toBeInTheDocument()
  })

  it('copies the source markdown of a block, not its rendered text', async () => {
    const s = await store()
    const copy = vi.spyOn(s.backend, 'copyText')
    render(PageView, { props: { page: s.currentPage as Page }, context: ctx(s) })
    const buttons = screen.getAllByRole('button', { name: 'Copy markdown' })
    await fireEvent.click(buttons[0] as HTMLElement)
    expect(copy).toHaveBeenCalledWith(expect.stringContaining('`system/status`'))
    await fireEvent.click(screen.getByRole('button', { name: 'Copy code' }))
    expect(copy).toHaveBeenLastCalledWith(expect.stringMatching(/^case "status":/))
    await fireEvent.click(screen.getByRole('button', { name: 'Copy question' }))
    expect(copy).toHaveBeenLastCalledWith('Why does TestMultiTurnPartial fail about one run in five?')
  })

  it('shows interrupted and failed turns', async () => {
    const s = await store()
    const base = s.currentPage as Page
    const { unmount } = render(PageView, { props: { page: { ...base, status: 'interrupted' } }, context: ctx(s) })
    expect(screen.getByText('Stopped before the answer finished.')).toBeInTheDocument()
    unmount()
    render(PageView, { props: { page: { ...base, status: 'error', error: 'authentication_failed: Not logged in' } }, context: ctx(s) })
    expect(screen.getByRole('alert')).toHaveTextContent('Not logged in')
  })
})

describe('Sidebar', () => {
  it('lists sessions with their activity', async () => {
    const s = await store()
    render(Sidebar, { context: ctx(s) })
    const list = screen.getByRole('navigation', { name: 'Sessions' })
    expect(within(list).getByText('Fix the flaky parser test')).toBeInTheDocument()
    expect(within(list).getByText('Plan a weekend in Lisbon')).toBeInTheDocument()
    expect(within(list).getByText('Unread')).toBeInTheDocument()
    expect(screen.getByText('Claude CLI 2.1.285')).toBeInTheDocument()
  })
})

describe('NavBar', () => {
  it('pages and bookmarks', async () => {
    const s = await store()
    await s.select('s-cowork')
    render(NavBar, { context: ctx(s) })
    expect(screen.getByText('Page 3 of 3')).toBeInTheDocument()
    await fireEvent.click(screen.getByRole('button', { name: 'Previous bookmark, or the first page' }))
    expect(screen.getByText('Page 1 of 3')).toBeInTheDocument()
    await fireEvent.click(screen.getByRole('button', { name: /Next page/ }))
    expect(screen.getByText('Page 2 of 3')).toBeInTheDocument()
    await fireEvent.click(screen.getByRole('button', { name: 'Bookmark this page (B)' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Bookmark this page (B)' })).toHaveAttribute('aria-pressed', 'true'))
    await fireEvent.click(screen.getByRole('button', { name: 'Next bookmark, or the last page' }))
    expect(screen.getByText('Page 3 of 3')).toBeInTheDocument()
  })
})

describe('App', () => {
  it('sends a turn, streams the answer and lands on the new page', async () => {
    render(App, { props: { backend: mockBackend() } })
    await screen.findByRole('heading', { name: 'Fix the flaky parser test' })
    const box = screen.getByRole('textbox', { name: 'Your message' })
    await fireEvent.input(box, { target: { value: 'How does stream-json work?' } })
    await fireEvent.keyDown(box, { key: 'Enter' })
    await screen.findByText('Page 2 of 2')
    expect(screen.getByText('How does stream-json work?')).toBeInTheDocument()
    expect(await screen.findByRole('button', { name: /Stop/ })).toBeInTheDocument()
    await screen.findByText("That's all the adapter needs to get started.", {}, { timeout: 8000 })
    await waitFor(() => expect(screen.queryByRole('button', { name: /Stop/ })).not.toBeInTheDocument())
  }, 15000)

  it('switches pages with the arrow keys and bookmarks with B', async () => {
    render(App, { props: { backend: mockBackend() } })
    await screen.findByRole('heading', { name: 'Fix the flaky parser test' })
    await fireEvent.click(screen.getByText('Summarise the Q3 planning notes'))
    await screen.findByText('Page 3 of 3')
    await fireEvent.keyDown(window, { key: 'ArrowLeft' })
    expect(screen.getByText('Page 2 of 3')).toBeInTheDocument()
    await fireEvent.keyDown(window, { key: 'b' })
    await waitFor(() => expect(screen.getByRole('button', { name: 'Bookmark this page (B)' })).toHaveAttribute('aria-pressed', 'true'))
  })

  it('shows the setup screen until the CLI is installed and signed in', async () => {
    render(App, { props: { backend: mockBackend({ cli: { installed: false, loggedIn: false } }) } })
    expect(await screen.findByRole('heading', { name: "Let's get you set up" })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Download' })).toBeInTheDocument()
  })

  it('toggles a modifier group exclusively', async () => {
    render(App, { props: { backend: mockBackend() } })
    await screen.findByRole('heading', { name: 'Fix the flaky parser test' })
    const toolbar = screen.getByRole('toolbar', { name: 'Session tools' })
    const thorough = within(toolbar).getByRole('button', { name: /Thorough/ })
    const efficiency = within(toolbar).getByRole('button', { name: /Efficiency Mode/ })
    expect(thorough).toHaveAttribute('aria-pressed', 'true')
    await fireEvent.click(efficiency)
    await waitFor(() => expect(efficiency).toHaveAttribute('aria-pressed', 'true'))
    expect(thorough).toHaveAttribute('aria-pressed', 'false')
  })
})
