import { fireEvent, render, screen } from '@testing-library/svelte'
import { describe, expect, it } from 'vitest'
import App from '../App.svelte'
import { mockBackend } from '../lib/api/mock'

describe('Session header', () => {
  it('collapses to two lines: the folder after the title, no status lines; remembered', async () => {
    localStorage.clear()
    const { unmount } = render(App, { props: { backend: mockBackend() } })
    await screen.findByRole('heading', { name: 'Fix the flaky parser test' })
    const header = document.querySelector('header.top') as HTMLElement
    expect(header.querySelector('.workdir')).toHaveTextContent(/Code · C:.Users.you.code/)
    expect(screen.getByText(/Claude prompts you before anything unsafe/)).toBeInTheDocument()

    const toggle = screen.getByRole('button', { name: 'Compact header: two lines' })
    expect(toggle).toHaveAttribute('aria-expanded', 'true')
    // The chevron is the last control on the title line, expanded or not.
    expect(header.querySelector('.title-row')?.lastElementChild).toBe(toggle)
    await fireEvent.click(toggle)
    expect(header.querySelector('.workdir')).toBeNull()
    expect(header.querySelector('.title-row .path')).toHaveTextContent(/Code · C:.Users.you.code/)
    expect(screen.queryByText(/Claude prompts you before anything unsafe/)).not.toBeInTheDocument()
    const expand = screen.getByRole('button', { name: 'Show the whole header' })
    expect(header.querySelector('.title-row')?.lastElementChild).toBe(expand)
    unmount()

    render(App, { props: { backend: mockBackend() } })
    await screen.findByRole('heading', { name: 'Fix the flaky parser test' })
    expect(screen.getByRole('button', { name: 'Show the whole header' })).toHaveAttribute('aria-expanded', 'false')
  })
})
