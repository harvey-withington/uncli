import { getContext, setContext } from 'svelte'
import type { AppStore } from '../stores/app.svelte'
import { APP_CONTEXT } from '@uncli/chat/lib/chatcontext'

export { APP_CONTEXT, CHAT_CONTEXT, provideChat, useChat } from '@uncli/chat/lib/chatcontext'

// The app (the window or the phone): its whole store. The chat's
// components read it as a ChatStore (lib/chatcontext.ts).
export function provideApp(store: AppStore) {
  setContext(APP_CONTEXT, store)
}

export function useApp(): AppStore {
  const s = getContext<AppStore | undefined>(APP_CONTEXT)
  if (!s) throw new Error('useApp() outside the app')
  return s
}
