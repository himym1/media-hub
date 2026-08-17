package com.mediahub.android.playback

import org.junit.Assert.assertEquals
import org.junit.Assert.fail
import org.junit.Test

class PlaybackDescriptorTest {
    @Test
    fun parsesSafeTemporaryDescription() {
        val value = parsePlaybackDescriptor(
            """{"streamUrl":"https://cdn.example/video.mkv?token=short","userAgent":"MediaHub Player","title":"Movie.mkv","startPositionMs":42000}""",
        )
        assertEquals("https://cdn.example/video.mkv?token=short", value.streamUrl)
        assertEquals("MediaHub Player", value.userAgent)
        assertEquals(null, value.expiresAt)
        assertEquals(42_000L, value.startPositionMs)
    }

    @Test
    fun rejectsUnsafePlaybackUrl() {
        try {
            parsePlaybackDescriptor(
                """{"streamUrl":"http://cdn.example/video.mkv","userAgent":"MediaHub Player","title":"Movie.mkv"}""",
            )
            fail("unsafe playback URL was accepted")
        } catch (_: IllegalArgumentException) {
            // Expected.
        }
    }
}
