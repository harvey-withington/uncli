// Session order: each session has a sortOrder; the list shows the highest
// first. Moving one picks a value between its new neighbours, so only the
// moved session changes.

export interface Ordered {
  id: string
  sortOrder: number
}

// orderAt returns the sortOrder that puts the session with id at index
// "to" of the list (indices as shown, before the move).
export function orderAt(list: readonly Ordered[], id: string, to: number): number | null {
  const from = list.findIndex(s => s.id === id)
  if (from < 0) return null
  const rest = list.filter(s => s.id !== id)
  const at = Math.max(0, Math.min(rest.length, to > from ? to - 1 : to))
  if (at === from) return null // dropped where it already is
  const above = rest[at - 1]
  const below = rest[at]
  if (above && below) return (above.sortOrder + below.sortOrder) / 2
  if (above) return above.sortOrder - 1
  if (below) return below.sortOrder + 1
  return null
}
