import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte'
import { describe, expect, it, vi } from 'vitest'
import { mockBackend } from '@uncli/client/mock'
import { APP_CONTEXT } from '../lib/context'
import { AppStore } from '../stores/app.svelte'
import SessionPane from '@uncli/chat/components/SessionPane.svelte'

// Mermaid needs a real layout engine, which jsdom hasn't: a stand-in draws
// a flowchart and refuses anything else, as Mermaid would.
vi.mock('@uncli/chat/lib/mermaid', async real => ({
  ...(await real<typeof import('@uncli/chat/lib/mermaid')>()),
  renderMermaid: async (src: string) => {
    if (!src.startsWith('flowchart')) throw new Error('Parse error on line 1:\nnot a diagram')
    return '<svg viewBox="0 0 400 100"><text>scan</text></svg>'
  },
}))

type Writer = { writeWatched(sid: string, path: string, text: string | null): void }

describe('Mermaid in the File tab', () => {
  it('draws a Markdown file’s Mermaid blocks, shows their code on request, and says when one can’t be drawn', async () => {
    localStorage.clear()
    const backend = mockBackend()
    const s = new AppStore(backend)
    await s.init()
    await s.select('s-code')
    s.watchFile('s-code', 'docs/notes.md')
    render(SessionPane, { props: { session: s.current! }, context: new Map([[APP_CONTEXT, s]]) })
    const side = within(screen.getByRole('complementary'))

    const frame = await side.findByTitle('Diagram')
    expect(frame.tagName).toBe('IFRAME')
    expect(frame).toHaveAttribute('sandbox', '')
    expect(frame.getAttribute('srcdoc')).toContain('<text>scan</text>')
    expect(frame.style.aspectRatio).toBe('400 / 100')
    expect(side.getByText('The parser reads one token ahead.')).toBeInTheDocument()

    // Its code, and back.
    await fireEvent.click(side.getByRole('button', { name: 'Code' }))
    await waitFor(() => expect(side.queryByTitle('Diagram')).not.toBeInTheDocument())
    expect(screen.getByRole('complementary').querySelector('.code')).toHaveTextContent('flowchart LR')
    await fireEvent.click(side.getByRole('button', { name: 'Diagram' }))
    expect(await side.findByTitle('Diagram')).toBeInTheDocument()

    // A block Mermaid can't read shows its code, with Mermaid's complaint.
    ;(backend as unknown as Writer).writeWatched('s-code', 'docs/notes.md', '# Notes\n\n```mermaid\nnonsense\n```\n')
    expect(await side.findByText("This diagram couldn't be drawn: Parse error on line 1:", {}, { timeout: 3000 })).toBeInTheDocument()
    expect(screen.getByRole('complementary').querySelector('.code')).toHaveTextContent('nonsense')
  }, 15000)
})
