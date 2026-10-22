// What UNCLI keeps for the viewer between runs, in localStorage: what they
// were typing in each session, the session they had open, and whether the
// page follows the end of an answer as it streams. Storage that fails
// (private mode, full) only means it isn't kept.

const DRAFT_PREFIX = 'uncli-draft:'
const LAST_SESSION = 'uncli-last-session'
const FOLLOW = 'uncli-scroll'

function get(key: string): string | null {
  try {
    return localStorage.getItem(key)
  } catch {
    return null
  }
}

function set(key: string, value: string | null) {
  try {
    if (value === null) localStorage.removeItem(key)
    else localStorage.setItem(key, value)
  } catch {
    // not kept; still there for this run
  }
}

// The message being typed in a session, kept on every change, so a restart
// (or a crash) mid-sentence loses nothing.
export function loadDraft(sessionId: string): string {
  return get(DRAFT_PREFIX + sessionId) ?? ''
}

export function saveDraft(sessionId: string, text: string) {
  set(DRAFT_PREFIX + sessionId, text ? text : null)
}

// The session that was open, to open it again at the next start.
export function loadLastSession(): string {
  return get(LAST_SESSION) ?? ''
}

export function saveLastSession(id: string) {
  set(LAST_SESSION, id || null)
}

// Whether the page follows the end of the answer (true) or stays where the
// viewer left it (false, the default).
export function loadFollow(): boolean {
  try {
    return JSON.parse(get(FOLLOW) ?? 'null')?.follow === true
  } catch {
    return false
  }
}

export function saveFollow(follow: boolean) {
  set(FOLLOW, JSON.stringify({ follow }))
}

// atEnd: the scroller shows the end of its content (within a pixel or two).
export function atEnd(el: { scrollHeight: number; scrollTop: number; clientHeight: number }): boolean {
  return el.scrollHeight - el.scrollTop - el.clientHeight <= 2
}
