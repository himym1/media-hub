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
import kotlinx.coroutines.cancel

@androidx.annotation.OptIn(UnstableApi::class)
class MediaHubPlaybackService : MediaSessionService() {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.Main.immediate)
    private lateinit var player: ExoPlayer
    private lateinit var mediaSession: MediaSession
    private lateinit var httpFactory: DefaultHttpDataSource.Factory
    private lateinit var sessionReporter: PlaybackSessionReporter
    private lateinit var sessionTracker: PlaybackSessionTracker
    private lateinit var commandCoordinator: PlaybackCommandCoordinator

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
        commandCoordinator = PlaybackCommandCoordinator(
            scope = scope,
            host = object : PlaybackCommandHost {
                override val currentMediaId: String? get() = player.currentMediaItem?.mediaId
                override val hasMediaItem: Boolean get() = player.currentMediaItem != null
                override val positionMs: Long get() = player.currentPosition.coerceAtLeast(0L)
                override val playWhenReady: Boolean get() = player.playWhenReady

                override suspend fun resolve(request: PlaybackRequest): PlaybackDescriptor {
                    val dependencies = (application as MediaHubApplication).container.requireConfigured()
                    if (!matchesServerIdentity(request, dependencies.serverIdentity)) {
                        throw PlaybackServerChangedException()
                    }
                    return dependencies.playbackRepository.createDescriptor(request)
                }

                override suspend fun stopSessionAndFlush() = sessionTracker.stopAndFlush()
                override fun stopSession() = sessionTracker.stop()
                override fun pause() { player.playWhenReady = false }
                override fun clearMedia() {
                    player.stop()
                    player.clearMediaItems()
                }
                override fun applyDescriptor(
                    request: PlaybackRequest,
                    descriptor: PlaybackDescriptor,
                    positionMs: Long,
                    autoPlay: Boolean,
                ) = play(request, descriptor, positionMs, autoPlay)
                override fun publishLoading() = publishState(STATE_LOADING, null)
                override fun publishError(failure: PlaybackFailure) =
                    publishState(STATE_ERROR, failure.message, failure.retryable)
                override fun stopService() = stopSelf()
            },
        )
        player.addListener(object : Player.Listener {
            override fun onPlaybackStateChanged(playbackState: Int) {
                if (playbackState == Player.STATE_READY) {
                    commandCoordinator.onReady()
                    publishState(STATE_READY, null)
                    sessionTracker.onReady()
                } else if (playbackState == Player.STATE_ENDED) {
                    sessionTracker.stop()
                }
            }

            override fun onPlayerError(error: PlaybackException) {
                commandCoordinator.onPlayerError(playbackHttpStatus(error))
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
            ACTION_INVALIDATE -> commandCoordinator.submitInvalidate()
            ACTION_PLAY -> PlaybackRequestIntentCodec.read(intent)?.let { request ->
                commandCoordinator.submitPlay(request, intent.getBooleanExtra(EXTRA_FORCE, false))
            }
        }
        return super.onStartCommand(intent, flags, startId)
    }

    override fun onGetSession(controllerInfo: MediaSession.ControllerInfo): MediaSession = mediaSession

    override fun onDestroy() {
        commandCoordinator.close()
        sessionTracker.closeBestEffort()
        scope.cancel()
        mediaSession.release()
        player.release()
        super.onDestroy()
    }


    private fun play(request: PlaybackRequest, descriptor: PlaybackDescriptor, positionMs: Long, autoPlay: Boolean) {
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


    private fun publishState(state: String, message: String?, retryable: Boolean = false) {
        mediaSession.setSessionExtras(stateExtras(state, message, retryable))
    }

    private fun stateExtras(state: String, message: String?, retryable: Boolean = false): Bundle = Bundle().apply {
        putString(EXTRA_STATE, state)
        putBoolean(EXTRA_RETRYABLE, retryable)
        if (message != null) putString(EXTRA_MESSAGE, message)
    }

    companion object {
        const val EXTRA_STATE = "state"
        const val EXTRA_MESSAGE = "message"
        const val EXTRA_RETRYABLE = "retryable"
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


private fun playbackHttpStatus(error: PlaybackException): Int? {
    var cause: Throwable? = error
    while (cause != null) {
        if (cause is HttpDataSource.InvalidResponseCodeException) return cause.responseCode
        cause = cause.cause
    }
    return null
}
