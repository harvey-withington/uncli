// The embeddable chat's entry (decision 0026): importing it registers
// <uncli-chat>.
import { defineUncliChat } from './element.svelte'

export { UncliChat, defineUncliChat } from './element.svelte'

defineUncliChat()
