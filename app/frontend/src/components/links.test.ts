import { fireEvent, render, screen, waitFor } from '@testing-library/svelte'
import { describe, expect, it, vi } from 'vitest'
import App from '../App.svelte'
import { mockBackend } from '../lib/api/mock'

// Links in an answer, as the Antigravity CLI writes them (file URLs with
// code as their text) and as paths in the session's folder.
const ANSWER = [
  '* [`README.md`](file:///C:/Users/you/code/README.md#L3) — the readme.',
  '* [the guide](docs/guide.md:12) and [`app/`](file:///C:/Users/you/code/app).',
  '* [the site](https://example.com).',
].join('\n')

describe('Links in answers', () => {
  it('open files in the editor, folders in place, and the web in the browser', async () => {
    const backend = mockBackend()
    const pages = backend.pages.bind(backend)
    vi.spyOn(backend, 'pages').mockImplementation(async id => (await pages(id)).map(p => ({ ...p, answerMd: ANSWER })))
    const openPath = vi.spyOn(backend, 'openPath')
    const openURL = vi.spyOn(backend, 'openURL').mockResolvedValue()
    render(App, { props: { backend } })

    const readme = await screen.findByRole('link', { name: 'README.md' })
    expect(screen.queryByText(/\]\(file:/)).not.toBeInTheDocument() // no markdown left showing
    await fireEvent.click(readme)
    await waitFor(() => expect(openPath).toHaveBeenCalledWith(expect.any(String), 'C:/Users/you/code/README.md', 3))

    await fireEvent.click(screen.getByRole('link', { name: 'the guide' }))
    await waitFor(() => expect(openPath).toHaveBeenLastCalledWith(expect.any(String), 'C:\\Users\\you\\code\\docs\\guide.md', 12))
    await fireEvent.click(screen.getByRole('link', { name: 'app/' }))
    await waitFor(() => expect(openPath).toHaveBeenLastCalledWith(expect.any(String), 'C:/Users/you/code/app', 0))

    await fireEvent.click(screen.getByRole('link', { name: 'the site' }))
    expect(openURL).toHaveBeenCalledWith('https://example.com')
    expect(openPath).toHaveBeenCalledTimes(3)
  })
})
