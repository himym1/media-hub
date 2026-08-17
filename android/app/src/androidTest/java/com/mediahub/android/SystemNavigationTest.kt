package com.mediahub.android

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.graphics.asAndroidBitmap
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.test.assertHasClickAction
import androidx.compose.ui.test.assertHeightIsAtLeast
import androidx.compose.ui.test.hasClickAction
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.assertIsSelected
import androidx.compose.ui.test.captureToImage
import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.onRoot
import androidx.compose.ui.test.performClick
import androidx.compose.ui.unit.dp
import androidx.test.core.graphics.writeToTestStorage
import androidx.test.ext.junit.runners.AndroidJUnit4
import com.mediahub.android.app.MainDestination
import com.mediahub.android.core.designsystem.MediaHubText
import com.mediahub.android.core.designsystem.MediaHubTheme
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class SystemNavigationTest {
    @get:Rule
    val composeRule = createComposeRule()

    @Test
    fun workspaceRestoresPrimaryDestinationAfterSystemLayer() {
        var destination by mutableStateOf(MainDestination.Services)
        composeRule.setContent {
            MediaHubTheme {
                WorkspaceShell(
                    destination = destination,
                    detailOpen = false,
                    onSystemBack = { destination = MainDestination.Library },
                    onOpenSystem = { destination = MainDestination.Services },
                    onSystemSelected = { destination = it },
                    onPrimarySelected = { destination = it },
                ) {
                    MediaHubText("系统内容")
                }
            }
        }

        composeRule.onNodeWithContentDescription("返回主页面")
            .assertHasClickAction().assertHeightIsAtLeast(48.dp)
        composeRule.onNodeWithText("服务").assertIsSelected().assertHeightIsAtLeast(48.dp)
        composeRule.onNode(hasText("运维") and hasClickAction())
            .assertHasClickAction().assertHeightIsAtLeast(48.dp).performClick()
        composeRule.onNode(hasText("运维") and hasClickAction()).assertIsSelected()
        composeRule.onNodeWithContentDescription("返回主页面").performClick()
        composeRule.runOnIdle { assertEquals(MainDestination.Library, destination) }
        composeRule.onNodeWithTag("workspace-bottom-nav").assertExists()
        saveScreenshot("mediahub-system-navigation")
    }

    @Test
    fun libraryDetailHasOnlyItsOwnChrome() {
        composeRule.setContent {
            MediaHubTheme {
                WorkspaceShell(
                    destination = MainDestination.Library,
                    detailOpen = true,
                    onSystemBack = {},
                    onOpenSystem = {},
                    onSystemSelected = {},
                    onPrimarySelected = {},
                ) {
                    MediaHubText("媒体详情", modifier = androidx.compose.ui.Modifier.testTag("detail-title"))
                }
            }
        }

        composeRule.onNodeWithTag("workspace-top-bar").assertDoesNotExist()
        composeRule.onNodeWithTag("workspace-bottom-nav").assertDoesNotExist()
        composeRule.onNodeWithTag("detail-title").assertExists()
        saveScreenshot("mediahub-workspace-detail")
    }

    private fun saveScreenshot(name: String) {
        val image = composeRule.onRoot().captureToImage()
        assertTrue(image.width > 0 && image.height > 0)
        image.asAndroidBitmap().writeToTestStorage(name)
    }
}
