package com.mediahub.android.playback
import androidx.media3.common.PlaybackException
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

    fun onPlayerError(error: PlaybackException) {
        handlePlayerError(playbackHttpStatus(error), error)
    }

    internal fun handlePlayerErrorForTest(responseCode: Int?) {
        handlePlayerError(responseCode, error = null)
    }

    private fun handlePlayerError(responseCode: Int?, error: PlaybackException?) {
        val request = currentRequest ?: return
        if (host.currentMediaId != request.mediaId || refreshing) return
        if (responseCode == null && error != null && isDecoderPlaybackError(error)) {
            host.pause()
            host.stopSession()
            host.publishError(playbackConnectionFailure(null, error))
            return
        }
        val action = recovery.recover(
            responseCode = responseCode,
            request = request,
            positionMs = host.positionMs,
            autoPlay = host.playWhenReady,
        )
        if (action == null) {
            host.pause()
            host.stopSession()
            host.publishError(playbackConnectionFailure(responseCode, error))
            return
        }
        launchLatest { command ->
            host.stopSessionAndFlush()
            if (!isCurrent(command) || currentRequest?.mediaId != action.request.mediaId) return@launchLatest
            host.clearMedia()
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
    private var refreshAttempts = 0

    fun recover(
        responseCode: Int?,
        request: PlaybackRequest,
        positionMs: Long,
        autoPlay: Boolean,
    ): PlaybackRecoveryAction? {
        if (refreshAttempts >= 2) return null
        if (responseCode != null && !isRefreshableHttpStatus(responseCode)) return null
        refreshAttempts++
        val restartPosition = when {
            responseCode == 416 -> 0L
            refreshAttempts == 2 && positionMs > 0L -> 0L
            else -> positionMs
        }
        return PlaybackRecoveryAction(request, restartPosition, autoPlay)
    }

    fun onReady() {
        refreshAttempts = 0
    }
}

internal fun isRefreshableHttpStatus(responseCode: Int): Boolean =
    responseCode in setOf(401, 403, 404, 410, 416, 500, 502, 503, 504)

internal fun playbackConnectionFailure(responseCode: Int?, error: PlaybackException? = null): PlaybackFailure {
    val suffix = when {
        responseCode != null -> " (HTTP $responseCode)"
        error != null -> playbackErrorDetail(error)
        else -> ""
    }
    return PlaybackFailure("视频连接中断$suffix", retryable = true)
}

internal fun playbackHttpStatus(error: PlaybackException): Int? {
    var cause: Throwable? = error
    while (cause != null) {
        if (cause is androidx.media3.datasource.HttpDataSource.InvalidResponseCodeException) {
            return cause.responseCode
        }
        cause = cause.cause
    }
    return null
}

internal fun isDecoderPlaybackError(error: PlaybackException): Boolean =
    error.errorCode in setOf(
        PlaybackException.ERROR_CODE_DECODER_INIT_FAILED,
        PlaybackException.ERROR_CODE_DECODING_FAILED,
        PlaybackException.ERROR_CODE_DECODER_QUERY_FAILED,
    )

internal fun playbackErrorDetail(error: PlaybackException): String = when (error.errorCode) {
    PlaybackException.ERROR_CODE_DECODER_INIT_FAILED,
    PlaybackException.ERROR_CODE_DECODING_FAILED,
    PlaybackException.ERROR_CODE_DECODER_QUERY_FAILED,
    -> "（设备无法解码此视频）"
    PlaybackException.ERROR_CODE_IO_NETWORK_CONNECTION_FAILED,
    PlaybackException.ERROR_CODE_IO_NETWORK_CONNECTION_TIMEOUT,
    -> "（网络连接失败）"
    else -> "（${error.errorCodeName}）"
}

internal fun matchesServerIdentity(request: PlaybackRequest, configuredIdentity: String): Boolean =
    request.serverIdentity == configuredIdentity

internal data class PlaybackFailure(val message: String, val retryable: Boolean)

internal fun playbackResolutionFailure(error: Exception, refresh: Boolean): PlaybackFailure = when {
    error is ApiException -> when (error.code) {
        "authentication_required" -> PlaybackFailure("登录已失效，请重新登录", false)
        "playback_source_not_configured" -> PlaybackFailure("直接播放尚未配置", false)
        "playback_source_unauthorized" -> PlaybackFailure("播放源授权已失效", false)
        "playable_media_not_found" -> PlaybackFailure("当前媒体无法直接播放", false)
        "direct_playback_unavailable" -> PlaybackFailure("暂时无法获取直链，请稍后重试", true)
        else -> PlaybackFailure(error.message.orEmpty().ifBlank { "暂时无法直接播放" }, true)
    }
    error is IOException -> PlaybackFailure("无法连接 Media Hub", true)
    refresh -> PlaybackFailure("播放地址已失效，重新连接失败", true)
    else -> PlaybackFailure("暂时无法直接播放", true)
}
