// A host's own controls in the message box (decision 0026), drawn by
// UNCLI so they fit it: a button for a single click, a toggle that moves
// through its states on each click, or a drop-down to pick one of several.
// Each item has an icon from UNCLI's set, a label and a tone (drawn from
// the --uncli-* colours the host themes); its hint is the tooltip. The host
// owns the value: onChange tells it what was picked, and it sets the
// controls again with the new value (or doesn't, to refuse).

export type Tone = 'accent' | 'success' | 'warning' | 'danger' | 'muted'

export interface ControlItem {
  id: string
  label: string
  icon?: string
  tone?: Tone
  hint?: string
}

// One click: an action. Its own label and icon, or a one-item options list
// (onChange gets that item's id).
export interface ButtonControl extends Partial<ControlItem> {
  kind?: 'button'
  id: string
  label: string
  onClick?: () => void
  disabled?: boolean
}

// States in order; each click moves to the next (Shift+click: back),
// wrapping round. Two states are an on/off switch.
export interface ToggleControl {
  kind: 'toggle'
  id: string
  label?: string // what's being chosen, read out and in the tooltip ("Mode")
  value: string
  states: ControlItem[]
  onChange: (id: string) => void
}

// One of several, from a drop-down. One option alone is a plain button.
export interface MenuControl {
  kind?: 'menu'
  id: string
  label?: string
  value?: string
  options: ControlItem[]
  onChange: (id: string) => void
}

export type ComposerControl = ButtonControl | ToggleControl | MenuControl

// kindOf is how a control is drawn: a toggle when asked for; a drop-down
// for several options; a plain button for one option, or none.
export function kindOf(c: ComposerControl): 'button' | 'toggle' | 'menu' {
  if (c.kind === 'toggle') return 'toggle'
  if ('options' in c && c.options.length > 1) return 'menu'
  return 'button'
}

// The item a button shows and what clicking it does: its own label and
// onClick, or its one option's, with onChange.
export function buttonOf(c: ComposerControl): { item: ControlItem; click: () => void; disabled: boolean } {
  if ('options' in c) {
    const only = c.options[0]
    return { item: only ?? { id: c.id, label: c.label ?? '' }, click: () => only && c.onChange(only.id), disabled: !only }
  }
  const b = c as ButtonControl
  return { item: { id: b.id, label: b.label, icon: b.icon, tone: b.tone, hint: b.hint }, click: () => b.onClick?.(), disabled: !!b.disabled }
}
