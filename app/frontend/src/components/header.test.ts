import { fireEvent, render, screen, within } from '@testing-library/svelte'
import { describe, expect, it } from 'vitest'
import App from '../App.svelte'
import { mockBackend } from '../lib/api/mock'
import { APP_CONTEXT } from '../lib/context'
import { AppStore } from '../stores/app.svelte'
import SessionPane from './SessionPane.svelte'
import SessionStatus from './SessionStatus.svelte'

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

describe('Session title', () => {
  it('renames the session when its title in the header is clicked; Escape cancels', async () => {
    localStorage.clear()
    render(App, { props: { backend: mockBackend() } })
    const heading = await screen.findByRole('heading', { name: 'Fix the flaky parser test' })
    await fireEvent.click(within(heading).getByRole('button'))
    let box = within(heading).getByRole('textbox', { name: 'Rename' })
    await fireEvent.input(box, { target: { value: 'Not this' } })
    await fireEvent.keyDown(box, { key: 'Escape' })
    expect(heading).toHaveAccessibleName('Fix the flaky parser test')

    await fireEvent.click(within(heading).getByRole('button'))
    box = within(heading).getByRole('textbox', { name: 'Rename' })
    await fireEvent.input(box, { target: { value: 'Parser test, fixed' } })
    await fireEvent.keyDown(box, { key: 'Enter' })
    expect(await screen.findByRole('heading', { name: 'Parser test, fixed' })).toBeInTheDocument()
  })
})

describe('A stopped session', () => {
  it('says the next message starts it again', async () => {
    const backend = mockBackend()
    const s = new AppStore(backend)
    await s.init()
    const v = s.sessions.find(x => x.id === s.currentId)!
    render(SessionStatus, { props: { session: { ...v, state: 'exited' } }, context: new Map([[APP_CONTEXT, s]]) })
    expect(screen.getByText(/isn't running for this session\. Your next message starts it again/)).toBeInTheDocument()
  })
})

describe('Notes strip', () => {
  async function store() {
    localStorage.clear()
    const s = new AppStore(mockBackend())
    await s.init()
    await s.select('s-code')
    return s
  }
  const ctx = (s: AppStore) => new Map([[APP_CONTEXT, s]])

  it('holds the status line and an error under the header, but not MCP servers (the plug button has them)', async () => {
    const s = await store()
    render(SessionPane, { props: { session: { ...s.current!, state: 'error', error: 'The CLI stopped.' } }, context: ctx(s) })
    const notes = document.querySelector('.notes') as HTMLElement
    expect(notes.previousElementSibling).toBe(document.querySelector('header.top'))
    expect(within(notes).getByText(/Claude prompts you before anything unsafe/)).toBeInTheDocument()
    expect(within(notes).queryByText(/BRUV/)).toBeNull()
    expect(within(notes).getByRole('alert')).toHaveTextContent('The CLI stopped.')
    expect(document.querySelector('header.top [role=alert]')).toBeNull()
  })

  it('shows only an error when the header is compact, and nothing without one', async () => {
    const s = await store()
    s.toggleHeaderCompact()
    const { rerender } = render(SessionPane, { props: { session: { ...s.current!, state: 'error', error: 'The CLI stopped.' } }, context: ctx(s) })
    const notes = document.querySelector('.notes') as HTMLElement
    expect(within(notes).getByRole('alert')).toHaveTextContent('The CLI stopped.')
    expect(within(notes).queryByText(/BRUV/)).toBeNull()
    expect(within(notes).queryByText(/prompts you/)).toBeNull()
    await rerender({ session: s.current! })
    expect(document.querySelector('.notes')).toBeNull()
  })

  it('words the answer-length hint for the session’s own provider', async () => {
    const s = await store()
    render(SessionPane, { props: { session: { ...s.current!, adapter: 'antigravity' } }, context: ctx(s) })
    const button = screen.getByRole('button', { name: /^Efficient \/ Normal \/ Thorough: / })
    expect(button.title).toContain('Asks Antigravity to use fewer tokens')
  })
})

