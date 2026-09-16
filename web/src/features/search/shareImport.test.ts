import { describe, expect, it } from 'vitest'
import { canSubmitShareImport } from './shareImport'

describe('canSubmitShareImport', () => {
  it('accepts 115 share URLs', () => {
    expect(canSubmitShareImport('https://115.com/s/shareABC123?password=ab12')).toBe(true)
    expect(canSubmitShareImport('anxia.com/s/shareABC123', 'xy9z')).toBe(true)
  })

  it('rejects other clouds', () => {
    expect(canSubmitShareImport('https://pan.quark.cn/s/nope', 'abcd')).toBe(false)
  })
})
