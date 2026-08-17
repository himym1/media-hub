package com.mediahub.android.playback

import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.launch
import kotlinx.coroutines.withTimeoutOrNull

internal data class PlaybackSessionReport(
    val sessionId: String,
    val event: PlaybackSessionEvent,
    val positionMs: Long,
    val paused: Boolean,
    val completion: CompletableDeferred<Unit>? = null,
)

internal class PlaybackSessionReporter(
    private val repository: () -> PlaybackRepository,
) {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private val reports = Channel<PlaybackSessionReport>(Channel.UNLIMITED)

    private val worker = scope.launch {
        for (report in reports) {
            runCatching {
                repository().reportSession(report.sessionId, report.event, report.positionMs, report.paused)
            }
            report.completion?.complete(Unit)
        }
    }

    fun enqueue(sessionId: String, event: PlaybackSessionEvent, positionMs: Long, paused: Boolean) {
        reports.trySend(PlaybackSessionReport(sessionId, event, positionMs.coerceAtLeast(0L), paused))
    }

    suspend fun stopAndFlush(sessionId: String, positionMs: Long, timeoutMs: Long = 2_000) {
        val completion = CompletableDeferred<Unit>()
        reports.send(
            PlaybackSessionReport(
                sessionId = sessionId,
                event = PlaybackSessionEvent.Stopped,
                positionMs = positionMs.coerceAtLeast(0L),
                paused = true,
                completion = completion,
            ),
        )
        withTimeoutOrNull(timeoutMs) { completion.await() }
    }

    fun close() {
        reports.close()
        worker.invokeOnCompletion { scope.cancel() }
    }
}
