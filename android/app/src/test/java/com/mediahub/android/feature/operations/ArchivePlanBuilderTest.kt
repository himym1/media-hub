package com.mediahub.android.feature.operations

import com.mediahub.android.core.network.ArchiveStep
import com.mediahub.android.core.network.ArchiveSuggestion
import org.junit.Assert.assertEquals
import org.junit.Test

class ArchivePlanBuilderTest {
    @Test
    fun createsExplicitRenameThenMoveForSelectedSuggestion() {
        val suggestions = listOf(
            ArchiveSuggestion("10", "Old Name", "Suggested Name", "file", "review"),
            ArchiveSuggestion("11", "Other", "Other", "file", "review"),
        )

        val result = buildArchiveSteps(
            suggestions = suggestions,
            selectedFileIds = setOf("10"),
            names = mapOf("10" to "  New Name  "),
            targetParentId = " 200 ",
        )

        assertEquals(
            listOf(
                ArchiveStep("rename", "10", name = "New Name"),
                ArchiveStep("move", "10", targetParentId = "200"),
            ),
            result,
        )
    }

    @Test
    fun ignoresUnselectedAndUnchangedSuggestions() {
        val suggestions = listOf(
            ArchiveSuggestion("10", "Same Name", "Same Name", "file", "review"),
            ArchiveSuggestion("11", "Other", "Other", "file", "review"),
        )

        val result = buildArchiveSteps(
            suggestions = suggestions,
            selectedFileIds = setOf("10"),
            names = mapOf("10" to "Same Name"),
            targetParentId = "",
        )

        assertEquals(emptyList<ArchiveStep>(), result)
    }
}
