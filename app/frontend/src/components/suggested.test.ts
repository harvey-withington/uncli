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

    // Turned down to what stays inside, the summary says so.
    await fireEvent.click(form.getByRole('checkbox', { name: /Your secrets folder/ }))
    await fireEvent.click(form.getByRole('checkbox', { name: /Your memories, skills/ }))
    await fireEvent.click(form.getByRole('button', { name: 'Remove step 2' }))
    expect(form.getByText(/It stays inside its container/)).toBeInTheDocument()
    expect(form.queryByText('It runs code it fetches from elsewhere.')).not.toBeInTheDocument()

    await fireEvent.click(form.getByRole('button', { name: 'Save' }))
    await waitFor(() => expect(imp).toHaveBeenCalledWith(expect.objectContaining({ id: 'team-dev', secrets: 'none', brain: 'sandboxed', setup: ['npm i -g pnpm'] }), 'C:\\Users\\you\\projects\\demo', 'team-dev'))
    // Changed on import, it's the user's change: Sync is offered quietly.
    expect(await dialog.findByText("You've changed your Team dev since importing it from this project.")).toBeInTheDocument()
    expect(dialog.getByRole('button', { name: 'Sync…' })).toBeInTheDocument()
  })
})
