package com.mediahub.android.playback

import kotlinx.coroutines.runBlocking
import org.junit.Assert.assertEquals
import org.junit.Test

class PlaybackSessionReporterTest {
    @Test
    fun reportsStartedProgressAndStoppedInOrder() = runBlocking {
        val events = mutableListOf<PlaybackSessionEvent>()
        val repository = object : PlaybackRepository {
            override suspend fun createDescriptor(request: PlaybackRequest): PlaybackDescriptor = error("not used")
            override suspend fun reportSession(sessionId: String, event: PlaybackSessionEvent, positionMs: Long, paused: Boolean) {
                synchronized(events) { events += event }
            }
        }
        val reporter = PlaybackSessionReporter { repository }
        val sessionId = "a".repeat(48)
        reporter.enqueue(sessionId, PlaybackSessionEvent.Started, 0, paused = false)
        reporter.enqueue(sessionId, PlaybackSessionEvent.Progress, 15_000, paused = false)
        reporter.stopAndFlush(sessionId, 30_000)
        reporter.close()

        assertEquals(
            listOf(PlaybackSessionEvent.Started, PlaybackSessionEvent.Progress, PlaybackSessionEvent.Stopped),
            synchronized(events) { events.toList() },
        )
    }
}
