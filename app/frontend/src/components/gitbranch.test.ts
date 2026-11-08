import { fireEvent, render, screen } from '@testing-library/svelte'
import { describe, expect, it } from 'vitest'
import App from '../App.svelte'
import { mockBackend } from '@uncli/client/mock'

describe('The session header', () => {
  it('shows the branch the session’s folder is on, and nothing outside a repository', async () => {
    render(App, { props: { backend: mockBackend() } })
    const branch = await screen.findByTitle('Git branch: fix/flaky-parser')
    expect(branch).toHaveTextContent('fix/flaky-parser')
    await fireEvent.click(await screen.findByText('Plan a weekend in Lisbon'))
    await screen.findByRole('heading', { name: /Plan a weekend in Lisbon/ })
    expect(screen.queryByTitle(/^Git branch:/)).toBeNull()
  })
})
