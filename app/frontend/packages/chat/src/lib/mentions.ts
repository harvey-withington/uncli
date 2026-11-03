// @mentions from an embedding host (decision 0026): while what's typed
// just before the cursor is "@" and a word, the host is asked what matches;
// picking one puts the host's text in its place.

export interface MentionQuery {
  start: number // where the "@" is
  query: string // what's typed after it, up to the cursor
}

// mentionQuery is the mention being typed at the cursor, if any: an "@" at
// the start of the message or after a space, with no space after it.
export function mentionQuery(text: string, caret: number): MentionQuery | null {
  const before = text.slice(0, caret)
  const m = /(^|\s)@([^\s@]*)$/.exec(before)
  if (!m) return null
  return { start: before.length - m[2]!.length - 1, query: m[2]! }
}

// insertMention puts insert where the mention being typed is, followed by a
// space, and says where the cursor goes.
export function insertMention(text: string, at: MentionQuery, insert: string): { text: string; caret: number } {
  const end = at.start + 1 + at.query.length
  const after = text.slice(end)
  const put = insert + (after.startsWith(' ') ? '' : ' ')
  return { text: text.slice(0, at.start) + put + after, caret: at.start + put.length }
}
