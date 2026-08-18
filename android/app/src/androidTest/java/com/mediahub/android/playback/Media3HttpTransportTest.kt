package com.mediahub.android.playback

import android.content.Context
import androidx.media3.common.MediaItem
import androidx.media3.common.C
import androidx.media3.common.Player
import androidx.media3.datasource.DefaultDataSource
import androidx.media3.datasource.DefaultHttpDataSource
import androidx.media3.exoplayer.ExoPlayer
import androidx.media3.exoplayer.source.DefaultMediaSourceFactory
import androidx.test.core.app.ApplicationProvider
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith

@RunWith(AndroidJUnit4::class)
class Media3HttpTransportTest {
    @Test
    fun seekUsesRangeAndPreservesConfiguredUserAgent() {
        val context = ApplicationProvider.getApplicationContext<Context>()
        val instrumentation = InstrumentationRegistry.getInstrumentation()
        val media = instrumentation.context.assets.open("media3-fixture.mp4").use { it.readBytes() }
        ThrottledRangeServer(media).use { server ->
            val ready = CountDownLatch(1)
            lateinit var player: ExoPlayer
            InstrumentationRegistry.getInstrumentation().runOnMainSync {
                val http = DefaultHttpDataSource.Factory()
                    .setUserAgent(TEST_USER_AGENT)
                    .setAllowCrossProtocolRedirects(false)
                player = ExoPlayer.Builder(context)
                    .setMediaSourceFactory(DefaultMediaSourceFactory(DefaultDataSource.Factory(context, http)))
                    .build()
                player.addListener(object : Player.Listener {
                    override fun onPlaybackStateChanged(playbackState: Int) {
                        if (playbackState == Player.STATE_READY) ready.countDown()
                    }
                })
                player.setMediaItem(MediaItem.fromUri(server.url))
                player.prepare()
            }
            try {
                assertTrue("fixture never became ready", ready.await(15, TimeUnit.SECONDS))
                InstrumentationRegistry.getInstrumentation().runOnMainSync {
                    val groups = player.currentTracks.groups
                    val audioTracks = groups.filter { it.type == C.TRACK_TYPE_AUDIO }.sumOf { it.length }
                    val textTracks = groups.filter { it.type == C.TRACK_TYPE_TEXT }.sumOf { it.length }
                    assertTrue("expected two embedded audio tracks, got $audioTracks", audioTracks >= 2)
                    assertTrue("expected an embedded subtitle track, got $textTracks", textTracks >= 1)
                }
                InstrumentationRegistry.getInstrumentation().runOnMainSync {
                    player.seekTo(20_000)
                }
                assertTrue("seek did not issue a positive Range request", server.positiveRange.await(15, TimeUnit.SECONDS))
                val requests = synchronized(server.requests) { server.requests.toList() }
                assertTrue(requests.isNotEmpty())
                assertTrue(requests.all { it.userAgent == TEST_USER_AGENT })
                assertTrue(requests.any { request ->
                    request.range?.removePrefix("bytes=")?.substringBefore('-')?.toLongOrNull()?.let { it > 0 } == true
                })
            } finally {
                InstrumentationRegistry.getInstrumentation().runOnMainSync { player.release() }
                server.assertNoUnexpectedFailures()
            }
        }
    }

    private companion object {
        const val TEST_USER_AGENT = "Media-Hub-Media3-Test/1"
    }
}
