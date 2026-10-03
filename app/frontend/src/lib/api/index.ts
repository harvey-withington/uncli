import { mockBackend } from './mock'
import type { Backend } from './types'
import { hasWails, wailsBackend } from './wails'

export type * from './types'

// The app runs on Wails; anywhere else (vite dev in a browser,
// screenshots) it runs on the mock. ?mock=setup, ?mock=signin,
// ?mock=empty or ?mock=unattended pick a starting scenario for the mock.
export function createBackend(): Backend {
  if (hasWails()) return wailsBackend()
  const scenario = new URLSearchParams(location.search).get('mock')
  if (scenario === 'setup') return mockBackend({ cli: { installed: false, loggedIn: false } })
  if (scenario === 'signin') return mockBackend({ cli: { loggedIn: false } })
  if (scenario === 'empty') return mockBackend({ empty: true })
  if (scenario === 'unattended') return mockBackend({ unattended: true })
  return mockBackend()
}
