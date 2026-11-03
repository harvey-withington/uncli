// The client's own few messages (errors a user may see), in English. An app
// with its own strings passes its translate function to setClientText, so
// they follow its language; the keys are the same as UNCLI's own.
const EN = {
  'remote.onlyAtServer': 'Only on the PC that runs this session',
  'remote.unpaired': 'This device is no longer paired.',
  'remote.badReply': 'The server sent something UNCLI didn’t expect ({status}). Try again in a moment.',
  'remote.unreachable': 'Couldn’t reach the server. Check the connection and try again.',
  'remote.closed': 'the server closed the connection',
}

export type ClientTextKey = keyof typeof EN
export type Translate = (key: ClientTextKey, params?: Record<string, string | number>) => string

export const CLIENT_TEXT: Readonly<Record<ClientTextKey, string>> = EN

let translate: Translate | null = null

// setClientText makes the client's messages come from translate; null goes
// back to English.
export function setClientText(f: Translate | null) {
  translate = f
}

export function text(key: ClientTextKey, params?: Record<string, string | number>): string {
  if (translate) return translate(key, params)
  let s: string = EN[key]
  for (const [k, v] of Object.entries(params ?? {})) s = s.replaceAll(`{${k}}`, String(v))
  return s
}
