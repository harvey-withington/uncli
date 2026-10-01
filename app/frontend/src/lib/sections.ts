// Section kinds: what each part of an answer is, shown as an icon in the
// outline. Headings get a kind from a local guess; a model summary picks
// one itself from the same list (kept in step with sectionKinds in
// internal/app/tasks.go).
import type { MdBlock } from './render/markdown'

export const SECTION_KINDS = [
  'overview',
  'commentary',
  'analysis',
  'steps',
  'code',
  'data',
  'example',
  'tip',
  'warning',
  'conclusion',
] as const

export type SectionKind = (typeof SECTION_KINDS)[number]

export const KIND_ICONS: Record<SectionKind, string> = {
  overview: 'info',
  commentary: 'message-square-text',
  analysis: 'scan-search',
  steps: 'list-ordered',
  code: 'code',
  data: 'table',
  example: 'flask-conical',
  tip: 'lightbulb',
  warning: 'triangle-alert',
  conclusion: 'flag',
}

export function isKind(k: unknown): k is SectionKind {
  return typeof k === 'string' && (SECTION_KINDS as readonly string[]).includes(k)
}

// Heading words that say what a section is, most specific first.
const HEADING_RULES: [SectionKind, RegExp][] = [
  ['warning', /\b(warnings?|caution|danger|careful|risks?|gotchas?|pitfalls?|important|beware|security)\b/i],
  ['tip', /\b(tips?|hints?|recommend(ed|ations?)?|best practices?|notes?)\b/i],
  ['example', /\b(examples?|e\.g\.|samples?|demo|walk-?through|use cases?)\b/i],
  ['conclusion', /\b(summary|conclusions?|tl;?dr|in short|wrap(ping)?[- ]up|takeaways?|next steps?|verdict|bottom line)\b/i],
  ['overview', /\b(overview|introduction|intro|background|context|about|what is)\b/i],
  ['analysis', /\b(analysis|analy[sz]e|comparison|compar(e|ing)|trade-?offs?|pros|cons|evaluation|assessment|diagnosis|root cause|findings|why)\b/i],
  ['steps', /\b(steps?|how to|instructions|set ?up|install(ation|ing)?|procedure|guide|getting started|walk me)\b/i],
  ['data', /\b(data|tables?|results|benchmarks?|stats|statistics|numbers|metrics|budget|costs?|pricing|schedule)\b/i],
  ['code', /\b(code|implementation|snippets?|scripts?|functions?|api|usage)\b/i],
]

const isTable = (b: MdBlock) => b.kind === 'md' && /^\|.*\|\s*\n\|?\s*:?-{3,}/.test(b.source)
const isOrdered = (b: MdBlock) => b.kind === 'md' && /^\d+[.)]\s/.test(b.source)
const isWarningQuote = (b: MdBlock) => b.kind === 'md' && /^>\s*(\*\*)?(⚠️?|warning|caution|important|danger)/i.test(b.source)

// classifySection guesses the kind of a section from its heading and the
// blocks under it (not counting the heading itself).
export function classifySection(heading: string, body: readonly MdBlock[]): SectionKind {
  for (const [kind, re] of HEADING_RULES) {
    if (re.test(heading)) return kind
  }
  if (body.some(isWarningQuote)) return 'warning'
  if (body.length > 0) {
    const share = (pred: (b: MdBlock) => boolean) => body.filter(pred).length / body.length
    if (share(b => b.kind === 'code') >= 0.5) return 'code'
    if (share(isTable) >= 0.5) return 'data'
    if (share(isOrdered) >= 0.5) return 'steps'
  }
  return 'commentary'
}
