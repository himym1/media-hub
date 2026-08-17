package com.mediahub.android.playback

import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch

internal class PlaybackSessionTracker(
    private val scope: CoroutineScope,
    private val reporter: PlaybackSessionReporter,
    private val positionMs: () -> Long,
    private val paused: () -> Boolean,
) {
    private var sessionId: String? = null
    private var startedSessionId: String? = null
    private var progressJob: Job? = null

    fun attach(nextSessionId: String?) {
        if (sessionId == nextSessionId) return
        stop()
        sessionId = nextSessionId
        startedSessionId = null
    }

    fun onReady() {
        val current = sessionId ?: return
        if (startedSessionId == current) return
        startedSessionId = current
        reporter.enqueue(current, PlaybackSessionEvent.Started, positionMs(), paused())
        progressJob?.cancel()
        progressJob = scope.launch {
            while (isActive && sessionId == current) {
                delay(15_000)
                reporter.enqueue(current, PlaybackSessionEvent.Progress, positionMs(), paused())
            }
        }
    }

    fun onPlayingChanged() {
        val current = sessionId ?: return
        if (startedSessionId == current) {
            reporter.enqueue(current, PlaybackSessionEvent.Progress, positionMs(), paused())
        }
    }

    fun stop() {
        val current = detach() ?: return
        reporter.enqueue(current, PlaybackSessionEvent.Stopped, positionMs(), paused = true)
    }

    suspend fun stopAndFlush() {
        val current = detach() ?: return
        reporter.stopAndFlush(current, positionMs())
    }

    suspend fun closeAndFlush() {
        stopAndFlush()
        reporter.close()
    }

    fun closeBestEffort() {
        stop()
        reporter.close()
    }

    private fun detach(): String? {
        val current = sessionId ?: return null
        sessionId = null
        startedSessionId = null
        progressJob?.cancel()
        progressJob = null
        return current
    }
}
