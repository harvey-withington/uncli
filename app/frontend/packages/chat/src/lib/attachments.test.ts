import { describe, expect, it } from 'vitest'
import { fitSize, fromRefs, toRefs } from './attachments'

describe('attachments', () => {
  it('sends each file the way it can travel', () => {
    expect(toRefs([
      { key: 'a', name: 'p.jpg', size: 1, kind: 'image', upload: 'u1' },
      { key: 'b', name: 'n.md', size: 1, kind: 'text', path: 'C:/n.md' },
      { key: 'c', name: 'Pasted image', size: 1, kind: 'image', mediaType: 'image/png', data: 'AAAA' },
    ])).toEqual([{ upload: 'u1', name: 'p.jpg' }, { path: 'C:/n.md', name: 'n.md' }, { name: 'Pasted image', mediaType: 'image/png', data: 'AAAA' }])
  })

  it('shows the files of a queued message again from how they travelled', () => {
    const back = fromRefs([
      { upload: 'u1', name: 'p.jpg', mediaType: 'image/jpeg' },
      { path: 'C:\\docs\\brief.pdf' },
      { name: 'Pasted image', mediaType: 'image/png', data: 'AAAA' },
    ])
    expect(back.map(a => [a.name, a.kind, a.size])).toEqual([['p.jpg', 'image', 0], ['brief.pdf', 'pdf', 0], ['Pasted image', 'image', 0]])
    expect(back[2]?.preview).toBe('data:image/png;base64,AAAA')
    // As they go again: the same refs.
    expect(toRefs(back)[0]).toEqual({ upload: 'u1', name: 'p.jpg', mediaType: 'image/jpeg' })
  })

  it('fits a photo inside the longest side, never enlarging it', () => {
    expect(fitSize(4000, 3000)).toEqual({ w: 2048, h: 1536 })
    expect(fitSize(3000, 4000)).toEqual({ w: 1536, h: 2048 })
    expect(fitSize(800, 600)).toEqual({ w: 800, h: 600 })
  })
})
