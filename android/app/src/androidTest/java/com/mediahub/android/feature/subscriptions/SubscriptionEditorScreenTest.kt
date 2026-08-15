package com.mediahub.android.feature.subscriptions

import androidx.compose.ui.test.assertCountEquals
import androidx.compose.ui.test.assertHasClickAction
import androidx.compose.ui.test.assertHeightIsAtLeast
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsSelected
import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.unit.dp
import androidx.test.ext.junit.runners.AndroidJUnit4
import com.mediahub.android.core.designsystem.MediaHubTheme
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class SubscriptionEditorScreenTest {
    @get:Rule
    val composeRule = createComposeRule()

    @Test
    fun requiredIdentityIsVisibleAndAdvancedRulesStayCollapsed() {
        composeRule.setContent {
            MediaHubTheme {
                SubscriptionEditorScreen(
                    state = SubscriptionUiState(
                        editing = true,
                        editor = SubscriptionEditorState(title = "验收影片", tmdbId = "100"),
                    ),
                    onEditorChanged = {},
                    onBack = {},
                    onSave = {},
                    onToggle = {},
                    onRun = {},
                    onDelete = {},
                )
            }
        }

        composeRule.onNodeWithText("TMDB ID").assertIsDisplayed()
        composeRule.onNodeWithText("标准").assertIsSelected().assertHeightIsAtLeast(48.dp)
        composeRule.onAllNodesWithText("原始标题").assertCountEquals(0)

        composeRule.onNodeWithText("资源来源").performScrollTo().assertHasClickAction().assertHeightIsAtLeast(48.dp).performClick()
        composeRule.onNodeWithText("不勾选时搜索全部已配置来源；暂不可用来源会自动跳过。").assertIsDisplayed()

        composeRule.onNodeWithText("高级规则与媒体身份").performScrollTo().performClick()
        composeRule.onNodeWithText("原始标题").assertIsDisplayed()
    }
}
