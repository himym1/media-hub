package com.mediahub.android.feature.player

import android.app.PictureInPictureParams
import android.content.ComponentName
import android.content.Context
import android.content.Intent
import android.content.pm.ActivityInfo
import android.content.res.Configuration
import android.graphics.Rect
import android.os.Build
import android.os.Bundle
import android.util.Rational
import android.view.WindowManager
import androidx.activity.ComponentActivity
import androidx.activity.OnBackPressedCallback
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.core.content.ContextCompat
import androidx.core.view.WindowCompat
import androidx.core.view.WindowInsetsCompat
import androidx.core.view.WindowInsetsControllerCompat
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.media3.common.Player
import androidx.media3.session.MediaController
import androidx.media3.session.SessionToken
import com.mediahub.android.MediaHubApplication
import com.mediahub.android.app.MediaHubViewModelFactory
import com.mediahub.android.app.ProvideWindowAdaptive
import com.mediahub.android.core.designsystem.MediaHubTheme
import com.mediahub.android.feature.subtitles.RemoteSubtitleUiState
import com.mediahub.android.feature.subtitles.RemoteSubtitleViewModel
import com.mediahub.android.playback.EmbyItemTarget
import com.mediahub.android.playback.MediaHubPlaybackService
import com.mediahub.android.playback.PlaybackRequest
import com.mediahub.android.playback.PlaybackRequestIntentCodec
import com.mediahub.android.playback.openPlaybackFallback
import kotlinx.coroutines.flow.MutableStateFlow

class PlayerActivity : ComponentActivity() {
    private var activeController: MediaController? = null
    private val pictureInPicture = MutableStateFlow(false)
    private lateinit var request: PlaybackRequest

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        val decodedRequest = PlaybackRequestIntentCodec.read(intent)
        if (decodedRequest == null) {
            finish()
            return
        }
        request = decodedRequest
        requestedOrientation = PlayerLandscapeOrientation
        enableEdgeToEdge()
        window.addFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON)
        hideSystemBars()
        onBackPressedDispatcher.addCallback(this, object : OnBackPressedCallback(true) {
            override fun handleOnBackPressed() = closePlayer()
        })
        setContent {
            MediaHubTheme {
                ProvideWindowAdaptive {
                val isPictureInPicture by pictureInPicture.collectAsState()
                val dependencies = remember { (application as MediaHubApplication).container.requireConfigured() }
                val factory = remember(dependencies.repository) { MediaHubViewModelFactory(dependencies.repository) }
                val subtitleViewModel = viewModel<RemoteSubtitleViewModel>(factory = factory)
                val subtitleState by subtitleViewModel.uiState.collectAsState()
                val embyItemId = (request.target as? EmbyItemTarget)?.itemId
                PlayerRoute(
                    request = request,
                    isPictureInPicture = isPictureInPicture,
                    onControllerChanged = { activeController = it },
                    subtitleState = subtitleState,
                    embyItemId = embyItemId,
                    subtitleActions = PlayerSubtitleActions(
                        onSearch = {
                            embyItemId?.let { id ->
                                subtitleViewModel.search(itemId = id, label = request.title)
                            }
                        },
                        onDownload = subtitleViewModel::download,
                        onClear = subtitleViewModel::clear,
                    ),
                    playerActions = PlayerActions(
                        onRetry = {},
                        onBack = ::closePlayer,
                        onToggleOrientation = ::toggleOrientation,
                        onEnterPictureInPicture = ::enterPictureInPicture,
                        onOpenFallback = request.fallback?.let { fallback ->
                            { openPlaybackFallback(this@PlayerActivity, fallback) }
                        },
                    ),
                )
                }
            }
        }
    }

    override fun onUserLeaveHint() {
        if (activeController?.isPlaying == true) enterPictureInPicture()
        super.onUserLeaveHint()
    }

    override fun onPictureInPictureModeChanged(isInPictureInPictureMode: Boolean, newConfig: Configuration) {
        super.onPictureInPictureModeChanged(isInPictureInPictureMode, newConfig)
        pictureInPicture.value = isInPictureInPictureMode
        if (!isInPictureInPictureMode) hideSystemBars()
    }

    override fun onWindowFocusChanged(hasFocus: Boolean) {
        super.onWindowFocusChanged(hasFocus)
        if (hasFocus && !isInPictureInPictureMode) hideSystemBars()
    }
    private fun closePlayer() {
        startService(MediaHubPlaybackService.invalidateIntent(this))
        finish()
    }



    private fun hideSystemBars() {
        WindowCompat.setDecorFitsSystemWindows(window, false)
        WindowInsetsControllerCompat(window, window.decorView).apply {
            hide(WindowInsetsCompat.Type.systemBars())
            systemBarsBehavior = WindowInsetsControllerCompat.BEHAVIOR_SHOW_TRANSIENT_BARS_BY_SWIPE
        }
    }

    private fun toggleOrientation() {
        requestedOrientation = nextPlayerOrientation(requestedOrientation)
    }

    private fun enterPictureInPicture() {
        if (activeController?.currentMediaItem == null || isInPictureInPictureMode) return
        val sourceRect = Rect().also(window.decorView::getGlobalVisibleRect)
        val params = PictureInPictureParams.Builder()
            .setAspectRatio(pipAspectRatio(activeController))
            .setSourceRectHint(sourceRect)
            .apply {
                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) setAutoEnterEnabled(true)
            }
            .build()
        setPictureInPictureParams(params)
        enterPictureInPictureMode(params)
    }

    companion object {
        fun intent(context: Context, request: PlaybackRequest): Intent =
            PlaybackRequestIntentCodec.put(Intent(context, PlayerActivity::class.java), request)
    }
}

@Composable
private fun PlayerRoute(
    request: PlaybackRequest,
    isPictureInPicture: Boolean,
    onControllerChanged: (MediaController?) -> Unit,
    subtitleState: RemoteSubtitleUiState,
    embyItemId: String?,
    subtitleActions: PlayerSubtitleActions,
    playerActions: PlayerActions,
) {
    var controller by remember { mutableStateOf<MediaController?>(null) }
    var state by remember { mutableStateOf<PlayerUiState>(PlayerUiState.Loading) }
    val latestState by rememberUpdatedState(state)
    val context = androidx.compose.ui.platform.LocalContext.current
    val appContext = context.applicationContext
    DisposableEffect(request.mediaId) {
        var connectedController: MediaController? = null
        val playerListener = object : Player.Listener {
            override fun onPlaybackStateChanged(playbackState: Int) {
                if (playbackState == Player.STATE_READY && latestState !is PlayerUiState.Error) {
                    state = PlayerUiState.Ready
                }
            }
        }
        val controllerListener = object : MediaController.Listener {
            override fun onExtrasChanged(controller: MediaController, extras: Bundle) {
                val next = extras.playerUiState()
                if (
                    next is PlayerUiState.Loading &&
                    controller.currentMediaItem != null &&
                    controller.playbackState != Player.STATE_IDLE
                ) {
                    return
                }
                state = next
            }
        }
        val future = MediaController.Builder(
            appContext,
            SessionToken(appContext, ComponentName(appContext, MediaHubPlaybackService::class.java)),
        ).setListener(controllerListener).buildAsync()
        future.addListener({
            if (!future.isCancelled) {
                connectedController = runCatching { future.get() }.getOrNull()
                controller = connectedController
                onControllerChanged(connectedController)
                connectedController?.let { connected ->
                    connected.addListener(playerListener)
                    state = connected.sessionExtras.playerUiState()
                    appContext.startService(MediaHubPlaybackService.playIntent(appContext, request))
                }
            }
        }, ContextCompat.getMainExecutor(appContext))
        onDispose {
            connectedController?.removeListener(playerListener)
            MediaController.releaseFuture(future)
            controller = null
            onControllerChanged(null)
        }
    }
    PlayerScreen(
        state = state,
        title = request.title,
        controller = controller,
        isPictureInPicture = isPictureInPicture,
        embyItemId = embyItemId,
        subtitleState = subtitleState,
        subtitleActions = subtitleActions,
        actions = playerActions.copy(onRetry = {
            state = PlayerUiState.Loading
            appContext.startService(MediaHubPlaybackService.playIntent(appContext, request, force = true))
        }),
    )
}

private fun Bundle.playerUiState(): PlayerUiState = when (getString(MediaHubPlaybackService.EXTRA_STATE)) {
    MediaHubPlaybackService.STATE_READY -> PlayerUiState.Ready
    MediaHubPlaybackService.STATE_ERROR -> PlayerUiState.Error(
        getString(MediaHubPlaybackService.EXTRA_MESSAGE).orEmpty().ifBlank { "播放失败" },
        getBoolean(MediaHubPlaybackService.EXTRA_RETRYABLE, false),
    )
    else -> PlayerUiState.Loading
}

internal const val PlayerLandscapeOrientation = ActivityInfo.SCREEN_ORIENTATION_SENSOR_LANDSCAPE

internal fun nextPlayerOrientation(current: Int): Int =
    if (current == ActivityInfo.SCREEN_ORIENTATION_REVERSE_LANDSCAPE) {
        PlayerLandscapeOrientation
    } else {
        ActivityInfo.SCREEN_ORIENTATION_REVERSE_LANDSCAPE
    }

internal fun pipAspectRatio(controller: MediaController?): Rational {
    val size = controller?.videoSize ?: return Rational(16, 9)
    if (size.width <= 0 || size.height <= 0) return Rational(16, 9)
    val width = size.width
    val height = size.height
    val aspect = width.toFloat() / height.toFloat()
    return when {
        aspect >= 2.39f -> Rational(239, 100)
        aspect <= 1f / 2.39f -> Rational(100, 239)
        else -> Rational(width, height)
    }
}

internal fun validRequest(request: PlaybackRequest): Boolean = PlaybackRequestIntentCodec.valid(request)
