import { mount } from 'svelte'
import '../app.css'
import { serverKey, withServer } from '../lib/api/identity'
import { mockBackend } from '../lib/api/mock'
import { applyTheme } from '../lib/theme.svelte'
import MobileApp from './MobileApp.svelte'
import { followViewport } from './viewport'

applyTheme()

const target = document.getElementById('app')
if (!target) throw new Error('#app missing')
followViewport(target)

// ?mock runs the phone app on the browser mock (screenshots, vite dev).
const mock = new URLSearchParams(location.search).has('mock')
export default mount(MobileApp, { target, props: { backend: mock ? withServer(mockBackend(), serverKey(location.origin)) : undefined } })

// The service worker keeps the app's shell for when the server can't be
// reached, and shows push notifications (decision 0021).
if ('serviceWorker' in navigator && !mock) {
  const build = typeof __BUILD_ID__ === 'string' ? __BUILD_ID__ : 'dev'
  void navigator.serviceWorker.register(`./sw.js?v=${build}`, { scope: './' }).catch(() => {})
}
