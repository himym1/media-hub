package com.mediahub.android

import android.graphics.Bitmap
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.graphics.asAndroidBitmap
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.test.assertCountEquals
import androidx.compose.ui.test.captureToImage
import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.onRoot
import androidx.compose.ui.test.performClick
import androidx.compose.ui.unit.Density
import androidx.test.core.graphics.writeToTestStorage
import androidx.test.ext.junit.runners.AndroidJUnit4
import com.mediahub.android.app.LocalTwoPane
import com.mediahub.android.core.designsystem.MediaHubTheme
import com.mediahub.android.core.network.IntegrationHealth
import com.mediahub.android.core.network.OperationalStatistics
import com.mediahub.android.core.network.ReleaseFacts
import com.mediahub.android.core.network.SearchCandidate
import com.mediahub.android.feature.operations.ArchiveActions
import com.mediahub.android.feature.operations.ArchiveState
import com.mediahub.android.feature.operations.Drive115Actions
import com.mediahub.android.feature.operations.Drive115State
import com.mediahub.android.feature.operations.LocalUploadActions
import com.mediahub.android.feature.operations.LocalUploadState
import com.mediahub.android.feature.operations.OperationsScreen
import com.mediahub.android.feature.search.SearchScreen
import com.mediahub.android.feature.search.SearchUiState
import com.mediahub.android.feature.services.ServicesScreen
import com.mediahub.android.feature.services.ServicesUiState
import org.junit.Assert.assertTrue
import org.junit.Assert.assertEquals
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class SecondStageLayoutTest {
    @get:Rule
    val composeRule = createComposeRule()

    @Test
    fun searchResultsRemainScannableAtLargeFont() {
        val candidate = SearchCandidate(
            id = "candidate-1",
            title = "一部标题很长但仍需完整显示的电影资源",
            year = 2026,
            season = 0,
            mediaType = "movie",
            tmdbId = "100",
            source = "mikan",
            provider = "Mikan",
            posterUrl = null,
            release = ReleaseFacts("2160p", "HEVC", "Dolby Vision", "TrueHD Atmos", 32L * 1024 * 1024 * 1024),
            transferState = "ready",
            transferToken = "token",
        )
        composeRule.setContent {
            val density = LocalDensity.current
            CompositionLocalProvider(LocalDensity provides Density(density.density, 2f)) {
                MediaHubTheme {
                    SearchScreen(
                        uiState = SearchUiState(
                            query = "电影",
                            submittedQuery = "电影",
                            results = listOf(candidate),
                            selectedCandidateId = candidate.id,
                        ),
                        onQueryChanged = {},
                        onSearch = {},
                        onRefreshOverview = {},
                        onTrendingSelected = {},
                        onCandidateSelected = {},
                        onRecommendationSelected = {},
                        onTransfer = {},
                        onSubscribe = {},
                    )
                }
            }
        }

        composeRule.onNodeWithText(candidate.title).assertExists()
        composeRule.onNodeWithText("开始转存").assertExists()
        saveScreenshot("mediahub-search-large")
    }

    @Test
    fun searchTwoPaneKeepsResultsBesideDetail() {
        val candidate = SearchCandidate(
            id = "candidate-1",
            title = "一部标题很长但仍需完整显示的电影资源",
            year = 2026,
            season = 0,
            mediaType = "movie",
            tmdbId = "100",
            source = "mikan",
            provider = "Mikan",
            posterUrl = null,
            release = ReleaseFacts("2160p", "HEVC", "Dolby Vision", "TrueHD Atmos", 32L * 1024 * 1024 * 1024),
            transferState = "ready",
            transferToken = "token",
        )
        composeRule.setContent {
            MediaHubTheme {
                CompositionLocalProvider(LocalTwoPane provides true) {
                    SearchScreen(
                        uiState = SearchUiState(
                            query = "电影",
                            submittedQuery = "电影",
                            results = listOf(candidate),
                            selectedCandidateId = candidate.id,
                        ),
                        onQueryChanged = {},
                        onSearch = {},
                        onRefreshOverview = {},
                        onTrendingSelected = {},
                        onCandidateSelected = {},
                        onRecommendationSelected = {},
                        onTransfer = {},
                        onSubscribe = {},
                    )
                }
            }
        }

        composeRule.onNodeWithTag("search-list-pane").assertExists()
        composeRule.onNodeWithTag("search-detail-pane").assertExists()
        composeRule.onNodeWithText(candidate.title).assertExists()
        composeRule.onNodeWithText("开始转存").assertExists()
        saveScreenshot("mediahub-search-tablet")
    }

    @Test
    fun servicesMetricsUseTwoRowsAtLargeFont() {
        composeRule.setContent {
            val density = LocalDensity.current
            CompositionLocalProvider(LocalDensity provides Density(density.density, 2f)) {
                MediaHubTheme {
                    ServicesScreen(
                        uiState = ServicesUiState(
                            integrations = listOf(
                                IntegrationHealth("emby", "Emby", "healthy", "媒体库在线"),
                                IntegrationHealth("emby-playback", "Media3 播放入口", "healthy", "QMediaSync 播放入口在线"),
                            ),
                            statistics = OperationalStatistics(4, 1, 2, 1, 0, 3, 2, 8, 1, 0, 0, 0),
                        ),
                        onRefresh = {},
                        onToggleSettings = {},
                        onSettingsDraftChange = {},
                        onSaveSettings = {},
                        onTestWeCom = {},
                        onStartDriveAuthorization = {},
                        onRetryCheckIn = {},
                        onCurrentPasswordChange = {},
                        onNewPasswordChange = {},
                        onConfirmationChange = {},
                        onChangePassword = {},
                        onChangeServer = {},
                        onCheckForUpdate = {},
                        onDownloadUpdate = {},
                        onInstallUpdate = {},
                        onLogout = {},
                    )
                }
            }
        }

        composeRule.onNodeWithText("进行中").assertExists()
        composeRule.onNodeWithText("失败运行").assertExists()
        composeRule.onNodeWithText("Emby").assertExists()
        composeRule.onNodeWithText("Media3 播放入口").assertExists()
        saveScreenshot("mediahub-services-large")
    }

    @Test
    fun servicesTwoPaneKeepsSectionListBesideContent() {
        composeRule.setContent {
            MediaHubTheme {
                CompositionLocalProvider(LocalTwoPane provides true) {
                    ServicesScreen(
                        uiState = ServicesUiState(
                            integrations = listOf(
                                IntegrationHealth("emby", "Emby", "healthy", "媒体库在线"),
                            ),
                            statistics = OperationalStatistics(4, 1, 2, 1, 0, 3, 2, 8, 1, 0, 0, 0),
                        ),
                        onRefresh = {},
                        onToggleSettings = {},
                        onSettingsDraftChange = {},
                        onSaveSettings = {},
                        onTestWeCom = {},
                        onStartDriveAuthorization = {},
                        onRetryCheckIn = {},
                        onCurrentPasswordChange = {},
                        onNewPasswordChange = {},
                        onConfirmationChange = {},
                        onChangePassword = {},
                        onChangeServer = {},
                        onCheckForUpdate = {},
                        onDownloadUpdate = {},
                        onInstallUpdate = {},
                        onLogout = {},
                    )
                }
            }
        }

        composeRule.onNodeWithTag("services-section-list").assertExists()
        composeRule.onNodeWithTag("services-section-detail").assertExists()
        composeRule.onNodeWithText("状态概览").assertExists()
        composeRule.onNodeWithText("服务配置").assertExists()
        composeRule.onNodeWithText("账户与更新").assertExists()
        composeRule.onNodeWithText("进行中").assertExists()
        composeRule.onNodeWithText("服务配置").performClick()
        composeRule.onNodeWithText("服务设置").assertExists()
        saveScreenshot("mediahub-services-tablet")
    }

    @Test
    fun operationsShowsOnlyTheSelectedTool() {
        var driveRefreshes = 0
        var uploadRefreshes = 0
        var archiveRefreshes = 0
        var section by mutableStateOf("drive")
        composeRule.setContent {
            MediaHubTheme {
                OperationsScreen(
                    driveState = Drive115State(loading = false, error = "115 读取失败"),
                    localUploadState = LocalUploadState(loading = false, error = "上传读取失败"),
                    archiveState = ArchiveState(loading = false, error = "归档读取失败"),
                    section = section,
                    onSectionChanged = { section = it },
                    driveActions = Drive115Actions({ driveRefreshes++ }, {}, {}, {}, {}, {}, {}, {}, {}),
                    localUploadActions = LocalUploadActions({ uploadRefreshes++ }, {}, {}, {}, {}, {}, {}),
                    archiveActions = ArchiveActions({ archiveRefreshes++ }, {}, {}, {}, {}, { _, _ -> }, {}, {}, {}),
                )
            }
        }

        composeRule.onAllNodesWithText("115 文件").assertCountEquals(2)
        composeRule.onNodeWithText("115 读取失败").assertExists()
        composeRule.onNodeWithContentDescription("刷新当前工具").performClick()
        assertEquals(1, driveRefreshes)
        composeRule.onNodeWithText("本地上传").performClick()
        composeRule.onNodeWithText("当前服务器未配置本地上传目录").assertExists()
        composeRule.onAllNodesWithText("115 文件").assertCountEquals(1)
        composeRule.onNodeWithText("上传读取失败").assertExists()
        composeRule.onAllNodesWithText("115 读取失败").assertCountEquals(0)
        composeRule.onNodeWithContentDescription("刷新当前工具").performClick()
        assertEquals(1, uploadRefreshes)
        composeRule.onNodeWithText("归档整理").performClick()
        composeRule.onNodeWithText("原生归档整理").assertExists()
        composeRule.onAllNodesWithText("当前服务器未配置本地上传目录").assertCountEquals(0)
        composeRule.onNodeWithText("归档读取失败").assertExists()
        composeRule.onNodeWithContentDescription("刷新当前工具").performClick()
        assertEquals(1, archiveRefreshes)
        assertEquals(1, driveRefreshes)
        assertEquals(1, uploadRefreshes)
        saveScreenshot("mediahub-operations-sections")
    }

    private fun saveScreenshot(name: String) {
        val image = composeRule.onRoot().captureToImage()
        assertTrue(image.width > 0 && image.height > 0)
        image.asAndroidBitmap().copy(Bitmap.Config.ARGB_8888, false).writeToTestStorage(name)
    }
}
