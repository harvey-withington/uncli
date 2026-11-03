// Page navigation: back and forward move one page; the bookmark buttons
// jump to the nearest bookmark in that direction, falling back to the
// first or last page when there is none.

export interface Bookmarkable {
  bookmarked: boolean
}

export function back(i: number): number {
  return Math.max(0, i - 1)
}

export function forward(i: number, count: number): number {
  return Math.max(0, Math.min(count - 1, i + 1))
}

export function prevBookmark(pages: readonly Bookmarkable[], i: number): number {
  for (let j = Math.min(i, pages.length) - 1; j >= 0; j--) {
    if (pages[j]?.bookmarked) return j
  }
  return 0
}

export function nextBookmark(pages: readonly Bookmarkable[], i: number): number {
  for (let j = i + 1; j < pages.length; j++) {
    if (pages[j]?.bookmarked) return j
  }
  return Math.max(0, pages.length - 1)
}

// Whether a keyboard shortcut should be ignored because the user is typing.
export function isTyping(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false
  const tag = target.tagName
  return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || target.isContentEditable === true
}
