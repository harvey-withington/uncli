// The sample host (decision 0026): a <uncli-session-list> and three
// <uncli-chat> elements on UNCLI's mock backend, sharing it, the list
// driving the first chat, and the events they send the page.
import { mockBackend } from '@uncli/client/mock'
import '../src/embed'
import type { UncliChat, UncliSessionList } from '../src/embed'

const backend = mockBackend()
for (const el of document.querySelectorAll<UncliChat | UncliSessionList>('uncli-chat, uncli-session-list')) el.backend = backend

// The list's pick shows in the first chat.
const list = document.getElementById('list') as UncliSessionList
const first = document.getElementById('first') as UncliChat
list.addEventListener('uncli-select', e => {
  const id = (e as CustomEvent<{ session: string }>).detail.session
  first.session = id
  list.selected = id
})

const log = document.getElementById('log') as HTMLElement
for (const type of ['uncli-ready', 'uncli-select', 'uncli-busy', 'uncli-idle', 'uncli-needs-approval']) {
  document.addEventListener(type, e => {
    const detail = (e as CustomEvent<{ session: string }>).detail
    log.textContent += `${type} ${detail.session}\n`
  })
}
