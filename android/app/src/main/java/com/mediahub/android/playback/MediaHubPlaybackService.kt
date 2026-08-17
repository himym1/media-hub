package com.mediahub.android.playback

import android.content.Context
import android.content.Intent
import android.os.Bundle
import androidx.core.net.toUri
import androidx.media3.common.MediaItem
import androidx.media3.common.PlaybackException
import androidx.media3.common.Player
import androidx.media3.common.util.UnstableApi
import androidx.media3.datasource.DefaultDataSource
import androidx.media3.datasource.DefaultHttpDataSource
import androidx.media3.datasource.HttpDataSource
import androidx.media3.exoplayer.ExoPlayer
import androidx.media3.exoplayer.source.DefaultMediaSourceFactory
import androidx.media3.session.MediaSession
import androidx.media3.session.MediaSessionService
import com.mediahub.android.MediaHubApplication
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.Job
import kotlinx.coroutines.cancel
import kotlinx.coroutines.launch

@androidx.annotation.OptIn(UnstableApi::class)
class MediaHubPlaybackService : MediaSessionService() {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main.immediate)
    private lateinit var player: ExoPlayer
    private lateinit var mediaSession: MediaSession
    private lateinit var httpFactory: DefaultHttpDataSource.Factory
    private lateinit var sessionReporter: PlaybackSessionReporter
    private lateinit var sessionTracker: PlaybackSessionTracker
    private var currentRequest: PlaybackRequest? = null
    private var refreshing = false
    private val recoveryCoordinator = PlaybackRecoveryCoordinator()
    private var resolveGeneration = 0L
    private var commandGeneration = 0L
    private var commandJob: Job? = null

    override fun onCreate() {
        super.onCreate()
        sessionReporter = PlaybackSessionReporter {
            (application as MediaHubApplication).container.requireConfigured().playbackRepository
        }
        httpFactory = DefaultHttpDataSource.Factory().setAllowCrossProtocolRedirects(false)
        player = ExoPlayer.Builder(this)
            .setMediaSourceFactory(DefaultMediaSourceFactory(DefaultDataSource.Factory(this, httpFactory)))
            .build()
        sessionTracker = PlaybackSessionTracker(
            scope = scope,
            reporter = sessionReporter,
            positionMs = { player.currentPosition.coerceAtLeast(0L) },
            paused = { !player.isPlaying },
        )
        player.addListener(object : Player.Listener {
            override fun onPlaybackStateChanged(playbackState: Int) {
                if (playbackState == Player.STATE_READY) {
                    recoveryCoordinator.onReady()
                    publishState(STATE_READY, null)
                    sessionTracker.onReady()
                } else if (playbackState == Player.STATE_ENDED) {
                    sessionTracker.stop()
                }
            }

            override fun onPlayerError(error: PlaybackException) {
                val request = currentRequest ?: return
                if (player.currentMediaItem?.mediaId != request.mediaId || refreshing) return
                val action = recoveryCoordinator.recover(
                    responseCode = playbackHttpStatus(error),
                    request = request,
                    positionMs = player.currentPosition.coerceAtLeast(0L),
                    autoPlay = player.playWhenReady,
                )
                if (action == null) {
                    publishState(STATE_ERROR, "视频连接中断")
                    return
                }
                val command = ++commandGeneration
                commandJob?.cancel()
                commandJob = scope.launch {
                    if (command != commandGeneration || currentRequest?.mediaId != action.request.mediaId) return@launch
                    resolveAndPlay(
                        action.request, action.positionMs, action.autoPlay, refresh = true, command = command,
                    )
                }
            }

            override fun onIsPlayingChanged(isPlaying: Boolean) {
                sessionTracker.onPlayingChanged()
            }
        })
        mediaSession = MediaSession.Builder(this, player)
            .setSessionExtras(stateExtras(STATE_LOADING, null))
            .build()
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_INVALIDATE -> {
                val command = ++commandGeneration
                commandJob?.cancel()
                commandJob = scope.launch {
                    if (command != commandGeneration) return@launch
                    sessionTracker.stopAndFlush()
                    if (command != commandGeneration) return@launch
                    invalidatePlayback()
                    stopSelf()
                }
            }
            ACTION_PLAY -> {
                val request = PlaybackRequestIntentCodec.read(intent)
                if (request != null) {
                    val force = intent.getBooleanExtra(EXTRA_FORCE, false)
                    val command = ++commandGeneration
                    commandJob?.cancel()
                    commandJob = scope.launch {
                        if (command != commandGeneration) return@launch
                        val sameMedia = currentRequest?.mediaId == request.mediaId && player.currentMediaItem != null
                        if (sameMedia && !force) return@launch
                        sessionTracker.stopAndFlush()
                        if (command != commandGeneration) return@launch
                        player.stop()
                        player.clearMediaItems()
                        currentRequest = request
                        recoveryCoordinator.onReady()
                        resolveAndPlay(request, 0L, true, refresh = force, command = command)
                    }
                }
            }
        }
        return super.onStartCommand(intent, flags, startId)
    }

    override fun onGetSession(controllerInfo: MediaSession.ControllerInfo): MediaSession = mediaSession

    override fun onDestroy() {
        commandJob?.cancel()
        sessionTracker.closeBestEffort()
        scope.cancel()
        mediaSession.release()
        player.release()
        super.onDestroy()
    }

    private fun invalidatePlayback() {
        resolveGeneration++
        refreshing = false
        currentRequest = null
        player.stop()
        player.clearMediaItems()
        publishState(STATE_LOADING, null)
    }

    private suspend fun resolveAndPlay(
        request: PlaybackRequest,
        positionMs: Long,
        autoPlay: Boolean,
        refresh: Boolean,
        command: Long,
    ) {
        if (command != commandGeneration) return
        val generation = ++resolveGeneration
        refreshing = refresh
        publishState(STATE_LOADING, null)
        try {
            val dependencies = (application as MediaHubApplication).container.requireConfigured()
            if (!matchesServerIdentity(request, dependencies.serverIdentity)) {
                currentRequest = null
                player.stop()
                player.clearMediaItems()
                publishState(STATE_ERROR, "服务器已切换，请返回后重新选择视频")
                return
            }
            val descriptor = dependencies.playbackRepository.createDescriptor(request)
            if (command != commandGeneration || generation != resolveGeneration || currentRequest?.mediaId != request.mediaId) return
            play(request, descriptor, positionMs, autoPlay)
        } catch (_: Exception) {
            if (command == commandGeneration && generation == resolveGeneration && currentRequest?.mediaId == request.mediaId) {
                publishState(STATE_ERROR, if (refresh) "播放地址已失效，重新连接失败" else "暂时无法直接播放")
            }
        } finally {
            if (generation == resolveGeneration) refreshing = false
        }
    }

    private fun play(request: PlaybackRequest, descriptor: PlaybackDescriptor, positionMs: Long, autoPlay: Boolean) {
        currentRequest = request
        sessionTracker.attach(descriptor.sessionId)
        httpFactory.setUserAgent(descriptor.userAgent)
        val item = MediaItem.Builder()
            .setMediaId(request.mediaId)
            .setUri(descriptor.streamUrl.toUri())
            .setMediaMetadata(androidx.media3.common.MediaMetadata.Builder().setTitle(descriptor.title).build())
            .build()
        val startPositionMs = positionMs.takeIf { it > 0L } ?: descriptor.startPositionMs
        player.setMediaItem(item, startPositionMs)
        player.prepare()
        player.playWhenReady = autoPlay
    }


    private fun publishState(state: String, message: String?) {
        mediaSession.setSessionExtras(stateExtras(state, message))
    }

    private fun stateExtras(state: String, message: String?): Bundle = Bundle().apply {
        putString(EXTRA_STATE, state)
        if (message != null) putString(EXTRA_MESSAGE, message)
    }

    companion object {
        const val EXTRA_STATE = "state"
        const val EXTRA_MESSAGE = "message"
        const val STATE_LOADING = "loading"
        const val STATE_READY = "ready"
        const val STATE_ERROR = "error"

        private const val ACTION_PLAY = "com.mediahub.android.action.PLAY_115"
        private const val ACTION_INVALIDATE = "com.mediahub.android.action.INVALIDATE_PLAYBACK"
        private const val EXTRA_FORCE = "force"

        fun invalidateIntent(context: Context): Intent =
            Intent(context, MediaHubPlaybackService::class.java).setAction(ACTION_INVALIDATE)

        fun playIntent(context: Context, request: PlaybackRequest, force: Boolean = false): Intent =
            PlaybackRequestIntentCodec.put(
                Intent(context, MediaHubPlaybackService::class.java).setAction(ACTION_PLAY),
                request,
            ).putExtra(EXTRA_FORCE, force)
    }
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

private fun playbackHttpStatus(error: PlaybackException): Int? {
    var cause: Throwable? = error
    while (cause != null) {
        if (cause is HttpDataSource.InvalidResponseCodeException) return cause.responseCode
        cause = cause.cause
    }
    return null
}


internal fun matchesServerIdentity(request: PlaybackRequest, configuredIdentity: String): Boolean =
    request.serverIdentity == configuredIdentity
