import { fireEvent, waitFor, within } from '@testing-library/svelte'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { chatClient, mountedBackend } from '@uncli/client'
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
