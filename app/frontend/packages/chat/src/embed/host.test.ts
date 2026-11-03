import { fireEvent, waitFor, within } from '@testing-library/svelte'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { chatClient } from '@uncli/client'
import { mockBackend } from '@uncli/client/mock'
import { setHostText } from '../lib/i18n.svelte'
import { allowLinkSchemes } from '../lib/render/markdown'
import { defineUncliChat, type UncliChat } from './element.svelte'

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
