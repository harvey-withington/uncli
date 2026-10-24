// UNCLI's phone app service worker (decision 0021). It keeps the app's
// shell, so the app opens and can say the server is out of reach when it
// is, and it shows push notifications and opens the session one is about.
// It never caches or touches the API: /rpc, /events, /upload, /pair and
// signed links always go to the server.

const SHELL = 'uncli-shell-v1'

self.addEventListener('install', event => {
  event.waitUntil(caches.open(SHELL).then(c => c.addAll(['./', './manifest.webmanifest', './icon-192.png'])).then(() => self.skipWaiting()))
})

self.addEventListener('activate', event => {
  event.waitUntil(
    caches.keys()
      .then(keys => Promise.all(keys.filter(k => k !== SHELL).map(k => caches.delete(k))))
      .then(() => self.clients.claim()),
  )
})

self.addEventListener('fetch', event => {
  const req = event.request
  const url = new URL(req.url)
  if (req.method !== 'GET' || url.origin !== self.location.origin || !url.pathname.startsWith('/m/')) return
  if (req.mode === 'navigate') {
    // The page: the server's when it answers, else the kept one.
    event.respondWith(
      fetch(req)
        .then(res => {
          const copy = res.clone()
          void caches.open(SHELL).then(c => c.put('./', copy))
          return res
        })
        .catch(() => caches.match('./').then(r => r ?? Response.error())),
    )
    return
  }
  if (url.pathname.startsWith('/m/assets/')) {
    // Built assets have hashed names: kept once fetched.
    event.respondWith(
      caches.match(req).then(hit => hit ?? fetch(req).then(res => {
        if (res.ok) {
          const copy = res.clone()
          void caches.open(SHELL).then(c => c.put(req, copy))
        }
        return res
      })),
    )
  }
})

// Push (decision 0021): {title, body, session, tag}. Shown even with the
// app closed; tapping it opens the session.
self.addEventListener('push', event => {
  let data = {}
  try {
    data = event.data ? event.data.json() : {}
  } catch {
    data = { body: event.data ? event.data.text() : '' }
  }
  const title = data.title || 'UNCLI'
  event.waitUntil(
    self.registration.showNotification(title, {
      body: data.body || '',
      tag: data.tag || data.session || 'uncli',
      renotify: true,
      icon: './icon-192.png',
      badge: './icon-192.png',
      data: { session: data.session || '' },
    }),
  )
})

self.addEventListener('notificationclick', event => {
  event.notification.close()
  const session = event.notification.data && event.notification.data.session
  const target = new URL(session ? `./#s=${encodeURIComponent(session)}` : './', self.registration.scope).href
  event.waitUntil(
    self.clients.matchAll({ type: 'window', includeUncontrolled: true }).then(list => {
      for (const c of list) {
        if (c.url.startsWith(self.registration.scope)) {
          c.navigate(target)
          return c.focus()
        }
      }
      return self.clients.openWindow(target)
    }),
  )
})
