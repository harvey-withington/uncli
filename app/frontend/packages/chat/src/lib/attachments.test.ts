import { describe, expect, it } from 'vitest'
import { fitSize, toRefs } from './attachments'

describe('attachments', () => {
  it('sends each file the way it can travel', () => {
    expect(toRefs([
      { key: 'a', name: 'p.jpg', size: 1, kind: 'image', upload: 'u1' },
      { key: 'b', name: 'n.md', size: 1, kind: 'text', path: 'C:/n.md' },
      { key: 'c', name: 'Pasted image', size: 1, kind: 'image', mediaType: 'image/png', data: 'AAAA' },
    ])).toEqual([{ upload: 'u1' }, { path: 'C:/n.md' }, { name: 'Pasted image', mediaType: 'image/png', data: 'AAAA' }])
  })

  it('fits a photo inside the longest side, never enlarging it', () => {
    expect(fitSize(4000, 3000)).toEqual({ w: 2048, h: 1536 })
    expect(fitSize(3000, 4000)).toEqual({ w: 1536, h: 2048 })
    expect(fitSize(800, 600)).toEqual({ w: 800, h: 600 })
  })
})
