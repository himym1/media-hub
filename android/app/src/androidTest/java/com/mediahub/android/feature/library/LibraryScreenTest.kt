package com.mediahub.android.feature.library

import androidx.compose.ui.test.assertHasClickAction
import androidx.compose.ui.test.assertHeightIsAtLeast
import androidx.compose.ui.test.assertIsSelected
import androidx.compose.ui.test.assertCountEquals
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.onAllNodesWithText
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
import com.mediahub.android.core.network.EmbyItemDetail
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
        composeRule.onAllNodesWithText("TMDB 100").assertCountEquals(0)
        composeRule.onNodeWithContentDescription("刷新当前媒体库").assertHasClickAction().assertHeightIsAtLeast(48.dp)
        composeRule.runOnIdle { assertEquals("item-1", selectedItem) }
    }

    @Test
    fun detailsLocalizeMetadataAndKeepTechnicalIdentityCollapsed() {
        composeRule.setContent {
            MediaHubTheme {
                LibraryDetailScreen(
                    state = LibraryUiState(
                        selectedItemId = "item-1",
                        selectedItem = EmbyItemDetail(
                            item = EmbyItem(id = "item-1", name = "验收影片", type = "Movie", year = 2026, tmdbId = "100"),
                            originalTitle = "Acceptance Movie",
                            overview = "用于验证媒体库详情。",
                            communityRating = 8.2,
                            runtimeMinutes = 118,
                            genres = listOf("Drama", "Science Fiction"),
                            mediaSourceCount = 1,
                            externalUrl = "https://emby.example/item-1",
                        ),
                    ),
                    onBack = {},
                    onRefresh = {},
                )
            }
        }

        composeRule.onNodeWithText("剧情 · 科幻").assertIsDisplayed()
        composeRule.onAllNodesWithText("Acceptance Movie").assertCountEquals(0)
        composeRule.onAllNodesWithText("TMDB 编号：100").assertCountEquals(0)
        composeRule.onAllNodesWithText("来自 Emby 的安全元数据").assertCountEquals(0)
        composeRule.onAllNodesWithText("Media Hub 不代理或删除媒体文件。").assertCountEquals(0)

        composeRule.onNode(hasText("更多信息") and hasClickAction())
            .assertHasClickAction().assertHeightIsAtLeast(48.dp).performClick()
        composeRule.onNodeWithText("原名：Acceptance Movie").assertIsDisplayed()
        composeRule.onNodeWithText("TMDB 编号：100").assertIsDisplayed()
        composeRule.onNodeWithText("媒体源：1 个").assertIsDisplayed()
    }
}
