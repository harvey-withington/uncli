// Per-type colour: a hue drawn at full saturation with the theme's
// lightness (--tint-l: medium-dark in light themes, lighter in dark). Sets
// --tint (icons, borders) and --tint-soft (tinted backgrounds); without a
// hue, components fall back to the app accent.
export function tintStyle(hue: number | null | undefined): string {
  if (!hue) return ''
  const h = ((Math.round(hue) % 360) + 360) % 360
  return `--tint: hsl(${h} 100% var(--tint-l)); --tint-soft: hsl(${h} 100% var(--tint-l) / 0.14);`
}
