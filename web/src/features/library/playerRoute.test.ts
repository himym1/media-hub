import { describe, expect, it } from 'vitest'
import { isPlayerView, playerItemId, playerSeriesId } from './playerRoute'

describe('playerRoute', () => {
  it('reads the dedicated player query', () => {
    expect(isPlayerView('?view=library&play=item-1')).toBe(false)
    expect(isPlayerView('?view=player&play=item-1&series=series-2')).toBe(true)
    expect(playerItemId('?view=player&play=item-1&series=series-2')).toBe('item-1')
    expect(playerSeriesId('?view=player&play=item-1&series=series-2')).toBe('series-2')
    expect(playerSeriesId('?view=player&play=item-1')).toBeNull()
  })
})
