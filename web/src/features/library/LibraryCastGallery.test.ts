import { describe, expect, it } from 'vitest'
import {
  formatPersonRole,
  personInitials,
} from './libraryCast'

describe('LibraryCastGallery utils', () => {
  it('formats person roles correctly', () => {
    expect(formatPersonRole({ id: '1', name: 'Nolan', type: 'Director' })).toBe('导演')
    expect(formatPersonRole({ id: '2', name: 'Jonathan', type: 'Writer' })).toBe('编剧')
    expect(formatPersonRole({ id: '3', name: 'Emma', type: 'Producer' })).toBe('制片人')
    expect(formatPersonRole({ id: '4', name: 'Cillian', type: 'Actor', role: 'Oppenheimer' })).toBe('饰 Oppenheimer')
    expect(formatPersonRole({ id: '5', name: 'Matt', type: 'Actor' })).toBe('演员')
    expect(formatPersonRole({ id: '6', name: 'Casey', type: 'GuestStar' })).toBe('客串')
    expect(formatPersonRole({ id: '7', name: 'Creator Guy', type: 'Creator' })).toBe('主创')
    expect(formatPersonRole({ id: '8', name: 'Director Guy', type: 'Director', role: 'Director' })).toBe('导演')
    expect(formatPersonRole({ id: '9', name: 'Writer Guy', type: 'Writer', role: 'Writer' })).toBe('编剧')
  })

  it('computes person initials correctly for various names', () => {
    expect(personInitials('克里斯托弗·诺兰')).toBe('克里')
    expect(personInitials('刘德华')).toBe('刘德')
    expect(personInitials('Christopher Nolan')).toBe('CN')
    expect(personInitials('Cillian Murphy')).toBe('CM')
    expect(personInitials('Zendaya')).toBe('ZE')
    expect(personInitials('   ')).toBe('?')
    expect(personInitials('')).toBe('?')
  })
})
