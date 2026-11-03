// What the window's file drop hands the composer (lib/drops.ts): events the
// composer listens for, and paths quoted as it inserts them.
export const INSERT_EVENT = 'uncli-insert' // detail: text to insert at the cursor
export const ATTACH_EVENT = 'uncli-attach' // detail: paths to attach

export function quotePath(p: string): string {
  return /\s/.test(p) ? `"${p}"` : p
}
