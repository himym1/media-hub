export type LibrarySort = 'default' | 'year-desc' | 'year-asc' | 'name-asc' | 'added-desc'

export function libraryBrowseSort(sort: LibrarySort, adult: boolean) {
  if (sort === 'default') return adult ? 'added-desc' : ''
  return sort
}

export function librarySortLabel(sort: LibrarySort, adult: boolean) {
  if (sort === 'default') return adult ? '排序：最新加入' : '排序：默认推荐'
  if (sort === 'added-desc') return '排序：最新加入'
  if (sort === 'year-desc') return '排序：最新年份'
  if (sort === 'year-asc') return '排序：经典上映'
  return '排序：名称 A-Z'
}

export function nextLibrarySort(sort: LibrarySort, adult: boolean): LibrarySort {
  const cycle: LibrarySort[] = adult
    ? ['default', 'name-asc', 'year-desc']
    : ['default', 'year-desc', 'year-asc', 'name-asc']
  return cycle[(cycle.indexOf(sort) + 1) % cycle.length] ?? 'default'
}
