// The window's backend: the Wails binding the desktop app runs on, or, in a
// plain browser (vite dev, screenshots), the mock. ?mock=setup,
// ?mock=signin, ?mock=empty, ?mock=unattended, ?mock=always or
// ?mock=never pick a starting scenario for the mock (?peers=on pairs it
// with two servers). Either way the window shows this PC's sessions and
// its paired servers' (decision 0031).
import { combinedBackend } from '@uncli/client'
import { mockBackend } from '@uncli/client/mock'
import type { Backend } from '@uncli/chat/lib/api'
import { hasWails, wailsBackend } from './wails'

export function createBackend(): Backend {
  return combinedBackend(baseBackend())
}

function baseBackend(): Backend {
  if (hasWails()) return wailsBackend()
  const scenario = new URLSearchParams(location.search).get('mock')
  if (scenario === 'setup') return mockBackend({ cli: { installed: false, loggedIn: false } })
  if (scenario === 'signin') return mockBackend({ cli: { loggedIn: false } })
  if (scenario === 'empty') return mockBackend({ empty: true })
  if (scenario === 'unattended') return mockBackend({ unattended: true })
  if (scenario === 'always' || scenario === 'never') return mockBackend({ mode: scenario })
  return mockBackend()
}
