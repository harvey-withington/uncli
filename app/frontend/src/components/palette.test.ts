import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte'
import { describe, expect, it, vi } from 'vitest'
import App from '../App.svelte'
import { mockBackend } from '../lib/api/mock'

async function start(backend = mockBackend()) {
  localStorage.clear()
  render(App, { props: { backend } })
  await screen.findByRole('heading', { name: 'Fix the flaky parser test' })
  return backend
}

describe('Command palette', () => {
  it('opens with Ctrl+Shift+P, filters as you type and runs a command with Enter', async () => {
    const backend = await start()
    const setMode = vi.spyOn(backend, 'setMode')
    await fireEvent.keyDown(window, { key: 'P', ctrlKey: true, shiftKey: true })
    const palette = await screen.findByRole('dialog', { name: 'Command palette' })
    const input = within(palette).getByRole('combobox')
    // The prompt level in use is marked.
    expect(within(within(palette).getByRole('option', { name: /Prompt me: When unsafe/ })).getByLabelText('In use now')).toBeInTheDocument()
    await fireEvent.input(input, { target: { value: 'pr nev' } })
    const options = within(palette).getAllByRole('option')
    expect(options[0]).toHaveTextContent('Prompt me: Never')
    expect(options[0]).toHaveAttribute('aria-selected', 'true')
    await fireEvent.keyDown(input, { key: 'Enter' })
    expect(setMode).toHaveBeenCalledWith('s-code', 'never')
    // Run last, it now comes first.
    await fireEvent.keyDown(window, { key: 'F1' })
    const again = await screen.findByRole('dialog', { name: 'Command palette' })
    await waitFor(() => expect(within(again).getAllByRole('option')[0]).toHaveTextContent('Prompt me: Never'))
  })

  it('opens with F1 even from the message box', async () => {
    await start()
    const box = screen.getByPlaceholderText(/Ask anything/)
    box.focus()
    await fireEvent.keyDown(box, { key: 'F1' })
    expect(await screen.findByRole('dialog', { name: 'Command palette' })).toBeInTheDocument()
  })

  it('moves with the arrows, says when nothing matches, and closes with Escape', async () => {
    const backend = await start()
    const bookmark = vi.spyOn(backend, 'setBookmark')
    await fireEvent.keyDown(window, { key: 'F1' })
    const palette = await screen.findByRole('dialog', { name: 'Command palette' })
    const input = within(palette).getByRole('combobox')
    await fireEvent.keyDown(input, { key: 'ArrowDown' })
    expect(within(palette).getAllByRole('option')[1]).toHaveAttribute('aria-selected', 'true')
    expect(input).toHaveAttribute('aria-activedescendant', 'palette-opt-1')
    await fireEvent.input(input, { target: { value: 'zzzz' } })
    expect(within(palette).getByText('No command matches.')).toBeInTheDocument()
    await fireEvent.keyDown(input, { key: 'Escape' })
    // Closed: shortcuts work again (a dialog owns the keyboard while open).
    await fireEvent.keyDown(document.body, { key: 'b' })
    expect(bookmark).toHaveBeenCalled()
  })

  it('opens other parts of the app: a session, the usage dashboard', async () => {
    await start()
    await fireEvent.keyDown(window, { key: 'F1' })
    let input = within(await screen.findByRole('dialog', { name: 'Command palette' })).getByRole('combobox')
    await fireEvent.input(input, { target: { value: 'go lisbon' } })
    await fireEvent.keyDown(input, { key: 'Enter' })
    expect(await screen.findByRole('heading', { name: 'Plan a weekend in Lisbon' })).toBeInTheDocument()
    await fireEvent.keyDown(window, { key: 'F1' })
    input = within(await screen.findByRole('dialog', { name: 'Command palette' })).getByRole('combobox')
    await fireEvent.input(input, { target: { value: 'usage' } })
    await fireEvent.keyDown(input, { key: 'Enter' })
    expect(await screen.findByRole('dialog', { name: 'Usage' })).toBeInTheDocument()
  })
})

describe('Keyboard map', () => {
  it('opens with ? and lists every shortcut, those of one place too', async () => {
    await start()
    await fireEvent.keyDown(window, { key: '?', shiftKey: true })
    const map = await screen.findByRole('dialog', { name: 'Keyboard shortcuts' })
    expect(within(map).getByText('Pin or unpin the page')).toBeInTheDocument()
    expect(within(map).getByText('Command palette')).toBeInTheDocument()
    expect(within(map).getByText('Send / new line')).toBeInTheDocument()
  })

  it('leaves arrows on a radio group to the group, not the pages', async () => {
    await start()
    const group = screen.getByRole('radiogroup', { name: /Prompt me/ })
    const before = screen.getByText(/^1/, { selector: '.pos span' }).textContent
    await fireEvent.keyDown(within(group).getByRole('radio', { name: /When unsafe/ }), { key: 'ArrowLeft' })
    expect(screen.getByText(/^1/, { selector: '.pos span' }).textContent).toBe(before)
  })
})
