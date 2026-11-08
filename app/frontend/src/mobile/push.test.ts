import { fireEvent, render, screen, waitFor } from '@testing-library/svelte'
import { describe, expect, it, vi } from 'vitest'
import { mockBackend } from '@uncli/client/mock'
import { APP_CONTEXT } from '../lib/context'
import { AppStore } from '../stores/app.svelte'
import type { Backend } from '@uncli/chat/lib/api'
import { keyBytes, noPushService, pushSupport, type PushEnv } from './push'
import PushSection from './PushSection.svelte'

const ok: PushEnv = { secure: true, serviceWorker: true, pushManager: true, permission: 'default' }

describe('push support', () => {
  it('says why notifications can’t be turned on', () => {
    expect(pushSupport(ok)).toBe('ok')
    expect(pushSupport({ ...ok, secure: false })).toBe('insecure')
    expect(pushSupport({ ...ok, pushManager: false })).toBe('unsupported')
    expect(pushSupport({ ...ok, permission: 'denied' })).toBe('denied')
  })

  it('knows a browser with no push service from other failures', () => {
    expect(noPushService(new DOMException('Registration failed - push service not available', 'AbortError'))).toBe(true)
    expect(noPushService(new DOMException('no', 'NotAllowedError'))).toBe(false)
    expect(noPushService(new Error('network'))).toBe(false)
  })

  it('reads the server’s key', () => {
    expect(Array.from(keyBytes('AQID_w'))).toEqual([1, 2, 3, 255])
  })
})

// PushSection inside an app store, as MobileSettings has it.
function renderSection(backend: Backend, env: PushEnv) {
  return render(PushSection, { props: { env }, context: new Map([[APP_CONTEXT, new AppStore(backend)]]) })
}

describe('PushSection', () => {
  it('stays visible, disabled with why, where notifications can’t work', async () => {
    renderSection(mockBackend(), { ...ok, secure: false })
    expect(await screen.findByRole('button', { name: 'Turn on notifications' })).toBeDisabled()
    expect(screen.getByText(/need the app opened over HTTPS/)).toBeInTheDocument()
  })

  it('once on, sets what this phone is told and sends a test', async () => {
    const b = mockBackend()
    await b.pushSubscribe({ endpoint: 'https://fcm.googleapis.com/x', keys: { p256dh: 'a', auth: 'b' } })
    const set = vi.spyOn(b, 'setPushNotifications')
    const test = vi.spyOn(b, 'testPush')
    renderSection(b, ok)
    const select = await screen.findByRole('combobox', { name: 'Notifications' })
    await fireEvent.change(select, { target: { value: 'approvals' } })
    await waitFor(() => expect(set).toHaveBeenCalledWith('approvals'))
    await fireEvent.click(screen.getByRole('button', { name: 'Show a test' }))
    await waitFor(() => expect(test).toHaveBeenCalled())
    expect(screen.getByRole('button', { name: 'Turn off on this phone' })).toBeEnabled()
  })
})
