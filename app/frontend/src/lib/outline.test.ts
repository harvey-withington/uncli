import { beforeEach, describe, expect, it } from 'vitest'
import { activeEntry, autoSummaryFor, clampWidth, loadLayout, outlineOf, saveLayout, summaryBlocks, summaryEntries, wordCount } from './outline'
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
    expect(clampWidth(20)).toBe(240)
    expect(clampWidth(9000)).toBe(960)
    expect(loadLayout()).toEqual({ open: true, width: 360 })
    saveLayout({ open: false, width: 333 })
    expect(loadLayout()).toEqual({ open: false, width: 333 })
  })

  it('ignores junk in storage', () => {
    localStorage.setItem('uncli-outline', '{not json')
    expect(loadLayout()).toEqual({ open: true, width: 360 })
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

  it('auto-summarise per the preference: off, long answers (headings or not), or always', () => {
    const words = (n: number) => Array.from({ length: n }, (_, i) => `word${i}`).join(' ')
    expect(wordCount('**Bold** | `code` | --- ')).toBe(2)
    expect(autoSummaryFor(words(400), 5, 'off')).toBe('off')
    expect(autoSummaryFor(words(150), 2, 'long')).toBe('summarise')
    expect(autoSummaryFor(`## Heading\n\n${words(149)}`, 2, 'long')).toBe('summarise')
    expect(autoSummaryFor(words(149), 4, 'long')).toBe('short')
    expect(autoSummaryFor(words(20), 2, 'always')).toBe('summarise')
    expect(autoSummaryFor(words(400), 1, 'always')).toBe('short') // one block: nothing to outline
  })
})

describe('paragraphs acting as headings', () => {
  // The shape of the answer in Harvey's screenshot.
  const ANSWER = [
    'A few things, mostly loose ends from this session:',
    "**1. Nothing I've told you has been measured.** I haven't run the tests, the demo or a profiler.",
    '**2. The two bugs might matter more than the speed work.**',
    '- **Sort and filter can read the wrong field.** ClientRowModel looks values up by key.',
    '**Bottom line:** benchmark first.',
    'Some **bold** words in the middle are not a heading.',
    '**This bold opener runs on** without a label, so it stays a paragraph.',
  ].join('\n\n')

  it('count numbered and labelled bold leads, and whole bold lines', () => {
    expect(outlineOf(toBlocks(ANSWER)).map(e => [e.level, e.text])).toEqual([
      [1, "1. Nothing I've told you has been measured"],
      [1, '2. The two bugs might matter more than the speed work'],
      [1, 'Bottom line'],
    ])
  })

  it('sit one level below the real heading before them', () => {
    const o = outlineOf(toBlocks(['## Findings', '**1. First.** Text.', '**2. Second.** Text.', '## Next', 'Prose.'].join('\n\n')))
    expect(o.map(e => [e.level, e.text])).toEqual([
      [1, 'Findings'],
      [2, '1. First'],
      [2, '2. Second'],
      [1, 'Next'],
    ])
  })

})
