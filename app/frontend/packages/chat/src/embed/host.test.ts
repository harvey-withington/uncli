import { fireEvent, waitFor, within } from '@testing-library/svelte'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { chatClient, httpBackend, mountedBackend } from '@uncli/client'
import { canSetUp } from '../lib/remoteonly'
import { mockBackend } from '@uncli/client/mock'
import { setHostText } from '../lib/i18n.svelte'
import { allowLinkSchemes } from '../lib/render/markdown'
import { defineUncliChat, type UncliChat } from './element.svelte'
import { defineUncliSessionList, type UncliSessionList } from './list.svelte'

defineUncliChat()

const inside = (el: Element) => within(el.shadowRoot as unknown as HTMLElement)
const QUESTION = 'Why does TestMultiTurnPartial fail about one run in five?'

afterEach(() => {
  document.body.innerHTML = ''
  setHostText(null)
  allowLinkSchemes([])
})

function chat(backend = mockBackend(), attrs: Record<string, string> = {}): UncliChat {
  const el = document.createElement('uncli-chat') as UncliChat
  for (const [k, v] of Object.entries(attrs)) el.setAttribute(k, v)
  el.session = 's-code'
  el.backend = backend
  document.body.append(el)
  return el
}

describe('src', () => {
  it('reaches the host’s own UNCLI at the path its server forwards, one client for the chat and the list', () => {
    defineUncliSessionList()
    const el = document.createElement('uncli-chat') as UncliChat
    const list = document.createElement('uncli-session-list') as UncliSessionList
    el.setAttribute('src', '/uncli')
    list.setAttribute('src', '/uncli/')
    expect(el.backend).toBe(mountedBackend('/uncli'))
    expect(list.backend).toBe(el.backend)
    el.removeAttribute('src')
    expect(el.backend).toBeNull()
  })
})

describe('the chat’s own life in a host', () => {
  it('connects a moment after it gets its backend, so a host set in the same go is in place first', async () => {
    const backend = mockBackend()
    const boot = vi.spyOn(backend, 'bootstrap')
    const el = document.createElement('uncli-chat') as UncliChat
    el.session = 's-code'
    el.backend = backend
    document.body.append(el)
    expect(boot).not.toHaveBeenCalled()
    el.host = { turnContext: () => 'set after the backend' }
    await waitFor(() => expect(boot).toHaveBeenCalled())
  })

  it('says when the session it was given isn’t there', async () => {
    const missing = vi.fn()
    document.addEventListener('uncli-missing', missing)
    try {
      const el = document.createElement('uncli-chat') as UncliChat
      el.session = 'gone'
      el.backend = mockBackend()
      document.body.append(el)
      await waitFor(() => expect(missing).toHaveBeenCalledOnce())
      expect((missing.mock.calls[0]![0] as CustomEvent).detail).toEqual({ session: 'gone' })
    } finally {
      document.removeEventListener('uncli-missing', missing)
    }
  })

  it('deletes its chat from the header, confirmed, and tells the host', async () => {
    const backend = mockBackend()
    const remove = vi.spyOn(backend, 'remove')
    const deleted = vi.fn()
    document.addEventListener('uncli-deleted', deleted)
    try {
      const el = chat(backend)
      await inside(el).findByText(QUESTION)
      await fireEvent.click(inside(el).getByRole('button', { name: 'Delete this chat' }))
      const dialog = await inside(el).findByRole('dialog')
      await fireEvent.click(within(dialog).getByRole('button', { name: 'Delete' }))
      await waitFor(() => expect(deleted).toHaveBeenCalledOnce())
      expect(remove).toHaveBeenCalledWith('s-code')
      expect((deleted.mock.calls[0]![0] as CustomEvent).detail).toEqual({ session: 's-code' })
    } finally {
      document.removeEventListener('uncli-deleted', deleted)
    }
  })

  it('starts a new chat from the header unless the host takes it', async () => {
    const created = vi.fn()
    document.addEventListener('uncli-new-session', created)
    try {
      const el = chat()
      await inside(el).findByText(QUESTION)
      // A host that takes it: nothing made here.
      const take = (e: Event) => e.preventDefault()
      document.addEventListener('uncli-new-chat', take)
      await fireEvent.click(inside(el).getByRole('button', { name: 'New chat' }))
      document.removeEventListener('uncli-new-chat', take)
      expect(created).not.toHaveBeenCalled()
      expect(el.session).toBe('s-code')
      // Otherwise: a new session of the same type, shown, and the host told.
      await fireEvent.click(inside(el).getByRole('button', { name: 'New chat' }))
      await waitFor(() => expect(created).toHaveBeenCalledOnce())
      const detail = (created.mock.calls[0]![0] as CustomEvent).detail
      expect(detail.previous).toBe('s-code')
      expect(el.session).toBe(detail.session)
    } finally {
      document.removeEventListener('uncli-new-session', created)
    }
  })
})

describe('a host’s own approval prompts', () => {
  // HostToolPolicy asked, with its words for the card (decision 0027).
  const hostCard = {
    requestId: 'h1',
    tool: 'mcp__board__update_checklist',
    input: { checklist: 'Preparation to Leave', add: ['Passport'] },
    learn: [{ kind: 'tool', words: 'mcp__board__update_checklist' }],
    why: [{ by: 'host', note: 'Suggest mode' }],
    askedAt: Date.now(),
    host: { title: "Update checklist 'Preparation to Leave'", detail: 'Adds: Passport\nTicks: Book taxi' },
  }

  for (const compact of ['false', '']) {
    it(`shows the host’s words on the card in the ${compact === '' ? 'compact' : 'desktop'} layout, and answers it like any card`, async () => {
      const backend = mockBackend()
      const answer = vi.spyOn(backend, 'answerApproval')
      const needs = vi.fn()
      document.addEventListener('uncli-needs-approval', needs)
      try {
        const el = chat(backend, { compact })
        await inside(el).findByText(QUESTION)
        const client = chatClient(backend)
        const view = client.session('s-code').snapshot().view!
        client.upsertSession({ ...view, approvals: [hostCard] as unknown as typeof view.approvals })
        const card = await inside(el).findByRole('alertdialog', { name: "Update checklist 'Preparation to Leave'" })
        expect(card.textContent).toContain('Adds: Passport')
        expect(card.textContent).toContain('Ticks: Book taxi')
        // What it does is in the host's words, not the tool's raw name and
        // arguments; "This is safe" still says what it would remember, in
        // UNCLI's scoping.
        const what = card.querySelector('.what')!.textContent ?? ''
        expect(what).not.toContain('update_checklist')
        expect(what).not.toContain('{')
        expect(card.textContent).toContain('Will remember as safe: update_checklist (board)')
        await waitFor(() => expect(needs).toHaveBeenCalled())
        if (compact === '') {
          // The folded header counts it.
          expect(inside(el).getByRole('button', { name: /^Options \(\d+ needs? attention\)$/ })).toBeInTheDocument()
        }
        await fireEvent.click(within(card).getByRole('button', { name: /Allow once/ }))
        expect(answer).toHaveBeenCalledWith('s-code', 'h1', 'allow')
      } finally {
        document.removeEventListener('uncli-needs-approval', needs)
      }
    })
  }
})

describe('a host’s controls in the message box', () => {
  const modes = [
    { id: 'chat', label: 'Chat', icon: 'message-circle', tone: 'success' as const, hint: "Can't change the board" },
    { id: 'suggest', label: 'Suggest', icon: 'list-checks', tone: 'warning' as const },
    { id: 'edit', label: 'Edit', icon: 'pencil-line', tone: 'danger' as const },
  ]

  it('draws a drop-down for several options, and tells the host the pick', async () => {
    const el = chat()
    const onChange = vi.fn()
    el.host = { composerControls: [{ id: 'mode', label: 'Mode', value: 'chat', options: modes, onChange }] }
    await inside(el).findByText(QUESTION)
    const button = inside(el).getByRole('button', { name: 'Mode: Chat' })
    expect(button).toHaveAttribute('aria-haspopup', 'menu')
    await fireEvent.click(button)
    await fireEvent.click(inside(el).getByRole('menuitemradio', { name: /Edit/ }))
    expect(onChange).toHaveBeenCalledWith('edit')
    // The host owns the value: it shows once the host sets it.
    el.host = { composerControls: [{ id: 'mode', label: 'Mode', value: 'edit', options: modes, onChange }] }
    expect(await inside(el).findByRole('button', { name: 'Mode: Edit' })).toBeInTheDocument()
  })

  it('moves a toggle to its next state on each click', async () => {
    const el = chat()
    const onChange = vi.fn()
    el.host = { composerControls: [{ kind: 'toggle', id: 'mode', label: 'Mode', value: 'suggest', states: modes, onChange }] }
    await inside(el).findByText(QUESTION)
    await fireEvent.click(inside(el).getByRole('button', { name: 'Mode: Suggest' }))
    expect(onChange).toHaveBeenCalledWith('edit')
  })

  it('draws a single option, or a plain action, as a button with no drop-down', async () => {
    const el = chat()
    const onChange = vi.fn()
    const onClick = vi.fn()
    el.host = {
      composerControls: [
        { id: 'mode', options: modes.slice(0, 1), onChange },
        { id: 'review', label: 'Review edits', icon: 'list-checks', onClick },
      ],
    }
    await inside(el).findByText(QUESTION)
    const only = inside(el).getByRole('button', { name: 'Chat' })
    expect(only).not.toHaveAttribute('aria-haspopup')
    await fireEvent.click(only)
    expect(onChange).toHaveBeenCalledWith('chat')
    await fireEvent.click(inside(el).getByRole('button', { name: 'Review edits' }))
    expect(onClick).toHaveBeenCalled()
  })

  it('makes room for anything the host puts in the composer-controls slot', async () => {
    const el = chat()
    await inside(el).findByText(QUESTION)
    expect(el.shadowRoot!.querySelector('slot[name="composer-controls"]')).toBeNull()
    const mine = document.createElement('button')
    mine.slot = 'composer-controls'
    mine.textContent = 'My own'
    el.append(mine)
    await waitFor(() => expect(el.shadowRoot!.querySelector('slot[name="composer-controls"]')).not.toBeNull())
  })
})

describe('the folded header', () => {
  it('marks Options when something inside wants the user, and says so', async () => {
    const el = chat(mockBackend({ unattended: true }), { compact: '' })
    await inside(el).findByText(QUESTION)
    // Unattended, and the mock's MCP servers that need signing in.
    const options = inside(el).getByRole('button', { name: /^Options \(\d+ needs? attention\)$/ })
    expect(options.querySelector('.dot')).not.toBeNull()
    await fireEvent.click(options)
    // Open, the option shows for itself.
    expect(inside(el).getByRole('button', { name: 'Options' }).querySelector('.dot')).toBeNull()
  })
})

describe('setting up the CLI', () => {
  it('shows the setup steps instead of the chat until the CLI is installed and signed in', async () => {
    const backend = mockBackend({ cli: { installed: false, loggedIn: false } })
    const el = chat(backend)
    const button = (name: string) => [...el.shadowRoot!.querySelectorAll('button')].find(b => b.textContent?.includes(name))
    await waitFor(() => expect(el.shadowRoot!.textContent).toContain("Let's get you set up"))
    await fireEvent.click(button('Download')!)
    await waitFor(() => expect(button('Sign in')).toBeTruthy(), { timeout: 3000 })
    expect(el.shadowRoot!.textContent).not.toContain(QUESTION)
  })

  it('on a host’s UNCLI, opens the sign-in page on a click, through the host’s openLink first', async () => {
    const mock = mockBackend({ cli: { loggedIn: false } })
    const backend = { ...mock, kind: 'http' as const, canSetUp: true }
    const opened = vi.spyOn(backend, 'openURL')
    const el = chat(backend)
    const openLink = vi.fn(() => true)
    el.host = { openLink }
    const button = (name: string) => [...el.shadowRoot!.querySelectorAll('button')].find(b => b.textContent?.includes(name))
    await waitFor(() => expect(button('Sign in')).toBeTruthy())
    await fireEvent.click(button('Sign in')!)
    await waitFor(() => expect(button('Open the sign-in page')).toBeTruthy())
    await fireEvent.click(button('Open the sign-in page')!)
    expect(openLink).toHaveBeenCalledWith(expect.stringMatching(/^https:\/\//))
    expect(opened).not.toHaveBeenCalled()
  })

  it('is offered by a host’s own UNCLI, and never on a phone', () => {
    expect(canSetUp(mountedBackend('/uncli'))).toBe(true)
    expect(canSetUp(httpBackend({ base: 'https://pc.example:9871', token: () => 'tok' }))).toBe(false)
    expect(canSetUp(mockBackend())).toBe(true)
  })
})

describe('the host adapter', () => {
  it('hands the host its own links from answers, kept by link-schemes, and opens nothing itself', async () => {
    const backend = mockBackend()
    const openLink = vi.fn((href: string) => href.startsWith('bruv:'))
    const opened = vi.spyOn(backend, 'openURL')
    const el = chat(backend, { 'link-schemes': 'bruv javascript' })
    el.host = { openLink }
    await inside(el).findByText(QUESTION)
    const client = chatClient(backend)
    const page = client.session('s-code').snapshot().pages![0]!
    client.upsertPage({ ...page, answerMd: 'See [the card](bruv:card:42) and [a script](javascript:alert(1)).' })
    const links = () => [...el.shadowRoot!.querySelectorAll<HTMLAnchorElement>('.answer a')]
    await waitFor(() => expect(links().map(a => a.textContent)).toContain('the card'))
    const link = links().find(a => a.textContent === 'the card')!
    expect(link).toHaveAttribute('href', 'bruv:card:42')
    // javascript: is never a host's scheme: that link loses its target.
    expect(links().find(a => a.textContent === 'a script')?.getAttribute('href') ?? '').not.toMatch(/javascript/)
    await fireEvent.click(link)
    expect(openLink).toHaveBeenCalledWith('bruv:card:42')
    expect(opened).not.toHaveBeenCalled()
  })

  it('shows the host’s strings for UNCLI’s', async () => {
    const el = document.createElement('uncli-chat') as UncliChat
    el.host = { t: key => (key === 'composer.placeholder' ? 'Frag etwas' : undefined) }
    el.session = 's-code'
    el.backend = mockBackend()
    document.body.append(el)
    await inside(el).findByText(QUESTION)
    expect(el.shadowRoot!.querySelector('textarea')).toHaveAttribute('placeholder', 'Frag etwas')
  })

  it('sends what the host says is in front of the user with each turn, hidden from the chat', async () => {
    const backend = mockBackend()
    const send = vi.spyOn(backend, 'send')
    const el = chat(backend)
    el.host = { turnContext: () => 'Card 42: Offline cards (in progress)' }
    await inside(el).findByText(QUESTION)
    const box = el.shadowRoot!.querySelector('textarea')!
    await fireEvent.input(box, { target: { value: 'What’s left on it?' } })
    await fireEvent.keyDown(box, { key: 'Enter' })
    await waitFor(() => expect(send).toHaveBeenCalledWith('s-code', 'What’s left on it?', [], { context: 'Card 42: Offline cards (in progress)' }))
    const page = chatClient(backend).session('s-code').snapshot().pages!.at(-1)!
    expect(page.question).toBe('What’s left on it?')
    expect(page.directives).toContain('Card 42: Offline cards (in progress)')
  })

  it('offers the host’s @mentions as the user types, and puts the chosen one in the message', async () => {
    const el = chat()
    const complete = vi.fn((q: string) => (q.toLowerCase().startsWith('off') ? [{ label: 'Offline cards', detail: 'card', insert: '[Offline cards](bruv:card:42)' }] : []))
    el.host = { complete }
    await inside(el).findByText(QUESTION)
    const box = el.shadowRoot!.querySelector('textarea')!
    box.focus()
    await fireEvent.input(box, { target: { value: 'Look at @Off' } })
    expect(await inside(el).findByRole('option', { name: /Offline cards/ })).toBeInTheDocument()
    expect(complete).toHaveBeenCalledWith('Off')
    // Enter picks the mention; it doesn't send.
    await fireEvent.keyDown(box, { key: 'Enter' })
    await waitFor(() => expect(box.value).toBe('Look at [Offline cards](bruv:card:42) '))
    expect(inside(el).queryByRole('option', { name: /Offline cards/ })).toBeNull()
  })

  it('lets the host say Enter starts a new line, with Ctrl+Enter to send', async () => {
    const backend = mockBackend()
    const send = vi.spyOn(backend, 'send')
    const el = chat(backend)
    el.host = { enter: 'newline' }
    await inside(el).findByText(QUESTION)
    const box = el.shadowRoot!.querySelector('textarea')!
    expect(box).toHaveAttribute('placeholder', 'Ask anything. Ctrl+Enter to send')
    await fireEvent.input(box, { target: { value: 'Two lines' } })
    await fireEvent.keyDown(box, { key: 'Enter' })
    expect(send).not.toHaveBeenCalled()
    await fireEvent.keyDown(box, { key: 'Enter', ctrlKey: true })
    await waitFor(() => expect(send).toHaveBeenCalledWith('s-code', 'Two lines', []))
  })

  it('asks the host first about Escape and Ctrl+Enter; a host that cancels keeps the key', async () => {
    const el = chat()
    await inside(el).findByText(QUESTION)
    const escapes = vi.fn((e: Event) => e.preventDefault())
    const submits = vi.fn()
    document.addEventListener('uncli-escape', escapes)
    document.addEventListener('uncli-submit', submits)
    const box = inside(el).getByRole('textbox')
    const esc = new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, composed: true, cancelable: true })
    box.dispatchEvent(esc)
    expect(escapes).toHaveBeenCalledOnce()
    expect(esc.defaultPrevented).toBe(true)
    box.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', ctrlKey: true, bubbles: true, composed: true, cancelable: true }))
    await waitFor(() => expect(submits).toHaveBeenCalledOnce())
    document.removeEventListener('uncli-escape', escapes)
    document.removeEventListener('uncli-submit', submits)
  })
})
