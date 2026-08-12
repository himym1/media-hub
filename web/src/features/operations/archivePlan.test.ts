import { describe, expect, it } from 'vitest'
import type { ArchiveSuggestion } from '../../shared/api/mediaHub'
import { buildArchiveSteps } from './archivePlan'

const suggestions: ArchiveSuggestion[] = [
  { fileId: '10', currentName: 'Old Name', suggestedName: 'Suggested Name', kind: 'file', confidence: 'review' },
  { fileId: '11', currentName: 'Other', suggestedName: 'Other', kind: 'file', confidence: 'review' },
]

describe('buildArchiveSteps', () => {
  it('creates an explicit rename followed by a move for a selected item', () => {
    expect(buildArchiveSteps(suggestions, new Set(['10']), { 10: '  New Name  ' }, ' 200 ')).toEqual([
      { operation: 'rename', fileId: '10', name: 'New Name' },
      { operation: 'move', fileId: '10', targetParentId: '200' },
    ])
  })

  it('ignores unselected and unchanged suggestions', () => {
    expect(buildArchiveSteps(suggestions, new Set(['10']), { 10: 'Old Name' }, '')).toEqual([])
  })
})
