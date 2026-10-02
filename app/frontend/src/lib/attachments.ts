// Files waiting in the message box to go with the next turn, and how they
// look. A file from disk travels as its path (the app reads it); a pasted
// image has no file behind it, so it travels as data.
import type { AttachmentKind, AttachmentRef } from './api'

export interface Pending {
  key: string
  name: string
  size: number
  kind: AttachmentKind
  path?: string
  mediaType?: string
  data?: string // base64, pasted images only
  preview?: string // a data: URL for an image thumbnail
}

let n = 0
export const pendingKey = () => `att-${++n}`

export function toRefs(list: readonly Pending[]): AttachmentRef[] {
  return list.map(a => (a.path ? { path: a.path } : { name: a.name, mediaType: a.mediaType, data: a.data }))
}

// The icon for a file, by kind or media type.
export function attachmentIcon(kindOrMedia: string): string {
  if (kindOrMedia === 'image' || kindOrMedia.startsWith('image/')) return 'image'
  if (kindOrMedia === 'pdf' || kindOrMedia === 'application/pdf') return 'file-text'
  return 'file'
}

export function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

// readImage turns a pasted image (no file behind it) into a pending
// attachment, keeping a thumbnail.
export function readImage(file: File, name: string): Promise<Pending> {
  return new Promise((resolve, reject) => {
    const r = new FileReader()
    r.onerror = () => reject(r.error ?? new Error('could not read the pasted image'))
    r.onload = () => {
      const url = String(r.result)
      resolve({
        key: pendingKey(), name, size: file.size, kind: 'image', mediaType: file.type || 'image/png',
        data: url.slice(url.indexOf(',') + 1), preview: url,
      })
    }
    r.readAsDataURL(file)
  })
}
