// Where a link in an answer leads. Web links open in the browser; links to
// files (file:///… as the Antigravity CLI writes them, or a path relative to
// the session's folder) open the file, at a line when the link says
// (#L12, #L12-L20 or :12). Anything else isn't followed.

export type LinkTarget =
  | { kind: 'web'; url: string }
  | { kind: 'file'; path: string; line: number }

const SCHEME = /^[a-z][a-z0-9+.-]*:/i

export function linkTarget(href: string, workdir: string): LinkTarget | null {
  const h = href.trim()
  if (/^https?:\/\//i.test(h)) return { kind: 'web', url: h }
  if (h === '' || h.startsWith('#')) return null
  let path: string
  let hash = ''
  if (/^file:/i.test(h)) {
    let rest = h.replace(/^file:(\/\/)?/i, '')
    ;[rest, hash] = cut(rest, '#')
    rest = safeDecode(rest)
    // file:///C:/x → C:/x; file:///home/x → /home/x
    path = /^\/[a-z]:[/\\]/i.test(rest) ? rest.slice(1) : rest
  } else if (SCHEME.test(h) && !/^[a-z]:[/\\]/i.test(h)) {
    return null // mailto:, vscode:, javascript: and the like
  } else {
    ;[path, hash] = cut(h, '#')
    path = safeDecode(path)
    if (!isAbsolute(path)) {
      if (!workdir) return null
      path = join(workdir, path)
    }
  }
  let line = 0
  const m = /^L(\d+)/i.exec(hash)
  if (m) line = Number(m[1])
  else {
    const c = /:(\d+)(?::\d+)?$/.exec(path)
    if (c && !/^[a-z]:$/i.test(path.slice(0, c.index))) {
      line = Number(c[1])
      path = path.slice(0, c.index)
    }
  }
  return path ? { kind: 'file', path, line } : null
}

function cut(s: string, sep: string): [string, string] {
  const i = s.indexOf(sep)
  return i < 0 ? [s, ''] : [s.slice(0, i), s.slice(i + 1)]
}

function safeDecode(s: string): string {
  try {
    return decodeURIComponent(s)
  } catch {
    return s
  }
}

function isAbsolute(p: string): boolean {
  return /^[a-z]:[/\\]/i.test(p) || p.startsWith('/') || p.startsWith('\\\\')
}

// join puts a relative path under a folder, in the folder's own style.
function join(dir: string, rel: string): string {
  const sep = dir.includes('\\') ? '\\' : '/'
  const parts = dir.replace(/[/\\]+$/, '').split(/[/\\]/)
  for (const seg of rel.replace(/^\.[/\\]/, '').split(/[/\\]/)) {
    if (seg === '' || seg === '.') continue
    if (seg === '..') parts.length > 1 && parts.pop()
    else parts.push(seg)
  }
  return parts.join(sep)
}
