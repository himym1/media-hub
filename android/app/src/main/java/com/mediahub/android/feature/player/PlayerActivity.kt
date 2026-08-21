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
import androidx.compose.runtime.setValue
import androidx.core.content.ContextCompat
import androidx.core.view.WindowCompat
import androidx.core.view.WindowInsetsCompat
import androidx.core.view.WindowInsetsControllerCompat
import androidx.media3.session.MediaController
import androidx.media3.session.SessionToken
import com.mediahub.android.app.ProvideWindowAdaptive
import com.mediahub.android.core.designsystem.MediaHubTheme
import com.mediahub.android.playback.MediaHubPlaybackService
import com.mediahub.android.playback.PlaybackRequest
import com.mediahub.android.playback.PlaybackRequestIntentCodec
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
                PlayerRoute(
                    request = request,
                    isPictureInPicture = isPictureInPicture,
                    onControllerChanged = { activeController = it },
                    actions = PlayerActions(
                        onRetry = {},
                        onBack = ::closePlayer,
                        onToggleOrientation = ::toggleOrientation,
                        onEnterPictureInPicture = ::enterPictureInPicture,
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
            .setAspectRatio(Rational(16, 9))
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
    actions: PlayerActions,
) {
    var controller by remember { mutableStateOf<MediaController?>(null) }
    var state by remember { mutableStateOf<PlayerUiState>(PlayerUiState.Loading) }
    val context = androidx.compose.ui.platform.LocalContext.current
    DisposableEffect(context, request.mediaId) {
        val controllerListener = object : MediaController.Listener {
            override fun onExtrasChanged(controller: MediaController, extras: Bundle) {
                state = extras.playerUiState()
            }
        }
        val future = MediaController.Builder(
            context,
            SessionToken(context, ComponentName(context, MediaHubPlaybackService::class.java)),
        ).setListener(controllerListener).buildAsync()
        future.addListener({
            if (!future.isCancelled) {
                controller = runCatching { future.get() }.getOrNull()
                onControllerChanged(controller)
                controller?.let { connected ->
                    state = connected.sessionExtras.playerUiState()
                    context.startService(MediaHubPlaybackService.playIntent(context, request))
                }
            }
        }, ContextCompat.getMainExecutor(context))
        onDispose {
            controller = null
            onControllerChanged(null)
            MediaController.releaseFuture(future)
        }
    }
    PlayerScreen(
        state = state,
        title = request.title,
        controller = controller,
        isPictureInPicture = isPictureInPicture,
        actions = actions.copy(onRetry = {
            state = PlayerUiState.Loading
            context.startService(MediaHubPlaybackService.playIntent(context, request, force = true))
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
    if (current == PlayerLandscapeOrientation) {
        ActivityInfo.SCREEN_ORIENTATION_UNSPECIFIED
    } else {
        PlayerLandscapeOrientation
    }

internal fun validRequest(request: PlaybackRequest): Boolean = PlaybackRequestIntentCodec.valid(request)
