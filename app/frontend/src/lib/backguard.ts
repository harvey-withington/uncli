// Back cancels an edit in progress: while a rename box is open, Back (the
// phone's back button, its back gesture or the hardware key) closes the box
// unsaved and the page stays where it is; with nothing being edited, Back
// goes back. The same rule as Escape.
//
// An edit holds Back while it's open. The phone's shell watches for holds
// (it adds a history entry, so the back gesture has something to undo) and
// asks take() before going back.

type Watcher = (held: boolean) => void

let current: (() => void) | null = null
let watcher: Watcher | null = null

// hold makes Back cancel the edit (calling cancel) until the returned
// release is called. A newer hold replaces an older one.
export function hold(cancel: () => void): () => void {
  current = cancel
  watcher?.(true)
  return () => {
    if (current !== cancel) return
    current = null
    watcher?.(false)
  }
}

// take is Back: it cancels the held edit and says so, or says there was
// none, so Back should go back.
export function take(): boolean {
  const cancel = current
  if (!cancel) return false
  current = null
  watcher?.(false)
  cancel()
  return true
}

// watch tells fn whenever Back starts or stops being held.
export function watch(fn: Watcher): () => void {
  watcher = fn
  return () => {
    if (watcher === fn) watcher = null
  }
}
