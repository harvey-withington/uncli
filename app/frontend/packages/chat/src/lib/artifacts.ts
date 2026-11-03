// The artifact pane's logic: which artifacts exist as of a page (each
// page records the versions its turn made, so paging back pages them back
// too), what kind each is, and the sandboxed documents HTML, SVG and
// Mermaid render in. Model output is untrusted: HTML runs in an iframe
// with an opaque origin and a policy that allows no network at all.
import type { ArtifactFile, Page } from './api'

export type ArtifactKind = 'html' | 'svg' | 'mermaid' | 'markdown' | 'image' | 'text' | 'pdf' | 'other'

const KINDS: Record<string, ArtifactKind> = {
  html: 'html', htm: 'html', svg: 'svg', mmd: 'mermaid', mermaid: 'mermaid', md: 'markdown', markdown: 'markdown',
  png: 'image', jpg: 'image', jpeg: 'image', gif: 'image', webp: 'image', bmp: 'image', ico: 'image', pdf: 'pdf',
}

// Text files shown with highlighting, by extension, with their Shiki language.
const TEXT: Record<string, string> = {
  txt: 'text', log: 'text', csv: 'text', tsv: 'text', json: 'json', yaml: 'yaml', yml: 'yaml', xml: 'xml', toml: 'toml',
  js: 'javascript', mjs: 'javascript', ts: 'typescript', jsx: 'jsx', tsx: 'tsx', css: 'css', py: 'python', go: 'go',
  rs: 'rust', java: 'java', kt: 'kotlin', cs: 'csharp', rb: 'ruby', php: 'php', sh: 'bash', ps1: 'powershell', sql: 'sql',
  c: 'c', h: 'c', cpp: 'cpp', swift: 'swift', svelte: 'svelte', vue: 'vue', ini: 'ini',
}

const ext = (path: string) => {
  const name = path.slice(Math.max(path.lastIndexOf('/'), path.lastIndexOf('\\')) + 1)
  const dot = name.lastIndexOf('.')
  return dot > 0 ? name.slice(dot + 1).toLowerCase() : ''
}

export function artifactKind(path: string): ArtifactKind {
  const e = ext(path)
  return KINDS[e] ?? (e in TEXT ? 'text' : 'other')
}

export function artifactLang(path: string): string {
  return TEXT[ext(path)] ?? 'text'
}

export const KIND_ICON: Record<ArtifactKind, string> = {
  html: 'code', svg: 'image', mermaid: 'layers', markdown: 'file-text', image: 'image', text: 'file-code', pdf: 'file', other: 'file',
}

const IMAGE_TYPES: Record<string, string> = {
  png: 'image/png', jpg: 'image/jpeg', jpeg: 'image/jpeg', gif: 'image/gif', webp: 'image/webp', bmp: 'image/bmp', ico: 'image/x-icon',
}

export function imageType(path: string): string {
  return IMAGE_TYPES[ext(path)] ?? 'application/octet-stream'
}

// An artifact as of a page: the version to show (by hash), or the file
// in the folder now (live) when no turn has versioned it.
export interface ArtifactEntry {
  path: string
  hash?: string
  size: number
  live: boolean // read from the folder, not a stored version
  changed: boolean // this page's turn added or changed it
  seq: number // the page whose turn made this version; 0 for a live file
}

// artifactsAt folds the pages up to index into the artifacts as they were
// after that page's turn. For the latest page, files in the folder that
// no turn recorded (made before versions were kept, or by the user) join
// as live entries. Sorted by path.
export function artifactsAt(pages: readonly Page[], index: number, live: readonly ArtifactFile[] | null): ArtifactEntry[] {
  const state = new Map<string, ArtifactEntry>()
  const last = Math.min(index, pages.length - 1)
  for (let i = 0; i <= last; i++) {
    const p = pages[i]
    for (const v of p?.artifacts ?? []) {
      if (v.deleted) state.delete(v.path)
      else state.set(v.path, { path: v.path, hash: v.hash || undefined, size: v.size, live: !v.hash, changed: i === last, seq: p?.seq ?? 0 })
    }
  }
  if (live && last === pages.length - 1) {
    for (const f of live) {
      if (!state.has(f.path)) state.set(f.path, { path: f.path, size: f.size, live: true, changed: false, seq: 0 })
    }
  }
  return [...state.values()].sort((a, b) => a.path.localeCompare(b.path))
}

// The policy every artifact document runs under: no network of any kind
// (no fetch, no images or fonts from the web, no form posts), inline
// styles, data: and blob: media, and inline scripts only when allowed.
export function artifactCSP(scripts: boolean): string {
  return [
    "default-src 'none'",
    scripts ? "script-src 'unsafe-inline'" : "script-src 'none'",
    "style-src 'unsafe-inline'",
    'img-src data: blob:',
    'font-src data:',
    'media-src data: blob:',
    "form-action 'none'",
    "base-uri 'none'",
  ].join('; ')
}

// sandboxDoc prepares an HTML artifact for its iframe: the policy goes
// first in the head, and anything that would send the frame elsewhere
// (a refresh, a base URL, its own policy) is taken out. The parse is
// inert: nothing runs or loads while it's prepared.
export function sandboxDoc(html: string, scripts = true): string {
  const doc = new DOMParser().parseFromString(html, 'text/html')
  doc.querySelectorAll('meta[http-equiv], base').forEach(el => el.remove())
  const meta = doc.createElement('meta')
  meta.httpEquiv = 'Content-Security-Policy'
  meta.content = artifactCSP(scripts)
  doc.head.prepend(meta)
  const charset = doc.createElement('meta')
  charset.setAttribute('charset', 'utf-8')
  doc.head.prepend(charset)
  return '<!DOCTYPE html>\n' + doc.documentElement.outerHTML
}

// svgDoc shows an SVG (an artifact, or a rendered Mermaid diagram) on its
// own page, fitted and centred, with no scripts.
export function svgDoc(svg: string, background: string): string {
  return sandboxDoc(
    `<html><head><style>html,body{margin:0;height:100%;background:${background}}body{display:grid;place-items:center}svg{max-width:100%;max-height:100vh;height:auto}</style></head><body>${svg}</body></html>`,
    false,
  )
}

// decodeText turns base64 into UTF-8 text.
export function decodeText(b64: string): string {
  const bin = atob(b64)
  const bytes = new Uint8Array(bin.length)
  for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i)
  return new TextDecoder().decode(bytes)
}

// artifactPathOf says where a touched file is in a session's artifacts
// folder (relative, with slashes), or null when it isn't in it.
export function artifactPathOf(workdir: string, path: string): string | null {
  const norm = (s: string) => s.replace(/\\/g, '/').replace(/\/+$/, '')
  const windows = /^[a-z]:/i.test(path) || path.includes('\\')
  const root = norm(workdir) + '/artifacts/'
  const p = norm(path)
  const hit = windows ? p.toLowerCase().startsWith(root.toLowerCase()) : p.startsWith(root)
  return hit ? p.slice(root.length) : null
}
