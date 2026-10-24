// The server's folders, uploads and push, for the browser mock: a small
// folder tree under C:\Users\you, uploads that remember only their name,
// and a push subscription that's only remembered.
import type { AttachmentKind, FolderListing, NotifySetting, PushState, PushSubscriptionInfo, Upload } from './types'

const TREE: Record<string, string[]> = {
  '': ['C:\\Users\\you', 'C:\\', 'D:\\'],
  'C:\\Users\\you': ['code', 'Documents', 'Downloads'],
  'C:\\Users\\you\\code': ['uncli', 'website', 'notes'],
  'C:\\': ['Users', 'Projects'],
  'D:\\': ['Backups'],
}

const parentOf = (p: string) => {
  if (p === '' || /^[A-Z]:\\$/.test(p)) return ''
  const up = p.slice(0, p.lastIndexOf('\\'))
  return /^[A-Z]:$/.test(up) ? up + '\\' : up
}

const join = (dir: string, name: string) => (dir.endsWith('\\') ? dir + name : `${dir}\\${name}`)

function kindOf(type: string, name: string): AttachmentKind {
  if (type.startsWith('image/')) return 'image'
  if (type === 'application/pdf' || name.endsWith('.pdf')) return 'pdf'
  if (type.startsWith('text/') || /\.(txt|md|json|ts|go|py)$/.test(name)) return 'text'
  return 'unsupported'
}

export function mockFiles() {
  let n = 0
  const push: PushState = { subscribed: false, setting: 'all' }
  return {
    async listFolder(path: string): Promise<FolderListing> {
      const names = TREE[path] ?? []
      return { path, parent: parentOf(path), folders: names.map(name => ({ name, path: path === '' ? name : join(path, name) })) }
    },
    async upload(file: Blob, name: string): Promise<Upload> {
      const kind = kindOf(file.type, name)
      return {
        id: `up${++n}`, path: `uploads\\up${n}\\${name}`, name, size: file.size, kind, mediaType: file.type || undefined,
        reason: kind === 'unsupported' ? 'only images, PDFs and text files can be attached' : undefined,
      }
    },
    // Push: subscribing works, and is remembered for the session.
    async pushKey(): Promise<string> { return 'BMockVapidPublicKeyForTheBrowserMockOnly' },
    async pushState(): Promise<PushState> { return { ...push } },
    async pushSubscribe(sub: PushSubscriptionInfo) { push.subscribed = !!sub.endpoint },
    async pushUnsubscribe() { push.subscribed = false },
    async setPushNotifications(setting: NotifySetting) { push.setting = setting },
    async testPush() {},
    async artifactLink(): Promise<string> { return 'about:blank' },
    async uploadLink(): Promise<string> { return 'about:blank' },
  }
}
