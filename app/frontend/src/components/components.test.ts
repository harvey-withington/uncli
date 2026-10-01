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

  it('renders a page whose lists arrive as null', async () => {
    const s = await store()
    const base = s.currentPage as Page
    const page = { ...base, status: 'open', answerMd: '', trace: null, modifiers: null } as unknown as Page
    render(PageView, { props: { page }, context: ctx(s) })
    expect(screen.getByRole('status')).toHaveTextContent('Thinking')
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

describe('Sign-in', () => {
  it('opens the sign-in link, takes the code and reports a wrong one', async () => {
    const backend = mockBackend({ cli: { loggedIn: false } })
    render(App, { props: { backend } })
    await fireEvent.click(await screen.findByRole('button', { name: 'Sign in' }))
    const box = await screen.findByRole('textbox', { name: 'Code from the sign-in page' })
    expect(screen.getByText(/Sign-in opened in your browser/)).toBeInTheDocument()
    await fireEvent.input(box, { target: { value: 'wrong' } })
    await fireEvent.click(screen.getByRole('button', { name: 'Finish sign-in' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Invalid code')
    await fireEvent.click(screen.getByRole('button', { name: 'Sign in' }))
    await fireEvent.input(await screen.findByRole('textbox', { name: 'Code from the sign-in page' }), { target: { value: 'good-code' } })
    await fireEvent.click(screen.getByRole('button', { name: 'Finish sign-in' }))
    expect(await screen.findByRole('heading', { name: 'Fix the flaky parser test' })).toBeInTheDocument()
  })
})

describe('Welcome cards', () => {
  it('open the new-session dialog on the card that was clicked', async () => {
    render(App, { props: { backend: mockBackend({ empty: true }) } })
    await fireEvent.click(await screen.findByRole('button', { name: /^Code/ }))
    const dialog = await screen.findByRole('dialog', { name: 'New session' })
    await waitFor(() => expect(within(dialog).getByRole('radio', { name: /Code/ })).toBeChecked())
    expect(within(dialog).getByText('Repository')).toBeInTheDocument()
    await fireEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }))
    await fireEvent.click(screen.getByRole('button', { name: /New session/ }))
    // The closed dialog may still be fading out; the new one is last.
    const again = (await screen.findAllByRole('dialog', { name: 'New session' })).at(-1) as HTMLElement
    await waitFor(() => expect(within(again).getByRole('radio', { name: /Chat/ })).toBeChecked())
  })
})

describe('New session dialog', () => {
  const lastDialog = async () => (await screen.findAllByRole('dialog', { name: 'New session' })).at(-1) as HTMLElement

  it('starts from the last choices: type, and that type’s model and folder', async () => {
    render(App, { props: { backend: mockBackend({ empty: true }) } })
    await fireEvent.click(await screen.findByRole('button', { name: /^Code/ }))
    let dialog = await lastDialog()
    await fireEvent.change(within(dialog).getByRole('combobox', { name: 'Model' }), { target: { value: 'haiku' } })
    await fireEvent.click(within(dialog).getByRole('button', { name: /Choose/ }))
    await within(dialog).findByText(/projects.demo$/)
    await fireEvent.click(within(dialog).getByRole('button', { name: 'Start session' }))
    await screen.findByRole('heading', { name: 'New session' }) // the new session's pane

    await fireEvent.keyDown(window, { key: 'n', ctrlKey: true })
    dialog = await lastDialog()
    await waitFor(() => expect(within(dialog).getByRole('radio', { name: /Code/ })).toBeChecked())
    expect(within(dialog).getByRole('combobox', { name: 'Model' })).toHaveValue('haiku')
    expect(within(dialog).getByText(/projects.demo$/)).toBeInTheDocument()

    // Another type starts from its own defaults.
    await fireEvent.click(within(dialog).getByRole('radio', { name: /Chat/ }))
    await waitFor(() => expect(within(dialog).getByRole('combobox', { name: 'Model' })).toHaveValue('sonnet'))
  })

  it('lets a clicked welcome card win over the remembered type', async () => {
    render(App, { props: { backend: mockBackend({ empty: true, lastNew: { profileId: 'code' } }) } })
    await fireEvent.click(await screen.findByRole('button', { name: /^Co-work/ }))
    const dialog = await lastDialog()
    await waitFor(() => expect(within(dialog).getByRole('radio', { name: /Co-work/ })).toBeChecked())
  })
})

describe('Outline panel', () => {
  it('lists the answer headings, jumps to them, and hides with O', async () => {
    localStorage.clear()
    render(App, { props: { backend: mockBackend() } })
    await fireEvent.click(await screen.findByText('Plan a weekend in Lisbon'))
    const panel = await screen.findByRole('complementary', { name: 'On this page' })
    const saturday = within(panel).getByRole('button', { name: /Saturday$/ })
    expect(within(panel).getByRole('button', { name: /Sunday$/ })).toBeInTheDocument()
    const scroller = document.querySelector('.scroll') as HTMLElement
    scroller.scrollTo = vi.fn()
    await fireEvent.click(saturday)
    expect(scroller.scrollTo).toHaveBeenCalled()
    await fireEvent.keyDown(window, { key: 'o' })
    expect(screen.queryByRole('complementary', { name: 'On this page' })).not.toBeInTheDocument()
    expect(JSON.parse(localStorage.getItem('uncli-outline') ?? '{}').open).toBe(false)
  })

  it('resizes with the arrow keys on its edge', async () => {
    localStorage.clear()
    render(App, { props: { backend: mockBackend() } })
    await fireEvent.click(await screen.findByText('Plan a weekend in Lisbon'))
    const handle = await screen.findByRole('separator', { name: 'Resize the outline' })
    await fireEvent.keyDown(handle, { key: 'ArrowLeft' })
    expect(handle).toHaveAttribute('aria-valuenow', '256')
  })
})

describe('Page summaries', () => {
  it('summarise a page without headings with the quick-task model', async () => {
    localStorage.clear()
    const backend = mockBackend()
    const spy = vi.spyOn(backend, 'summarisePage')
    render(App, { props: { backend } })
    await fireEvent.click(await screen.findByText('Summarise the Q3 planning notes'))
    const panel = await screen.findByRole('complementary', { name: 'On this page' })
    expect(within(panel).getByText(/This answer has no headings/)).toBeInTheDocument()
    await fireEvent.click(within(panel).getByRole('button', { name: 'Summarise this page with Haiku 4.5' }))
    expect(await within(panel).findByText(/Summary by Haiku 4.5/)).toBeInTheDocument()
    expect(spy).toHaveBeenCalledWith('s-cowork', 's-cowork-p3', expect.arrayContaining([expect.stringContaining('shipping desktop first')]))
    expect(within(panel).getAllByRole('button').length).toBeGreaterThan(2)
  })

  it('switch between summary and headings when a page has both', async () => {
    localStorage.clear()
    render(App, { props: { backend: mockBackend() } })
    await fireEvent.click(await screen.findByText('Plan a weekend in Lisbon'))
    const panel = await screen.findByRole('complementary', { name: 'On this page' })
    await fireEvent.click(within(panel).getByRole('button', { name: /Summarise this page/ }))
    const headings = await within(panel).findByRole('button', { name: 'Headings' })
    await fireEvent.click(headings)
    expect(within(panel).getByRole('button', { name: /Saturday$/ })).toBeInTheDocument()
    expect(within(panel).queryByText(/Summary by/)).not.toBeInTheDocument()
  })
})

describe('Settings', () => {
  it('saves the quick-task model and the auto-summary option', async () => {
    const backend = mockBackend()
    const spy = vi.spyOn(backend, 'setPreferences')
    render(App, { props: { backend } })
    await fireEvent.click(await screen.findByRole('button', { name: /Claude CLI/ }))
    const dialog = await screen.findByRole('dialog', { name: 'Settings' })
    const model = within(dialog).getByRole('combobox', { name: 'Model' })
    expect(model).toHaveValue('haiku')
    await fireEvent.change(model, { target: { value: 'sonnet' } })
    await waitFor(() => expect(spy).toHaveBeenLastCalledWith({ quickTaskModel: { provider: 'claude', model: 'sonnet' }, autoSummarise: false }))
    await fireEvent.click(within(dialog).getByRole('checkbox', { name: /Summarise long answers/ }))
    await waitFor(() => expect(spy).toHaveBeenLastCalledWith({ quickTaskModel: { provider: 'claude', model: 'sonnet' }, autoSummarise: true }))
  })
})

describe('Sidebar resizing', () => {
  it('resizes with the arrow keys on its edge and remembers the width', async () => {
    localStorage.clear()
    render(App, { props: { backend: mockBackend() } })
    const handle = await screen.findByRole('separator', { name: 'Resize the session list' })
    expect(handle).toHaveAttribute('aria-valuenow', '272')
    await fireEvent.keyDown(handle, { key: 'ArrowRight' })
    expect(handle).toHaveAttribute('aria-valuenow', '288')
    await fireEvent.keyDown(handle, { key: 'ArrowLeft', shiftKey: true })
    expect(handle).toHaveAttribute('aria-valuenow', '240')
    expect(JSON.parse(localStorage.getItem('uncli-sidebar') ?? '{}').width).toBe(240)
    expect(document.querySelector('.sidebar')).toHaveStyle({ width: '240px' })
  })
})
