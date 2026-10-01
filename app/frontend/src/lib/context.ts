import { getContext, setContext } from 'svelte'
import type { AppStore } from '../stores/app.svelte'

export const APP_CONTEXT = Symbol('app')

export function provideApp(store: AppStore) {
  setContext(APP_CONTEXT, store)
}

export function useApp(): AppStore {
  const s = getContext<AppStore | undefined>(APP_CONTEXT)
  if (!s) throw new Error('useApp() outside the app')
  return s
}
