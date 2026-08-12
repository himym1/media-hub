import type { ArchiveStep, ArchiveSuggestion } from '../../shared/api/mediaHub'

export function buildArchiveSteps(
  suggestions: ArchiveSuggestion[],
  selectedFileIds: Set<string>,
  names: Record<string, string>,
  targetParentId: string,
): ArchiveStep[] {
  const target = targetParentId.trim()
  return suggestions.flatMap<ArchiveStep>((item) => {
    if (!selectedFileIds.has(item.fileId)) return []

    const steps: ArchiveStep[] = []
    const name = names[item.fileId]?.trim()
    if (name && name !== item.currentName) steps.push({ operation: 'rename', fileId: item.fileId, name })
    if (target) steps.push({ operation: 'move', fileId: item.fileId, targetParentId: target })
    return steps
  })
}
