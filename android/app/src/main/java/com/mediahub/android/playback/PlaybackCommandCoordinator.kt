package com.mediahub.android.playback

import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.launch

internal interface PlaybackCommandHost {
    val currentMediaId: String?
    val hasMediaItem: Boolean
    val positionMs: Long
    val playWhenReady: Boolean

    suspend fun resolve(request: PlaybackRequest): PlaybackDescriptor
    suspend fun stopSessionAndFlush()
    fun stopSession()
    fun pause()
    fun clearMedia()
    fun applyDescriptor(request: PlaybackRequest, descriptor: PlaybackDescriptor, positionMs: Long, autoPlay: Boolean)
    fun publishLoading()
    fun publishError(message: String)
    fun stopService()
}

internal class PlaybackServerChangedException : Exception()

internal class PlaybackCommandCoordinator(
    private val scope: CoroutineScope,
    private val host: PlaybackCommandHost,
) {
    private val recovery = PlaybackRecoveryCoordinator()
    private var currentRequest: PlaybackRequest? = null
    private var commandGeneration = 0L
    private var commandJob: Job? = null
    private var refreshing = false

    fun submitPlay(request: PlaybackRequest, force: Boolean) {
        launchLatest { command ->
            val sameMedia = currentRequest?.mediaId == request.mediaId && host.hasMediaItem
            if (sameMedia && !force) return@launchLatest
            host.stopSessionAndFlush()
            if (!isCurrent(command)) return@launchLatest
            host.clearMedia()
            currentRequest = request
            recovery.onReady()
            resolveAndApply(request, 0L, autoPlay = true, refresh = force, command = command)
        }
    }

    fun submitInvalidate() {
        launchLatest { command ->
            host.stopSessionAndFlush()
            if (!isCurrent(command)) return@launchLatest
            currentRequest = null
            refreshing = false
            host.clearMedia()
            host.publishLoading()
            host.stopService()
        }
    }

    fun onReady() {
        recovery.onReady()
    }

    fun onPlayerError(responseCode: Int?) {
        val request = currentRequest ?: return
        if (host.currentMediaId != request.mediaId || refreshing) return
        val action = recovery.recover(
            responseCode = responseCode,
            request = request,
            positionMs = host.positionMs,
            autoPlay = host.playWhenReady,
        )
        if (action == null) {
            host.pause()
            host.stopSession()
            host.publishError("视频连接中断")
            return
        }
        launchLatest { command ->
            host.stopSessionAndFlush()
            if (!isCurrent(command) || currentRequest?.mediaId != action.request.mediaId) return@launchLatest
            resolveAndApply(
                action.request,
                action.positionMs,
                action.autoPlay,
                refresh = true,
                command = command,
            )
        }
    }

    fun close() {
        commandGeneration++
        commandJob?.cancel()
        commandJob = null
    }

    private fun launchLatest(block: suspend (Long) -> Unit) {
        val command = ++commandGeneration
        commandJob?.cancel()
        commandJob = scope.launch { block(command) }
    }

    private suspend fun resolveAndApply(
        request: PlaybackRequest,
        positionMs: Long,
        autoPlay: Boolean,
        refresh: Boolean,
        command: Long,
    ) {
        if (!isCurrent(command) || currentRequest?.mediaId != request.mediaId) return
        refreshing = refresh
        host.publishLoading()
        try {
            val descriptor = host.resolve(request)
            if (!isCurrent(command) || currentRequest?.mediaId != request.mediaId) return
            host.applyDescriptor(request, descriptor, positionMs, autoPlay)
        } catch (error: CancellationException) {
            throw error
        } catch (_: PlaybackServerChangedException) {
            if (isCurrent(command) && currentRequest?.mediaId == request.mediaId) {
                currentRequest = null
                host.clearMedia()
                host.publishError("服务器已切换，请返回后重新选择视频")
            }
        } catch (_: Exception) {
            if (isCurrent(command) && currentRequest?.mediaId == request.mediaId) {
                host.publishError(if (refresh) "播放地址已失效，重新连接失败" else "暂时无法直接播放")
            }
        } finally {
            if (isCurrent(command)) refreshing = false
        }
    }

    private fun isCurrent(command: Long): Boolean = command == commandGeneration
}

internal data class PlaybackRecoveryAction(
    val request: PlaybackRequest,
    val positionMs: Long,
    val autoPlay: Boolean,
)

internal class PlaybackRecoveryCoordinator {
    private var acquired = false

    fun recover(
        responseCode: Int?,
        request: PlaybackRequest,
        positionMs: Long,
        autoPlay: Boolean,
    ): PlaybackRecoveryAction? {
        if (responseCode == null || acquired || !isRefreshableHttpStatus(responseCode)) return null
        acquired = true
        return PlaybackRecoveryAction(request, positionMs, autoPlay)
    }

    fun onReady() {
        acquired = false
    }
}

internal fun isRefreshableHttpStatus(responseCode: Int): Boolean = responseCode in setOf(401, 403, 404, 410)

internal fun matchesServerIdentity(request: PlaybackRequest, configuredIdentity: String): Boolean =
    request.serverIdentity == configuredIdentity
