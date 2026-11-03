// Section kinds: what each part of an answer is. A model summary picks one
// for each section from this list, so it's part of the server's contract
// (kept in step with sectionKinds in internal/app/tasks.go).
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
