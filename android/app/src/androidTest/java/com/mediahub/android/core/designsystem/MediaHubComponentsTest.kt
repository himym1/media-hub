package com.mediahub.android.core.designsystem

import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.ImageBitmap
import androidx.compose.ui.graphics.asAndroidBitmap
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.test.assertHasClickAction
import androidx.compose.ui.test.assertHeightIsAtLeast
import androidx.compose.ui.test.assertIsNotSelected
import androidx.compose.ui.test.assertIsSelected
import androidx.compose.ui.test.captureToImage
import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.onNodeWithTag
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.unit.Density
import androidx.compose.ui.unit.dp
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.core.graphics.writeToTestStorage
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class MediaHubComponentsTest {
    @get:Rule
    val composeRule = createComposeRule()

    @Test
    fun segmentedControlExposesSelectionAndMinimumTouchHeight() {
        var selected by mutableStateOf("overview")
        composeRule.setContent {
            MediaHubTheme {
                MediaHubSegmentedControl(
                    options = listOf("overview" to "概览", "providers" to "Provider"),
                    selected = selected,
                    onSelected = { selected = it },
                    modifier = Modifier.testTag("settings-sections"),
                )
            }
        }

        composeRule.onNodeWithText("概览").assertIsSelected().assertHeightIsAtLeast(48.dp)
        composeRule.onNodeWithText("Provider").assertIsNotSelected().assertHasClickAction().performClick()
        composeRule.onNodeWithText("Provider").assertIsSelected().assertHeightIsAtLeast(48.dp)
        saveScreenshot(composeRule.onNodeWithTag("settings-sections").captureToImage(), "mediahub-segmented-control.png")
    }

    @Test
    fun tabRowExposesSelectionAndMinimumTouchHeight() {
        var selected by mutableStateOf("movies")
        composeRule.setContent {
            MediaHubTheme {
                MediaHubTabRow(
                    options = listOf("movies" to "电影", "shows" to "电视剧"),
                    selected = selected,
                    onSelected = { selected = it },
                    modifier = Modifier.testTag("library-tabs"),
                )
            }
        }

        composeRule.onNodeWithText("电影").assertIsSelected().assertHeightIsAtLeast(48.dp)
        composeRule.onNodeWithText("电视剧").assertIsNotSelected().assertHasClickAction().performClick()
        composeRule.onNodeWithText("电视剧").assertIsSelected().assertHeightIsAtLeast(48.dp)
        saveScreenshot(composeRule.onNodeWithTag("library-tabs").captureToImage(), "mediahub-tab-row.png")
    }

    @Test
    fun overflowMenuExposesActionsAndMinimumTouchHeight() {
        var imported = false
        composeRule.setContent {
            MediaHubTheme {
                MediaHubOverflowMenu(
                    expanded = true,
                    onExpandedChange = {},
                    contentDescription = "更多订阅操作",
                    actions = listOf(
                        MediaHubMenuAction("导入订阅") { imported = true },
                        MediaHubMenuAction("导出订阅") {},
                    ),
                    modifier = Modifier.testTag("subscription-overflow"),
                )
            }
        }

        composeRule.onNodeWithContentDescription("更多订阅操作").assertHasClickAction().assertHeightIsAtLeast(48.dp)
        composeRule.onNodeWithText("导入订阅").assertHasClickAction().performClick()
        composeRule.runOnIdle { assertTrue(imported) }
    }

    @Test
    fun controlsRemainUsableAtTwoHundredPercentFontScale() {
        var clicks = 0
        composeRule.setContent {
            val density = LocalDensity.current
            CompositionLocalProvider(LocalDensity provides Density(density.density, fontScale = 2f)) {
                MediaHubTheme {
                    MediaHubButton(
                        label = "保存设置",
                        onClick = { clicks += 1 },
                        modifier = Modifier.testTag("primary-action"),
                    )
                }
            }
        }

        val button = composeRule.onNodeWithTag("primary-action")
        button.assertHasClickAction().assertHeightIsAtLeast(48.dp).performClick()
        composeRule.runOnIdle { assertTrue(clicks == 1) }
        saveScreenshot(button.captureToImage(), "mediahub-large-font.png")
    }
    private fun saveScreenshot(image: ImageBitmap, name: String) {
        assertTrue(image.width > 0 && image.height > 0)
        image.asAndroidBitmap().writeToTestStorage(name.removeSuffix(".png"))
    }

}
