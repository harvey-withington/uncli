import { beforeEach, describe, expect, it } from 'vitest'
import { activeEntry, clampWidth, loadLayout, outlineOf, saveLayout, summaryBlocks, summaryEntries, wantsAutoSummary } from './outline'
import { toBlocks } from './render/markdown'

const SRC = `Intro paragraph.

## Saturday

Morning plan.

### Lunch with \`code\` and **bold**

\`\`\`text
# not a heading
\`\`\`

## Sunday

Done.`

describe('outlineOf', () => {
  it('lists headings with their block and normalised level', () => {
    const blocks = toBlocks(SRC)
    const o = outlineOf(blocks)
    expect(o.map(e => [e.level, e.text])).toEqual([
      [1, 'Saturday'],
      [2, 'Lunch with code and bold'],
      [1, 'Sunday'],
    ])
    expect(blocks[o[0]?.block ?? -1]?.source).toBe('## Saturday')
  })

  it('is empty without headings', () => {
    expect(outlineOf(toBlocks('Just text.\n\n- a list'))).toEqual([])
  })
})

describe('activeEntry', () => {
  it('picks the last heading above the reading line', () => {
    expect(activeEntry([300, 800, 1400], 100)).toBe(-1)
    expect(activeEntry([90, 800, 1400], 100)).toBe(0)
    expect(activeEntry([-500, 60, 1400], 100)).toBe(1)
  })
})

describe('layout', () => {
  beforeEach(() => localStorage.clear())

  it('clamps width and survives a reload', () => {
    expect(clampWidth(20)).toBe(180)
    expect(clampWidth(9000)).toBe(480)
    expect(loadLayout()).toEqual({ open: true, width: 240 })
    saveLayout({ open: false, width: 333 })
    expect(loadLayout()).toEqual({ open: false, width: 333 })
  })

  it('ignores junk in storage', () => {
    localStorage.setItem('uncli-outline', '{not json')
    expect(loadLayout()).toEqual({ open: true, width: 240 })
  })
})

describe('summaries', () => {
  it('describes code blocks instead of sending them', () => {
    const blocks = toBlocks('Intro\n\n```go\na\nb\n```\n\n## Next')
    expect(summaryBlocks(blocks)).toEqual(['Intro', '(code block: go, 2 lines)', '## Next'])
  })

  it('keeps only sections that point at a block', () => {
    const blocks = toBlocks(['One', 'Two', 'Three'].join(String.fromCharCode(10, 10)))
    expect(summaryEntries([{ block: 0, title: 'A', kind: 'analysis' }, { block: 7, title: 'Gone' }], blocks)).toEqual([{ block: 0, level: 1, text: 'A', kind: 'analysis' }])
    expect(summaryEntries([{ block: 1, title: 'Earlier section', kind: 'nonsense' }], blocks)[0]?.kind).toBe('commentary')
  })

  it('auto-summarises only long answers without headings', () => {
    const long = toBlocks(Array.from({ length: 12 }, (_, i) => `Paragraph ${i}.`).join('\n\n'))
    expect(wantsAutoSummary(long)).toBe(true)
    expect(wantsAutoSummary(toBlocks('## H\n\n' + Array.from({ length: 12 }, (_, i) => `P ${i}.`).join('\n\n')))).toBe(false)
    expect(wantsAutoSummary(toBlocks('Short.'))).toBe(false)
  })
})
