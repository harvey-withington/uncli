import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte'
import { afterEach, describe, expect, it, vi } from 'vitest'
import App from '../App.svelte'
import { mockBackend } from '../lib/api/mock'

async function newSession() {
  await fireEvent.click(await screen.findByRole('button', { name: /New session/ }))
  return within(await screen.findByRole('dialog', { name: 'New session' }))
}

describe('A second AI provider', () => {
  afterEach(() => history.replaceState(null, '', '/'))

  it('starts a session on it, with its own models and no containers', async () => {
    history.replaceState(null, '', '/?containers=built')
    const backend = mockBackend()
    const create = vi.spyOn(backend, 'createSession')
    render(App, { props: { backend } })
    const dialog = await newSession()
    const provider = dialog.getByRole('combobox', { name: 'AI provider' })
    expect(within(provider).getAllByRole('option').map(o => o.textContent)).toEqual(['Claude Code', 'Antigravity CLI'])
    expect(dialog.getByRole('combobox', { name: 'Run in' })).toBeInTheDocument()

    await fireEvent.change(provider, { target: { value: 'antigravity' } })
    const model = dialog.getByRole('combobox', { name: 'Model' })
    expect(within(model).getAllByRole('option').map(o => o.textContent)).toEqual(['Gemini 3.8 Flash (High)', 'Gemini 3.8 Flash (Low)', 'Gemini 3.1 Pro (High)'])
    expect(dialog.queryByRole('combobox', { name: 'Run in' })).not.toBeInTheDocument()
    await fireEvent.change(model, { target: { value: 'gemini-3.8-flash-low' } })
    await fireEvent.click(dialog.getByRole('button', { name: /Start/ }))
    await waitFor(() => expect(create).toHaveBeenCalledWith('chat', '', 'gemini-3.8-flash-low', '', 'antigravity'))

    // Its session's model picker offers its models, and its text names it.
    const picker = await screen.findByRole('combobox', { name: /Model/ })
    expect(within(picker).getAllByRole('option').map(o => o.textContent)).toContain('Gemini 3.1 Pro (High)')
    expect(within(picker).queryByRole('option', { name: 'Sonnet' })).not.toBeInTheDocument()
  })

  it("can't start on a provider that isn't set up, and says where to set it up", async () => {
    history.replaceState(null, '', '/?agy=none')
    render(App, { props: { backend: mockBackend() } })
    const dialog = await newSession()
    const provider = dialog.getByRole('combobox', { name: 'AI provider' })
    expect(within(provider).getByRole('option', { name: 'Antigravity CLI (not set up)' })).toBeInTheDocument()
    await fireEvent.change(provider, { target: { value: 'antigravity' } })
    expect(dialog.getByText(/Antigravity CLI isn't installed yet/)).toBeInTheDocument()
    expect(dialog.getByRole('button', { name: /Start/ })).toBeDisabled()

    await fireEvent.click(dialog.getByRole('button', { name: 'Set up under AI Providers' }))
    const settings = within(await screen.findByRole('dialog', { name: 'Settings' }))
    await waitFor(() => expect(settings.getByRole('tab', { name: 'AI Providers' })).toHaveAttribute('aria-selected', 'true'))
    const section = within(settings.getByRole('region', { name: 'Antigravity CLI' }))
    await fireEvent.click(section.getByRole('button', { name: 'Install Antigravity CLI 1.3.1' }))
    expect(await section.findByText(/Signed in with your Google account/, {}, { timeout: 3000 })).toBeInTheDocument()
  })

  it('says its CLI shares the sign-in of its own app, and checks again on request', async () => {
    history.replaceState(null, '', '/?agy=signedout')
    const backend = mockBackend()
    const status = vi.spyOn(backend, 'providerStatus')
    render(App, { props: { backend } })
    await fireEvent.click(await screen.findByRole('button', { name: /Claude Code 2\./ }))
    const settings = within(await screen.findByRole('dialog', { name: 'Settings' }))
    await fireEvent.click(settings.getByRole('tab', { name: 'AI Providers' }))
    const section = within(settings.getByRole('region', { name: 'Antigravity CLI' }))
    expect(section.getByText(/Not signed in. Sign in to your Google account in the Antigravity app/)).toBeInTheDocument()
    // Containers are the first provider's only.
    expect(section.queryByRole('heading', { name: 'Containers' })).not.toBeInTheDocument()
    await fireEvent.click(section.getByRole('button', { name: 'Check again' }))
    await waitFor(() => expect(status).toHaveBeenCalledWith('antigravity', true))
  })
})
