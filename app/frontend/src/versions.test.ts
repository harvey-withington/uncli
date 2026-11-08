import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

// UNCLI's packages move with UNCLI's own version (decision 0026): both carry
// the same one, @uncli/chat depends on exactly that @uncli/client, and it's
// a version CHANGELOG.md has (the latest release, between releases). Set it
// with npm run set-version.
const read = (p: string) => readFileSync(resolve(__dirname, p), 'utf8')
const client = JSON.parse(read('../packages/client/package.json')) as { version: string }
const chat = JSON.parse(read('../packages/chat/package.json')) as { version: string; dependencies: Record<string, string> }

describe("UNCLI's packages' versions", () => {
  it('match each other and a released version', () => {
    expect(chat.version).toBe(client.version)
    expect(chat.dependencies['@uncli/client']).toBe(client.version)
    expect(read('../../../CHANGELOG.md').split('\n')).toContain(`## ${client.version}`)
  })
})
