import type { AttachmentRef, Page } from './api'

// The error code of a turn the provider refused because the CLI's sign-in
// failed or expired (core.ErrAuthFailed); the session stops the CLI, so the
// next message starts one with the user's new sign-in.
export const AUTH_FAILED = 'authentication_failed'

// signInFailed: the page's turn failed on the CLI's sign-in.
export function signInFailed(page: Pick<Page, 'status' | 'error'>): boolean {
  return page.status === 'error' && !!page.error?.startsWith(AUTH_FAILED + ':')
}

// The CLI's own words, without the error code in front.
export function errorDetail(error: string | undefined): string {
  return (error ?? '').replace(/^[a-z_]+:\s*/, '')
}

// What sending a page's question again sends: its text, and the files it
// had that are still on disk (pasted ones had no path).
export function resend(page: Pick<Page, 'question' | 'attachments'>): { text: string; refs: AttachmentRef[] } {
  return { text: page.question, refs: (page.attachments ?? []).filter(a => a.path).map(a => ({ path: a.path })) }
}
