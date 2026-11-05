// Types for @uncli/chat/element, the built <uncli-chat> and
// <uncli-session-list> (decisions 0026 and 0027), for TypeScript hosts. The
// element is plain JavaScript with Svelte inside; these describe only what
// a host uses. Kept in step with src/embed by embed/types.test.ts.
import type { Backend, ChatClient } from '@uncli/client'

// A host's own controls in the message box (src/lib/composercontrols.ts).
export type Tone = 'accent' | 'success' | 'warning' | 'danger' | 'muted'

export interface ControlItem {
  id: string
  label: string
  icon?: string // a Lucide name from UNCLI's set
  tone?: Tone
  hint?: string
}

// One click: an action, or one option alone.
export interface ButtonControl extends Partial<ControlItem> {
  kind?: 'button'
  id: string
  label: string
  onClick?: () => void
  disabled?: boolean
}

// Each click moves to the next state (Shift+click back), wrapping round.
export interface ToggleControl {
  kind: 'toggle'
  id: string
  label?: string
  value: string
  states: ControlItem[]
  onChange: (id: string) => void
}

// One of several, from a drop-down.
export interface MenuControl {
  kind?: 'menu'
  id: string
  label?: string
  value?: string
  options: ControlItem[]
  onChange: (id: string) => void
}

export type ComposerControl = ButtonControl | ToggleControl | MenuControl

// An @mention the host offers: label and detail in the menu, insert in the message.
export interface Mention {
  label: string
  detail?: string
  insert: string
}

// What a host does for the chat (the element's host property).
export interface ChatHost {
  // Every link clicked in an answer comes here first: true when the host took it.
  openLink?(href: string): boolean
  // The host's text for one of UNCLI's strings, or nothing to keep UNCLI's.
  t?(key: string, params?: Record<string, string | number>): string | undefined | null
  // @mentions: what matches the word typed after an @.
  complete?(query: string): Promise<Mention[]> | Mention[]
  // What the user has in front of them now, sent (hidden) with each turn.
  turnContext?(): string | undefined | null
  // The host's own sign-in, sent with every request to src.
  headers?(): Record<string, string> | undefined | null
  // What Enter does in the message box; left out: the viewer's, or a new line on touch screens.
  enter?: 'send' | 'newline'
  // The host's own controls in the message box, drawn by UNCLI.
  composerControls?: ComposerControl[]
}

// A host's review of a batch of its own changes, on one card in the
// approval cards' place (review()).
export interface ReviewItem {
  id: string
  label: string
  detail?: string
}

export interface ReviewAction {
  id: string
  label: string
  tone?: 'primary' | 'danger' | 'neutral'
}

export interface ReviewRequest {
  title: string
  detail?: string
  // Each change, ticked to untick; none: all or nothing.
  items?: ReviewItem[]
  // Apply (primary) and Discard when left out.
  actions?: ReviewAction[]
}

// The button chosen ("dismissed" when withdrawn) and the items still ticked.
export interface ReviewAnswer {
  action: string
  items: string[]
}

export declare class UncliChat extends HTMLElement {
  backend: Backend | null
  host: ChatHost | null
  session: string
  // Starts a session and shows it; meta is the host's word on what it's for.
  // provider picks the CLI (an id from the host's core.Providers()); left out, the first. One that
  // can't run now (unknown, not installed, signed out) rejects with an Error whose message says why,
  // to show as it is ("can't start a session on Claude Code: Claude Code isn't signed in"); nothing is made.
  newSession(profile?: string, workdir?: string, model?: string, opts?: { meta?: string; provider?: string }): Promise<string>
  // The provider (CLI id) and model the shown session runs on, once known; uncli-runs-on says when it changes.
  readonly runsOn: { provider: string; model: string } | null
  // Asks the user about a batch of the host's changes on one card; aborting signal withdraws it.
  review(request: ReviewRequest, opts?: { session?: string; signal?: AbortSignal }): Promise<ReviewAnswer>
}

export declare class UncliSessionList extends HTMLElement {
  backend: Backend | null
  selected: string
}

export declare function defineUncliChat(name?: string): void
export declare function defineUncliSessionList(name?: string): void

// The client the elements use, from the same bundle (one connection).
export declare function chatClient(backend: Backend): ChatClient
export declare function mountedBackend(src: string): Backend
export declare function setHostHeaders(headers: (() => Record<string, string> | null | undefined) | null): void

// The events the elements send: detail { session } (uncli-new-session, uncli-runs-on and
// uncli-select add to it).
export interface UncliEventMap {
  'uncli-ready': CustomEvent<{ session: string }>
  'uncli-busy': CustomEvent<{ session: string }>
  'uncli-idle': CustomEvent<{ session: string }>
  'uncli-needs-approval': CustomEvent<{ session: string }>
  'uncli-missing': CustomEvent<{ session: string }>
  'uncli-deleted': CustomEvent<{ session: string }>
  'uncli-new-chat': CustomEvent<{ session: string }>
  'uncli-new-session': CustomEvent<{ session: string; previous: string }>
  'uncli-runs-on': CustomEvent<{ session: string; provider: string; model: string }>
  'uncli-select': CustomEvent<{ session: string }>
  'uncli-escape': CustomEvent<{ session: string }>
  'uncli-submit': CustomEvent<{ session: string }>
}

declare global {
  interface HTMLElementTagNameMap {
    'uncli-chat': UncliChat
    'uncli-session-list': UncliSessionList
  }
  interface HTMLElementEventMap extends UncliEventMap {}
}
