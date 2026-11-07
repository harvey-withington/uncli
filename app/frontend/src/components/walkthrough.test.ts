import { fireEvent, render, screen, within } from '@testing-library/svelte'
import { describe, expect, it } from 'vitest'
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
})
