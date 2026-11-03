// Questions and plans the model puts to the user (Approval.ask): what the
// cards show, read from the tool's input, and the answers they send back.
// The shapes are the CLI's (fixtures ask-user-question, exit-plan-mode);
// the backend's adapter turns the answers into what its CLI takes.

export interface AskOption {
  label: string
  description?: string
}

export interface Question {
  question: string
  header?: string
  options: AskOption[]
  multiSelect: boolean
}

// The questions in a question's input; none when it isn't one.
export function questionsOf(input: unknown): Question[] {
  const qs = (input as { questions?: unknown })?.questions
  if (!Array.isArray(qs)) return []
  return qs
    .map(q => q as Partial<Question>)
    .filter(q => typeof q.question === 'string' && q.question.trim() !== '')
    .map(q => ({
      question: q.question!,
      header: typeof q.header === 'string' ? q.header : undefined,
      options: (Array.isArray(q.options) ? q.options : []).filter(o => typeof o?.label === 'string').map(o => ({ label: o.label, description: o.description })),
      multiSelect: q.multiSelect === true,
    }))
}

// The plan in a plan's input, as Markdown; '' when the CLI gave none.
export function planOf(input: unknown): string {
  const p = (input as { plan?: unknown })?.plan
  return typeof p === 'string' ? p.trim() : ''
}

// One question's answer: the options picked, then what was typed under
// Other, joined as the CLI joins several picks.
export function answerOf(picked: string[], other: string): string {
  return [...picked, other.trim()].filter(Boolean).join(', ')
}

// The answers by question, once every question has one; null until then.
export function answersFor(questions: Question[], picked: Record<string, string[]>, other: Record<string, string>): Record<string, string> | null {
  const out: Record<string, string> = {}
  for (const q of questions) {
    const a = answerOf(picked[q.question] ?? [], other[q.question] ?? '')
    if (!a) return null
    out[q.question] = a
  }
  return out
}

// A pick: a single-choice question has one, a multiple-choice one toggles.
export function pick(q: Question, have: string[], label: string): string[] {
  if (!q.multiSelect) return have[0] === label ? [] : [label]
  return have.includes(label) ? have.filter(l => l !== label) : [...have, label]
}
