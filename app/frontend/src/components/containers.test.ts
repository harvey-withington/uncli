import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte'
import { afterEach, describe, expect, it, vi } from 'vitest'
import App from '../App.svelte'
import { mockBackend } from '../lib/api/mock'

async function openSettings(tab: string) {
  await fireEvent.click(await screen.findByRole('button', { name: /Claude Code 2\./ }))
  const dialog = within(await screen.findByRole('dialog', { name: 'Settings' }))
  await fireEvent.click(dialog.getByRole('tab', { name: tab }))
  return dialog
}

describe('Containers', () => {
  afterEach(() => history.replaceState(null, '', '/'))

  it('builds a container, signs its provider in, and offers it for a new session', async () => {
    const backend = mockBackend()
    const create = vi.spyOn(backend, 'createSession')
    render(App, { props: { backend } })
    const settings = await openSettings('Containers')
    expect(await settings.findByText('Sandbox')).toBeInTheDocument()
    expect(settings.getByText(/^Alpine Linux 3\.24 · coreutils · .* · py3-pip$/)).toBeInTheDocument()
    expect(settings.getByText('Shares your memories, MCP servers, and connectors')).toBeInTheDocument()
    expect(settings.getByText(/Your connectors need the whole-account sign-in/)).toBeInTheDocument()
    expect(settings.getByText('Alpine Linux 3.24 · nodejs · npm')).toBeInTheDocument()
    expect(settings.getByText("Claude Code isn't signed in for containers yet.")).toBeInTheDocument()

    await fireEvent.click(settings.getAllByRole('button', { name: 'Build' })[0]!)
    expect(await settings.findByText('Downloading…')).toBeInTheDocument()
    await waitFor(() => expect(settings.getByText('Built')).toBeInTheDocument(), { timeout: 5000 })

    // Sign-in belongs to the provider: the link goes to its tab.
    await fireEvent.click(settings.getByRole('button', { name: 'Sign in under AI Providers' }))
    await waitFor(() => expect(settings.getByRole('tab', { name: 'AI Providers' })).toHaveAttribute('aria-selected', 'true'))
    expect(settings.getByRole('heading', { name: 'Claude Code' })).toBeInTheDocument()
    await fireEvent.click(settings.getByRole('button', { name: 'Sign in for containers' }))
    const code = await settings.findByRole('textbox', { name: 'Code from the sign-in page' })
    expect(settings.getByRole('button', { name: 'Open the link' })).toBeInTheDocument()
    await fireEvent.input(code, { target: { value: 'abc#state' } })
    await fireEvent.click(settings.getByRole('button', { name: 'Finish' }))
    expect(await settings.findByText(/Signed in\. The token is kept in Windows Credential Manager/)).toBeInTheDocument()
    // The whole-account sign-in, for containers that share connectors.
    expect(settings.getByText(/can use this sign-in to reach your account and connectors/)).toBeInTheDocument()
    await fireEvent.click(settings.getByRole('button', { name: 'Sign in to your Claude account' }))
    const code2 = await settings.findByRole('textbox', { name: 'Code from the sign-in page' })
    await fireEvent.input(code2, { target: { value: 'def#state' } })
    await fireEvent.click(settings.getByRole('button', { name: 'Finish' }))
    expect(await settings.findByText(/Signed in to your whole account/)).toBeInTheDocument()
    await fireEvent.click(settings.getByRole('tab', { name: 'Containers' }))
    expect(settings.getByText('Claude Code is signed in for containers.')).toBeInTheDocument()
    expect(settings.queryByText(/Your connectors need the whole-account sign-in/)).not.toBeInTheDocument()

    await fireEvent.click(screen.getByRole('button', { name: 'Close' }))
    await fireEvent.click(screen.getByRole('button', { name: /New session/ }))
    const dialog = within(await screen.findByRole('dialog', { name: 'New session' }))
    const runIn = dialog.getByRole('combobox', { name: 'Run in' })
    expect(within(runIn).getAllByRole('option').map(o => o.textContent)).toEqual(['This computer', 'Sandbox'])
    await fireEvent.change(runIn, { target: { value: 'sandbox' } })
    await fireEvent.click(dialog.getByRole('button', { name: /Start/ }))
    await waitFor(() => expect(create).toHaveBeenCalledWith('chat', '', expect.any(String), 'sandbox'))
    expect(await screen.findByText(/in the Sandbox container/)).toBeInTheDocument()
  }, 15000)

  it('shows a build at once, so Build can\'t be clicked twice', async () => {
    const backend = mockBackend()
    let release: () => void = () => {}
    const build = vi.spyOn(backend, 'buildContainer').mockImplementation(() => new Promise<void>(r => (release = r)))
    render(App, { props: { backend } })
    const settings = await openSettings('Containers')
    await settings.findByText('Sandbox')
    const button = settings.getAllByRole('button', { name: 'Build' })[0]!
    await fireEvent.click(button)
    expect(button).toBeDisabled()
    expect(settings.getByText('Starting…')).toBeInTheDocument()
    await fireEvent.click(button)
    expect(build).toHaveBeenCalledTimes(1)
    // Started, before any progress arrives: it says so, still disabled.
    release()
    expect(await settings.findByText('Downloading…')).toBeInTheDocument()
    expect(button).toBeDisabled()
  })

  it('when WSL stops responding, says so and stops only its own containers after asking', async () => {
    history.replaceState(null, '', '/?containers=hung')
    const backend = mockBackend()
    const restart = vi.spyOn(backend, 'stopContainers')
    render(App, { props: { backend } })
    const settings = await openSettings('Containers')
    expect(await settings.findByRole('alert')).toHaveTextContent(/WSL isn't responding. Its service/)
    await fireEvent.click(settings.getByRole('button', { name: "Stop UNCLI's containers" }))
    const confirm = within(await screen.findByRole('dialog', { name: "Stop UNCLI's containers?" }))
    expect(confirm.getByText(/Other apps' WSL distros and containers are left alone/)).toBeInTheDocument()
    await fireEvent.click(confirm.getByRole('button', { name: "Stop UNCLI's containers" }))
    await waitFor(() => expect(restart).toHaveBeenCalled())
    expect(await settings.findByText('WSL 3.0.1.0')).toBeInTheDocument()
  })

  it('says when a built container no longer matches its config, until rebuilt', async () => {
    history.replaceState(null, '', '/?containers=built')
    render(App, { props: { backend: mockBackend() } })
    const settings = await openSettings('Containers')
    expect(await settings.findByText('Changed')).toBeInTheDocument()
    expect(settings.getByText('Changed since it was built: its packages. Rebuild to update it.')).toBeInTheDocument()
    await fireEvent.click(settings.getByRole('button', { name: 'Rebuild' }))
    await waitFor(() => expect(settings.getByText('Built')).toBeInTheDocument(), { timeout: 6000 })
    expect(settings.queryByText(/Changed since it was built/)).not.toBeInTheDocument()
  }, 10000)

  it('puts a dot on Settings when a container needs the user, and opens it there', async () => {
    history.replaceState(null, '', '/?containers=built') // the Sandbox's packages changed
    render(App, { props: { backend: mockBackend() } })
    const button = await screen.findByRole('button', { name: /Something in Settings needs you/ })
    expect(button).toHaveAttribute('title', expect.stringContaining('A container needs a rebuild'))
    await fireEvent.click(button)
    const dialog = within(await screen.findByRole('dialog', { name: 'Settings' }))
    const tab = dialog.getByRole('tab', { name: 'Containers' })
    expect(tab).toHaveAttribute('aria-selected', 'true')
    expect(tab).toHaveAccessibleDescription(/Something in Settings needs you. A container needs a rebuild/)
    await fireEvent.click(dialog.getByRole('button', { name: 'Rebuild' }))
    await waitFor(() => expect(screen.queryByRole('button', { name: /Something in Settings needs you/ })).not.toBeInTheDocument(), { timeout: 6000 })
  }, 10000)

  it('without WSL, offers to turn it on', async () => {
    history.replaceState(null, '', '/?containers=nowsl')
    render(App, { props: { backend: mockBackend() } })
    const settings = await openSettings('Containers')
    expect(await settings.findByText(/Containers need WSL/)).toBeInTheDocument()
    await fireEvent.click(settings.getByRole('button', { name: 'Turn on WSL' }))
    expect(await screen.findByText(/Restart Windows|restart before WSL works/)).toBeInTheDocument()
  })
})

describe('Settings tabs', () => {
  it('opens on a section\'s tab and moves with the arrow keys', async () => {
    render(App, { props: { backend: mockBackend() } })
    await fireEvent.click(await screen.findByRole('button', { name: /Claude Code 2\./ }))
    const dialog = within(await screen.findByRole('dialog', { name: 'Settings' }))
    const tabs = dialog.getAllByRole('tab').map(t => t.textContent)
    expect(tabs).toEqual(['General', 'Approvals', 'AI Providers', 'Containers'])
    const general = dialog.getByRole('tab', { name: 'General' })
    await fireEvent.click(general)
    expect(dialog.getByRole('combobox', { name: 'Theme' })).toBeInTheDocument()
    await fireEvent.keyDown(general, { key: 'ArrowDown' })
    expect(dialog.getByRole('tab', { name: 'Approvals' })).toHaveAttribute('aria-selected', 'true')
    expect(dialog.getByRole('combobox', { name: "When UNCLI can't tell" })).toBeInTheDocument()
    await fireEvent.keyDown(dialog.getByRole('tab', { name: 'Approvals' }), { key: 'End' })
    expect(dialog.getByRole('tab', { name: 'Containers' })).toHaveAttribute('aria-selected', 'true')
  })
})
