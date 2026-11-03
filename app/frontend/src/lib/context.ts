import { getContext, setContext } from 'svelte'
import type { AppStore } from '../stores/app.svelte'
import type { ChatStore } from '../stores/chat.svelte'

export const APP_CONTEXT = Symbol('app')
export const CHAT_CONTEXT = Symbol('chat')

// The app (the window or the phone): its whole store.
export function provideApp(store: AppStore) {
  setContext(APP_CONTEXT, store)
}

export function useApp(): AppStore {
  const s = getContext<AppStore | undefined>(APP_CONTEXT)
  if (!s) throw new Error('useApp() outside the app')
  return s
}

// The chat (decision 0026): what the chat's components use, so they work
// in the app and in an embedded element alike. In the app, the app's store
// is the chat's.
export function provideChat(store: ChatStore) {
  setContext(CHAT_CONTEXT, store)
}

export function useChat(): ChatStore {
  const s = getContext<ChatStore | undefined>(CHAT_CONTEXT) ?? getContext<AppStore | undefined>(APP_CONTEXT)
  if (!s) throw new Error('useChat() outside a chat')
  return s
}
