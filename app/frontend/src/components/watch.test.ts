import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte'
import { describe, expect, it } from 'vitest'
import { mockBackend } from '../lib/api/mock'
import { commands } from '../lib/commands'
import { APP_CONTEXT } from '../lib/context'
import { loadWatched } from '../lib/watched'
import { AppStore } from '../stores/app.svelte'
import SessionPane from './SessionPane.svelte'

type Writer = { writeWatched(sid: string, path: string, text: string | null): void }

async function code() {
  localStorage.clear()
  const backend = mockBackend()
  const s = new AppStore(backend)
  await s.init()
  await s.select('s-code')
  return { s, files: backend as unknown as Writer }
}

const ctx = (s: AppStore) => new Map([[APP_CONTEXT, s]])
const options = () => within(screen.getByRole('listbox', { name: 'File name or path' })).getAllByRole('option').map(o => o.querySelector('.name')?.textContent)

describe('File tab', () => {
  it('finds a file by name, newest first, and shows it live', async () => {
    const { s, files } = await code()
    render(SessionPane, { props: { session: s.current! }, context: ctx(s) })
    await fireEvent.click(screen.getByRole('tab', { name: /File/ }))
    // No file yet: the switcher is the tab, the folder newest first.
    await waitFor(() => expect(options()).toEqual(['TODO.md', 'parser.ts', 'notes.md', 'build.log', 'TODO-later.md']))

    // Typing TODO puts the latest TODO file at the top.
    await fireEvent.input(screen.getByRole('combobox', { name: 'File name or path' }), { target: { value: 'todo' } })
    await waitFor(() => expect(options()).toEqual(['TODO.md', 'TODO-later.md']))
    await fireEvent.keyDown(screen.getByRole('combobox', { name: 'File name or path' }), { key: 'Enter' })

    // Markdown is rendered, and the tab is named for the file.
    const side = screen.getByRole('complementary')
    await waitFor(() => expect(within(side).getByRole('heading', { name: 'TODO' })).toBeInTheDocument())
    expect(screen.getByRole('tab', { name: /TODO\.md/ })).toHaveAttribute('aria-selected', 'true')
    expect(loadWatched('s-code')).toMatchObject({ current: 'TODO.md', recent: ['TODO.md'] })

    // It follows the file as it changes, and says when it goes.
    files.writeWatched('s-code', 'TODO.md', '# Done\n\nAll of it.\n')
    await waitFor(() => expect(within(side).getByRole('heading', { name: 'Done' })).toBeInTheDocument(), { timeout: 3000 })
    files.writeWatched('s-code', 'TODO.md', null)
    await waitFor(() => expect(within(side).getByText(/isn’t in the session folder/)).toBeInTheDocument(), { timeout: 3000 })
    files.writeWatched('s-code', 'TODO.md', '# Back\n')
    await waitFor(() => expect(within(side).getByRole('heading', { name: 'Back' })).toBeInTheDocument(), { timeout: 3000 })
  })

  it('stars a file, lists it first, and shows code as code', async () => {
    const { s } = await code()
    s.watchFile('s-code', 'src/parser.ts')
    render(SessionPane, { props: { session: s.current! }, context: ctx(s) })
    const side = screen.getByRole('complementary')
    await waitFor(() => expect(side.querySelector('.code')).toHaveTextContent('export function scan'))
    await fireEvent.click(within(side).getByRole('button', { name: 'Star this file' }))
    expect(loadWatched('s-code').starred).toEqual(['src/parser.ts'])
    expect(within(side).getByRole('button', { name: 'Unstar this file' })).toHaveAttribute('aria-pressed', 'true')

    // The switcher, from the file's name: the starred one first.
    await fireEvent.click(within(side).getByRole('button', { name: /^Switch file/ }))
    await waitFor(() => expect(options()[0]).toBe('parser.ts'))
    expect(within(screen.getByRole('listbox', { name: 'File name or path' })).getAllByRole('option')[0]).toContainElement(within(side).getByLabelText('Starred'))
    await fireEvent.keyDown(screen.getByRole('combobox', { name: 'File name or path' }), { key: 'Escape' })
    expect(screen.queryByRole('combobox', { name: 'File name or path' })).not.toBeInTheDocument()
  })

  it('opens from the palette on its switcher, and from a page’s changed files', async () => {
    const { s } = await code()
    render(SessionPane, { props: { session: s.current! }, context: ctx(s) })
    commands(s).find(c => c.id === 'watch')!.run()
    await waitFor(() => expect(screen.getByRole('combobox', { name: 'File name or path' })).toHaveFocus())
    expect(commands(s).find(c => c.id === 'file')?.keys?.[0]?.key).toBe('f')

    // A changed file in the session folder has a Watch button.
    s.goTo(s.currentPages.findIndex(p => p.touchedFiles?.length))
    await waitFor(async () => {
      await fireEvent.click(screen.getByRole('button', { name: 'Watch it on the File tab: parser.go' }))
      expect(loadWatched('s-code').current).toBe('internal/adapter/claude/parser.go')
    })
  })
})
