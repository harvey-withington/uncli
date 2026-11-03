import { getContext, setContext } from 'svelte'
import type { ChatStore } from '../stores/chat.svelte'

// The chat's context (decision 0026): what the chat's components use, so
// they work in the app and in an embedded element alike. The app's store
// is a ChatStore too, provided under APP_CONTEXT (lib/context.ts, the
// app's), and the chat finds it there when no chat of its own is given.
export const APP_CONTEXT = Symbol('app')
export const CHAT_CONTEXT = Symbol('chat')

export function provideChat(store: ChatStore) {
  setContext(CHAT_CONTEXT, store)
}

export function useChat(): ChatStore {
  const s = getContext<ChatStore | undefined>(CHAT_CONTEXT) ?? getContext<ChatStore | undefined>(APP_CONTEXT)
  if (!s) throw new Error('useChat() outside a chat')
  return s
}
