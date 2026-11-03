// The sample host (decision 0026): two <uncli-chat> elements on UNCLI's
// mock backend, sharing it, and the events they send the page.
import { mockBackend } from '@uncli/client/mock'
import '../src/embed'
import type { UncliChat } from '../src/embed'

const backend = mockBackend()
for (const el of document.querySelectorAll<UncliChat>('uncli-chat')) el.backend = backend

const log = document.getElementById('log') as HTMLElement
for (const type of ['uncli-ready', 'uncli-busy', 'uncli-idle', 'uncli-needs-approval']) {
  document.addEventListener(type, e => {
    const detail = (e as CustomEvent<{ session: string }>).detail
    log.textContent += `${type} ${detail.session}\n`
  })
}
