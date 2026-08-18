package com.mediahub.android.feature.player

import android.content.pm.ActivityInfo
import androidx.lifecycle.Lifecycle
import androidx.test.core.app.ActivityScenario
import androidx.test.core.app.ApplicationProvider
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.compose.ui.test.junit4.v2.createEmptyComposeRule
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.performClick
import com.mediahub.android.MediaHubApplication
import com.mediahub.android.playback.Drive115Target
import com.mediahub.android.playback.PlaybackRequest
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class PlayerActivityTest {
    @get:Rule
    val composeRule = createEmptyComposeRule()

    private var scenario: ActivityScenario<PlayerActivity>? = null

    @After
    fun tearDown() {
        scenario?.close()
        ApplicationProvider.getApplicationContext<MediaHubApplication>().container.clearConfiguration()
    }

    @Test
    fun rotationCommandAndSystemBackUseActivityLifecycle() {
        val application = ApplicationProvider.getApplicationContext<MediaHubApplication>()
        val dependencies = application.container.configureServer("http://127.0.0.1:9")
        val request = PlaybackRequest(
            target = Drive115Target("10", "20"),
            title = "Activity lifecycle movie",
            serverIdentity = dependencies.serverIdentity,
        )
        scenario = ActivityScenario.launch(PlayerActivity.intent(application, request))
        composeRule.waitForIdle()

        composeRule.onNodeWithContentDescription("旋转屏幕").performClick()
        scenario?.onActivity { activity ->
            assertEquals(ActivityInfo.SCREEN_ORIENTATION_SENSOR_LANDSCAPE, activity.requestedOrientation)
            activity.onBackPressedDispatcher.onBackPressed()
        }
        composeRule.waitForIdle()

        assertTrue(scenario?.state == Lifecycle.State.DESTROYED)
    }
}
