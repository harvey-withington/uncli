import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte'
import { describe, expect, it, vi } from 'vitest'
import App from '../App.svelte'
import { mockBackend } from '../lib/api/mock'
import type { Page } from '../lib/api'
import { APP_CONTEXT } from '../lib/context'
import { AppStore } from '../stores/app.svelte'
import NavBar from './NavBar.svelte'
import OutlinePanel from './OutlinePanel.svelte'
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
    expect(screen.getByText('5 tool calls')).toBeInTheDocument()
    expect(screen.getByText('1 denied')).toBeInTheDocument()
    // A failed call says who allowed it and why it failed; its output opens on request.
    await fireEvent.click(screen.getByRole('button', { name: /5 tool calls/ }))
    expect(screen.getByText('allowed by you')).toBeInTheDocument()
    expect(screen.getByText(/Exit code 1 · node.exe : 'vitest' is not recognized/)).toBeInTheDocument()
    await fireEvent.click(screen.getByRole('button', { name: 'output' }))
    expect(screen.getByText(/operable program or batch file/)).toBeInTheDocument()
  })

  it('collapses the question to one line with its page number, and remembers it', async () => {
    localStorage.clear()
    const s = await store()
    const page = s.currentPage as Page
    const { unmount } = render(PageView, { props: { page }, context: ctx(s) })
    const collapse = screen.getByRole('button', { name: 'Compact question: one line' })
    expect(collapse).toHaveAttribute('aria-expanded', 'true')
    await fireEvent.click(collapse)
    const header = document.querySelector('header.question') as HTMLElement
    expect(header).toHaveClass('compact')
    expect(within(header).getByTitle(page.question)).toHaveTextContent(page.question)
    expect(within(header).getByText(`Page ${page.seq}`)).toBeInTheDocument()
    expect(within(header).getByRole('button', { name: 'Copy question' })).toBeInTheDocument()
    expect(within(header).queryByText('Opus 5.5')).not.toBeInTheDocument() // chips hidden
    unmount()
    // A fresh start keeps it compact.
    const again = await store()
    render(PageView, { props: { page: again.currentPage as Page }, context: ctx(again) })
    await fireEvent.click(screen.getByRole('button', { name: 'Show the whole question' }))
    expect(screen.getByText('Opus 5.5')).toBeInTheDocument()
    expect(again.questionCompact).toBe(false)
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

  it('summarise a long answer as soon as it finishes, with the outline closed, when opted in', async () => {
    localStorage.clear()
    const s = new AppStore(mockBackend({ prefs: { autoSummary: 'long' } }))
    await s.init()
    s.outline.open = false
    const answerMd = '## Heading\n\n' + Array.from({ length: 40 }, (_, i) => `Paragraph ${i} has four words.`).join('\n\n')
    const open: Page = { ...(s.currentPage as Page), id: 'p-new', seq: 99, status: 'open', answerMd, outline: undefined }
    const done: Page = { ...open, status: 'done' }
    const spy = vi.spyOn(s.backend, 'summarisePage').mockResolvedValue(done)
    s.upsertPage(open)
    expect(spy).not.toHaveBeenCalled()
    s.upsertPage(done)
    expect(spy).toHaveBeenCalledTimes(1)
    expect(spy).toHaveBeenCalledWith(open.sessionId, 'p-new', expect.arrayContaining(['Paragraph 0 has four words.']))
    s.upsertPage({ ...done }) // already tried: not again
    await waitFor(() => expect(s.summarising['p-new']).toBeUndefined())
    expect(spy).toHaveBeenCalledTimes(1)
  })

  it('say when an answer was too short to summarise automatically', async () => {
    for (const [mode, shown] of [['long', true], ['off', false]] as const) {
      const s = new AppStore(mockBackend({ prefs: { autoSummary: mode } }))
      await s.init()
      const spy = vi.spyOn(s.backend, 'summarisePage')
      const page: Page = { ...(s.currentPage as Page), status: 'done', answerMd: '## A\n\nShort.\n\n## B\n\nAlso short.', outline: undefined }
      const { unmount } = render(OutlinePanel, { props: { page, scroller: undefined }, context: ctx(s) })
      if (shown) expect(screen.getByText('Short answer, not summarised.')).toBeInTheDocument()
      else expect(screen.queryByText('Short answer, not summarised.')).not.toBeInTheDocument()
      expect(spy).not.toHaveBeenCalled()
      unmount()
    }
  })

  it('mark the outline entries in the answer margin, following the Summary/Headings switch', async () => {
    localStorage.clear()
    render(App, { props: { backend: mockBackend() } })
    await fireEvent.click(await screen.findByText('Plan a weekend in Lisbon'))
    const panel = await screen.findByRole('complementary', { name: 'On this page' })
    const markers = () => [...document.querySelectorAll('.answer .marker')].map(m => m.getAttribute('title'))
    expect(markers()).toEqual(['Steps: Saturday', expect.stringMatching(/: Sunday$/)])
    await fireEvent.click(within(panel).getByRole('button', { name: /Summarise this page/ }))
    await within(panel).findByText(/Summary by/)
    const summarised = markers()
    expect(summarised.length).toBeGreaterThan(2)
    await fireEvent.click(within(panel).getByRole('button', { name: 'Headings' }))
    expect(markers()).toHaveLength(2)
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
    await waitFor(() => expect(spy).toHaveBeenLastCalledWith({ quickTaskModel: { provider: 'claude', model: 'sonnet' }, autoSummary: 'off' }))
    const summaries = within(dialog).getByRole('combobox', { name: 'Summaries' })
    expect(within(summaries).getAllByRole('option').map(o => o.textContent)).toEqual(["Don't summarise", 'Summarise long answers', 'Always summarise'])
    await fireEvent.change(summaries, { target: { value: 'always' } })
    await waitFor(() => expect(spy).toHaveBeenLastCalledWith({ quickTaskModel: { provider: 'claude', model: 'sonnet' }, autoSummary: 'always' }))
    expect(await within(dialog).findByText(/Every answer of more than one paragraph/)).toBeInTheDocument()
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

describe('New session from the keyboard', () => {
  const lastDialog = async () => (await screen.findAllByRole('dialog', { name: 'New session' })).at(-1) as HTMLElement

  it('Chat: Ctrl+N, Enter starts the session', async () => {
    const backend = mockBackend({ empty: true })
    const create = vi.spyOn(backend, 'createSession')
    render(App, { props: { backend } })
    await screen.findByRole('heading', { name: 'Welcome to UNCLI' })
    await fireEvent.keyDown(window, { key: 'n', ctrlKey: true })
    const dialog = await lastDialog()
    const chat = within(dialog).getByRole('radio', { name: /Chat/ })
    await waitFor(() => expect(chat).toBeChecked())
    await fireEvent.keyDown(chat, { key: 'Enter' })
    await waitFor(() => expect(create).toHaveBeenCalledWith('chat', '', 'sonnet'))
  })

  it('Code without a folder: Enter picks one, the next Enter starts', async () => {
    const backend = mockBackend({ empty: true })
    const pick = vi.spyOn(backend, 'pickFolder')
    const create = vi.spyOn(backend, 'createSession')
    render(App, { props: { backend } })
    await fireEvent.click(await screen.findByRole('button', { name: /^Code/ }))
    const dialog = await lastDialog()
    const code = within(dialog).getByRole('radio', { name: /Code/ })
    await waitFor(() => expect(code).toBeChecked())
    await fireEvent.keyDown(code, { key: 'Enter' })
    await waitFor(() => expect(pick).toHaveBeenCalled())
    const start = within(dialog).getByRole('button', { name: 'Start session' })
    await waitFor(() => expect(start).toHaveFocus())
    expect(create).not.toHaveBeenCalled()
    await fireEvent.keyDown(code, { key: 'Enter' }) // Enter from anywhere in the body also starts now
    await waitFor(() => expect(create).toHaveBeenCalledWith('code', expect.stringMatching(/projects.demo$/), 'opus'))
  })

  it('a remembered folder means Ctrl+N, Enter', async () => {
    const backend = mockBackend({ empty: true, lastNew: { profileId: 'code', folders: { code: 'D:/work/repo' }, models: { code: 'haiku' } } })
    const pick = vi.spyOn(backend, 'pickFolder')
    const create = vi.spyOn(backend, 'createSession')
    render(App, { props: { backend } })
    await screen.findByRole('heading', { name: 'Welcome to UNCLI' })
    await fireEvent.keyDown(window, { key: 'n', ctrlKey: true })
    const code = within(await lastDialog()).getByRole('radio', { name: /Code/ })
    await waitFor(() => expect(code).toBeChecked())
    await fireEvent.keyDown(code, { key: 'Enter' })
    await waitFor(() => expect(create).toHaveBeenCalledWith('code', expect.stringMatching(/repo$/), 'haiku'))
    expect(pick).not.toHaveBeenCalled()
  })
})

describe('Reordering sessions', () => {
  const titles = () => Array.from(document.querySelectorAll('.sidebar li .title')).map(el => el.textContent)

  it('moves a session with Alt+Arrow', async () => {
    const backend = mockBackend()
    const spy = vi.spyOn(backend, 'setSortOrder')
    render(App, { props: { backend } })
    await screen.findByRole('heading', { name: 'Fix the flaky parser test' })
    expect(titles()[0]).toBe('Fix the flaky parser test')
    const first = document.querySelector('.sidebar li .main') as HTMLElement
    await fireEvent.keyDown(first, { key: 'ArrowDown', altKey: true })
    await waitFor(() => expect(titles()).toEqual(['Plan a weekend in Lisbon', 'Fix the flaky parser test', 'Summarise the Q3 planning notes']))
    expect(spy).toHaveBeenCalledTimes(1)
  })

  it('moves a session by dragging it', async () => {
    render(App, { props: { backend: mockBackend() } })
    await screen.findByRole('heading', { name: 'Fix the flaky parser test' })
    // jsdom has no layout: give each item a 50px-tall box.
    document.querySelectorAll<HTMLElement>('.sidebar li[data-id]').forEach((li, i) => {
      li.getBoundingClientRect = () => ({ top: i * 50, height: 50, bottom: i * 50 + 50, left: 0, right: 200, width: 200, x: 0, y: i * 50, toJSON: () => ({}) })
    })
    const first = document.querySelector('.sidebar li .main') as HTMLElement
    await fireEvent.pointerDown(first, { button: 0, clientY: 10 })
    await fireEvent.pointerMove(document, { clientY: 60 })
    await fireEvent.pointerMove(document, { clientY: 140 }) // past the middle of the last item
    const items = [...document.querySelectorAll<HTMLElement>('.sidebar li[data-id]')]
    expect(items[0]).toHaveClass('dragging') // its slot, moved to the end
    expect(items[0]?.style.transform).toBe(`translateY(${(items.length - 1) * 50}px)`)
    expect(items[1]?.style.transform).toBe('translateY(-50px)') // the others make room
    expect(document.querySelector('body > .drag-ghost')).not.toBeNull()
    await fireEvent.pointerUp(document)
    await waitFor(() => expect(titles().at(-1)).toBe('Fix the flaky parser test'))
    await waitFor(() => expect(document.querySelector('.drag-ghost')).toBeNull())
    expect(document.querySelector('.sidebar li.dragging')).toBeNull()
    expect([...document.querySelectorAll<HTMLElement>('.sidebar li[data-id]')].every(li => !li.style.transform)).toBe(true)
  })

  it('cancels a drag with Escape, leaving the order as it was', async () => {
    render(App, { props: { backend: mockBackend() } })
    await screen.findByRole('heading', { name: 'Fix the flaky parser test' })
    document.querySelectorAll<HTMLElement>('.sidebar li[data-id]').forEach((li, i) => {
      li.getBoundingClientRect = () => ({ top: i * 50, height: 50, bottom: i * 50 + 50, left: 0, right: 200, width: 200, x: 0, y: i * 50, toJSON: () => ({}) })
    })
    const before = titles()
    await fireEvent.pointerDown(document.querySelector('.sidebar li .main') as HTMLElement, { button: 0, clientY: 10 })
    await fireEvent.pointerMove(document, { clientY: 140 })
    await fireEvent.keyDown(document, { key: 'Escape' })
    await waitFor(() => expect(document.querySelector('.drag-ghost')).toBeNull())
    await fireEvent.pointerUp(document)
    expect(titles()).toEqual(before)
  })
})

describe('Approvals', () => {
  async function ask(text: string) {
    const box = (await screen.findByRole('textbox', { name: 'Your message' })) as HTMLTextAreaElement
    await fireEvent.input(box, { target: { value: text } })
    await fireEvent.click(await screen.findByRole('button', { name: 'Send' }, { timeout: 4000 }))
  }
  const card = () => screen.queryByRole('alertdialog')

  it('shows what Claude wants to run, and Allow lets it go ahead', async () => {
    const backend = mockBackend()
    const answer = vi.spyOn(backend, 'answerApproval')
    render(App, { props: { backend } })
    await ask('commit and push')
    const c = await screen.findByRole('alertdialog', { name: 'Run a command' }, { timeout: 2000 })
    expect(within(c).getByText('git push origin main')).toBeInTheDocument()
    expect(await screen.findByText('Needs approval', { selector: '.sidebar *' })).toBeInTheDocument()
    await fireEvent.click(within(c).getByRole('button', { name: 'Allow' }))
    expect(answer).toHaveBeenCalledWith(expect.any(String), 'req_1', 'allow', undefined)
    await waitFor(() => expect(card()).toBeNull())
    expect(await screen.findByText(/Pushed/, {}, { timeout: 3000 })).toBeInTheDocument()
  })

  it('Unattended declines a waiting card and the next request without asking', async () => {
    const backend = mockBackend()
    const answer = vi.spyOn(backend, 'answerApproval')
    render(App, { props: { backend } })
    await ask('push')
    await screen.findByRole('alertdialog', {}, { timeout: 2000 })
    const away = screen.getByRole('button', { name: 'Unattended' })
    expect(away).toHaveAttribute('aria-pressed', 'false')
    await fireEvent.click(away)
    await waitFor(() => expect(card()).toBeNull())
    expect(away).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByText(/anything that would ask you is declined automatically/)).toBeInTheDocument()
    expect(await screen.findByText(/you're away, so the push was declined/, {}, { timeout: 3000 })).toBeInTheDocument()
    await ask('push again')
    expect(await screen.findByText('Page 3', { selector: '.seq' }, { timeout: 3000 })).toBeInTheDocument()
    expect(await screen.findByText(/you're away, so the push was declined/, {}, { timeout: 3000 })).toBeInTheDocument()
    expect(card()).toBeNull()
    expect(answer).not.toHaveBeenCalled()
  })

  it('Deny tells Claude no', async () => {
    render(App, { props: { backend: mockBackend() } })
    await ask('push it')
    const c = await screen.findByRole('alertdialog', {}, { timeout: 2000 })
    await fireEvent.click(within(c).getByRole('button', { name: 'Deny' }))
    expect(await screen.findByText(/the push was denied/, {}, { timeout: 3000 })).toBeInTheDocument()
  })

  it('Always adds the chosen rule for the project, and the next push goes without asking', async () => {
    const backend = mockBackend()
    const answer = vi.spyOn(backend, 'answerApproval')
    render(App, { props: { backend } })
    await ask('push')
    const c = await screen.findByRole('alertdialog', {}, { timeout: 2000 })
    const scope = within(c).getByRole('combobox', { name: 'What to always allow in this project' })
    expect(within(scope).getAllByRole('option').map(o => o.textContent)).toEqual(['git push …', 'Git pushes', 'git …'])
    await fireEvent.change(scope, { target: { value: '1' } })
    await fireEvent.change(within(c).getByRole('combobox', { name: 'How long to allow it' }), { target: { value: 'always' } })
    await fireEvent.click(within(c).getByRole('button', { name: 'Always allow' }))
    expect(answer).toHaveBeenLastCalledWith(expect.any(String), 'req_1', 'always', { tool: 'Bash', prefix: 'git:publish', action: 'allow' })
    await screen.findByText(/Pushed/, {}, { timeout: 3000 })
    await ask('push again')
    // The new page (the session already had one, so page 3) answers without a card.
    expect(await screen.findByText('Page 3', { selector: '.seq' }, { timeout: 3000 })).toBeInTheDocument()
    expect(await screen.findByText(/Pushed/, {}, { timeout: 3000 })).toBeInTheDocument()
    expect(card()).toBeNull()
    expect(answer).toHaveBeenCalledTimes(1)
  })

  it('Always allows for this session by default, and the rule can be made permanent', async () => {
    const backend = mockBackend()
    const answer = vi.spyOn(backend, 'answerApproval')
    const promote = vi.spyOn(backend, 'promoteSessionToolRule')
    render(App, { props: { backend } })
    await ask('push')
    const c = await screen.findByRole('alertdialog', {}, { timeout: 2000 })
    expect(within(c).getByRole('combobox', { name: 'How long to allow it' })).toHaveValue('session')
    await fireEvent.click(within(c).getByRole('button', { name: 'Always allow' }))
    expect(answer).toHaveBeenLastCalledWith(expect.any(String), 'req_1', 'session', { tool: 'Bash', prefix: 'git push', action: 'allow' })
    await screen.findByText(/Pushed/, {}, { timeout: 3000 })
    await ask('push again')
    expect(await screen.findByText('Page 3', { selector: '.seq' }, { timeout: 3000 })).toBeInTheDocument()
    expect(card()).toBeNull()
    await fireEvent.click(screen.getByRole('button', { name: 'Permissions for this project' }))
    const dlg = await screen.findByRole('dialog', { name: 'Permissions' })
    const sess = await within(dlg).findByRole('list', { name: 'This session' })
    expect(within(sess).getByText('git push …')).toBeInTheDocument()
    await fireEvent.click(within(sess).getByRole('button', { name: 'Make permanent' }))
    await waitFor(() => expect(promote).toHaveBeenCalledWith(expect.any(String), { tool: 'Bash', prefix: 'git push', action: 'allow' }))
    expect(await within(dlg).findByRole('list', { name: 'Rules' })).toBeInTheDocument()
    await waitFor(() => expect(within(dlg).queryByRole('list', { name: 'This session' })).toBeNull())
  })

  it('lists, changes, adds and removes the project rules', async () => {
    const backend = mockBackend()
    const set = vi.spyOn(backend, 'setToolRule')
    render(App, { props: { backend } })
    await fireEvent.click(await screen.findByRole('button', { name: 'Permissions for this project' }))
    const dlg = await screen.findByRole('dialog', { name: 'Permissions' })
    expect(within(dlg).getByText(/No rules yet/)).toBeInTheDocument()
    // Anything but commit and push: git allowed, commit and push asked.
    const what = within(dlg).getByRole('combobox', { name: 'What the new rule covers' })
    await fireEvent.change(what, { target: { value: 'command' } })
    await fireEvent.input(within(dlg).getByRole('textbox', { name: 'Command it starts with' }), { target: { value: 'git' } })
    await fireEvent.click(within(dlg).getByRole('button', { name: 'Add' }))
    await waitFor(() => expect(set).toHaveBeenLastCalledWith(expect.any(String), { tool: 'Bash', prefix: 'git', action: 'allow' }))
    await fireEvent.change(what, { target: { value: 'git:publish' } })
    await fireEvent.change(within(dlg).getByRole('combobox', { name: 'What to do' }), { target: { value: 'ask' } })
    await fireEvent.click(within(dlg).getByRole('button', { name: 'Add' }))
    const list = await within(dlg).findByRole('list', { name: 'Rules' })
    await waitFor(() => expect(within(list).getAllByRole('listitem')).toHaveLength(2))
    expect(within(list).getByText('Git pushes')).toBeInTheDocument()
    await fireEvent.change(within(list).getByRole('combobox', { name: 'What to do for Git pushes' }), { target: { value: 'deny' } })
    await waitFor(() => expect(set).toHaveBeenLastCalledWith(expect.any(String), { tool: 'Bash', prefix: 'git:publish', action: 'deny' }))
    await fireEvent.click(within(list).getByRole('button', { name: 'Remove the rule for git …' }))
    await waitFor(() => expect(within(list).getAllByRole('listitem')).toHaveLength(1))
  })
})

describe('Search', () => {
  const box = () => screen.getByRole('searchbox', { name: 'Search all sessions' }) as HTMLInputElement
  const results = () => document.getElementById('search-results')

  it('focuses with Ctrl+K, finds across sessions (accents folded), and opens the page on Enter', async () => {
    localStorage.clear()
    const backend = mockBackend()
    const search = vi.spyOn(backend, 'search')
    render(App, { props: { backend } })
    await screen.findByRole('heading', { name: 'Fix the flaky parser test' })
    await fireEvent.keyDown(window, { key: 'k', ctrlKey: true })
    expect(document.activeElement).toBe(box())
    await fireEvent.input(box(), { target: { value: 'jeronimos' } })
    // The session list gives way to grouped results.
    await waitFor(() => expect(results()).not.toBeNull())
    const region = within(results() as HTMLElement)
    expect(await region.findByText('1 page')).toBeInTheDocument()
    expect(region.getByText('Plan a weekend in Lisbon')).toBeInTheDocument()
    expect(region.getByText('Jerónimos', { selector: 'mark' })).toBeInTheDocument()
    expect(search).toHaveBeenLastCalledWith(expect.objectContaining({ text: 'jeronimos' }))
    await fireEvent.keyDown(box(), { key: 'Enter' })
    await waitFor(() => expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Plan a weekend in Lisbon'))
    // Escape clears the box and brings the session list back.
    await fireEvent.keyDown(box(), { key: 'Escape' })
    expect(box()).toHaveValue('')
    await waitFor(() => expect(results()).toBeNull())
    expect(screen.getByText('Fix the flaky parser test', { selector: '.title' })).toBeInTheDocument()
  })

  it('debounces typing and keeps only the latest answer', async () => {
    const backend = mockBackend()
    const search = vi.spyOn(backend, 'search')
    render(App, { props: { backend } })
    await screen.findByRole('heading', { name: 'Fix the flaky parser test' })
    for (const v of ['p', 'pa', 'par', 'pars']) await fireEvent.input(box(), { target: { value: v } })
    await waitFor(() => expect(search).toHaveBeenCalled())
    expect(search).toHaveBeenCalledTimes(1)
    expect(search).toHaveBeenLastCalledWith(expect.objectContaining({ text: 'pars' }))
  })

  it('filters by session type and bookmarks, and says when nothing matches', async () => {
    const backend = mockBackend()
    const search = vi.spyOn(backend, 'search')
    render(App, { props: { backend } })
    await screen.findByRole('heading', { name: 'Fix the flaky parser test' })
    await fireEvent.input(box(), { target: { value: 'the' } })
    await waitFor(() => expect(results()).not.toBeNull())
    await fireEvent.click(screen.getByRole('button', { name: 'Only Code' }))
    await waitFor(() => expect(search).toHaveBeenLastCalledWith(expect.objectContaining({ profiles: ['code'] })))
    await fireEvent.click(screen.getByRole('button', { name: 'Bookmarked' }))
    await waitFor(() => expect(search).toHaveBeenLastCalledWith(expect.objectContaining({ profiles: ['code'], bookmarked: true })))
    await fireEvent.input(box(), { target: { value: 'zzzqqq' } })
    expect(await within(results() as HTMLElement).findByText('No pages match.')).toBeInTheDocument()
  })
})

describe('Dropping files and folders', () => {
  it('the composer inserts dropped paths at the cursor', async () => {
    render(App, { props: { backend: mockBackend() } })
    const box = (await screen.findByRole('textbox', { name: 'Your message' })) as HTMLTextAreaElement
    await fireEvent.input(box, { target: { value: 'Look at' } })
    box.setSelectionRange(7, 7)
    box.dispatchEvent(new CustomEvent('uncli-insert', { detail: 'C:/repo/a.go "C:/My Docs/b.md"' }))
    await waitFor(() => expect(box).toHaveValue('Look at C:/repo/a.go "C:/My Docs/b.md"'))
  })

  // A drag or clipboard event carrying files, as WebView2 delivers them.
  function withFiles<T extends Event>(ev: T, files: File[], extra: Record<string, unknown> = {}): T {
    const data = { types: ['Files'], files, dropEffect: 'none', getData: () => '' }
    Object.defineProperty(ev, 'dataTransfer', { value: data })
    Object.defineProperty(ev, 'clipboardData', { value: data })
    for (const [k, v] of Object.entries(extra)) Object.defineProperty(ev, k, { value: v })
    return ev
  }

  it('takes dropped files itself when Wails drop handling is off, so the message box never gets their names', async () => {
    const { listenForDrops } = await import('../lib/drops')
    const w = window as unknown as { runtime?: unknown; wails?: unknown }
    const resolve = vi.fn()
    w.runtime = { OnFileDrop: vi.fn(), ResolveFilePaths: resolve }
    const s = new AppStore(mockBackend())
    const off = listenForDrops(s)
    try {
      const file = new File(['x'], 'notes.md')
      const over = withFiles(new Event('dragover', { cancelable: true }), [file])
      window.dispatchEvent(over)
      expect(over.defaultPrevented).toBe(true)
      const drop = withFiles(new Event('drop', { cancelable: true }), [file], { clientX: 40, clientY: 60 })
      window.dispatchEvent(drop)
      expect(drop.defaultPrevented).toBe(true)
      expect(resolve).toHaveBeenCalledWith(40, 60, [file])
      // With Wails handling drops, it sends them; we don't send twice.
      w.wails = { flags: { enableWailsDragAndDrop: true } }
      window.dispatchEvent(withFiles(new Event('drop', { cancelable: true }), [file], { clientX: 1, clientY: 1 }))
      expect(resolve).toHaveBeenCalledTimes(1)
      // A plain text drag is left alone.
      const text = new Event('dragover', { cancelable: true })
      Object.defineProperty(text, 'dataTransfer', { value: { types: ['text/plain'] } })
      window.dispatchEvent(text)
      expect(text.defaultPrevented).toBe(false)
    } finally {
      off()
      delete w.runtime
      delete w.wails
    }
  })

  // Pastes as WebView2 delivers them.
  function paste(types: string[], files: File[], text = ''): Event {
    const e = new Event('paste', { cancelable: true, bubbles: true })
    Object.defineProperty(e, 'clipboardData', { value: { types, files, getData: () => text } })
    return e
  }
  const chips = () => [...document.querySelectorAll('.composer .chip .cname')].map(c => c.textContent)

  it('attaches files copied in Explorer (paths from the OS clipboard), and leaves text pastes alone', async () => {
    const backend = mockBackend()
    const list = vi.spyOn(backend, 'clipboardFiles').mockResolvedValue(['C:/repo/a.go', 'C:/My Docs/b.md'])
    render(App, { props: { backend } })
    const box = (await screen.findByRole('textbox', { name: 'Your message' })) as HTMLTextAreaElement
    // What WebView2 hands the page: the files, and their names as text.
    const e = paste(['text/plain', 'Files'], [new File(['x'], 'a.go')], 'a.go b.md')
    box.dispatchEvent(e)
    expect(e.defaultPrevented).toBe(true)
    await waitFor(() => expect(chips()).toEqual(['a.go', 'b.md']))
    expect(box).toHaveValue('') // no names typed in
    const t = paste(['text/plain'], [], 'hello')
    box.dispatchEvent(t)
    expect(t.defaultPrevented).toBe(false)
    expect(list).toHaveBeenCalledTimes(1)
  })

  it('attaches a pasted screenshot as an image with a thumbnail', async () => {
    const backend = mockBackend()
    vi.spyOn(backend, 'clipboardFiles').mockResolvedValue([])
    const send = vi.spyOn(backend, 'send')
    render(App, { props: { backend } })
    const box = (await screen.findByRole('textbox', { name: 'Your message' })) as HTMLTextAreaElement
    box.dispatchEvent(paste(['Files'], [new File([new Uint8Array([137, 80, 78, 71])], 'image.png', { type: 'image/png' })]))
    await waitFor(() => expect(chips()).toEqual(['Pasted image']))
    expect(document.querySelector('.composer .chip img.thumb')).not.toBeNull()
    // Files only: the turn can go without text.
    await fireEvent.click(screen.getByRole('button', { name: 'Send' }))
    await waitFor(() => expect(send).toHaveBeenCalledWith(expect.any(String), '', [{ name: 'Pasted image', mediaType: 'image/png', data: 'iVBORw==' }]))
    await waitFor(() => expect(chips()).toEqual([]))
  })

  it('attaches files dropped on the message box, puts folders and others in as paths, and sends the rest', async () => {
    const backend = mockBackend()
    const send = vi.spyOn(backend, 'send')
    render(App, { props: { backend } })
    const box = (await screen.findByRole('textbox', { name: 'Your message' })) as HTMLTextAreaElement
    const { ATTACH_EVENT } = await import('../lib/drops')
    box.dispatchEvent(new CustomEvent(ATTACH_EVENT, { detail: ['C:/shots/a.png', 'C:/docs/brief.pdf', 'C:/repo/notes.md', 'C:/repo', 'C:/bin/tool.exe'] }))
    await waitFor(() => expect(chips()).toEqual(['a.png', 'brief.pdf', 'notes.md']))
    await waitFor(() => expect(box).toHaveValue('C:/repo C:/bin/tool.exe'))
    expect(await screen.findByText(/2 items weren't attached/)).toBeInTheDocument()
    // The same file twice attaches once; a chip can be removed.
    box.dispatchEvent(new CustomEvent(ATTACH_EVENT, { detail: ['C:/shots/a.png'] }))
    await fireEvent.click(screen.getByRole('button', { name: 'Remove notes.md' }))
    expect(chips()).toEqual(['a.png', 'brief.pdf'])
    await fireEvent.input(box, { target: { value: 'Compare these' } })
    await fireEvent.click(screen.getByRole('button', { name: 'Send' }))
    await waitFor(() => expect(send).toHaveBeenCalledWith(expect.any(String), 'Compare these', [{ path: 'C:/shots/a.png' }, { path: 'C:/docs/brief.pdf' }]))
    // The page shows what went with the question.
    const files = await screen.findByRole('list', { name: 'Files sent with this question' })
    expect(within(files).getByText('a.png')).toBeInTheDocument()
    expect(within(files).getByText('brief.pdf')).toBeInTheDocument()
  })

  it('keeps attachments with the session they were added in', async () => {
    render(App, { props: { backend: mockBackend() } })
    const box = (await screen.findByRole('textbox', { name: 'Your message' })) as HTMLTextAreaElement
    const { ATTACH_EVENT } = await import('../lib/drops')
    box.dispatchEvent(new CustomEvent(ATTACH_EVENT, { detail: ['C:/docs/brief.pdf'] }))
    await waitFor(() => expect(chips()).toEqual(['brief.pdf']))
    await fireEvent.click(screen.getByText('Plan a weekend in Lisbon'))
    await waitFor(() => expect(chips()).toEqual([]))
    await fireEvent.click(screen.getByText('Fix the flaky parser test'))
    await waitFor(() => expect(chips()).toEqual(['brief.pdf']))
  })

  it('ignores files whose paths could not be resolved', async () => {
    const { handleDrop } = await import('../lib/drops')
    const s = new AppStore(mockBackend())
    await s.init()
    await handleDrop(s, document.body, ['', ''])
    expect(s.newSessionOpen).toBe(false)
  })

  it('opens a new session on a dropped folder, in a folder type', async () => {
    const { handleDrop } = await import('../lib/drops')
    const s = new AppStore(mockBackend())
    await s.init()
    await handleDrop(s, document.body, ['D:/work/project'])
    expect(s.newSessionOpen).toBe(true)
    expect(s.newSessionProfile).toBe('code')
    expect(s.newSessionFolder).toBe('D:/work/project')
  })

  it('routes a drop on the composer to it', async () => {
    const { handleDrop, ATTACH_EVENT } = await import('../lib/drops')
    const s = new AppStore(mockBackend())
    await s.init()
    const composer = document.createElement('div')
    composer.className = 'composer'
    const ta = document.createElement('textarea')
    composer.append(ta)
    document.body.append(composer)
    const got = vi.fn()
    ta.addEventListener(ATTACH_EVENT, e => got((e as CustomEvent).detail))
    await handleDrop(s, ta, ['C:/My Docs/b.md'])
    expect(got).toHaveBeenCalledWith(['C:/My Docs/b.md'])
    expect(s.newSessionOpen).toBe(false)
    composer.remove()
  })
})
