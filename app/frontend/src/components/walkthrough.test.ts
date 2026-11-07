import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte'
import { describe, expect, it } from 'vitest'
import App from '../App.svelte'
import { mockBackend } from '@uncli/client/mock'

// The mock's first code page recorded its changes to parser.go (two places)
// and parser_test.go (a new file), and a command changed coverage.out.
describe('Walk through the changes', () => {
  it('plays a page’s recorded changes as slides in a dialog', async () => {
    render(App, { props: { backend: mockBackend() } })
    await fireEvent.click(await screen.findByRole('button', { name: /Fix the flaky parser test/ }))
    const walk = await screen.findByRole('button', { name: 'Walk through' })
    expect(walk).toHaveAttribute('title', '3 changes in 2 files')
    await fireEvent.click(walk)
    const dialog = within(await screen.findByRole('dialog', { name: 'Walk through the changes' }))
    expect(dialog.getByRole('button', { name: 'Full screen' })).toBeInTheDocument()
    const frame = dialog.getByTitle('Walk through the changes') as HTMLIFrameElement
    expect(frame).toHaveAttribute('sandbox', 'allow-scripts')
    const doc = frame.getAttribute('srcdoc') ?? ''
    expect(doc).toContain('Why does TestMultiTurnPartial fail about one run in five?')
    expect(doc).toContain('3 changes in 2 files')
    expect(doc).toContain('internal/adapter/claude/parser.go · lines 145–149 · 1 of 2')
    expect(doc).toContain('internal/adapter/claude/parser_test.go · lines 1–13 · 1 of 1')
    expect(doc).toContain('coverage.out')
    expect(doc).toContain('class="line add"')

    // Escape inside the deck closes the dialog.
    window.dispatchEvent(new MessageEvent('message', { data: { 'uncli-deck': 'escape' }, source: frame.contentWindow }))
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Walk through the changes' })).not.toBeInTheDocument())
  })
})
