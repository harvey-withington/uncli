import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte'
import { describe, expect, it, vi } from 'vitest'
import App from '../App.svelte'
import { mockBackend } from '@uncli/client/mock'

// The mock's repository at C:\Users\you\projects\demo suggests Team dev (your
// brain and secrets, and a curl | sh step) and Docs (stays inside).
describe('Suggested containers', () => {
  it('imports a project container through the editor, saying what reaches outside it', async () => {
    const backend = mockBackend({ lastNew: { profileId: 'code', folders: { code: 'C:\\Users\\you\\projects\\demo' } } })
    const imp = vi.spyOn(backend, 'importContainer')
    render(App, { props: { backend } })
    await fireEvent.click(await screen.findByRole('button', { name: /New session/ }))
    const dialog = within(await screen.findByRole('dialog', { name: 'New session' }))
    expect(await dialog.findByText('This project suggests a container: Team dev')).toBeInTheDocument()
    expect(dialog.getByText('This project suggests a container: Docs')).toBeInTheDocument()

    await fireEvent.click(dialog.getAllByRole('button', { name: 'Import…' })[0]!)
    const form = within(await screen.findByRole('dialog', { name: 'Import Team dev' }))
    expect(form.getByText('Suggested by demo. Check what it does before you save it.')).toBeInTheDocument()
    expect(form.getByText('It reaches outside its container:')).toBeInTheDocument()
    expect(form.getByText('Your secrets folder, read-only')).toBeInTheDocument()
    expect(form.getByText('A setup step worth a look (marked below)')).toBeInTheDocument()
    expect(form.getByText('It runs code it fetches from elsewhere.')).toBeInTheDocument()
    // Its own copy of node_modules comes with it, and doesn't reach outside.
    expect(form.getByRole('textbox', { name: 'Own copy 1: path in the project' })).toHaveValue('node_modules')

    // Turned down to what stays inside, the summary says so.
    await fireEvent.click(form.getByRole('checkbox', { name: /Your secrets folder/ }))
    await fireEvent.click(form.getByRole('checkbox', { name: /Your memories, skills/ }))
    await fireEvent.click(form.getByRole('button', { name: 'Remove step 2' }))
    expect(form.getByText(/It stays inside its container/)).toBeInTheDocument()
    expect(form.queryByText('It runs code it fetches from elsewhere.')).not.toBeInTheDocument()

    await fireEvent.click(form.getByRole('button', { name: 'Save' }))
    await waitFor(() => expect(imp).toHaveBeenCalledWith(expect.objectContaining({ id: 'team-dev', secrets: 'none', brain: 'sandboxed', setup: ['npm i -g pnpm'], local: ['node_modules'] }), 'C:\\Users\\you\\projects\\demo', 'team-dev'))
    // Changed on import, it's the user's change: Sync is offered quietly.
    expect(await dialog.findByText("You've changed your Team dev since importing it from this project.")).toBeInTheDocument()
    expect(dialog.getByRole('button', { name: 'Sync…' })).toBeInTheDocument()
  })

  it('lists a project’s containers on a paired UNCLI too, imported there', async () => {
    history.replaceState(null, '', '/?peers=on')
    try {
      const backend = mockBackend()
      const workshop = backend.peer('workshop.tail2ebd58.ts.net:9871')
      const there = vi.spyOn(workshop, 'importContainer')
      render(App, { props: { backend } })
      await fireEvent.click(await screen.findByRole('button', { name: /New session/ }))
      const dialog = within(await screen.findByRole('dialog', { name: 'New session' }))
      await fireEvent.change(dialog.getByRole('combobox', { name: 'Where' }), { target: { value: 'workshop.tail2ebd58.ts.net:9871' } })
      await fireEvent.click(await dialog.findByRole('radio', { name: /^Code/ }))
      await fireEvent.click(dialog.getByRole('button', { name: 'Choose…' }))
      const browser = within(await screen.findByRole('dialog', { name: /Choose/ }))
      for (const name of ['C:\\Users\\you', 'projects', 'demo']) await fireEvent.click(await browser.findByRole('button', { name }))
      await waitFor(() => expect(browser.getByText('C:\\Users\\you\\projects\\demo')).toBeInTheDocument())
      await fireEvent.click(browser.getByRole('button', { name: 'Use this folder' }))

      expect(await dialog.findByText('This project suggests a container: Docs')).toBeInTheDocument()
      await fireEvent.click(dialog.getAllByRole('button', { name: 'Import…' })[1]!)
      const form = within(await screen.findByRole('dialog', { name: 'Import Docs' }))
      await fireEvent.click(form.getByRole('button', { name: 'Save' }))
      await waitFor(() => expect(there).toHaveBeenCalledWith(expect.objectContaining({ id: 'docs' }), 'C:\\Users\\you\\projects\\demo', 'docs'))
    } finally {
      history.replaceState(null, '', '/')
    }
  })
})
