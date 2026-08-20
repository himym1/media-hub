package com.mediahub.android

import android.graphics.Bitmap
import androidx.activity.ComponentActivity
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.graphics.asAndroidBitmap
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.test.captureToImage
import androidx.compose.ui.test.junit4.v2.createAndroidComposeRule
import androidx.compose.ui.test.hasScrollToIndexAction
import androidx.compose.ui.test.performScrollToIndex
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.onRoot
import androidx.compose.ui.test.performClick
import androidx.compose.ui.unit.Density
import androidx.test.core.graphics.writeToTestStorage
import androidx.test.ext.junit.runners.AndroidJUnit4
import com.mediahub.android.app.MainDestination
import com.mediahub.android.app.MainNavigationHistory
import com.mediahub.android.app.WorkspaceDetail
import com.mediahub.android.core.designsystem.MediaHubTheme
import com.mediahub.android.core.network.MediaSubscription
import com.mediahub.android.core.network.SubscriptionPreferences
import com.mediahub.android.core.network.TransferJob
import com.mediahub.android.feature.subscriptions.SubscriptionEditorScreen
import com.mediahub.android.feature.subscriptions.SubscriptionEditorActions
import com.mediahub.android.feature.subscriptions.SubscriptionListActions
import com.mediahub.android.feature.subscriptions.SubscriptionEditorState
import com.mediahub.android.feature.subscriptions.SubscriptionListScreen
import com.mediahub.android.feature.subscriptions.SubscriptionUiState
import com.mediahub.android.feature.transfers.TransferActions
import com.mediahub.android.feature.transfers.TransferScreen
import com.mediahub.android.feature.transfers.TransferUiState
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class WorkspaceDetailFlowTest {
    @get:Rule
    val composeRule = createAndroidComposeRule<ComponentActivity>()

    @Test
    fun transferListOpensOneDetailPageAndReturnsToShell() {
        val job = transferJob()
        val navigation = MainNavigationHistory()
        var route by mutableStateOf(navigation.show(MainDestination.Transfers))
        var state by mutableStateOf(TransferUiState(jobs = listOf(job)))
        composeRule.setContent {
            MediaHubTheme {
                WorkspaceShell(
                    destination = MainDestination.Transfers,
                    detailOpen = route.detail is WorkspaceDetail.Transfer,
                    onSystemBack = {},
                    onOpenSystem = {},
                    onPrimarySelected = {},
                ) {
                    TransferScreen(
                        uiState = state,
                        detailOpen = route.detail is WorkspaceDetail.Transfer,
                        actions = TransferActions(
                            select = {
                                state = state.copy(selectedId = it, selected = job)
                                route = navigation.openDetail(WorkspaceDetail.Transfer(it))
                            },
                            refresh = {},
                            retry = {},
                            back = {
                                state = state.copy(selectedId = null, selected = null)
                                route = navigation.closeDetail()
                            },
                            retryNotification = {},
                            showArchived = {},
                            setArchived = {},
                        ),
                    )
                }
            }
        }

        composeRule.onNodeWithText("示例任务").performClick()
        composeRule.onNodeWithTag("workspace-top-bar").assertDoesNotExist()
        composeRule.onNodeWithTag("workspace-bottom-nav").assertDoesNotExist()
        composeRule.onNodeWithText("任务详情").assertExists()
        saveScreenshot("mediahub-transfer-detail")

        composeRule.onNodeWithContentDescription("返回任务列表").performClick()
        composeRule.onNodeWithTag("workspace-top-bar").assertExists()
        composeRule.onNodeWithTag("workspace-bottom-nav").assertExists()
        composeRule.onNodeWithText("示例任务").assertExists()
    }

    @Test
    fun subscriptionListOpensLargeFontEditorAndReturnsToShell() {
        val subscription = subscription()
        val navigation = MainNavigationHistory()
        var route by mutableStateOf(navigation.show(MainDestination.Subscriptions))
        var state by mutableStateOf(SubscriptionUiState(subscriptions = listOf(subscription)))
        composeRule.setContent {
            val density = LocalDensity.current
            CompositionLocalProvider(LocalDensity provides Density(density.density, 2f)) {
                MediaHubTheme {
                    WorkspaceShell(
                        destination = MainDestination.Subscriptions,
                        detailOpen = route.detail is WorkspaceDetail.SubscriptionEditor,
                        onSystemBack = {},
                        onOpenSystem = {},
                        onPrimarySelected = {},
                    ) {
                        if (route.detail is WorkspaceDetail.SubscriptionEditor) {
                            SubscriptionEditorScreen(
                                state = state,
                                actions = SubscriptionEditorActions(
                                    editorChanged = { state = state.copy(editor = it) },
                                    back = {
                                        state = state.copy(selectedId = null)
                                        route = navigation.closeDetail()
                                    },
                                    save = {},
                                    toggle = {},
                                    run = {},
                                    delete = {},
                                ),
                            )
                        } else {
                            SubscriptionListScreen(
                                state = state,
                                actions = SubscriptionListActions(
                                    create = {},
                                    select = {
                                        state = state.copy(
                                            selectedId = it,
                                                editor = SubscriptionEditorState(
                                                tmdbId = subscription.tmdbId,
                                                title = subscription.title,
                                                mediaType = subscription.mediaType,
                                                policy = subscription.policy,
                                                intervalMinutes = subscription.intervalMinutes.toString(),
                                                enabled = subscription.enabled,
                                            ),
                                        )
                                        route = navigation.openDetail(WorkspaceDetail.SubscriptionEditor(subscription.id))
                                    },
                                    setAllEnabled = {},
                                    export = {},
                                    exportConsumed = {},
                                    import = {},
                                    fileError = {},
                                ),
                            )
                        }
                    }
                }
            }
        }

        composeRule.onNodeWithContentDescription("更多订阅操作").assertExists()
        composeRule.onNodeWithContentDescription("新建订阅").assertExists()
        composeRule.onNode(hasScrollToIndexAction()).performScrollToIndex(0)
        composeRule.onNodeWithContentDescription("打开订阅 示例订阅").assertExists()
        saveScreenshot("mediahub-subscription-list-large")
        composeRule.onNodeWithContentDescription("打开订阅 示例订阅").performClick()
        composeRule.onNodeWithTag("workspace-top-bar").assertDoesNotExist()
        composeRule.onNodeWithTag("workspace-bottom-nav").assertDoesNotExist()
        composeRule.onNodeWithText("编辑订阅").assertExists()
        saveScreenshot("mediahub-subscription-editor-large")

        composeRule.onNodeWithContentDescription("返回订阅列表").performClick()
        composeRule.onNodeWithTag("workspace-top-bar").assertExists()
        composeRule.onNodeWithTag("workspace-bottom-nav").assertExists()
        composeRule.onNode(hasScrollToIndexAction()).performScrollToIndex(0)
        composeRule.onNodeWithContentDescription("打开订阅 示例订阅").assertExists()
    }

    private fun transferJob() = TransferJob(
        id = "transfer-1",
        title = "示例任务",
        year = 2026,
        season = 0,
        mediaType = "movie",
        tmdbId = "101",
        source = "mikan",
        state = "completed",
        errorCode = null,
        errorMessage = null,
        retryable = false,
        archived = false,
        createdAt = "2026-08-17T12:00:00Z",
        updatedAt = "2026-08-17T12:05:00Z",
    )

    private fun subscription() = MediaSubscription(
        id = "subscription-1",
        tmdbId = "202",
        title = "示例订阅",
        originalTitle = "Example",
        year = 2026,
        mediaType = "series",
        season = 1,
        policy = "upgrade",
        enabled = true,
        intervalMinutes = 60,
        sourceIds = emptyList(),
        preferences = SubscriptionPreferences(),
        nextRunAt = "2026-08-17T13:00:00Z",
        lastRunAt = null,
        createdAt = "2026-08-17T12:00:00Z",
        updatedAt = "2026-08-17T12:00:00Z",
    )

    private fun saveScreenshot(name: String) {
        val image = composeRule.onRoot().captureToImage()
        assertTrue(image.width > 0 && image.height > 0)
        image.asAndroidBitmap().copy(Bitmap.Config.ARGB_8888, false).writeToTestStorage(name)
    }
}
