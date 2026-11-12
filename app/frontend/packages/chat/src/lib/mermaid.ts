// Mermaid diagrams, for the artifact viewer (.mmd files) and Mermaid code
// blocks in Markdown documents (the file panel, Markdown artifacts). Mermaid
// is big, so it loads the first time a diagram is shown. Its strict level
// escapes labels and turns off clicks and scripts; the SVG is still shown
// in a frame with no scripts (svgDoc).

let seq = 0

// renderMermaid turns a diagram's source into SVG, or throws Mermaid's
// complaint about it.
export async function renderMermaid(src: string): Promise<string> {
  const { default: mermaid } = await import('mermaid')
  mermaid.initialize({ startOnLoad: false, securityLevel: 'strict', theme: 'default' })
  const { svg } = await mermaid.render(`uncli-mermaid-${++seq}`, src)
  return svg
}

// svgSize is an SVG's width and height from its viewBox, for a frame of
// the same shape; null when it has none worth using.
export function svgSize(svg: string): { width: number; height: number } | null {
  const m = /<svg\b[^>]*\bviewBox\s*=\s*["']\s*[-\d.e]+[\s,]+[-\d.e]+[\s,]+([\d.e]+)[\s,]+([\d.e]+)\s*["']/i.exec(svg)
  if (!m) return null
  const width = Number(m[1])
  const height = Number(m[2])
  return width > 0 && height > 0 && Number.isFinite(width) && Number.isFinite(height) ? { width, height } : null
}
