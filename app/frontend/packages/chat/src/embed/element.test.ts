import { waitFor, within } from '@testing-library/svelte'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { chatClient } from '@uncli/client'
import { mockBackend } from '@uncli/client/mock'
import { defineUncliChat, type UncliChat } from './element.svelte'

defineUncliChat()

const inside = (el: Element) => within(el.shadowRoot as unknown as HTMLElement)

function chat(backend: ReturnType<typeof mockBackend>, session: string, attrs: Record<string, string> = {}): UncliChat {
  const el = document.createElement('uncli-chat') as UncliChat
  for (const [k, v] of Object.entries(attrs)) el.setAttribute(k, v)
  el.session = session
  el.backend = backend
  document.body.append(el)
  return el
}

afterEach(() => {
  document.body.innerHTML = ''
  localStorage.clear()
})

describe('<uncli-chat>', () => {
  it('shows its session in its own shadow root, and tells the page when it’s ready', async () => {
    const backend = mockBackend()
    const ready = vi.fn()
    document.addEventListener('uncli-ready', ready)
    const el = chat(backend, 's-code')
    expect(await inside(el).findByText('Why does TestMultiTurnPartial fail about one run in five?')).toBeInTheDocument()
    // Nothing of UNCLI's is in the page itself.
    expect(document.body.querySelector('.pane')).toBeNull()
    await waitFor(() => expect(ready).toHaveBeenCalledWith(expect.objectContaining({ detail: { session: 's-code' } })))
    document.removeEventListener('uncli-ready', ready)
  })

  it('shares one client between chats on the same backend, each showing its own session', async () => {
    const backend = mockBackend()
    const subscribe = vi.spyOn(backend, 'subscribe')
    const a = chat(backend, 's-code')
    const b = chat(backend, 's-chat')
    expect(await inside(a).findByText('Why does TestMultiTurnPartial fail about one run in five?')).toBeInTheDocument()
    expect(await inside(b).findByRole('heading', { name: 'Plan a weekend in Lisbon' })).toBeInTheDocument()
    expect(inside(b).queryByText('Why does TestMultiTurnPartial fail about one run in five?')).toBeNull()
    expect(subscribe).toHaveBeenCalledTimes(1)
    // One chat going doesn't stop the other's client.
    a.remove()
    expect(chatClient(backend).snapshot().ready).toBe(true)
  })

  it('takes its scheme from an attribute, and remembers its layout only under a storage key', async () => {
    const plain = chat(mockBackend(), 's-code', { scheme: 'dark' })
    expect(plain.getAttribute('data-uncli-scheme')).toBe('dark')
    await inside(plain).findByText('Why does TestMultiTurnPartial fail about one run in five?')
    expect(Object.keys(localStorage).filter(k => k.startsWith('uncli-outline') || k.startsWith('uncli-layout'))).toEqual([])
    plain.removeAttribute('scheme')
    expect(plain.hasAttribute('data-uncli-scheme')).toBe(false)
  })

  it('shows the phone’s compact layout when asked, without a Back button, and the desktop’s again when not', async () => {
    const el = chat(mockBackend(), 's-code', { compact: '' })
    await inside(el).findByText('Why does TestMultiTurnPartial fail about one run in five?')
    expect(el.shadowRoot!.querySelector('section.session')).not.toBeNull()
    expect(el.shadowRoot!.querySelector('section.pane')).toBeNull()
    expect(inside(el).queryByRole('button', { name: 'Back' })).toBeNull()
    expect(inside(el).getByRole('button', { name: 'Options' })).toBeInTheDocument()
    el.setAttribute('compact', 'false')
    await waitFor(() => expect(el.shadowRoot!.querySelector('section.pane')).not.toBeNull())
  })

  it('puts the host’s own controls in its slots, and names its parts, in both layouts', async () => {
    for (const compact of ['false', '']) {
      const el = document.createElement('uncli-chat') as UncliChat
      el.setAttribute('compact', compact)
      el.innerHTML = '<button slot="header-extra">Pending edits</button><p slot="below-composer">Model: house</p>'
      el.session = 's-code'
      el.backend = mockBackend()
      document.body.append(el)
      await inside(el).findByText('Why does TestMultiTurnPartial fail about one run in five?')
      const slot = (name: string) => el.shadowRoot!.querySelector<HTMLSlotElement>(`slot[name="${name}"]`)!
      expect(slot('header-extra').assignedElements().map(e => e.textContent)).toEqual(['Pending edits'])
      expect(slot('below-composer').assignedElements().map(e => e.textContent)).toEqual(['Model: house'])
      for (const part of ['header', 'page', 'composer', 'navbar']) expect(el.shadowRoot!.querySelector(`[part~="${part}"]`), `${part} (compact="${compact}")`).not.toBeNull()
      el.remove()
    }
  })

  it('writes nothing to the page’s storage: a draft stays in memory', async () => {
    const el = chat(mockBackend(), 's-code')
    await inside(el).findByText('Why does TestMultiTurnPartial fail about one run in five?')
    const box = el.shadowRoot!.querySelector('textarea')!
    box.value = 'half a thought'
    box.dispatchEvent(new Event('input', { bubbles: true }))
    await new Promise(r => setTimeout(r, 600))
    expect(Object.keys(localStorage).filter(k => k.startsWith('uncli'))).toEqual([])
  })

  it('switches session when its attribute changes', async () => {
    const el = chat(mockBackend(), 's-code')
    await inside(el).findByText('Why does TestMultiTurnPartial fail about one run in five?')
    el.session = 's-chat'
    expect(await inside(el).findByRole('heading', { name: 'Plan a weekend in Lisbon' })).toBeInTheDocument()
  })
})
