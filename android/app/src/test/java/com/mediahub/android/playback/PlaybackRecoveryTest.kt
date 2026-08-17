package com.mediahub.android.playback

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertSame
import org.junit.Assert.assertTrue
import org.junit.Test

class PlaybackRecoveryTest {
    @Test
    fun refreshesKnownExpiredHttpResponsesOnly() {
        listOf(401, 403, 404, 410).forEach { assertTrue(isRefreshableHttpStatus(it)) }
        listOf(400, 416, 429, 500).forEach { assertFalse(isRefreshableHttpStatus(it)) }
    }

    @Test
    fun recoveryPreservesTargetPositionAndAllowsOnlyOneAttemptUntilReady() {
        val coordinator = PlaybackRecoveryCoordinator()
        val request = PlaybackRequest(Drive115Target("10", "20"), "Movie.mkv", serverIdentity = "a".repeat(64))

        val action = coordinator.recover(403, request, positionMs = 91_234L, autoPlay = true)
        assertSame(request, action?.request)
        assertEquals(91_234L, action?.positionMs)
        assertTrue(action?.autoPlay == true)
        assertNull(coordinator.recover(410, request, positionMs = 1L, autoPlay = true))

        coordinator.onReady()
        assertEquals(2L, coordinator.recover(404, request, positionMs = 2L, autoPlay = false)?.positionMs)
    }

    @Test
    fun serverIdentityMustMatchCurrentConfiguration() {
        val request = PlaybackRequest(Drive115Target("10", "20"), "Movie.mkv", serverIdentity = "a".repeat(64))
        assertTrue(matchesServerIdentity(request, "a".repeat(64)))
        assertFalse(matchesServerIdentity(request, "b".repeat(64)))
    }
}
