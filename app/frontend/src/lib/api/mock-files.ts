// The server's folders and uploads, for the browser mock: a small folder
// tree under C:\Users\you, and uploads that remember only their name.
import type { AttachmentKind, FolderListing, Upload } from './types'

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
    async artifactLink(): Promise<string> { return 'about:blank' },
    async uploadLink(): Promise<string> { return 'about:blank' },
  }
}
