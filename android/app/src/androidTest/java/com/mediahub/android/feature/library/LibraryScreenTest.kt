package com.mediahub.android.feature.library

import androidx.compose.ui.test.assertHasClickAction
import androidx.compose.ui.test.assertHeightIsAtLeast
import androidx.compose.ui.test.assertIsSelected
import androidx.compose.ui.test.isSelectable
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.unit.dp
import androidx.test.ext.junit.runners.AndroidJUnit4
import com.mediahub.android.core.designsystem.MediaHubTheme
import com.mediahub.android.core.network.EmbyItem
import com.mediahub.android.core.network.MediaLibrary
import org.junit.Assert.assertEquals
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class LibraryScreenTest {
    @get:Rule
    val composeRule = createComposeRule()

    @Test
    fun libraryAndMediaRowsExposeSelectionAndTouchTargets() {
        var selectedItem = ""
        composeRule.setContent {
            MediaHubTheme {
                LibraryScreen(
                    uiState = LibraryUiState(
                        libraries = listOf(MediaLibrary(id = "movies", name = "电影", collectionType = "movies")),
                        selectedLibraryId = "movies",
                        items = listOf(EmbyItem(id = "item-1", name = "验收影片", type = "Movie", year = 2026, tmdbId = "100")),
                        total = 1,
                    ),
                    onQueryChanged = {},
                    onSearch = {},
                    onClearSearch = {},
                    onRefreshLibraries = {},
                    onRefreshSelectedLibrary = {},
                    onSelectLibrary = {},
                    onSelectItem = { selectedItem = it },
                    onChangePage = {},
                )
            }
        }

        composeRule.onNode(hasText("电影") and isSelectable())
            .assertIsSelected().assertHasClickAction().assertHeightIsAtLeast(48.dp)
        composeRule.onNodeWithText("验收影片").assertHasClickAction().assertHeightIsAtLeast(48.dp).performClick()
        composeRule.onNodeWithContentDescription("刷新当前媒体库").assertHasClickAction().assertHeightIsAtLeast(48.dp)
        composeRule.runOnIdle { assertEquals("item-1", selectedItem) }
    }
}
