import { describe, expect, it } from 'vitest'
import { adultLibraryId, childLibraries, libraryRootId, rootLibraries } from './libraryGroups'

describe('libraryGroups', () => {
  const libraries = [
    { id: 'movies', name: '电影', collectionType: 'movies' },
    { id: adultLibraryId, name: '成人影视', collectionType: 'movies' },
    { id: 'group-jp', name: '日本', collectionType: 'movies', parentId: adultLibraryId },
    { id: 'group-eu', name: '欧美', collectionType: 'movies', parentId: adultLibraryId },
  ]

  it('keeps adult groups off the root tab row', () => {
    expect(rootLibraries(libraries).map((library) => library.id)).toEqual(['movies', adultLibraryId])
    expect(childLibraries(libraries, adultLibraryId).map((library) => library.id)).toEqual(['group-jp', 'group-eu'])
  })

  it('keeps the adult root tab selected while browsing a group', () => {
    expect(libraryRootId(libraries, 'group-jp')).toBe(adultLibraryId)
    expect(libraryRootId(libraries, adultLibraryId)).toBe(adultLibraryId)
    expect(libraryRootId(libraries, 'movies')).toBe('movies')
  })
})
