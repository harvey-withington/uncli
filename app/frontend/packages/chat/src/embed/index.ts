// The embeddable UI's entry (decision 0026): importing it registers
// <uncli-chat> and <uncli-session-list>.
import { defineUncliChat } from './element.svelte'
import { defineUncliSessionList } from './list.svelte'

export { UncliChat, defineUncliChat, type ChatHost } from './element.svelte'
export type { Mention } from '../stores/chat.svelte'
export { UncliSessionList, defineUncliSessionList } from './list.svelte'

defineUncliChat()
defineUncliSessionList()
