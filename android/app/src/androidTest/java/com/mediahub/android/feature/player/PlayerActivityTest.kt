package com.mediahub.android.feature.player

import android.content.pm.ActivityInfo
import android.content.ComponentName
import android.content.pm.PackageManager
import androidx.lifecycle.Lifecycle
import androidx.media3.common.MediaItem
import androidx.media3.common.Player
import androidx.media3.session.MediaController
import androidx.media3.session.SessionToken
import androidx.test.core.app.ActivityScenario
import androidx.test.core.app.ApplicationProvider
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import androidx.compose.ui.test.junit4.v2.createEmptyComposeRule
import androidx.compose.ui.test.onNodeWithContentDescription
import androidx.compose.ui.test.performClick
import com.mediahub.android.MediaHubApplication
import com.mediahub.android.playback.Drive115Target
import com.mediahub.android.playback.PlaybackRequest
import com.mediahub.android.playback.MediaHubPlaybackService
import com.mediahub.android.playback.ThrottledRangeServer
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Assume.assumeTrue
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

        composeRule.waitUntil(timeoutMillis = 5_000) { scenario?.state == Lifecycle.State.DESTROYED }
        assertTrue(scenario?.state == Lifecycle.State.DESTROYED)
    }

    @Test
    fun readyMediaEntersRealPictureInPictureAndHidesChrome() {
        val application = ApplicationProvider.getApplicationContext<MediaHubApplication>()
        assumeTrue(application.packageManager.hasSystemFeature(PackageManager.FEATURE_PICTURE_IN_PICTURE))
        val dependencies = application.container.configureServer("http://127.0.0.1:9")
        val request = PlaybackRequest(
            target = Drive115Target("10", "20"),
            title = "Picture in picture movie",
            serverIdentity = dependencies.serverIdentity,
        )
        scenario = ActivityScenario.launch(PlayerActivity.intent(application, request))
        composeRule.waitForIdle()

        val instrumentation = InstrumentationRegistry.getInstrumentation()
        val media = instrumentation.context.assets.open("media3-fixture.mp4").use { it.readBytes() }
        ThrottledRangeServer(media).use { server ->
            val token = SessionToken(application, ComponentName(application, MediaHubPlaybackService::class.java))
            val future = MediaController.Builder(application, token).buildAsync()
            val controller = future.get(10, TimeUnit.SECONDS)
            val ready = CountDownLatch(1)
            instrumentation.runOnMainSync {
                controller.addListener(object : Player.Listener {
                    override fun onPlaybackStateChanged(playbackState: Int) {
                        if (playbackState == Player.STATE_READY) ready.countDown()
                    }
                })
                controller.setMediaItem(MediaItem.fromUri(server.url))
                controller.prepare()
            }
            try {
                assertTrue("service player never became ready", ready.await(15, TimeUnit.SECONDS))
                composeRule.onNodeWithContentDescription("进入画中画").performClick()
                instrumentation.waitForIdleSync()
                scenario?.onActivity { activity -> assertTrue(activity.isInPictureInPictureMode) }
            } finally {
                instrumentation.runOnMainSync { controller.release() }
                server.assertNoUnexpectedFailures()
            }
        }
    }
}
