package com.mediahub.android.feature.library

import android.graphics.Bitmap
import android.graphics.Color
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.graphics.asAndroidBitmap
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.test.assertCountEquals
import androidx.compose.ui.test.assertHasClickAction
import androidx.compose.ui.test.assertHeightIsAtLeast
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsSelected
import androidx.compose.ui.test.captureToImage
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.isSelectable
import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.onRoot
import androidx.compose.ui.test.performClick
import androidx.compose.ui.unit.Density
import androidx.compose.ui.unit.dp
import androidx.test.core.graphics.writeToTestStorage
import androidx.test.ext.junit.runners.AndroidJUnit4
import com.mediahub.android.core.designsystem.MediaHubTheme
import com.mediahub.android.core.image.PosterLoader
import com.mediahub.android.core.network.EmbyEpisode
import com.mediahub.android.core.network.EmbyItem
import com.mediahub.android.core.network.EmbyItemDetail
import com.mediahub.android.core.network.MediaLibrary
import com.mediahub.android.playback.PlaybackFallback
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class LibraryScreenTest {
    private val poster = Bitmap.createBitmap(40, 60, Bitmap.Config.ARGB_8888).apply {
        eraseColor(Color.rgb(71, 116, 104))
    }.asImageBitmap()
    @get:Rule
    val composeRule = createComposeRule()
    private val posterLoader = object : PosterLoader {
        override fun cached(itemId: String): ImageBitmap = poster
        override suspend fun load(itemId: String): ImageBitmap = poster
    }

    @Test
    fun browseUsesContinuousRowsAndScopedActions() {
        var selectedItem = ""
        composeRule.setContent {
            MediaHubTheme {
                LibraryScreen(
                    uiState = LibraryBrowseState(
                        libraries = listOf(MediaLibrary(id = "movies", name = "电影", collectionType = "movies")),
                        selectedLibraryId = "movies",
                        items = listOf(EmbyItem(id = "item-1", name = "验收影片", type = "Movie", year = 2026, tmdbId = "100")),
                        total = 1,
                    ),
                    actions = browseActions(onSelectItem = { selectedItem = it }),
                    posterLoader = posterLoader,
                )
            }
        }

        composeRule.onNode(hasText("电影") and isSelectable())
            .assertIsSelected().assertHasClickAction().assertHeightIsAtLeast(48.dp)
        composeRule.onNodeWithText("验收影片").assertHasClickAction().assertHeightIsAtLeast(48.dp).performClick()
        composeRule.onAllNodesWithText("TMDB 100").assertCountEquals(0)
        composeRule.onNodeWithContentDescription("刷新当前媒体库").assertHasClickAction().assertHeightIsAtLeast(48.dp)
        composeRule.runOnIdle { assertEquals("item-1", selectedItem) }
        saveScreenshot("mediahub-library-browse")
    }

    @Test
    fun movieDetailPrioritizesDirectPlayAndKeepsEmbyFallback() {
        var played: Pair<EmbyItem, PlaybackFallback>? = null
        composeRule.setContent {
            MediaHubTheme {
                LibraryDetailScreen(
                    state = LibraryDetailState(itemId = "item-1", item = movieDetail()),
                    actions = detailActions { item, fallback -> played = item to fallback },
                    posterLoader = posterLoader,
                )
            }
        }

        composeRule.onNodeWithText("直接播放").assertHasClickAction().assertHeightIsAtLeast(48.dp).performClick()
        composeRule.onNodeWithText("在 Emby 网页中播放").assertHasClickAction().assertHeightIsAtLeast(48.dp)
        composeRule.runOnIdle {
            assertEquals("item-1", played?.first?.id)
            assertEquals("https://emby.example/item-1", played?.second?.webUrl)
        }
        composeRule.onNode(hasText("媒体信息") and hasClickAction()).performClick()
        composeRule.onNodeWithText("原名：Acceptance Movie").assertIsDisplayed()
        composeRule.onNodeWithText("TMDB 编号：100").assertIsDisplayed()
        saveScreenshot("mediahub-library-movie-detail")
    }

    @Test
    fun episodePickerUsesEpisodeSpecificTargetAndFallbackAtLargeFont() {
        val episode = EmbyEpisode(
            item = EmbyItem(id = "episode-2", name = "第二集", type = "Episode", year = 2026, tmdbId = null, season = 1, episode = 2),
            externalUrl = "https://emby.example/episode-2",
            appUrl = "emby://items/server-1/episode-2",
        )
        var played: Pair<EmbyItem, PlaybackFallback>? = null
        composeRule.setContent {
            val density = LocalDensity.current
            CompositionLocalProvider(LocalDensity provides Density(density.density, fontScale = 2f)) {
                MediaHubTheme {
                    LibraryDetailScreen(
                        state = LibraryDetailState(
                            itemId = "series-1",
                            item = movieDetail(type = "Series", id = "series-1"),
                            episodes = listOf(episode),
                        ),
                        actions = detailActions { item, fallback -> played = item to fallback },
                        posterLoader = posterLoader,
                    )
                }
            }
        }

        composeRule.onNodeWithText("第 1 季").assertHasClickAction().assertHeightIsAtLeast(48.dp)
        composeRule.onNodeWithContentDescription("播放 第 2 集 · 第二集").assertHasClickAction().assertHeightIsAtLeast(48.dp).performClick()
        composeRule.runOnIdle {
            assertEquals("episode-2", played?.first?.id)
            assertEquals("emby://items/server-1/episode-2", played?.second?.appUrl)
            assertEquals("https://emby.example/episode-2", played?.second?.webUrl)
        }
        saveScreenshot("mediahub-library-episodes-large")
    }

    private fun browseActions(onSelectItem: (String) -> Unit) = LibraryBrowseActions(
        onQueryChanged = {},
        onSearch = {},
        onClearSearch = {},
        onRefreshLibraries = {},
        onRefreshSelectedLibrary = {},
        onSelectLibrary = {},
        onSelectItem = onSelectItem,
        onChangePage = {},
    )

    private fun detailActions(onPlay: (EmbyItem, PlaybackFallback) -> Unit) = LibraryDetailActions(
        onClose = {},
        onRefresh = {},
        onPlayItem = onPlay,
    )

    private fun movieDetail(type: String = "Movie", id: String = "item-1") = EmbyItemDetail(
        item = EmbyItem(id = id, name = "验收影片", type = type, year = 2026, tmdbId = "100"),
        originalTitle = "Acceptance Movie",
        overview = "用于验证媒体库详情。",
        communityRating = 8.2,
        runtimeMinutes = 118,
        genres = listOf("Drama", "Science Fiction"),
        mediaSourceCount = if (type == "Series") 0 else 1,
        externalUrl = "https://emby.example/$id",
        appUrl = "emby://items/server-1/$id",
    )

    private fun saveScreenshot(name: String) {
        val image = composeRule.onRoot().captureToImage()
        assertTrue(image.width > 0 && image.height > 0)
        image.asAndroidBitmap().writeToTestStorage(name)
    }
}
