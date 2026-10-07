import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte'
import { describe, expect, it, vi } from 'vitest'
import App from '../App.svelte'
import { mockBackend } from '../lib/api/mock'

async function openImport(backend = mockBackend()) {
  localStorage.clear()
  render(App, { props: { backend } })
  await fireEvent.click(await screen.findByRole('button', { name: /New session/ }))
  await fireEvent.click(await screen.findByRole('button', { name: /Continue a conversation from the terminal/ }))
  const dialog = await screen.findByRole('dialog', { name: 'Import a conversation' })
  await within(dialog).findByRole('list', { name: 'Saved conversations' })
  return { backend, dialog }
}

describe('Import', () => {
  it('lists saved conversations, hides those whose folder is gone, and marks what is already in UNCLI', async () => {
    const { dialog } = await openImport()
    const rows = () => within(dialog).getAllByRole('listitem')
    expect(rows()).toHaveLength(3)
    expect(within(dialog).queryByText('Try the stream-json mode with two turns.')).not.toBeInTheDocument()
    expect(within(rows()[1] as HTMLElement).getByText('deleted from UNCLI')).toBeInTheDocument()
    expect(within(rows()[2] as HTMLElement).getByRole('button', { name: /Open/ })).toBeInTheDocument()
    await fireEvent.click(within(dialog).getByRole('checkbox', { name: /Show 1 whose folder is gone/ }))
    expect(rows()).toHaveLength(4)
    expect(within(dialog).getByText('folder gone')).toBeInTheDocument()
    await fireEvent.input(within(dialog).getByRole('textbox', { name: 'Search by question or folder' }), { target: { value: 'billing' } })
    expect(rows()).toHaveLength(1)
  })

  it('imports one as the type chosen and opens it on its latest page', async () => {
    const { backend, dialog } = await openImport()
    const spy = vi.spyOn(backend, 'importTranscript')
    const row = within(dialog).getByText(/invoices round to the wrong cent/).closest('li') as HTMLElement
    const type = within(row).getByRole('combobox')
    expect(type).toHaveValue('code')
    await fireEvent.change(type, { target: { value: 'cowork' } })
    await fireEvent.click(within(row).getByRole('button', { name: /Import/ }))
    expect(spy).toHaveBeenCalledWith('tr-terminal', 'cowork')
    expect(await screen.findByText(/Imported 6 pages/)).toBeInTheDocument()
    await waitFor(() => expect(screen.getByRole('heading', { name: /invoices round to the wrong cent/ })).toBeInTheDocument())
    expect(screen.getByText('Page 6 of 6')).toBeInTheDocument()
  })
})
