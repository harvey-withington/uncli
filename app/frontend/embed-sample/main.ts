// The sample host (decision 0026): a <uncli-session-list> and three
// <uncli-chat> elements on UNCLI's mock backend, sharing it, the list
// driving the first chat, and the events they send the page. Opened with
// ?src=/uncli (by uncli-sample-host, say), they use the host's own UNCLI
// at that path instead (decision 0027), and New chat starts a session.
import { mockBackend } from '@uncli/client/mock'
import { mountedBackend, type ChatHost, type UncliChat, type UncliSessionList } from '@uncli/chat/embed'

const src = new URLSearchParams(location.search).get('src')
const backend = src ? mountedBackend(src) : mockBackend()
for (const el of document.querySelectorAll<UncliChat | UncliSessionList>('uncli-chat, uncli-session-list')) {
  if (src) {
    el.removeAttribute(el.localName === 'uncli-chat' ? 'session' : 'selected')
    el.setAttribute('src', src)
  } else el.backend = backend
}

const newChat = document.getElementById('new-chat') as HTMLButtonElement
newChat.hidden = !src
newChat.addEventListener('click', async () => {
  list.selected = await first.newSession()
})

// The list's pick shows in the first chat.
const list = document.getElementById('list') as UncliSessionList
const first = document.getElementById('first') as UncliChat
list.addEventListener('uncli-select', e => {
  const id = (e as CustomEvent<{ session: string }>).detail.session
  first.session = id
  list.selected = id
})

const log = document.getElementById('log') as HTMLElement
for (const type of ['uncli-ready', 'uncli-select', 'uncli-busy', 'uncli-idle', 'uncli-needs-approval', 'uncli-missing', 'uncli-deleted', 'uncli-new-session']) {
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
// Its own controls in the message box: a mode, picked from a drop-down,
// and a switch that moves on each click. The host owns the values, so it
// sets host again when one changes.
let mode = 'chat'
let length = 'brief'
const modes = [
  { id: 'chat', label: 'Chat', icon: 'message-circle', tone: 'success' as const, hint: "Can't change the board" },
  { id: 'suggest', label: 'Suggest', icon: 'list-checks', tone: 'warning' as const, hint: 'Changes wait for your review' },
  { id: 'edit', label: 'Edit', icon: 'pencil-line', tone: 'danger' as const, hint: 'Changes apply at once' },
]
const host = (): ChatHost => ({
  complete: query => things.filter(x => x.label.toLowerCase().includes(query.toLowerCase())),
  turnContext: () => `The user has the card "Lisbon trip" open, in ${mode} mode.`,
  openLink: href => {
    log.textContent += `host opens ${href}\n`
    return href.startsWith('sample:')
  },
  composerControls: [
    { id: 'mode', label: 'Mode', value: mode, options: modes, onChange: id => ((mode = id), (second.host = host())) },
    {
      kind: 'toggle',
      id: 'length',
      label: 'Answers',
      value: length,
      states: [
        { id: 'brief', label: 'Brief', icon: 'shrink' },
        { id: 'full', label: 'In full', icon: 'list-checks', tone: 'accent' as const },
      ],
      onChange: id => ((length = id), (second.host = host())),
    },
  ],
})
second.host = host()

// Suggest mode's one review of a turn's staged changes, in the dark chat.
document.getElementById('review')!.addEventListener('click', async () => {
  const { action, items } = await second.review({
    title: 'Apply 3 suggested changes to Lisbon trip?',
    detail: 'From this turn.',
    items: [
      { id: 'c1', label: 'Rename “Flights” to “Flights booked”' },
      { id: 'c2', label: 'Add “Passport” to Preparation to Leave', detail: 'Checklist item' },
      { id: 'c3', label: 'Move “Hotel” to Done' },
    ],
  })
  log.textContent += `review: ${action} ${items.join(', ')}\n`
})
