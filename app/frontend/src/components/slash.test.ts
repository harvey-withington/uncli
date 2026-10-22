import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte'
import { describe, expect, it, vi } from 'vitest'
import App from '../App.svelte'
import { mockBackend } from '../lib/api/mock'

const box = async () => (await screen.findByRole('textbox', { name: 'Your message' })) as HTMLTextAreaElement
async function type(el: HTMLTextAreaElement, text: string) {
  el.value = text
  el.setSelectionRange(text.length, text.length)
  await fireEvent.input(el, { target: { value: text } })
}

describe('Slash commands', () => {
  it('opens a menu of the CLI’s commands on "/", filters it, and completes one', async () => {
    const backend = mockBackend()
    const send = vi.spyOn(backend, 'send')
    render(App, { props: { backend } })
    await screen.findByRole('heading', { name: 'Fix the flaky parser test' })
    const el = await box()
    await type(el, '/')
    const menu = within(await screen.findByRole('listbox', { name: 'Commands' }))
    expect(menu.getAllByRole('option').length).toBe(8)

    await type(el, '/co')
    // Names that start with "co" first; then ones that mention it.
    await waitFor(() => expect(menu.getAllByRole('option').map(o => o.querySelector('.name')?.textContent).slice(0, 4)).toEqual(['/code-review', '/compact', '/context', '/cost']))
    expect(menu.getByText('[low|medium|high]')).toBeInTheDocument()
    await fireEvent.keyDown(el, { key: 'ArrowDown' })
    expect(menu.getAllByRole('option')[1]).toHaveAttribute('aria-selected', 'true')
    await fireEvent.keyDown(el, { key: 'Tab' })
    expect(el.value).toBe('/compact ')
    expect(screen.queryByRole('listbox', { name: 'Commands' })).not.toBeInTheDocument()
    expect(send).not.toHaveBeenCalled() // choosing doesn't send

    await type(el, '/si')
    await screen.findByRole('listbox', { name: 'Commands' })
    await fireEvent.keyDown(el, { key: 'Escape' })
    expect(screen.queryByRole('listbox', { name: 'Commands' })).not.toBeInTheDocument()
    await fireEvent.keyDown(el, { key: 'Enter' }) // the menu is closed: Enter sends
    await waitFor(() => expect(send).toHaveBeenCalledWith('s-code', '/si', []))
  })

  it('lists the CLI’s commands in the command palette', async () => {
    const backend = mockBackend()
    const spy = vi.spyOn(backend, 'commands')
    render(App, { props: { backend } })
    await screen.findByRole('heading', { name: 'Fix the flaky parser test' })
    await waitFor(() => expect(spy).toHaveBeenCalledWith('s-code'))
    await spy.mock.results[0]?.value
    await fireEvent.keyDown(window, { key: 'P', ctrlKey: true, shiftKey: true })
    const palette = within(await screen.findByRole('dialog', { name: 'Command palette' }))
    await fireEvent.input(palette.getByRole('combobox'), { target: { value: 'simplify' } })
    expect(await palette.findByText('Run /simplify')).toBeInTheDocument()
  })

  it('offers none for a CLI that takes none', async () => {
    history.replaceState(null, '', '/?plugins')
    const backend = mockBackend({ empty: true })
    render(App, { props: { backend } })
    await screen.findByRole('heading', { name: 'Welcome to UNCLI' })
    await fireEvent.keyDown(window, { key: 'n', ctrlKey: true })
    const dialog = within((await screen.findAllByRole('dialog')).at(-1) as HTMLElement)
    await fireEvent.change(dialog.getByRole('combobox', { name: 'AI provider' }), { target: { value: 'antigravity' } })
    await fireEvent.click(dialog.getByRole('button', { name: 'Start session' }))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'New session' })).not.toBeInTheDocument())
    expect(screen.queryByRole('button', { name: 'Compact' })).not.toBeInTheDocument() // the toolbar's /compact
    await type(await box(), '/')
    await new Promise(r => setTimeout(r, 50))
    expect(screen.queryByRole('listbox', { name: 'Commands' })).not.toBeInTheDocument()
    history.replaceState(null, '', '/')
  })
})

describe('Slash commands before any CLI has reported them', () => {
  it('says it is getting them, then shows them when the CLI reports', async () => {
    const backend = mockBackend({ commandsLate: true })
    render(App, { props: { backend } })
    await screen.findByRole('heading', { name: 'Fix the flaky parser test' })
    const el = await box()
    await type(el, '/')
    const menu = within(await screen.findByRole('listbox', { name: 'Commands' }))
    expect(menu.getByText("Getting Claude Code's commands…")).toBeInTheDocument()
    await waitFor(() => expect(menu.getAllByRole('option').length).toBe(8), { timeout: 3000 })
    expect(menu.queryByText(/Getting/)).not.toBeInTheDocument()
  })
})
