import { describe, expect, it } from 'vitest'
import { updateUrl } from './urlState'

describe('updateUrl', () => {
  it('preserves unrelated parameters while replacing scoped state', () => {
    const next = updateUrl(new URL('https://media.example/?view=settings&settings=overview'), {
      settings: 'providers',
    })

    expect(next.pathname).toBe('/')
    expect(next.searchParams.get('view')).toBe('settings')
    expect(next.searchParams.get('settings')).toBe('providers')
  })

  it('removes empty state without changing the hash', () => {
    const next = updateUrl(new URL('https://media.example/?view=discover&q=test#results'), { q: null })

    expect(next.searchParams.get('view')).toBe('discover')
    expect(next.searchParams.has('q')).toBe(false)
    expect(next.hash).toBe('#results')
  })
})
