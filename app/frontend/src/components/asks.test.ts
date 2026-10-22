import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte'
import { describe, expect, it, vi } from 'vitest'
import App from '../App.svelte'
import { mockBackend } from '../lib/api/mock'

async function ask(text: string) {
  const box = (await screen.findByRole('textbox', { name: 'Your message' })) as HTMLTextAreaElement
  await fireEvent.input(box, { target: { value: text } })
  await fireEvent.click(await screen.findByRole('button', { name: 'Send' }, { timeout: 4000 }))
}

describe('Questions and plans from the model', () => {
  it('answers a question by picking options and typing one’s own', async () => {
    const backend = mockBackend()
    const answer = vi.spyOn(backend, 'answerAsk')
    render(App, { props: { backend } })
    await ask('ask me where to go')
    const card = within(await screen.findByRole('alertdialog', { name: 'Claude asks' }, { timeout: 3000 }))
    expect(card.getByText('Which city do you prefer?')).toBeInTheDocument()
    expect(card.getByText('Pick any')).toBeInTheDocument() // the meals question takes several
    const answerBtn = card.getByRole('button', { name: 'Answer' })
    expect(answerBtn).toBeDisabled()

    await fireEvent.click(card.getByRole('radio', { name: /Lisbon/ }))
    await fireEvent.click(card.getByRole('radio', { name: /Porto/ })) // one city: Porto replaces Lisbon
    expect(card.getByRole('radio', { name: /Lisbon/ })).toHaveAttribute('aria-checked', 'false')
    await fireEvent.click(card.getByRole('checkbox', { name: 'Breakfast' }))
    await fireEvent.click(card.getByRole('checkbox', { name: 'Lunch' }))
    await fireEvent.input(card.getByRole('textbox', { name: 'Your own answer to question 2' }), { target: { value: 'a picnic' } })
    await fireEvent.click(answerBtn)
    expect(answer).toHaveBeenCalledWith('s-code', expect.any(String), true,
      { 'Which city do you prefer?': 'Porto', 'Which meals should I plan?': 'Breakfast, Lunch, a picnic' }, '')
    expect(await screen.findByText(/places that fit what you picked/, {}, { timeout: 4000 })).toBeInTheDocument()
  }, 15000)

  it('shows a plan, sends it back with a note, and approves the next', async () => {
    const backend = mockBackend()
    const answer = vi.spyOn(backend, 'answerAsk')
    render(App, { props: { backend } })
    await ask('plan the test fix')
    let card = within(await screen.findByRole('alertdialog', { name: "Claude's plan" }, { timeout: 3000 }))
    expect(card.getByRole('heading', { name: 'Plan: tidy the parser tests' })).toBeInTheDocument()
    expect(card.getByText(/Give each test its own fixture copy/)).toBeInTheDocument()
    await fireEvent.input(card.getByRole('textbox', { name: 'What should change' }), { target: { value: 'Run them fifty times.' } })
    await fireEvent.click(card.getByRole('button', { name: 'Keep planning' }))
    expect(answer).toHaveBeenCalledWith('s-code', expect.any(String), false, null, 'Run them fifty times.')
    await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull())

    await ask('plan again')
    card = within(await screen.findByRole('alertdialog', { name: "Claude's plan" }, { timeout: 5000 }))
    await fireEvent.click(card.getByRole('button', { name: 'Approve and go ahead' }))
    expect(answer).toHaveBeenLastCalledWith('s-code', expect.any(String), true, null, '')
  }, 20000)
})
