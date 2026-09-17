import { describe, expect, it } from 'vitest'
import { libraryBrowseSort, librarySortLabel, nextLibrarySort } from './librarySort'

describe('librarySort', () => {
  it('sends newest-first for adult default so Hub imports stay on page one', () => {
    expect(libraryBrowseSort('default', true)).toBe('added-desc')
    expect(libraryBrowseSort('default', false)).toBe('')
    expect(librarySortLabel('default', true)).toBe('排序：最新加入')
  })

  it('cycles adult sorts through added, name, and year', () => {
    expect(nextLibrarySort('default', true)).toBe('name-asc')
    expect(nextLibrarySort('name-asc', true)).toBe('year-desc')
    expect(nextLibrarySort('year-desc', true)).toBe('default')
  })
})
