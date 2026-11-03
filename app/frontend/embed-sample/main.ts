// The sample host (decision 0026): a <uncli-session-list> and three
// <uncli-chat> elements on UNCLI's mock backend, sharing it, the list
// driving the first chat, and the events they send the page.
import { mockBackend } from '@uncli/client/mock'
import '@uncli/chat/embed'
import type { UncliChat, UncliSessionList } from '@uncli/chat/embed'

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

// The dark chat's host offers @mentions of its own things, and says what
// the user is looking at with each turn.
const second = document.getElementById('second') as UncliChat
const things = [
  { label: 'Lisbon trip', detail: 'card', insert: '[Lisbon trip](sample:card:1)' },
  { label: 'Lisbon budget', detail: 'sheet', insert: '[Lisbon budget](sample:sheet:7)' },
  { label: 'Packing list', detail: 'card', insert: '[Packing list](sample:card:2)' },
]
second.setAttribute('link-schemes', 'sample')
second.host = {
  complete: query => things.filter(x => x.label.toLowerCase().includes(query.toLowerCase())),
  turnContext: () => 'The user has the card "Lisbon trip" open.',
  openLink: href => {
    log.textContent += `host opens ${href}\n`
    return href.startsWith('sample:')
  },
}
