// Files waiting in the message box to go with the next turn, and how they
// look. A file from disk travels as its path (the app reads it); a file a
// remote client sent to its server travels as the upload's id; a pasted
// image has no file behind it, so it travels as data.
import type { AttachmentKind, AttachmentRef } from './api'

export interface Pending {
  key: string
  name: string
  size: number
  kind: AttachmentKind
  path?: string
  upload?: string // an upload's id, on the server
  mediaType?: string
  data?: string // base64, pasted images only
  preview?: string // a data: or blob: URL for an image thumbnail
}

let n = 0
export const pendingKey = () => `att-${++n}`

// releasePreviews frees the blob: URLs of thumbnails no longer shown (an
// uploaded photo's); data: URLs need nothing.
export function releasePreviews(list: readonly Pending[]) {
  for (const a of list) if (a.preview?.startsWith('blob:')) URL.revokeObjectURL(a.preview)
}

// toRefs is how each file travels. The name and media type ride along, so
// a queued message taken back (fromRefs) shows its files as they were.
export function toRefs(list: readonly Pending[]): AttachmentRef[] {
  return list.map(a => {
    const about = { name: a.name, ...(a.mediaType ? { mediaType: a.mediaType } : {}) }
    return a.upload ? { upload: a.upload, ...about } : a.path ? { path: a.path, ...about } : { ...about, data: a.data }
  })
}

// fromRefs makes files to show again from how they travelled: a queued
// message taken back to edit. Their sizes aren't known (0: not shown).
export function fromRefs(refs: readonly AttachmentRef[]): Pending[] {
  return refs.map(r => {
    const name = r.name || (r.path ?? '').replace(/^.*[\\/]/, '') || 'file'
    const media = r.mediaType ?? ''
    const kind: AttachmentKind = media.startsWith('image/') || /\.(png|jpe?g|gif|webp)$/i.test(name) ? 'image' : media === 'application/pdf' || /\.pdf$/i.test(name) ? 'pdf' : 'text'
    const preview = r.data && media.startsWith('image/') ? `data:${media};base64,${r.data}` : undefined
    return { key: pendingKey(), name, size: 0, kind, path: r.path, upload: r.upload, mediaType: r.mediaType, data: r.data, preview }
  })
}

// The longest side a photo is sent at: plenty for a model to read, and
// well under the 5 MB image limit as a JPEG.
export const PHOTO_MAX = 2048

// fitSize scales w × h down (never up) so its longer side is at most max.
export function fitSize(w: number, h: number, max = PHOTO_MAX): { w: number; h: number } {
  const k = Math.min(1, max / Math.max(w, h, 1))
  return { w: Math.round(w * k), h: Math.round(h * k) }
}

// shrinkPhoto makes a phone's photo small enough to send: an image larger
// than PHOTO_MAX on a side, or over 4 MB, is redrawn as a JPEG that fits.
// Anything else (and any image the browser can't draw) goes as it is.
export async function shrinkPhoto(file: File): Promise<Blob> {
  if (!file.type.startsWith('image/') || file.type === 'image/gif' || typeof createImageBitmap !== 'function') return file
  try {
    const bmp = await createImageBitmap(file)
    const { w, h } = fitSize(bmp.width, bmp.height)
    if (w === bmp.width && file.size <= 4 << 20) return file
    const canvas = document.createElement('canvas')
    canvas.width = w
    canvas.height = h
    canvas.getContext('2d')?.drawImage(bmp, 0, 0, w, h)
    bmp.close()
    return await new Promise<Blob>(resolve => canvas.toBlob(b => resolve(b ?? file), 'image/jpeg', 0.85))
  } catch {
    return file
  }
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
