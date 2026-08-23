package com.mediahub.android.playback

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class PlaybackRecoveryTest {
    @Test
    fun refreshesKnownExpiredHttpResponsesOnly() {
        listOf(401, 403, 404, 410, 416, 500, 502, 503, 504).forEach { assertTrue(isRefreshableHttpStatus(it)) }
        listOf(400, 429).forEach { assertFalse(isRefreshableHttpStatus(it)) }
    }

    @Test
    fun recoveryAllowsTwoRefreshesBeforeReady() {
        val coordinator = PlaybackRecoveryCoordinator()
        val request = PlaybackRequest(Drive115Target("10", "20"), "Movie.mkv", serverIdentity = "a".repeat(64))

        assertEquals(91_234L, coordinator.recover(403, request, positionMs = 91_234L, autoPlay = true)?.positionMs)
        assertEquals(0L, coordinator.recover(403, request, positionMs = 91_234L, autoPlay = true)?.positionMs)
        assertNull(coordinator.recover(403, request, positionMs = 1L, autoPlay = true))

        coordinator.onReady()
        assertEquals(2L, coordinator.recover(404, request, positionMs = 2L, autoPlay = false)?.positionMs)
    }

    @Test
    fun recoveryRestartsFromBeginningOnRangeFailure() {
        val coordinator = PlaybackRecoveryCoordinator()
        val request = PlaybackRequest(Drive115Target("10", "20"), "Movie.mkv", serverIdentity = "a".repeat(64))

        assertEquals(0L, coordinator.recover(416, request, positionMs = 91_234L, autoPlay = true)?.positionMs)
    }

    @Test
    fun recoveryAllowsOneRefreshWhenHttpStatusIsUnknown() {
        val coordinator = PlaybackRecoveryCoordinator()
        val request = PlaybackRequest(Drive115Target("10", "20"), "Movie.mkv", serverIdentity = "a".repeat(64))

        assertEquals(91_234L, coordinator.recover(null, request, positionMs = 91_234L, autoPlay = true)?.positionMs)
        assertEquals(0L, coordinator.recover(null, request, positionMs = 91_234L, autoPlay = true)?.positionMs)
        assertNull(coordinator.recover(null, request, positionMs = 1L, autoPlay = true))
    }

    @Test
    fun serverIdentityMustMatchCurrentConfiguration() {
        val request = PlaybackRequest(Drive115Target("10", "20"), "Movie.mkv", serverIdentity = "a".repeat(64))
        assertTrue(matchesServerIdentity(request, "a".repeat(64)))
        assertFalse(matchesServerIdentity(request, "b".repeat(64)))
    }
}
