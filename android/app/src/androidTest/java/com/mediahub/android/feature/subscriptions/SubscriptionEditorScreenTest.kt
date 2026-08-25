package com.mediahub.android.feature.subscriptions

import androidx.compose.ui.test.assertCountEquals
import androidx.compose.ui.test.assertHasClickAction
import androidx.compose.ui.test.assertHeightIsAtLeast
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.assertIsSelected
import androidx.compose.ui.test.hasScrollToIndexAction
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.junit4.v2.createComposeRule
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.test.performScrollTo
import androidx.compose.ui.test.performScrollToNode
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
                        editor = SubscriptionEditorState(title = "验收影片", tmdbId = "100"),
                    ),
                    actions = SubscriptionEditorActions(
                        editorChanged = {},
                        back = {},
                        save = {},
                        toggle = {},
                        run = {},
                        delete = {},
                    ),
                )
            }
        }

        composeRule.onNodeWithText("TMDB ID").assertIsDisplayed()
        val list = composeRule.onNode(hasScrollToIndexAction())
        list.performScrollToNode(hasText("标准"))
        composeRule.onNodeWithText("标准").assertIsSelected().assertHeightIsAtLeast(48.dp)
        composeRule.onAllNodesWithText("原始标题").assertCountEquals(0)

        list.performScrollToNode(hasText("资源来源"))
        composeRule.onNodeWithText("资源来源").assertHasClickAction().assertHeightIsAtLeast(48.dp).performClick()
        val sourceHelp = "来源清单暂不可用；不选择时会搜索全部来源。"
        list.performScrollToNode(hasText(sourceHelp))
        composeRule.onNodeWithText(sourceHelp).assertIsDisplayed()

        list.performScrollToNode(hasText("高级规则与媒体身份"))
        composeRule.onNodeWithText("高级规则与媒体身份").performClick()
        list.performScrollToNode(hasText("原始标题"))
        composeRule.onNodeWithText("原始标题").assertIsDisplayed()
    }
}
