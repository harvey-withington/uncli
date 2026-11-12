import { describe, expect, it } from 'vitest'
import { svgSize } from './mermaid'

describe('a diagram’s shape', () => {
  it('comes from its viewBox', () => {
    expect(svgSize('<svg id="x" width="100%" viewBox="-8 -8 640.5 212" xmlns="http://www.w3.org/2000/svg"></svg>')).toEqual({ width: 640.5, height: 212 })
    expect(svgSize("<svg viewBox='0,0,300,150'></svg>")).toEqual({ width: 300, height: 150 })
  })

  it('is unknown without a usable one', () => {
    expect(svgSize('<svg width="300" height="150"></svg>')).toBeNull()
    expect(svgSize('<svg viewBox="0 0 0 150"></svg>')).toBeNull()
  })
})
