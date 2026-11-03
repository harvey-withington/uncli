import { fireEvent, waitFor, within } from '@testing-library/svelte'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { mockBackend } from '@uncli/client/mock'
import { defineUncliChat, type UncliChat } from './element.svelte'
import { defineUncliSessionList, type UncliSessionList } from './list.svelte'

defineUncliChat()
defineUncliSessionList()

const inside = (el: Element) => within(el.shadowRoot as unknown as HTMLElement)

afterEach(() => {
  document.body.innerHTML = ''
  localStorage.clear()
})

describe('<uncli-session-list>', () => {
  it('lists the sessions; picking one tells the host, which shows it in a chat on the same client', async () => {
    const backend = mockBackend()
    const subscribe = vi.spyOn(backend, 'subscribe')
    const list = document.createElement('uncli-session-list') as UncliSessionList
    const chat = document.createElement('uncli-chat') as UncliChat
    list.backend = backend
    chat.backend = backend
    chat.session = 's-code'
    list.selected = 's-code'
    document.body.append(list, chat)
    // The host: the list's pick shows that session in the chat.
    list.addEventListener('uncli-select', e => {
      const id = (e as CustomEvent<{ session: string }>).detail.session
      chat.session = id
      list.selected = id
    })

    const items = inside(list)
    const lisbon = await items.findByRole('button', { name: /Plan a weekend in Lisbon/ })
    expect(items.getByRole('button', { name: /Fix the flaky parser test/ })).toHaveAttribute('aria-current', 'true')
    await fireEvent.click(lisbon)
    expect(await inside(chat).findByRole('heading', { name: 'Plan a weekend in Lisbon' })).toBeInTheDocument()
    await waitFor(() => expect(items.getByRole('button', { name: /Plan a weekend in Lisbon/ })).toHaveAttribute('aria-current', 'true'))
    expect(subscribe).toHaveBeenCalledTimes(1)
  })

  it('shows the archived sessions apart, on request', async () => {
    const list = document.createElement('uncli-session-list') as UncliSessionList
    list.backend = mockBackend()
    document.body.append(list)
    const toggle = await inside(list).findByRole('button', { name: /Archived/ })
    expect(toggle).toHaveAttribute('aria-expanded', 'false')
    await fireEvent.click(toggle)
    expect(inside(list).getByRole('list', { name: 'Archived sessions' })).toBeInTheDocument()
  })
})
