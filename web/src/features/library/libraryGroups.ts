import type { EmbyLibrary } from '../../shared/api/mediaHub'

export const adultLibraryId = 'adult'

export function isSharedEmbyId(id: string | null | undefined) {
  return Boolean(id?.startsWith('r_'))
}

export function libraryDisplayName(name: string) {
  return name.replace(/^共享\//, '')
}

export function mineLibraries(libraries: EmbyLibrary[]) {
  return libraries.filter((library) => !isSharedEmbyId(library.id))
}

export function sharedLibraries(libraries: EmbyLibrary[]) {
  return libraries.filter((library) => isSharedEmbyId(library.id))
}

export function rootLibraries(libraries: EmbyLibrary[]) {
  return libraries.filter((library) => !library.parentId)
}

export function childLibraries(libraries: EmbyLibrary[], parentId: string) {
  return libraries.filter((library) => library.parentId === parentId)
}

export function libraryRootId(libraries: EmbyLibrary[], libraryId: string | null) {
  const selected = libraries.find((library) => library.id === libraryId)
  if (libraryId === adultLibraryId || selected?.parentId === adultLibraryId) {
    return adultLibraryId
  }
  return libraryId
}
