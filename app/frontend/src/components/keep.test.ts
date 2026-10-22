import { fireEvent, render, screen, waitFor } from '@testing-library/svelte'
import { tick } from 'svelte'
import { describe, expect, it } from 'vitest'
import App from '../App.svelte'
import { mockBackend } from '../lib/api/mock'
import { APP_CONTEXT } from '../lib/context'
import { AppStore } from '../stores/app.svelte'
import SessionPane from './SessionPane.svelte'

const nextFrame = () => new Promise(r => requestAnimationFrame(() => r(null)))

describe('The page scroller', () => {
  it('stays where it is when the page updates, and each page opens where it was left', async () => {
    const s = new AppStore(mockBackend())
    await s.init()
    await s.select('s-chat') // two pages
    render(SessionPane, { props: { session: s.current! }, context: new Map([[APP_CONTEXT, s]]) })
    const scroller = document.querySelector('.scroll') as HTMLElement
    await nextFrame()
    scroller.scrollTop = 300
    await fireEvent.scroll(scroller)

    // An update to the page being read (a tool finishing, text arriving).
    const p = s.currentPage!
    s.upsertPage({ ...p, answerMd: p.answerMd + '\n\nMore.' })
    await tick()
    await nextFrame()
    expect(scroller.scrollTop).toBe(300)

    // Another page starts at the top; coming back finds the place again.
    s.goTo(0)
    await tick()
    await nextFrame()
    expect(scroller.scrollTop).toBe(0)
    s.goTo(s.currentPages.length - 1)
    await tick()
    await nextFrame()
    expect(scroller.scrollTop).toBe(300)
  })

  it('has a toggle under its scrollbar to follow the end, kept for next time', async () => {
    render(App, { props: { backend: mockBackend() } })
    await screen.findByRole('heading', { name: 'Fix the flaky parser test' })
    const toggle = screen.getByRole('button', { name: 'Follow the end of the answer' })
    expect(toggle).toHaveAttribute('aria-pressed', 'false')
    expect(toggle).toHaveAttribute('title', expect.stringMatching(/^Staying where you are/))
    await fireEvent.click(toggle)
    expect(toggle).toHaveAttribute('aria-pressed', 'true')
    expect(JSON.parse(localStorage.getItem('uncli-scroll') ?? '{}').follow).toBe(true)
  })
})

describe('What is typed', () => {
  it('survives a restart, in the session that was open', async () => {
    const first = render(App, { props: { backend: mockBackend() } })
    await screen.findByRole('heading', { name: 'Fix the flaky parser test' })
    await fireEvent.click(screen.getByText('Plan a weekend in Lisbon'))
    await screen.findByRole('heading', { name: 'Plan a weekend in Lisbon' })
    const box = screen.getByPlaceholderText(/Ask anything/) as HTMLTextAreaElement
    await fireEvent.input(box, { target: { value: 'And on Sunday morning, we' } })
    first.unmount()

    render(App, { props: { backend: mockBackend() } })
    await screen.findByRole('heading', { name: 'Plan a weekend in Lisbon' })
    await waitFor(() => expect(screen.getByPlaceholderText(/Ask anything/)).toHaveValue('And on Sunday morning, we'))
  })
})
