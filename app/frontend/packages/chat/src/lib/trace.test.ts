import { describe, expect, it } from 'vitest'
import { failureGist } from './trace'

describe('failure gist', () => {
  it('picks the exit code and the line that says what went wrong', () => {
    // The npm test failure from Harvey's code review (PowerShell, vitest not installed).
    const out = "Exit code 1\n> ultimate-grid-monorepo@0.1.0 test\r\n> vitest run --root packages/core\r\n\r\nnode.exe : 'vitest' is not recognized as an internal or external command,\r\nAt line:1 char:1\r\n+ & \"C:\\Program Files\\nodejs/node.exe\" ..."
    expect(failureGist(out)).toBe("Exit code 1 · node.exe : 'vitest' is not recognized as an internal or external command,")
  })

  it('falls back to the last lines when nothing reads as an error', () => {
    expect(failureGist('Exit code 2\nstep one\nstep two\nstep three')).toBe('Exit code 2 · step two step three')
    expect(failureGist('just one line')).toBe('just one line')
  })
})
