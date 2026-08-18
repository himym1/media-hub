package com.mediahub.android.playback

import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.NonCancellable
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.withContext
import kotlinx.coroutines.yield
import org.junit.Assert.assertEquals
import org.junit.Test

class PlaybackCommandCoordinatorTest {
    @Test
    fun latestPlayWinsWhenOlderResolveReturnsLate() = runBlocking {
        val host = FakePlaybackCommandHost()
        val firstResult = CompletableDeferred<PlaybackDescriptor>()
        val secondResult = CompletableDeferred<PlaybackDescriptor>()
        host.resolveResults += firstResult
        host.resolveResults += secondResult
        val coordinator = PlaybackCommandCoordinator(this, host)
        val first = request("first")
        val second = request("second")

        coordinator.submitPlay(first, force = false)
        yield()
        coordinator.submitPlay(second, force = false)
        yield()
        secondResult.complete(descriptor("second"))
        yield()
        firstResult.complete(descriptor("first"))
        yield()

        assertEquals(listOf("second"), host.appliedMediaIds)
        coordinator.close()
    }

    @Test
    fun recoverableErrorStopsOldSessionBeforeResolvingReplacement() = runBlocking {
        val host = FakePlaybackCommandHost()
        val initialResult = CompletableDeferred<PlaybackDescriptor>()
        val refreshResult = CompletableDeferred<PlaybackDescriptor>()
        host.resolveResults += initialResult
        host.resolveResults += refreshResult
        val coordinator = PlaybackCommandCoordinator(this, host)
        val request = request("movie")

        coordinator.submitPlay(request, force = false)
        yield()
        initialResult.complete(descriptor("initial"))
        yield()
        coordinator.onReady()
        host.events.clear()
        host.positionMs = 91_234L
        host.playWhenReady = false

        coordinator.onPlayerError(403)
        yield()
        assertEquals(listOf("stop-session-flush", "loading", "resolve"), host.events)
        refreshResult.complete(descriptor("refresh"))
        yield()

        assertEquals(91_234L, host.appliedPositionMs)
        assertEquals(false, host.appliedAutoPlay)
        coordinator.close()
    }

    @Test
    fun fatalErrorPausesStopsSessionAndPublishesError() = runBlocking {
        val host = FakePlaybackCommandHost().apply {
            currentMediaId = "movie"
            hasMediaItem = true
        }
        val coordinator = PlaybackCommandCoordinator(this, host)
        val result = CompletableDeferred<PlaybackDescriptor>()
        host.resolveResults += result
        coordinator.submitPlay(request("movie"), force = false)
        yield()
        result.complete(descriptor("movie"))
        yield()
        host.events.clear()

        coordinator.onPlayerError(500)

        assertEquals(listOf("pause", "stop-session", "error:视频连接中断"), host.events)
        coordinator.close()
    }

    @Test
    fun invalidateFlushesAndClearsBeforeStoppingService() = runBlocking {
        val host = FakePlaybackCommandHost()
        val result = CompletableDeferred<PlaybackDescriptor>()
        host.resolveResults += result
        val coordinator = PlaybackCommandCoordinator(this, host)
        coordinator.submitPlay(request("movie"), force = false)
        yield()
        result.complete(descriptor("movie"))
        yield()
        host.events.clear()

        coordinator.submitInvalidate()
        yield()

        assertEquals(listOf("stop-session-flush", "clear", "loading", "stop-service"), host.events)
        coordinator.close()
    }

    private fun request(id: String) = PlaybackRequest(
        target = EmbyItemTarget(id),
        title = id,
        serverIdentity = "a".repeat(64),
    )

    private fun descriptor(title: String) = PlaybackDescriptor(
        streamUrl = "https://cdn.example/$title.mkv",
        userAgent = "player",
        title = title,
        expiresAt = null,
    )
}

private class FakePlaybackCommandHost : PlaybackCommandHost {
    val resolveResults = ArrayDeque<CompletableDeferred<PlaybackDescriptor>>()
    val events = mutableListOf<String>()
    val appliedMediaIds = mutableListOf<String>()
    override var currentMediaId: String? = null
    override var hasMediaItem: Boolean = false
    override var positionMs: Long = 0L
    override var playWhenReady: Boolean = true
    var appliedPositionMs = -1L
    var appliedAutoPlay = false

    override suspend fun resolve(request: PlaybackRequest): PlaybackDescriptor {
        events += "resolve"
        return withContext(NonCancellable) { resolveResults.removeFirst().await() }
    }

    override suspend fun stopSessionAndFlush() {
        events += "stop-session-flush"
    }

    override fun stopSession() {
        events += "stop-session"
    }

    override fun pause() {
        events += "pause"
        playWhenReady = false
    }

    override fun clearMedia() {
        events += "clear"
        currentMediaId = null
        hasMediaItem = false
    }

    override fun applyDescriptor(
        request: PlaybackRequest,
        descriptor: PlaybackDescriptor,
        positionMs: Long,
        autoPlay: Boolean,
    ) {
        events += "apply"
        currentMediaId = request.mediaId
        hasMediaItem = true
        appliedMediaIds += (request.target as EmbyItemTarget).itemId
        appliedPositionMs = positionMs
        appliedAutoPlay = autoPlay
    }

    override fun publishLoading() {
        events += "loading"
    }

    override fun publishError(message: String) {
        events += "error:$message"
    }

    override fun stopService() {
        events += "stop-service"
    }
}
