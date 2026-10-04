import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte'
import { describe, expect, it, vi } from 'vitest'
import App from '../App.svelte'
import { mockBackend } from '../lib/api/mock'

// When to prompt and teaching the safe list, through the whole app on the
// mock backend.
async function ask(text: string) {
  const box = (await screen.findByRole('textbox', { name: 'Your message' })) as HTMLTextAreaElement
  await fireEvent.input(box, { target: { value: text } })
  await fireEvent.click(await screen.findByRole('button', { name: 'Send' }, { timeout: 4000 }))
}
const card = () => screen.queryByRole('alertdialog')
const modes = () => screen.getByRole('radiogroup', { name: 'Prompt me:' })

describe('Prompt me', () => {
  it('Never lets a waiting push run, and the next one runs without a card', async () => {
    const backend = mockBackend()
    const setMode = vi.spyOn(backend, 'setMode')
    render(App, { props: { backend } })
    await ask('push')
    await screen.findByRole('alertdialog', { name: 'Run a command' }, { timeout: 2000 })
    expect(within(modes()).getByRole('radio', { name: 'When unsafe' })).toHaveAttribute('aria-checked', 'true')

    await fireEvent.click(within(modes()).getByRole('radio', { name: 'Never' }))
    expect(setMode).toHaveBeenCalledWith('s-code', 'never')
    await waitFor(() => expect(card()).toBeNull())
    expect(screen.getByText(/Claude runs everything without prompting you/)).toBeInTheDocument()
    expect(await screen.findByText(/Pushed/, {}, { timeout: 3000 })).toBeInTheDocument()
    await ask('push again')
    expect(await screen.findByText('Page 3', { selector: '.seq' }, { timeout: 3000 })).toBeInTheDocument()
    expect(card()).toBeNull()
  })

  it('moves between levels with the arrow keys', async () => {
    const backend = mockBackend()
    const setMode = vi.spyOn(backend, 'setMode')
    render(App, { props: { backend } })
    const unsafe = await screen.findByRole('radio', { name: 'When unsafe' })
    expect(unsafe).toHaveAttribute('tabindex', '0')
    await fireEvent.keyDown(unsafe, { key: 'ArrowRight' })
    await waitFor(() => expect(setMode).toHaveBeenLastCalledWith('s-code', 'never'))
    await fireEvent.keyDown(modes(), { key: 'ArrowRight' }) // wraps round
    await waitFor(() => expect(setMode).toHaveBeenLastCalledWith('s-code', 'always'))
  })

  it('under Always a card says why, and offers no "This is safe"', async () => {
    render(App, { props: { backend: mockBackend({ mode: 'always' }) } })
    expect(await screen.findByText('Claude prompts you before anything except reading.')).toBeInTheDocument()
    await ask('push')
    const c = await screen.findByRole('alertdialog', {}, { timeout: 2000 })
    expect(within(c).getByLabelText('Why Claude is asking')).toHaveTextContent('You asked to be prompted before anything except reading.')
    expect(within(c).queryByRole('button', { name: 'This is safe' })).toBeNull()
    expect(within(c).getByRole('button', { name: 'Allow once' })).toBeInTheDocument()
  })

  it('the shield opens Settings at Safe and unsafe', async () => {
    render(App, { props: { backend: mockBackend() } })
    await fireEvent.click(await screen.findByRole('button', { name: 'Safe and unsafe' }))
    const dlg = await screen.findByRole('dialog', { name: 'Settings' })
    expect(within(dlg).getByRole('heading', { name: 'Safe and unsafe' })).toBeInTheDocument()
  })
})

describe('What ran, and why', () => {
  it('a trace row opens the reason for each part, and "This should prompt" marks it unsafe', async () => {
    const backend = mockBackend()
    const teach = vi.spyOn(backend, 'teach')
    render(App, { props: { backend } })
    await fireEvent.click(await screen.findByRole('button', { name: /5 tool calls/ }))
    const label = screen.getByRole('button', { name: 'ran: safe' })
    expect(label).toHaveAttribute('aria-expanded', 'false')
    await fireEvent.click(label)
    const parts = screen.getByRole('list', { name: 'Why each part ran' })
    expect(within(parts).getByText('git status --short')).toBeInTheDocument()
    expect(within(parts).getByText('only reads, so it runs')).toBeInTheDocument()
    expect(within(parts).getByText('safe: routine work in this folder')).toBeInTheDocument()
    await fireEvent.click(within(parts).getByRole('button', { name: 'This should prompt' }))
    await waitFor(() => expect(teach).toHaveBeenCalledWith('s-code', [{ kind: 'command', words: 'go vet' }], 'unsafe', 'project'))
    expect(await within(parts).findByText('Claude will prompt before go vet from now on.')).toBeInTheDocument()
  })

  it('Settings shows the session type list and checks a command part by part', async () => {
    const backend = mockBackend()
    const explain = vi.spyOn(backend, 'explainCommand')
    render(App, { props: { backend } })
    await fireEvent.click(await screen.findByRole('button', { name: 'Safe and unsafe' }))
    const dlg = await screen.findByRole('dialog', { name: 'Settings' })
    const builtin = await within(dlg).findByRole('list', { name: 'Safe in Code sessions (built in)' })
    expect(within(builtin).getByText('npm test …')).toBeInTheDocument()

    await fireEvent.input(within(dlg).getByRole('textbox', { name: 'A command to check' }), { target: { value: 'git status --short; npm test; npm publish' } })
    await fireEvent.click(within(dlg).getByRole('button', { name: 'Check' }))
    expect(explain).toHaveBeenCalledWith('s-code', 'git status --short; npm test; npm publish')
    const result = await within(dlg).findByRole('status', { name: 'What would happen to git status --short; npm test; npm publish' })
    expect(within(result).getByText('Claude prompts you first')).toBeInTheDocument()
    expect(within(result).getByText('the session type allows npm test …')).toBeInTheDocument()
    expect(within(result).getByText('unsafe: it sends or publishes something beyond this computer')).toBeInTheDocument()
  })
})

describe('Prompting only when unsafe', () => {
  it('a card says in plain words why Claude is asking, before the command', async () => {
    render(App, { props: { backend: mockBackend() } })
    await ask('push')
    const c = await screen.findByRole('alertdialog', { name: 'Run a command' }, { timeout: 2000 })
    const why = within(c).getByLabelText('Why Claude is asking')
    expect(why).toHaveTextContent('Sends or publishes something beyond this computer.')
    // The reason comes before the command.
    expect(why.compareDocumentPosition(within(c).getByText('git push origin main')) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })

  it('a tool UNCLI can not place is checked by the quick-task model first, without a card', async () => {
    render(App, { props: { backend: mockBackend() } })
    await ask('save a note')
    expect(await screen.findByText('Checking whether a command is safe to run…', {}, { timeout: 2000 })).toBeInTheDocument()
    expect(card()).toBeNull()
    const c = await screen.findByRole('alertdialog', {}, { timeout: 3000 })
    expect(within(c).getByLabelText('Why Claude is asking')).toHaveTextContent('Saves a note to your notes service, outside this computer.')
    expect(screen.queryByText('Checking whether a command is safe to run…')).toBeNull()
  })

  it('the setting can treat what UNCLI can not place as safe inside the folder', async () => {
    render(App, { props: { backend: mockBackend({ prefs: { unknownCommands: 'inside' } }) } })
    await ask('save a note')
    expect(await screen.findByText(/Saved the note/, {}, { timeout: 3000 })).toBeInTheDocument()
    expect(card()).toBeNull()
  })

  it('Settings chooses what happens when UNCLI can not tell', async () => {
    const backend = mockBackend()
    const save = vi.spyOn(backend, 'setPreferences')
    render(App, { props: { backend } })
    await fireEvent.click(await screen.findByRole('button', { name: /Claude CLI/ }))
    const select = await screen.findByRole('combobox', { name: "When UNCLI can't tell" })
    expect(select).toHaveValue('model')
    await fireEvent.change(select, { target: { value: 'ask' } })
    await waitFor(() => expect(save).toHaveBeenLastCalledWith(expect.objectContaining({ unknownCommands: 'ask' })))
  })
})
