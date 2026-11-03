import { describe, expect, it } from 'vitest'
import { outlineOf } from './outline'
import { toBlocks } from './render/markdown'
import { classifySection, KIND_ICONS, SECTION_KINDS } from './sections'

describe('classifySection', () => {
  it('reads the heading first', () => {
    expect(classifySection('Security warning', [])).toBe('warning')
    expect(classifySection('Example: a minimal server', [])).toBe('example')
    expect(classifySection('TL;DR', [])).toBe('conclusion')
    expect(classifySection('Why the test was flaky', [])).toBe('analysis')
    expect(classifySection('Getting started', [])).toBe('steps')
    expect(classifySection('Pro tips', [])).toBe('tip')
  })

  it('falls back to the content', () => {
    expect(classifySection('Saturday', toBlocks('```go\nx\n```'))).toBe('code')
    expect(classifySection('Sunday', toBlocks('| a | b |\n| --- | --- |\n| 1 | 2 |'))).toBe('data')
    expect(classifySection('Plan', toBlocks('1. one\n2. two'))).toBe('steps')
    expect(classifySection('Heads up', toBlocks('> **Warning:** this deletes files'))).toBe('warning')
    expect(classifySection('Thoughts', toBlocks('Just some prose.'))).toBe('commentary')
  })

  it('has an icon for every kind', () => {
    for (const k of SECTION_KINDS) expect(KIND_ICONS[k]).toBeTruthy()
  })

  it('classifies heading outlines by their sections', () => {
    const o = outlineOf(toBlocks('## Overview\n\nText.\n\n## Install\n\n1. a\n2. b\n\n## Code\n\n```ts\nx\n```\n\n## Gotchas\n\nCareful.'))
    expect(o.map(e => e.kind)).toEqual(['overview', 'steps', 'code', 'warning'])
  })
})
