// What Enter does in the message box. "send": Enter sends, Shift+Enter
// starts a new line. "newline": Enter starts a new line, Ctrl+Enter (or the
// Send button) sends. A touch keyboard has no practical Shift+Enter, so on
// a touch screen a new line is the default.
//
// Who decides, first to last: an embedding host (ChatHost.enter, as
// ChatStore.enterKey), the viewer's setting (Settings → General, kept for
// this viewer), and the device.
import { store } from './storage'

export type EnterKey = 'send' | 'newline'
export type EnterSetting = EnterKey | 'device'

const KEY = 'uncli-enter-key'

function load(): EnterSetting {
  try {
    const v = store.getItem(KEY)
    if (v === 'send' || v === 'newline') return v
  } catch {
    // storage unavailable: by device
  }
  return 'device'
}

// The viewer's setting.
export const enterSetting = $state<{ value: EnterSetting }>({ value: load() })

export function setEnterSetting(v: EnterSetting) {
  enterSetting.value = v
  try {
    if (v === 'device') store.removeItem(KEY)
    else store.setItem(KEY, v)
  } catch {
    // not remembered; fine for this run
  }
}

// touchScreen: the main pointer is a finger.
export function touchScreen(): boolean {
  return typeof matchMedia === 'function' && matchMedia('(pointer: coarse)').matches
}

// resolveEnter is what Enter does: the host's choice, the viewer's, then
// the device's.
export function resolveEnter(host: EnterKey | undefined, setting: EnterSetting, touch: boolean): EnterKey {
  if (host) return host
  if (setting !== 'device') return setting
  return touch ? 'newline' : 'send'
}
