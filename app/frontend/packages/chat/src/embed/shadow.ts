// An embedded chat's stylesheet (decision 0026): UNCLI's theme as its
// shadow root reads it. The theme keys on :root in the app; inside the
// element that's .uncli-root, the element the chat mounts in, and the
// element's own scheme attribute picks light or dark (:host([…])
// .uncli-root). Not :host itself: a host sets --uncli-* on the element,
// often from tokens of its own with the same names as UNCLI's working ones
// (--uncli-accent: var(--accent)). Were UNCLI's --accent on the element
// too, the host's var(--accent) would find it, not the host's, and the two
// would loop and both be dropped. One level down, the host's --uncli-*
// values arrive already resolved. Then what the window's page gives the
// app, the element gives itself: its box, its type, the host's font unless
// the host sets one, and toasts and dialogs that cover the element rather
// than the page.

export function shadowCss(theme: string): string {
  return theme.replace(/:root((?::not\([^)]*\)|\[[^\]]*\])*)/g, (_m, q: string) => (q ? `:host(${q}) .uncli-root` : '.uncli-root')) + HOST
}

const HOST = `
:host {
  display: block;
  position: relative;
  overflow: hidden;
  contain: layout paint;
}
:host([hidden]) { display: none; }
.uncli-root {
  height: 100%;
  --font: var(--uncli-font, var(--uncli-host-font, system-ui, sans-serif));
  --mono: var(--uncli-mono, ui-monospace, 'Cascadia Code', Consolas, monospace);
  background: var(--bg);
  color: var(--text);
  font-family: var(--font);
  font-size: var(--text-md);
  line-height: 1.5;
  -webkit-font-smoothing: antialiased;
}
:host(.grabbing), :host(.grabbing) * { cursor: grabbing !important; user-select: none; }
:host(.resizing), :host(.resizing) * { cursor: col-resize; user-select: none; }
.scrim, .toasts { position: absolute !important; }
`
