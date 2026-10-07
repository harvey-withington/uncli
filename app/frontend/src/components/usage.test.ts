import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte'
import { describe, expect, it } from 'vitest'
import App from '../App.svelte'
import { mockBackend } from '../lib/api/mock'
import { duration } from '../lib/format'

describe('Usage dashboard', () => {
  it('opens from the Usage popover, totals the period, and switches measure, period and table', async () => {
    localStorage.clear()
    render(App, { props: { backend: mockBackend() } })
    await fireEvent.click(await screen.findByRole('button', { name: 'Usage' }))
    await fireEvent.click(screen.getByRole('button', { name: 'Usage over time' }))
    const dash = await screen.findByRole('dialog', { name: 'Usage' })
    await within(dash).findByText('Cost by day')
    expect(within(dash).getByRole('radio', { name: '30 days' })).toHaveAttribute('aria-checked', 'true')
    expect(within(dash).getByRole('img', { name: /^Cost by day, 30 days$/ })).toBeInTheDocument()
    expect(within(dash).getByText('By model')).toBeInTheDocument()
    expect(within(dash).getByText('Opus 5.5')).toBeInTheDocument()
    expect(within(dash).getByText(/at API prices/)).toBeInTheDocument()

    await fireEvent.click(within(dash).getByRole('radio', { name: 'Tokens' }))
    expect(within(dash).getByText('Tokens by day')).toBeInTheDocument()
    await fireEvent.click(within(dash).getByRole('radio', { name: '7 days' }))
    await waitFor(() => expect(within(dash).getByRole('img', { name: /, 7 days$/ })).toBeInTheDocument())

    await fireEvent.click(within(dash).getByRole('button', { name: 'Table' }))
    const table = within(dash).getByRole('table')
    expect(within(table).getAllByRole('columnheader').map(h => h.textContent)).toEqual(['Day', 'Cost', 'Tokens', 'Turns'])
    expect(within(table).getAllByRole('row').length).toBeGreaterThan(1)

    // A session row opens the session.
    await fireEvent.click(within(dash).getByRole('button', { name: /Plan a weekend in Lisbon/ }))
    // (The dialog fades out; jsdom never finishes the fade, so check the session instead.)
    expect(await screen.findByRole('heading', { name: 'Plan a weekend in Lisbon' })).toBeInTheDocument()
  })

  it('says long times in hours', () => {
    expect(duration(6_258_000)).toBe('1 h 44 min')
    expect(duration(95_000)).toBe('1 min 35 s')
  })
})
