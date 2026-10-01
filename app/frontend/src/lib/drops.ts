// Files and folders dropped onto the window (the OS drop, with real paths,
// from Wails). Where they land decides what happens:
// - on the composer: their paths are inserted at the cursor, quoted when
//   they contain spaces, so a Code or Co-work turn can point at them;
// - anywhere else: the first folder (or the first file's folder) starts a
//   new session there, in the last type used that works on a folder.
import type { AppStore } from '../stores/app.svelte'

export const INSERT_EVENT = 'uncli-insert'

export function quotePath(p: string): string {
  return /\s/.test(p) ? `"${p}"` : p
}

export async function handleDrop(app: AppStore, target: Element | null, paths: string[]) {
  if (paths.length === 0) return
  const composer = target?.closest('.composer')?.querySelector('textarea')
  if (composer && app.current) {
    composer.dispatchEvent(new CustomEvent(INSERT_EVENT, { detail: paths.map(quotePath).join(' ') }))
    return
  }
  if (!app.cliReady) return
  const [first] = await app.backend.describePaths(paths)
  if (!first) return
  const profiles = app.boot?.profiles ?? []
  const last = profiles.find(p => p.id === app.boot?.lastNewSession.profileId)
  const folderType = last && last.folder !== 'scratch' ? last : profiles.find(p => p.folder === 'repo') ?? profiles.find(p => p.folder !== 'scratch')
  app.openNewSession(folderType?.id ?? null, first.dir)
}

// listenForDrops wires the Wails file drop to handleDrop, if running in the app.
export function listenForDrops(app: AppStore) {
  const rt = (window as unknown as { runtime?: { OnFileDrop?: (cb: (x: number, y: number, paths: string[]) => void, useDropTarget: boolean) => void } }).runtime
  rt?.OnFileDrop?.((x, y, paths) => {
    handleDrop(app, document.elementFromPoint(x, y), paths)
  }, false)
}
