import { beforeEach, describe, expect, it } from 'vitest'
import { EXPLAIN_DEFAULTS, loadExplainOptions, saveExplainOptions, tokensLabel } from './explainoptions'

describe('Explain options', () => {
  beforeEach(() => localStorage.removeItem('uncli-explain'))

  it('starts from the defaults and remembers what was used', () => {
    expect(loadExplainOptions()).toEqual(EXPLAIN_DEFAULTS)
    saveExplainOptions({ model: 'opus', context: 'pages', pages: 5, detail: 'thorough' })
    expect(loadExplainOptions()).toEqual({ model: 'opus', context: 'pages', pages: 5, detail: 'thorough' })
  })

  it('mends what it can’t use', () => {
    localStorage.setItem('uncli-explain', JSON.stringify({ model: 3, context: 'everything', pages: 900, detail: 'loud' }))
    expect(loadExplainOptions()).toEqual(EXPLAIN_DEFAULTS)
    localStorage.setItem('uncli-explain', 'not json')
    expect(loadExplainOptions()).toEqual(EXPLAIN_DEFAULTS)
  })

  it('rounds the size', () => {
    expect([tokensLabel(40), tokensLabel(640), tokensLabel(2600), tokensLabel(23000)]).toEqual(['100', '600', '3k', '23k'])
  })
})
