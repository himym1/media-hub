package com.mediahub.android.feature.player

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.graphics.asAndroidBitmap
import androidx.compose.ui.test.assertHasClickAction
import androidx.compose.ui.test.assertHeightIsAtLeast
import androidx.compose.ui.test.captureToImage
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.compose.ui.test.onRoot
import androidx.compose.ui.test.performClick
import androidx.compose.ui.unit.dp
import androidx.test.core.graphics.writeToTestStorage
import androidx.test.ext.junit.runners.AndroidJUnit4
import com.mediahub.android.core.designsystem.MediaHubTheme
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class PlayerScreenTest {
    @get:Rule
    val composeRule = createComposeRule()

    @Test
    fun playerActionsMeetTouchTargetsAndPipHidesChrome() {
        var back = 0
        var rotate = 0
        var pip = 0
        var pipMode by mutableStateOf(false)
        composeRule.setContent {
            MediaHubTheme {
                PlayerScreen(
                    state = PlayerUiState.Loading,
                    title = "测试视频",
                    controller = null,
                    isPictureInPicture = pipMode,
                    actions = PlayerActions(
                        onRetry = {},
                        onBack = { back++ },
                        onToggleOrientation = { rotate++ },
                        onEnterPictureInPicture = { pip++ },
                    ),
                )
            }
        }

        composeRule.onNodeWithContentDescription("返回").assertHasClickAction().assertHeightIsAtLeast(48.dp).performClick()
        composeRule.onNodeWithContentDescription("旋转屏幕").assertHasClickAction().assertHeightIsAtLeast(48.dp).performClick()
        composeRule.onNodeWithContentDescription("进入画中画").assertHasClickAction().assertHeightIsAtLeast(48.dp).performClick()
        val normalImage = composeRule.onRoot().captureToImage()
        assertTrue(normalImage.width > 0 && normalImage.height > 0)
        normalImage.asAndroidBitmap().writeToTestStorage("mediahub-player")
        composeRule.runOnIdle {
            assertEquals(1, back)
            assertEquals(1, rotate)
            assertEquals(1, pip)
            pipMode = true
        }
        composeRule.waitForIdle()
        composeRule.onNodeWithContentDescription("返回").assertDoesNotExist()
        val pipImage = composeRule.onRoot().captureToImage()
        assertTrue(pipImage.width > 0 && pipImage.height > 0)
        pipImage.asAndroidBitmap().writeToTestStorage("mediahub-player-pip")
    }
}
