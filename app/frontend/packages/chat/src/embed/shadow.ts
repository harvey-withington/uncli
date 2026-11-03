// An embedded chat's stylesheet (decision 0026): UNCLI's theme as its
// shadow root reads it. The theme keys on :root in the app; inside the
// element that's :host, so a host's --uncli-* values set on the element
// reach the tokens, and the element's own scheme attribute picks light or
// dark. Then what the window's page gives the app, the element gives
// itself: its box, its type, the host's font unless the host sets one,
// and toasts and dialogs that cover the element rather than the page.

export function shadowCss(theme: string): string {
  return theme.replace(/:root((?::not\([^)]*\)|\[[^\]]*\])*)/g, (_m, q: string) => (q ? `:host(${q})` : ':host')) + HOST
}

const HOST = `
:host {
  display: block;
  position: relative;
  overflow: hidden;
  contain: layout paint;
  --font: var(--uncli-font, var(--uncli-host-font, system-ui, sans-serif));
  --mono: var(--uncli-mono, ui-monospace, 'Cascadia Code', Consolas, monospace);
  background: var(--bg);
  color: var(--text);
  font-family: var(--font);
  font-size: var(--text-md);
  line-height: 1.5;
  -webkit-font-smoothing: antialiased;
}
:host([hidden]) { display: none; }
.uncli-root { height: 100%; }
:host(.grabbing), :host(.grabbing) * { cursor: grabbing !important; user-select: none; }
:host(.resizing), :host(.resizing) * { cursor: col-resize; user-select: none; }
.scrim, .toasts { position: absolute !important; }
`
