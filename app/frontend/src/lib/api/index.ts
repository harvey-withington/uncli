// The app's API layer: @uncli/client (decision 0026), the Wails backend the
// desktop app runs on, and the choice between them.
import { setClientText, type Backend } from '@uncli/client'
import { mockBackend } from '@uncli/client/mock'
import { t } from '../i18n.svelte'
import { hasWails, wailsBackend } from './wails'

export * from '@uncli/client'

// The client's messages come from the app's strings, in the app's language.
setClientText((key, params) => t(key, params))

// The app runs on Wails; anywhere else (vite dev in a browser,
// screenshots) it runs on the mock. ?mock=setup, ?mock=signin,
// ?mock=empty, ?mock=unattended, ?mock=always or ?mock=never pick a
// starting scenario for the mock.
export function createBackend(): Backend {
  if (hasWails()) return wailsBackend()
  const scenario = new URLSearchParams(location.search).get('mock')
  if (scenario === 'setup') return mockBackend({ cli: { installed: false, loggedIn: false } })
  if (scenario === 'signin') return mockBackend({ cli: { loggedIn: false } })
  if (scenario === 'empty') return mockBackend({ empty: true })
  if (scenario === 'unattended') return mockBackend({ unattended: true })
  if (scenario === 'always' || scenario === 'never') return mockBackend({ mode: scenario })
  return mockBackend()
}
