// What a walkthrough hands a new session started from it: the first
// message, carrying what the walkthrough knows (the question the changes
// answered, the explanation's summary, each changed place with its note,
// the files changed without a record). The new session reads the files
// itself; the old conversation doesn't come with it. Pure; it's for the
// model, so it isn't translated.
import type { PageExplanation } from './api'
import type { Deck } from './deck'

export function walkContext(question: string, deck: Deck, explanation: PageExplanation | null): string {
  const notes = new Map((explanation?.notes ?? []).map(n => [n.place, n.text.trim()]))
  const out = [
    'This session starts from a walkthrough of changes made in another session in this folder. Read the changed files as they are now before you rely on this.',
    '',
    'The request those changes answered:',
    quote(question.trim() || '(none given)'),
  ]
  if (explanation?.summary.trim()) out.push('', 'What the changes do:', explanation.summary.trim())
  if (deck.slides.length) {
    out.push('', 'The changes, place by place:')
    for (const s of deck.slides) {
      const where = s.first === s.last ? `line ${s.first}` : `lines ${s.first}-${s.last}`
      const counts = [s.added ? `+${s.added}` : '', s.removed ? `-${s.removed}` : ''].filter(Boolean).join(' ')
      const note = notes.get(s.place)
      out.push(`- ${s.rel}, ${where}${counts ? ` (${counts})` : ''}${note ? `: ${note}` : ''}`)
    }
  }
  if (deck.others.length) out.push('', 'Also changed (no record of how):', ...deck.others.map(f => `- ${f}`))
  out.push('', "Say briefly that you've read this, then wait for what I'd like to do next.")
  return out.join('\n')
}

// The question as a Markdown quote, each line of it.
const quote = (s: string) => s.split('\n').map(l => `> ${l}`.trimEnd()).join('\n')
