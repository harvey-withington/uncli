import { describe, expect, it } from 'vitest'
import { errorDetail, resend, signInFailed } from './signin'

describe('a turn that failed on the sign-in', () => {
  it('is told from other failures by its error code', () => {
    expect(signInFailed({ status: 'error', error: 'authentication_failed: Failed to authenticate. API Error: 401 OAuth token has expired.' })).toBe(true)
    expect(signInFailed({ status: 'error', error: 'model_not_found: no such model' })).toBe(false)
    expect(signInFailed({ status: 'done', error: 'authentication_failed: old' })).toBe(false)
    expect(signInFailed({ status: 'error' })).toBe(false)
  })

  it('shows the CLI’s words without the code', () => {
    expect(errorDetail('authentication_failed: Failed to authenticate.')).toBe('Failed to authenticate.')
    expect(errorDetail(undefined)).toBe('')
  })

  it('sends the question again with the files that have a path', () => {
    expect(resend({ question: 'Plan it', attachments: [{ name: 'a.pdf', path: 'C:/a.pdf', mediaType: 'application/pdf', size: 1 }, { name: 'pasted.png', mediaType: 'image/png', size: 1 }] }))
      .toEqual({ text: 'Plan it', refs: [{ path: 'C:/a.pdf' }] })
  })
})
