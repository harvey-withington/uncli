// Where the chat keeps what it remembers for the viewer between runs:
// drafts, scroll spots, the File tab's choices, the panel layout, the
// approval scope. In UNCLI's own app that's localStorage. An embedded
// chat (decision 0026) keeps it in memory for the page instead, so UNCLI
// never writes to a host's storage unasked; a host that wants the layout
// remembered gives a storage-key (lib/layoutmemory.ts).

type Place = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>

class Memory implements Place {
  private items = new Map<string, string>()
  getItem(key: string) {
    return this.items.get(key) ?? null
  }
  setItem(key: string, value: string) {
    this.items.set(key, value)
  }
  removeItem(key: string) {
    this.items.delete(key)
  }
}

let memory: Memory | null = null
const place = (): Place => memory ?? globalThis.localStorage

export const store: Place = {
  getItem: key => place().getItem(key),
  setItem: (key, value) => place().setItem(key, value),
  removeItem: key => place().removeItem(key),
}

// keepInMemory: from now on, for the page, nothing goes to localStorage.
export function keepInMemory() {
  memory ??= new Memory()
}
