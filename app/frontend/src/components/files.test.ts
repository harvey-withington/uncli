import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte'
import { describe, expect, it, vi } from 'vitest'
import App from '../App.svelte'
import { mockBackend } from '../lib/api/mock'
import type { Page } from '../lib/api'
import { APP_CONTEXT } from '../lib/context'
import { AppStore } from '../stores/app.svelte'
import PageView from './PageView.svelte'

const ctx = (s: AppStore) => new Map([[APP_CONTEXT, s]])

describe('Changed files', () => {
  it('lists each file relative to the session folder, opens it in the editor at its line and shows it in its folder', async () => {
    const backend = mockBackend()
    const open = vi.spyOn(backend, 'openFile')
    const reveal = vi.spyOn(backend, 'revealFile')
    const s = new AppStore(backend)
    await s.init()
    render(PageView, { props: { page: s.currentPage as Page }, context: ctx(s) })
    const list = screen.getByRole('button', { name: /4 files changed/ })
    expect(list).toHaveAttribute('aria-expanded', 'true')
    const row = screen.getByText('parser.go').closest('li') as HTMLElement
    expect(within(row).getByText('internal/adapter/claude/')).toBeInTheDocument()
    expect(within(row).getByText(':148')).toBeInTheDocument()
    expect(within(row).getByTitle('12 lines added, 3 removed')).toHaveTextContent('+12−3')
    expect(within(row).getByText('Edited, 12 lines added, 3 removed')).toBeInTheDocument()
    await fireEvent.click(within(row).getByRole('button', { name: 'Open in VS Code: parser.go' }))
    expect(open).toHaveBeenCalledWith('s-code', 'C:\\Users\\you\\code\\internal\\adapter\\claude\\parser.go', 148)
    // The name itself opens it too.
    open.mockClear()
    await fireEvent.click(within(row).getByTitle(/^Open in VS Code: C:/))
    expect(open).toHaveBeenCalledWith('s-code', 'C:\\Users\\you\\code\\internal\\adapter\\claude\\parser.go', 148)
    await fireEvent.click(within(row).getByRole('button', { name: 'Show in folder: parser.go' }))
    expect(reveal).toHaveBeenCalledWith('C:\\Users\\you\\code\\internal\\adapter\\claude\\parser.go')
    // A deleted file has nothing to open.
    const gone = screen.getByText('old.jsonl').closest('li') as HTMLElement
    expect(within(gone).getByText('Deleted by a command')).toBeInTheDocument()
    expect(within(gone).queryByRole('button')).not.toBeInTheDocument()
    await fireEvent.click(list)
    expect(list).toHaveAttribute('aria-expanded', 'false')
  })

  it('Settings picks the editor, or a command of your own', async () => {
    const backend = mockBackend()
    const save = vi.spyOn(backend, 'setPreferences')
    render(App, { props: { backend } })
    await fireEvent.click(await screen.findByRole('button', { name: /Claude Code 2./ }))
    const dialog = await screen.findByRole('dialog', { name: 'Settings' })
    await fireEvent.click(within(dialog).getByRole('tab', { name: 'General' }))
    const select = within(dialog).getByRole('combobox', { name: 'Editor' })
    expect(within(select).getAllByRole('option').map(o => o.textContent)).toEqual([
      'Automatic: VS Code', 'VS Code', 'Cursor (not installed)', 'Antigravity (not installed)', 'My own command…',
    ])
    await fireEvent.change(select, { target: { value: 'custom' } })
    expect(save).not.toHaveBeenCalled() // needs a command first
    const input = within(dialog).getByRole('textbox', { name: 'Editor command' })
    await fireEvent.input(input, { target: { value: 'subl {file}:{line}' } })
    await fireEvent.click(within(dialog).getByRole('button', { name: 'Save' }))
    await waitFor(() => expect(save).toHaveBeenLastCalledWith(expect.objectContaining({ editor: 'custom', editorCommand: 'subl {file}:{line}' })))
  })
})
