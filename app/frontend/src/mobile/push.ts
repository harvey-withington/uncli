// Push on the phone (decision 0021): whether this browser can get
// notifications from UNCLI, and turning them on and off. The browser's
// subscription goes to the server, which pushes to it when a session
// needs approval or finishes; the service worker (m/public/sw.js) shows it.
import type { Backend, PushSubscriptionInfo } from '@uncli/chat/lib/api'

// noService: the browser has the API but no push service behind it (the
// Meta Quest's), found only when subscribing fails.
export type PushSupport = 'ok' | 'insecure' | 'unsupported' | 'denied' | 'noService'

export interface PushEnv {
  secure: boolean // HTTPS (or localhost)
  serviceWorker: boolean
  pushManager: boolean
  permission: NotificationPermission | 'none'
}

export function pushEnv(): PushEnv {
  return {
    secure: globalThis.isSecureContext === true,
    serviceWorker: typeof navigator !== 'undefined' && 'serviceWorker' in navigator,
    pushManager: typeof globalThis.PushManager !== 'undefined',
    permission: typeof globalThis.Notification !== 'undefined' ? Notification.permission : 'none',
  }
}

// pushSupport says whether notifications can be turned on here, or why not.
export function pushSupport(env: PushEnv): PushSupport {
  if (!env.secure) return 'insecure'
  if (!env.serviceWorker || !env.pushManager || env.permission === 'none') return 'unsupported'
  if (env.permission === 'denied') return 'denied'
  return 'ok'
}

// keyBytes turns the server's VAPID key (base64url) into what the browser
// subscribes with.
export function keyBytes(key: string): Uint8Array<ArrayBuffer> {
  const b64 = (key + '='.repeat((4 - (key.length % 4)) % 4)).replace(/-/g, '+').replace(/_/g, '/')
  const raw = atob(b64)
  const out = new Uint8Array(new ArrayBuffer(raw.length))
  for (let i = 0; i < raw.length; i++) out[i] = raw.charCodeAt(i)
  return out
}

// noPushService says whether subscribing failed because the browser has no
// push service to subscribe with (Chromium's AbortError, "Registration
// failed - push service not available").
export function noPushService(e: unknown): boolean {
  return e instanceof DOMException && e.name === 'AbortError'
}

export type EnableResult = 'on' | 'refused' | 'noService'

// enablePush asks for permission, subscribes this browser with the
// server's key (replacing a subscription made with another), and tells
// the server.
export async function enablePush(backend: Backend): Promise<EnableResult> {
  if ((await Notification.requestPermission()) !== 'granted') return 'refused'
  const key = await backend.pushKey()
  const reg = await navigator.serviceWorker.ready
  let sub = await reg.pushManager.getSubscription()
  if (sub) await sub.unsubscribe()
  try {
    sub = await reg.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: keyBytes(key) })
  } catch (e) {
    if (noPushService(e)) return 'noService'
    throw e
  }
  await backend.pushSubscribe(sub.toJSON() as PushSubscriptionInfo)
  return 'on'
}

// disablePush drops this browser's subscription, here and at the server.
export async function disablePush(backend: Backend) {
  try {
    const reg = await navigator.serviceWorker.ready
    await (await reg.pushManager.getSubscription())?.unsubscribe()
  } finally {
    await backend.pushUnsubscribe()
  }
}
