import { fireEvent, render, screen, waitFor, within } from '@testing-library/svelte'
import { describe, expect, it, vi } from 'vitest'
import { mockBackend } from '../lib/api/mock'
import type { MCPServer } from '../lib/api'
import { commands } from '../lib/commands'
import { APP_CONTEXT } from '../lib/context'
import { connectedNow, needOf, statusKey } from '../lib/mcp'
import { toasts } from '../lib/toasts.svelte'
import { AppStore } from '../stores/app.svelte'
import SessionPane from './SessionPane.svelte'

async function code(opts: Parameters<typeof mockBackend>[0] = {}) {
  localStorage.clear()
  const backend = mockBackend(opts)
  const s = new AppStore(backend)
  await s.init()
  await s.select('s-code')
  return s
}

const ctx = (s: AppStore) => new Map([[APP_CONTEXT, s]])
const sv = (x: Partial<MCPServer>): MCPServer => ({ name: 'x', status: 'connected', tools: 0, ...x })

describe('MCP logic', () => {
  it('says what each server needs', () => {
    expect(needOf(sv({ status: 'connected' }))).toBeNull()
    expect(needOf(sv({ status: 'pending' }))).toBeNull()
    expect(needOf(sv({ status: 'needs-auth' }))).toBe('sign-in')
    expect(needOf(sv({ status: 'needs-auth', signing: true }))).toBe('signing')
    expect(needOf(sv({ status: 'needs-auth', site: true }))).toBe('connector')
    expect(needOf(sv({ status: 'failed' }))).toBe('failed')
    expect(statusKey(sv({ status: 'needs-auth', signing: true }))).toBe('mcp.status.signing')
  })

  it('toasts only servers the user acted on, as they connect', () => {
    const acted = new Set(['s1:a'])
    const before = [sv({ name: 'a', status: 'needs-auth' }), sv({ name: 'b', status: 'pending' })]
    const after = [sv({ name: 'a', status: 'connected', tools: 3 }), sv({ name: 'b', status: 'connected' })]
    expect(connectedNow('s1', before, after, acted).map(x => x.name)).toEqual(['a'])
    expect(connectedNow('s1', after, after, acted)).toEqual([])
  })
})

// The header's popover of MCP servers, opened, and a server's row in it.
async function popover() {
  await fireEvent.click(screen.getByRole('button', { name: /^MCP servers/ }))
  const pop = screen.getByRole('dialog', { name: 'MCP servers' })
  return { pop, row: (name: string) => within(pop).getByText(name).closest('li') as HTMLElement }
}

describe('MCP sign-in', () => {
  it('signs a server in from the popover through the browser, and toasts when it connects', async () => {
    const s = await code()
    const opened = vi.spyOn(s.backend, 'openURL')
    const { rerender } = render(SessionPane, { props: { session: s.current! }, context: ctx(s) })
    // No lines under the header: the servers button carries the attention dot.
    expect(screen.queryByText(/needs signing in/)).not.toBeInTheDocument()
    const { row } = await popover()
    expect(within(row('BRUV')).getByText('needs signing in')).toBeInTheDocument()

    await fireEvent.click(within(row('BRUV')).getByRole('button', { name: 'Sign in' }))
    await waitFor(() => expect(opened).toHaveBeenCalledWith('https://auth.example.com/authorize?client_id=uncli'))
    await rerender({ session: s.current! })
    await waitFor(() => expect(within(row('BRUV')).getByText('waiting for you to sign in')).toBeInTheDocument())
    expect(within(row('BRUV')).getByRole('button', { name: 'Open the page again' })).toBeInTheDocument()

    // The mock connects it a moment later, and a toast says so.
    await waitFor(() => expect(s.current?.mcp?.find(x => x.name === 'BRUV')?.status).toBe('connected'), { timeout: 3000 })
    await rerender({ session: s.current! })
    expect(within(row('BRUV')).getByText('connected, 12 tools')).toBeInTheDocument()
    expect(toasts.list.map(t => t.message)).toContain('BRUV connected: 12 tools')
  })

  it('takes the address pasted from the browser', async () => {
    const s = await code({ mcpSlow: true })
    const pasted = vi.spyOn(s.backend, 'mcpCallback')
    const { rerender } = render(SessionPane, { props: { session: s.current! }, context: ctx(s) })
    await s.mcpSignIn('s-code', 'BRUV')
    await rerender({ session: s.current! })
    const { row } = await popover()
    await fireEvent.click(within(row('BRUV')).getByRole('button', { name: 'Paste the address' }))
    const box = within(row('BRUV')).getByRole('textbox', { name: /after signing in to BRUV/ })
    await fireEvent.input(box, { target: { value: ' http://localhost:4567/callback?code=c&state=s ' } })
    await fireEvent.click(within(row('BRUV')).getByRole('button', { name: 'Use it' }))
    expect(pasted).toHaveBeenCalledWith('s-code', 'BRUV', 'http://localhost:4567/callback?code=c&state=s')
  })

  it('signs in to a connector on the provider’s site, and it connects by itself', async () => {
    const s = await code()
    const opened = vi.spyOn(s.backend, 'openURL')
    const { rerender } = render(SessionPane, { props: { session: s.current! }, context: ctx(s) })
    await s.mcpSignIn('s-code', 'Google Drive')
    expect(opened).toHaveBeenCalledWith('https://example.com/settings/connectors')
    await rerender({ session: s.current! })
    const { row } = await popover()
    const line = row('Google Drive')
    expect(within(line).getByText('waiting for you to sign in on the page that opened')).toBeInTheDocument()
    // Reconnect (now) and Cancel are there; no Sign in, no paste box.
    expect(within(line).getByRole('button', { name: 'Reconnect' })).toBeInTheDocument()
    expect(within(line).getByRole('button', { name: 'Cancel' })).toBeInTheDocument()
    expect(within(line).queryByRole('button', { name: 'Paste the address' })).toBeNull()
    await waitFor(() => expect(s.current?.mcp?.find(x => x.name === 'Google Drive')?.status).toBe('connected'), { timeout: 3000 })
    expect(toasts.list.map(t => t.message)).toContain('Google Drive connected: 5 tools')
  })

  it('lists every server in the popover and the palette, with what can be done', async () => {
    const s = await code()
    render(SessionPane, { props: { session: s.current! }, context: ctx(s) })
    expect(screen.getByRole('button', { name: 'MCP servers: one needs you' })).toBeInTheDocument()
    const { row } = await popover()
    expect(within(row('notes')).getByText('connected, 3 tools')).toBeInTheDocument()
    expect(within(row('notes')).getByRole('button', { name: 'Sign out' })).toBeInTheDocument()
    expect(within(row('BRUV')).getByRole('button', { name: 'Sign in' })).toBeInTheDocument()
    const ids = commands(s).map(c => c.id)
    expect(ids).toEqual(expect.arrayContaining(['mcp.signIn.BRUV', 'mcp.reconnect.notes', 'mcp.signOut.notes', 'mcp.signIn.Google Drive']))
  })
})
