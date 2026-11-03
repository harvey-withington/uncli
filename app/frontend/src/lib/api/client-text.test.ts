import { describe, expect, it } from 'vitest'
import { CLIENT_TEXT } from '@uncli/client'
import en from '../../locales/en.json'

describe('the client’s own messages', () => {
  it('match the app’s strings, key for key', () => {
    const app = en as Record<string, string>
    for (const [key, value] of Object.entries(CLIENT_TEXT)) expect({ key, value: app[key] }).toEqual({ key, value })
  })
})
