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

  it('shows what a plugin would run, offers it only once enabled, and lists a broken one with its error', async () => {
    history.replaceState(null, '', '/?plugins')
    const backend = mockBackend()
    const enable = vi.spyOn(backend, 'enablePlugin')
    render(App, { props: { backend } })

    // Not offered for a session while disabled, nor the broken one.
    let dialog = await newSession()
    const names = () => within(dialog.getByRole('combobox', { name: 'AI provider' })).getAllByRole('option').map(o => o.textContent)
    expect(names()).toEqual(['Claude Code', 'Antigravity CLI', 'Grok Build (not set up)'])
    await fireEvent.click(dialog.getByRole('button', { name: 'Cancel' }))

    await fireEvent.click(await screen.findByRole('button', { name: /Claude Code 2\./ }))
    const settings = within(await screen.findByRole('dialog', { name: 'Settings' }))
    await fireEvent.click(settings.getByRole('tab', { name: 'AI Providers' }))
    const section = within(settings.getByRole('region', { name: /Example CLI/ }))
    expect(section.getByText('Plugin')).toBeInTheDocument()
    expect(section.getByText(/It does nothing until you enable it/)).toBeInTheDocument()
    expect(section.getByText('downloads.example.com')).toBeInTheDocument()
    expect(section.getByText('example --state={state} --json --yes-to-all')).toBeInTheDocument()
    expect(section.getByText(/switches off Example CLI's own permission checks/)).toBeInTheDocument()
    expect(section.queryByRole('button', { name: /Install/ })).not.toBeInTheDocument()

    const broken = within(settings.getByRole('region', { name: /half-done/ }))
    expect(broken.getByRole('alert')).toHaveTextContent("This plugin can't be used: provider.yaml: launch.args is empty")

    await fireEvent.click(section.getByRole('button', { name: 'Enable Example CLI' }))
    const confirm = within(await screen.findByRole('dialog', { name: 'Enable Example CLI?' }))
    await fireEvent.click(confirm.getByRole('button', { name: 'Enable Example CLI' }))
    await waitFor(() => expect(enable).toHaveBeenCalledWith('example-cli', 'abc123'))
    await fireEvent.click(await section.findByRole('button', { name: 'Install Example CLI 0.4.2' }))
    expect(await section.findByRole('button', { name: 'Disable Example CLI' })).toBeInTheDocument()

    await fireEvent.click(settings.getByRole('button', { name: 'Close' }))
    dialog = await newSession()
    expect(names()).toEqual(['Claude Code', 'Antigravity CLI', 'Example CLI', 'Grok Build (not set up)'])
  })

  it('signs a CLI in with a device code, and notices when it has', async () => {
    history.replaceState(null, '', '/?plugins')
    const backend = mockBackend()
    const start = vi.spyOn(backend, 'startDeviceSignIn')
    render(App, { props: { backend } })
    await fireEvent.click(await screen.findByRole('button', { name: /Claude Code 2\./ }))
    const settings = within(await screen.findByRole('dialog', { name: 'Settings' }))
    await fireEvent.click(settings.getByRole('tab', { name: 'AI Providers' }))
    const section = within(settings.getByRole('region', { name: /Grok Build/ }))
    expect(section.getByText(/Grok Build signs in with a code/)).toBeInTheDocument()
    await fireEvent.click(section.getByRole('button', { name: 'Sign in' }))
    expect(start).toHaveBeenCalledWith('grok')
    expect(await section.findByText('ABCD-1234')).toBeInTheDocument()
    expect(section.getByRole('button', { name: 'Open the link again' })).toBeInTheDocument()
    // Approved elsewhere: it says so by itself, and the code goes.
    expect(await section.findByText(/Signed in with your Grok account/, {}, { timeout: 3000 })).toBeInTheDocument()
    expect(section.queryByText('ABCD-1234')).not.toBeInTheDocument()
  })

  it('opens its CLI in a terminal to sign in, and notices when it has', async () => {
    history.replaceState(null, '', '/?agy=signedout')
    const backend = mockBackend()
    const status = vi.spyOn(backend, 'providerStatus')
    const terminal = vi.spyOn(backend, 'signInTerminal')
    const reveal = vi.spyOn(backend, 'revealCLI')
    render(App, { props: { backend } })
    await fireEvent.click(await screen.findByRole('button', { name: /Claude Code 2\./ }))
    const settings = within(await screen.findByRole('dialog', { name: 'Settings' }))
    await fireEvent.click(settings.getByRole('tab', { name: 'AI Providers' }))
    const section = within(settings.getByRole('region', { name: 'Antigravity CLI' }))
    expect(section.getByText(/Not signed in. Antigravity CLI signs in in its own window/)).toBeInTheDocument()
    // Containers are the first provider's only.
    expect(section.queryByRole('heading', { name: 'Containers' })).not.toBeInTheDocument()
    await fireEvent.click(section.getByRole('button', { name: 'Check again' }))
    await waitFor(() => expect(status).toHaveBeenCalledWith('antigravity', true))
    await fireEvent.click(section.getByRole('button', { name: 'Show Antigravity CLI in its folder' }))
    expect(reveal).toHaveBeenCalledWith('antigravity')

    await fireEvent.click(section.getByRole('button', { name: 'Sign in in a terminal' }))
    expect(terminal).toHaveBeenCalledWith('antigravity')
    expect(await section.findByText(/is open in a terminal window/)).toBeInTheDocument()
    // Signed in there: it says so by itself.
    expect(await section.findByText('Signed in with your Google account.', {}, { timeout: 3000 })).toBeInTheDocument()
    expect(section.queryByRole('button', { name: /Sign in in a terminal|Open it again/ })).not.toBeInTheDocument()
  })
})
