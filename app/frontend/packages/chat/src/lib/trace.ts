// The trace strip's reading of a failed tool call: its exit code and the
// line that says what went wrong, rather than the command's preamble.
const ERROR_LINE = /(error|not recognized|cannot|can't|failed|not found|no such|denied|exception|fatal|enoent|eacces|permission)/i
const EXIT_LINE = /^exit code \d+/i

export function failureGist(output: string): string {
  const lines = output.split(/\r?\n/).map(l => l.trim()).filter(Boolean)
  const exit = lines.find(l => EXIT_LINE.test(l))
  const rest = lines.filter(l => !EXIT_LINE.test(l))
  const why = rest.find(l => ERROR_LINE.test(l)) ?? rest.slice(-2).join(' ')
  return [exit, why].filter(Boolean).join(' · ')
}
