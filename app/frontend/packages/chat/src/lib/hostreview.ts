// A host's review of a batch of its own changes (decision 0027): one card
// in the chat's approval place, in UNCLI's look, asking the user about
// everything at once (BRUV's Suggest mode: the model's twenty staged board
// edits, applied or discarded together), where approving each tool call
// would ask twenty times. The host raises it from its page with
// <uncli-chat>'s review(), and gets the user's answer back.

// One change, ticked by default; the user can untick it.
export interface ReviewItem {
  id: string
  label: string
  detail?: string // plain text, line breaks kept
}

// A button on the card. Two by default: Apply (primary) and Discard.
export interface ReviewAction {
  id: string
  label: string
  tone?: 'primary' | 'danger' | 'neutral'
}

export interface ReviewRequest {
  title: string // "Apply 20 suggested changes to Trip Planning?"
  detail?: string // plain text, line breaks kept
  items?: ReviewItem[] // each change, to tick or untick; none: the card is all-or-nothing
  actions?: ReviewAction[]
}

// What the user chose: the action's id, and the items ticked when they did
// (all of them for a card without items). "dismissed" when the host
// withdrew it or the chat went away.
export interface ReviewAnswer {
  action: string
  items: string[]
}

export interface HostReview extends ReviewRequest {
  id: string
  session: string
  resolve: (a: ReviewAnswer) => void
}

export const DISMISSED = 'dismissed'
