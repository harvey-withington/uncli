// Files and folders dropped onto the window (the OS drop, with real paths,
// from Wails). Where they land decides what happens:
// - on the composer: they are attached to the next turn (folders, and
//   files that can't be attached, go in as their paths instead);
// - anywhere else: the first folder (or the first file's folder) starts a
//   new session there, in the last type used that works on a folder.
import type { AppStore } from '../stores/app.svelte'

export const INSERT_EVENT = 'uncli-insert' // detail: text to insert at the cursor
export const ATTACH_EVENT = 'uncli-attach' // detail: paths to attach

export function quotePath(p: string): string {
  return /\s/.test(p) ? `"${p}"` : p
}

export async function handleDrop(app: AppStore, target: Element | null, dropped: string[]) {
  const paths = dropped.filter(Boolean) // a file the WebView couldn't resolve comes back empty
  if (paths.length === 0) return
  const composer = target?.closest('.composer')?.querySelector('textarea')
  if (composer && app.current) {
    composer.dispatchEvent(new CustomEvent(ATTACH_EVENT, { detail: paths }))
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

interface WailsRuntime {
  OnFileDrop?: (cb: (x: number, y: number, paths: string[]) => void, useDropTarget: boolean) => void
  ResolveFilePaths?: (x: number, y: number, files: File[]) => void // Windows: asks Go for the files' real paths
}

const runtime = () => (window as unknown as { runtime?: WailsRuntime }).runtime
const wailsDropOn = () => !!(window as unknown as { wails?: { flags?: { enableWailsDragAndDrop?: boolean } } }).wails?.flags?.enableWailsDragAndDrop
const hasFiles = (dt: DataTransfer | null) => !!dt && Array.from(dt.types).includes('Files')

// resolveFiles asks the app for the real paths of File objects (from a drop
// or a paste); they come back through the file-drop event, as if dropped at
// (x, y), and handleDrop routes them by what is there. False outside the
// app or where paths can't be resolved.
export function resolveFiles(x: number, y: number, files: File[]): boolean {
  const resolve = runtime()?.ResolveFilePaths
  if (!resolve || files.length === 0) return false
  resolve(Math.round(x), Math.round(y), files)
  return true
}

// listenForDrops wires the Wails file drop to handleDrop, if running in the
// app. It returns a function that removes its own listeners.
export function listenForDrops(app: AppStore): () => void {
  const rt = runtime()
  if (!rt?.OnFileDrop) return () => {}
  rt.OnFileDrop((x, y, paths) => {
    handleDrop(app, document.elementFromPoint(x, y), paths)
  }, false)
  // Wails switches its own drop handling on with a flag it sets once the
  // page has loaded, and a dev reload can leave it off. Without it the
  // message box takes a dropped file as text (its name). So file drags are
  // accepted everywhere here, before any element sees them, and a drop is
  // sent for its paths directly when Wails isn't handling it.
  const ondragover = (e: DragEvent) => {
    if (!hasFiles(e.dataTransfer)) return
    e.preventDefault()
    if (e.dataTransfer) e.dataTransfer.dropEffect = 'copy'
  }
  const ondrop = (e: DragEvent) => {
    if (!hasFiles(e.dataTransfer)) return
    e.preventDefault()
    if (!wailsDropOn()) resolveFiles(e.clientX, e.clientY, Array.from(e.dataTransfer?.files ?? []))
  }
  window.addEventListener('dragover', ondragover, true)
  window.addEventListener('drop', ondrop, true)
  return () => {
    window.removeEventListener('dragover', ondragover, true)
    window.removeEventListener('drop', ondrop, true)
  }
}
