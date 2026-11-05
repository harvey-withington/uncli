// The embeddable UI's entry (decision 0026): importing it registers
// <uncli-chat> and <uncli-session-list>.
import { defineUncliChat } from './element.svelte'
import { defineUncliSessionList } from './list.svelte'

export { UncliChat, defineUncliChat, type ChatHost } from './element.svelte'
export type { Mention } from '../stores/chat.svelte'
export type { ReviewAction, ReviewAnswer, ReviewItem, ReviewRequest } from '../lib/hostreview'
export type { ButtonControl, ComposerControl, ControlItem, MenuControl, ToggleControl, Tone } from '../lib/composercontrols'
export { UncliSessionList, defineUncliSessionList } from './list.svelte'
// The client the elements use, from the same bundle, so a host that needs
// it shares their connection rather than opening a second one.
export { chatClient, mountedBackend, setHostHeaders } from '@uncli/client'

defineUncliChat()
defineUncliSessionList()
