import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte'
import { describe, expect, it, vi } from 'vitest'
import App from '../App.svelte'
import { mockBackend } from '../lib/api/mock'

describe('A turn whose sign-in failed', () => {
  it('signs in again right there, then sends the question again', async () => {
    const backend = mockBackend({ signInFailed: true })
    const send = vi.spyOn(backend, 'send')
    render(App, { props: { backend } })
    await screen.findByRole('heading', { name: 'Fix the flaky parser test' })
    await fireEvent.click(screen.getByText('Plan a weekend in Lisbon'))
    await screen.findByRole('heading', { name: 'Plan a weekend in Lisbon' })
    // The latest page is the failed one.
    await fireEvent.keyDown(window, { key: 'End' })
    const box = (await screen.findByText(/Claude couldn't use your Claude sign-in/)).closest('.signin-failed') as HTMLElement
    expect(within(box).getByText(/401 OAuth token has expired/)).toBeInTheDocument()

    await fireEvent.click(within(box).getByRole('button', { name: 'Sign in again' }))
    const code = await within(box).findByRole('textbox', { name: 'Code from the sign-in page' })
    await fireEvent.input(code, { target: { value: 'wrong' } })
    await fireEvent.click(within(box).getByRole('button', { name: 'Finish sign-in' }))
    expect(await within(box).findByText(/Invalid code/)).toBeInTheDocument()

    await fireEvent.click(within(box).getByRole('button', { name: 'Sign in again' }))
    await fireEvent.input(await within(box).findByRole('textbox', { name: 'Code from the sign-in page' }), { target: { value: 'good-code' } })
    await fireEvent.click(within(box).getByRole('button', { name: 'Finish sign-in' }))
    expect(await within(box).findByText(/Signed in again/)).toBeInTheDocument()

    await fireEvent.click(within(box).getByRole('button', { name: 'Send again' }))
    await waitFor(() => expect(send).toHaveBeenCalledWith('s-chat', 'Which of those cafés open before 8?', []))
  }, 15000)

  it('can also be signed in again from Settings, where the sign-in may still look fine', async () => {
    render(App, { props: { backend: mockBackend() } })
    await screen.findByRole('heading', { name: 'Fix the flaky parser test' })
    await fireEvent.click(await screen.findByRole('button', { name: /Claude Code 2\./ }))
    const settings = within(await screen.findByRole('dialog', { name: 'Settings' }))
    await fireEvent.click(settings.getByRole('tab', { name: 'AI Providers' }))
    const section = within(settings.getByRole('region', { name: 'Claude Code' }))
    expect(section.getByText(/If Claude says your sign-in failed or expired, sign in again here/)).toBeInTheDocument()
    await fireEvent.click(section.getByRole('button', { name: 'Sign in again' }))
    expect(await section.findByRole('textbox', { name: 'Code from the sign-in page' })).toBeInTheDocument()
  })
})
