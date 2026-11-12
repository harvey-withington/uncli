import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte'
import { describe, expect, it, vi } from 'vitest'
import App from '../App.svelte'
import { mockBackend } from '@uncli/client/mock'

// The mock's first code page recorded its changes to parser.go (two places)
// and parser_test.go (a new file), and a command changed coverage.out.
describe('Walk through the changes', () => {
  it('shows a page’s recorded changes as slides, a changed place a slide', async () => {
    render(App, { props: { backend: mockBackend() } })
    await fireEvent.click(await screen.findByRole('button', { name: /Fix the flaky parser test/ }))
    const walk = await screen.findByRole('button', { name: 'Walk through' })
    expect(walk).toHaveAttribute('title', '3 changes in 2 files')
    await fireEvent.click(walk)
    const dialog = within(await screen.findByRole('dialog', { name: 'Walk through the changes' }))
    const slides = dialog.getByRole('region', { name: 'Why does TestMultiTurnPartial fail about one run in five?' })

    // A title slide, three changed places, then the file a command changed.
    expect(dialog.getByText('3 changes in 2 files')).toBeInTheDocument()
    expect(dialog.getByText('1 / 5')).toBeInTheDocument()
    const first = dialog.getByLabelText('Slide 2 of 5')
    expect(within(first).getByText('internal/adapter/claude/parser.go')).toBeInTheDocument()
    expect(within(first).getByText('lines 145–149')).toBeInTheDocument()
    expect(within(first).getByText('+5')).toBeInTheDocument()
    expect(first.querySelectorAll('.line.add').length).toBe(5)
    expect(first.querySelectorAll('.line.del').length).toBe(2)
    expect(first.querySelector('.line.del')).toHaveAttribute('data-n', '145')
    expect(within(dialog.getByLabelText('Slide 5 of 5')).getByText('coverage.out')).toBeInTheDocument()

    // The arrows (and keys) move, and the counter follows.
    expect(dialog.getByRole('button', { name: 'Previous' })).toBeDisabled()
    await fireEvent.click(dialog.getByRole('button', { name: 'Next' }))
    expect(dialog.getByText('2 / 5')).toBeInTheDocument()
    await fireEvent.keyDown(slides, { key: 'End' })
    expect(dialog.getByText('5 / 5')).toBeInTheDocument()
    expect(dialog.getByRole('button', { name: 'Next' })).toBeDisabled()
    expect(dialog.getByRole('button', { name: 'Full screen' })).toBeInTheDocument()
  })

  it('explains the changes, with the options last used or chosen first', async () => {
    localStorage.removeItem('uncli-explain')
    const backend = mockBackend()
    const explain = vi.spyOn(backend, 'explainPage')
    render(App, { props: { backend } })
    await fireEvent.click(await screen.findByRole('button', { name: /Fix the flaky parser test/ }))
    await fireEvent.click(await screen.findByRole('button', { name: 'Walk through' }))
    const dialog = within(await screen.findByRole('dialog', { name: 'Walk through the changes' }))

    // Explain, with the defaults: this page, brief, the session's model.
    await fireEvent.click(dialog.getByRole('button', { name: 'Explain' }))
    expect(await dialog.findByText(/kept the partial line until the message that ends it|keeps the partial line until the message that ends it/)).toBeInTheDocument()
    expect(explain).toHaveBeenLastCalledWith('s-code', expect.any(String), { model: '', context: 'page', pages: 3, detail: 'brief' })
    expect(within(dialog.getByLabelText('Slide 2 of 5')).getByText(/only flushed by the message it belongs to/)).toBeInTheDocument()
    expect(within(dialog.getByLabelText('Slide 4 of 5')).getByText(/replays the recorded stream fifty times/)).toBeInTheDocument()
    expect(dialog.getByText(/^Explained by .* · \$0\.00/)).toBeInTheDocument()

    // The notes fold away on every slide, for the code's sake, and come back.
    await fireEvent.click(within(dialog.getByLabelText('Slide 2 of 5')).getByRole('button', { name: 'Hide the notes, for more room for the code' }))
    expect(dialog.queryByText(/only flushed by the message it belongs to/)).not.toBeInTheDocument()
    expect(dialog.queryByText(/replays the recorded stream fifty times/)).not.toBeInTheDocument()
    expect(within(dialog.getByLabelText('Slide 2 of 5')).getByText('internal/adapter/claude/parser.go')).toBeInTheDocument()
    await fireEvent.click(within(dialog.getByLabelText('Slide 4 of 5')).getByRole('button', { name: 'Show the notes' }))
    expect(dialog.getByText(/only flushed by the message it belongs to/)).toBeInTheDocument()

    // With options: the whole session, thorough; then remembered.
    await fireEvent.click(dialog.getByRole('button', { name: 'Explain with options…' }))
    const options = within(await screen.findByRole('dialog', { name: 'Explain these changes' }))
    expect(await options.findByText('Sends about 3k tokens, in a call of its own (not the session).')).toBeInTheDocument()
    await fireEvent.click(options.getByRole('radio', { name: 'The whole session' }))
    await fireEvent.click(options.getByRole('radio', { name: /Thorough/ }))
    expect(await options.findByText('Sends about 23k tokens, in a call of its own (not the session).')).toBeInTheDocument()
    await fireEvent.click(options.getByRole('button', { name: 'Explain' }))
    await waitFor(() => expect(explain).toHaveBeenLastCalledWith('s-code', expect.any(String), { model: '', context: 'session', pages: 3, detail: 'thorough' }))
    expect(JSON.parse(localStorage.getItem('uncli-explain') ?? '{}')).toMatchObject({ context: 'session', detail: 'thorough' })
    expect(await dialog.findByRole('button', { name: 'Explain again' })).toBeInTheDocument()
  })

  it('starts a new session in the same place from what the walkthrough knows', async () => {
    const backend = mockBackend()
    const create = vi.spyOn(backend, 'createSession')
    const send = vi.spyOn(backend, 'send')
    render(App, { props: { backend } })
    await fireEvent.click(await screen.findByRole('button', { name: /Fix the flaky parser test/ }))
    await fireEvent.click(await screen.findByRole('button', { name: 'Walk through' }))
    const walk = within(await screen.findByRole('dialog', { name: 'Walk through the changes' }))
    await fireEvent.click(walk.getByRole('button', { name: 'Explain' }))
    await walk.findByText(/only flushed by the message it belongs to/)

    // The walkthrough gives way to the new session dialog, in the session's folder.
    await fireEvent.click(walk.getByRole('button', { name: 'New session' }))
    const dialog = within(await screen.findByRole('dialog', { name: 'New session' }))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Walk through the changes' })).not.toBeInTheDocument())
    expect(dialog.getByText(/^From the walkthrough: it starts with what that showed/)).toBeInTheDocument()
    expect(dialog.getByText('C:\\Users\\you\\code')).toBeInTheDocument()

    await fireEvent.click(dialog.getByRole('button', { name: 'Start session' }))
    await waitFor(() => expect(send).toHaveBeenCalled())
    expect(create).toHaveBeenLastCalledWith('code', 'C:\\Users\\you\\code', expect.any(String), '', '', null, expect.objectContaining({ title: 'From: Why does TestMultiTurnPartial fail about one run in five?' }))
    const [, text] = send.mock.lastCall!
    expect(text).toContain('> Why does TestMultiTurnPartial fail about one run in five?')
    expect(text).toContain('internal/adapter/claude/parser.go, lines 145-149 (+5 -2): ')
    expect(text).toMatch(/only flushed by the message it belongs to/)
    expect(send.mock.lastCall![0]).not.toBe('s-code')
  })
})
