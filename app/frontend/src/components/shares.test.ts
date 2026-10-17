import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte'
import { describe, expect, it, vi } from 'vitest'
import App from '../App.svelte'
import { mockBackend } from '../lib/api/mock'

const lastDialog = async () => {
  const all = await screen.findAllByRole('dialog')
  return all[all.length - 1] as HTMLElement
}

describe('Your tools and skills in a chat', () => {
  it('are chosen in the new-session dialog for Chat only, and remembered', async () => {
    const backend = mockBackend({ empty: true })
    const create = vi.spyOn(backend, 'createSession')
    render(App, { props: { backend } })
    await screen.findByRole('heading', { name: 'Welcome to UNCLI' })
    await fireEvent.keyDown(window, { key: 'n', ctrlKey: true })
    let dialog = await lastDialog()
    const tools = within(dialog).getByRole('checkbox', { name: 'Your MCP servers and connectors' })
    const skills = within(dialog).getByRole('checkbox', { name: 'Your skills' })
    expect(tools).toBeChecked()
    expect(skills).toBeChecked()
    expect(within(dialog).getByText(/Nothing else of your Claude Code setup comes along/)).toBeInTheDocument()

    await fireEvent.click(within(dialog).getByRole('radio', { name: /Code/ }))
    expect(within(dialog).queryByRole('checkbox', { name: 'Your skills' })).not.toBeInTheDocument()
    await fireEvent.click(within(dialog).getByRole('radio', { name: /Chat/ }))

    await fireEvent.click(within(dialog).getByRole('checkbox', { name: 'Your skills' }))
    await fireEvent.click(within(dialog).getByRole('button', { name: 'Start session' }))
    await waitFor(() => expect(create).toHaveBeenCalledWith('chat', '', 'sonnet', '', '', ['tools']))

    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'New session' })).not.toBeInTheDocument())
    await fireEvent.click(screen.getAllByRole('button', { name: /New session/ })[0]!)
    dialog = (await screen.findAllByRole('dialog', { name: 'New session' })).at(-1) as HTMLElement
    expect(within(dialog).getByRole('checkbox', { name: 'Your MCP servers and connectors' })).toBeChecked()
    expect(within(dialog).getByRole('checkbox', { name: 'Your skills' })).not.toBeChecked()
  })

  it('show in the header, and change from there for the next message', async () => {
    const backend = mockBackend()
    const setShares = vi.spyOn(backend, 'setShares')
    render(App, { props: { backend } })
    await screen.findByRole('heading', { name: 'Fix the flaky parser test' })
    const header = () => document.querySelector('header.top') as HTMLElement
    expect(within(header()).queryByRole('button', { name: /Your tools/ })).not.toBeInTheDocument() // Code has your settings anyway

    await fireEvent.click(screen.getByText('Plan a weekend in Lisbon'))
    await screen.findByRole('heading', { name: 'Plan a weekend in Lisbon' })
    const note = within(header()).getByRole('button', { name: 'Your tools and skills' })
    await fireEvent.click(note)
    const pop = within(header()).getByRole('dialog', { name: 'What this session can use of yours' })
    expect(within(pop).getByText(/Applies from your next message: Claude Code restarts/)).toBeInTheDocument()
    await fireEvent.click(within(pop).getByRole('checkbox', { name: 'Your MCP servers and connectors' }))
    await waitFor(() => expect(setShares).toHaveBeenCalledWith(expect.any(String), ['skills']))
    await waitFor(() => expect(within(header()).getByRole('button', { name: 'Your skills' })).toBeInTheDocument())

    await fireEvent.click(within(pop).getByRole('checkbox', { name: 'Your skills' }))
    await waitFor(() => expect(within(header()).getByRole('button', { name: 'None of your tools' })).toBeInTheDocument())
  })
})
