package com.mediahub.android.feature.operations

import com.mediahub.android.core.network.ArchiveStep
import com.mediahub.android.core.network.ArchiveSuggestion

internal fun buildArchiveSteps(
    suggestions: List<ArchiveSuggestion>,
    selectedFileIds: Set<String>,
    names: Map<String, String>,
    targetParentId: String,
): List<ArchiveStep> {
    val target = targetParentId.trim()
    return suggestions
        .filter { it.fileId in selectedFileIds }
        .flatMap { item ->
            buildList {
                val name = names[item.fileId].orEmpty().trim()
                if (name.isNotEmpty() && name != item.currentName) {
                    add(ArchiveStep("rename", item.fileId, name = name))
                }
                if (target.isNotEmpty()) {
                    add(ArchiveStep("move", item.fileId, targetParentId = target))
                }
            }
        }
}
