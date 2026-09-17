import { renderToString } from 'react-dom/server'
import { describe, expect, it } from 'vitest'
import { LibraryDetailSkeleton, LibraryEpisodesSkeleton, LibraryPosterGridSkeleton } from './LibrarySkeletons'

describe('LibrarySkeletons', () => {
  it('renders poster grid skeleton with shimmer and aria attributes', () => {
    const html = renderToString(<LibraryPosterGridSkeleton count={8} />)
    expect(html).toContain('aria-busy="true"')
    expect(html).toContain('skeleton-shimmer')
    expect(html).toContain('library-poster-card')
  })

  it('renders detail skeleton with backdrop, poster, and cast row', () => {
    const html = renderToString(<LibraryDetailSkeleton />)
    expect(html).toContain('library-detail-skeleton')
    expect(html).toContain('skeleton-backdrop')
    expect(html).toContain('skeleton-cast-row')
  })

  it('renders episodes skeleton with up-next, season tabs, and stills', () => {
    const html = renderToString(<LibraryEpisodesSkeleton />)
    expect(html).toContain('library-episodes-skeleton')
    expect(html).toContain('skeleton-up-next')
    expect(html).toContain('skeleton-season-tabs')
    expect(html).toContain('skeleton-episode-card-list')
  })
})
