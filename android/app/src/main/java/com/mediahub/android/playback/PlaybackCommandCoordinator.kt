package com.mediahub.android.playback
import com.mediahub.android.core.network.ApiException
import java.io.IOException

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
    fun publishError(failure: PlaybackFailure)
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
            host.publishError(PlaybackFailure("视频连接中断", retryable = true))
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
                host.publishError(PlaybackFailure("服务器已切换，请返回后重新选择视频", retryable = false))
            }
        } catch (error: Exception) {
            if (isCurrent(command) && currentRequest?.mediaId == request.mediaId) {
                host.publishError(playbackResolutionFailure(error, refresh))
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

internal data class PlaybackFailure(val message: String, val retryable: Boolean)

internal fun playbackResolutionFailure(error: Exception, refresh: Boolean): PlaybackFailure = when {
    error is ApiException -> when (error.code) {
        "authentication_required" -> PlaybackFailure("登录已失效，请重新登录", false)
        "playback_source_not_configured" -> PlaybackFailure("直接播放尚未配置", false)
        "playback_source_unauthorized" -> PlaybackFailure("播放源授权已失效", false)
        "playable_media_not_found" -> PlaybackFailure("当前媒体无法直接播放", false)
        "direct_playback_unavailable" -> PlaybackFailure("当前媒体暂无直链", false)
        else -> PlaybackFailure(error.message.orEmpty().ifBlank { "暂时无法直接播放" }, true)
    }
    error is IOException -> PlaybackFailure("无法连接 Media Hub", true)
    refresh -> PlaybackFailure("播放地址已失效，重新连接失败", true)
    else -> PlaybackFailure("暂时无法直接播放", true)
}
