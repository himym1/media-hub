package com.mediahub.android.feature.player

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.material3.Slider
import androidx.compose.material3.SliderDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableLongStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.viewinterop.AndroidView
import androidx.annotation.OptIn
import androidx.media3.common.C
import androidx.media3.common.Player
import androidx.media3.common.util.UnstableApi
import androidx.media3.session.MediaController
import androidx.media3.ui.PlayerView
import androidx.media3.ui.TrackSelectionDialogBuilder
import com.composables.icons.lucide.ArrowLeft
import com.composables.icons.lucide.ChevronLeft
import com.composables.icons.lucide.ChevronRight
import com.composables.icons.lucide.Lucide
import com.composables.icons.lucide.Pause
import com.composables.icons.lucide.PictureInPicture2
import com.composables.icons.lucide.Play
import com.composables.icons.lucide.RotateCw
import com.composables.icons.lucide.Settings2
import com.mediahub.android.core.designsystem.MediaHubButton
import com.mediahub.android.core.designsystem.MediaHubIconButton
import com.mediahub.android.core.designsystem.MediaHubText
import com.mediahub.android.feature.library.formatPlaybackPosition
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive

internal sealed interface PlayerUiState {
    data object Loading : PlayerUiState
    data object Ready : PlayerUiState
    data class Error(val message: String, val retryable: Boolean) : PlayerUiState
}

internal data class PlayerActions(
    val onRetry: () -> Unit,
    val onBack: () -> Unit,
    val onToggleOrientation: () -> Unit,
    val onEnterPictureInPicture: () -> Unit,
)

private val PlaybackSpeeds = listOf(0.75f, 1.0f, 1.25f, 1.5f, 2.0f)

@Composable
internal fun PlayerScreen(
    state: PlayerUiState,
    title: String,
    controller: MediaController?,
    isPictureInPicture: Boolean,
    actions: PlayerActions,
) {
    var controlsVisible by remember { mutableStateOf(true) }
    Box(Modifier.fillMaxSize().background(Color.Black)) {
        if (controller != null) {
            AndroidView(
                factory = { context -> createPlayerView(context) },
                update = {
                    it.player = controller
                    it.useController = false
                },
                modifier = Modifier.fillMaxSize(),
            )
        }
        when (state) {
            PlayerUiState.Loading -> MediaHubText(
                "正在准备视频…",
                color = Color.White,
                modifier = Modifier.align(Alignment.Center),
            )
            is PlayerUiState.Error -> Column(
                modifier = Modifier
                    .align(Alignment.Center)
                    .fillMaxWidth()
                    .padding(horizontal = 24.dp)
                    .widthIn(max = 480.dp),
                horizontalAlignment = Alignment.CenterHorizontally,
                verticalArrangement = Arrangement.spacedBy(16.dp),
            ) {
                MediaHubText(state.message, color = Color.White)
                if (state.retryable) MediaHubButton("重试", onClick = actions.onRetry)
            }
            PlayerUiState.Ready -> Unit
        }
        if (!isPictureInPicture && state is PlayerUiState.Ready) {
            Box(
                modifier = Modifier
                    .fillMaxSize()
                    .clickable(
                        indication = null,
                        interactionSource = remember { MutableInteractionSource() },
                        onClick = { controlsVisible = !controlsVisible },
                    ),
            )
        }
        if (!isPictureInPicture && controlsVisible) {
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .align(Alignment.TopCenter)
                    .background(
                        Brush.verticalGradient(
                            colors = listOf(Color.Black.copy(alpha = 0.62f), Color.Transparent),
                        ),
                    )
                    .statusBarsPadding()
                    .padding(horizontal = 8.dp, vertical = 4.dp),
            ) {
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(4.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    MediaHubIconButton(
                        imageVector = Lucide.ArrowLeft,
                        contentDescription = "返回",
                        onClick = actions.onBack,
                        tint = Color.White,
                    )
                    MediaHubText(
                        title,
                        color = Color.White,
                        modifier = Modifier.weight(1f).padding(horizontal = 4.dp),
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                    MediaHubIconButton(
                        imageVector = Lucide.RotateCw,
                        contentDescription = "旋转屏幕",
                        onClick = actions.onToggleOrientation,
                        tint = Color.White,
                    )
                    MediaHubIconButton(
                        imageVector = Lucide.PictureInPicture2,
                        contentDescription = "进入画中画",
                        onClick = actions.onEnterPictureInPicture,
                        tint = Color.White,
                    )
                }
            }
            if (state is PlayerUiState.Ready && controller != null) {
                PlayerBottomControls(
                    controller = controller,
                    modifier = Modifier
                        .align(Alignment.BottomCenter)
                        .fillMaxWidth(),
                    onUserInteraction = { controlsVisible = true },
                )
            }
        }
        if (!isPictureInPicture && state is PlayerUiState.Ready && controller != null) {
            LaunchedEffect(controlsVisible, controller.isPlaying) {
                if (controlsVisible && controller.isPlaying) {
                    delay(4_000)
                    controlsVisible = false
                }
            }
        }
    }
}

@Composable
private fun PlayerBottomControls(
    controller: MediaController,
    modifier: Modifier = Modifier,
    onUserInteraction: () -> Unit,
) {
    val context = LocalContext.current
    var positionMs by remember { mutableLongStateOf(0L) }
    var durationMs by remember { mutableLongStateOf(0L) }
    var playing by remember { mutableStateOf(controller.isPlaying) }
    var scrubbing by remember { mutableStateOf(false) }
    var scrubFraction by remember { mutableFloatStateOf(0f) }
    var speedIndex by remember {
        mutableStateOf(
            PlaybackSpeeds.indexOf(controller.playbackParameters.speed).let { if (it >= 0) it else 1 },
        )
    }

    DisposableEffect(controller) {
        val listener = object : Player.Listener {
            override fun onIsPlayingChanged(isPlaying: Boolean) {
                playing = isPlaying
            }
        }
        controller.addListener(listener)
        onDispose { controller.removeListener(listener) }
    }

    LaunchedEffect(controller) {
        while (isActive) {
            if (!scrubbing) {
                positionMs = controller.currentPosition.coerceAtLeast(0L)
                val duration = controller.duration
                durationMs = if (duration == C.TIME_UNSET || duration < 0L) 0L else duration
                playing = controller.isPlaying
            }
            delay(250)
        }
    }

    val fraction = when {
        scrubbing -> scrubFraction
        durationMs <= 0L -> 0f
        else -> (positionMs.toFloat() / durationMs.toFloat()).coerceIn(0f, 1f)
    }
    val displayPosition = if (scrubbing && durationMs > 0L) {
        (scrubFraction * durationMs).toLong()
    } else {
        positionMs
    }

    Column(
        modifier = modifier
            .background(
                Brush.verticalGradient(
                    colors = listOf(Color.Transparent, Color.Black.copy(alpha = 0.72f)),
                ),
            )
            .clickable(
                indication = null,
                interactionSource = remember { MutableInteractionSource() },
                onClick = onUserInteraction,
            )
            .navigationBarsPadding()
            .padding(horizontal = 12.dp, vertical = 8.dp),
        verticalArrangement = Arrangement.spacedBy(2.dp),
    ) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            MediaHubText(
                text = formatPlaybackPosition(displayPosition),
                color = Color.White,
                fontSize = 12.sp,
            )
            MediaHubText(
                text = if (durationMs > 0L) formatPlaybackPosition(durationMs) else "--:--",
                color = Color.White.copy(alpha = 0.82f),
                fontSize = 12.sp,
            )
        }
        Slider(
            value = fraction,
            onValueChange = { value ->
                onUserInteraction()
                scrubbing = true
                scrubFraction = value
            },
            onValueChangeFinished = {
                onUserInteraction()
                if (durationMs > 0L) {
                    controller.seekTo((scrubFraction * durationMs).toLong().coerceIn(0L, durationMs))
                }
                scrubbing = false
            },
            enabled = durationMs > 0L,
            modifier = Modifier
                .fillMaxWidth()
                .height(40.dp),
            colors = SliderDefaults.colors(
                thumbColor = Color.White,
                activeTrackColor = Color.White,
                inactiveTrackColor = Color.White.copy(alpha = 0.28f),
                disabledThumbColor = Color.White.copy(alpha = 0.4f),
                disabledActiveTrackColor = Color.White.copy(alpha = 0.4f),
                disabledInactiveTrackColor = Color.White.copy(alpha = 0.16f),
            ),
        )
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceEvenly,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            MediaHubIconButton(
                imageVector = Lucide.ChevronLeft,
                contentDescription = "后退 10 秒",
                onClick = {
                    onUserInteraction()
                    controller.seekBack()
                },
                tint = Color.White,
            )
            MediaHubIconButton(
                imageVector = if (playing) Lucide.Pause else Lucide.Play,
                contentDescription = if (playing) "暂停" else "播放",
                onClick = {
                    onUserInteraction()
                    controller.playWhenReady = !controller.playWhenReady
                },
                tint = Color.White,
                modifier = Modifier.size(56.dp),
            )
            MediaHubIconButton(
                imageVector = Lucide.ChevronRight,
                contentDescription = "前进 10 秒",
                onClick = {
                    onUserInteraction()
                    controller.seekForward()
                },
                tint = Color.White,
            )
            MediaHubIconButton(
                imageVector = Lucide.Settings2,
                contentDescription = "音轨与字幕",
                onClick = {
                    onUserInteraction()
                    openTrackSelector(context, controller)
                },
                tint = Color.White,
            )
            MediaHubButton(
                label = "${PlaybackSpeeds[speedIndex]}x",
                onClick = {
                    onUserInteraction()
                    speedIndex = (speedIndex + 1) % PlaybackSpeeds.size
                    controller.setPlaybackSpeed(PlaybackSpeeds[speedIndex])
                },
            )
        }
    }
}

@OptIn(UnstableApi::class)
private fun openTrackSelector(context: android.content.Context, player: Player) {
    runCatching {
        android.app.AlertDialog.Builder(context)
            .setTitle("音轨与字幕")
            .setItems(arrayOf("字幕", "音轨")) { _, which ->
                val trackType = if (which == 0) C.TRACK_TYPE_TEXT else C.TRACK_TYPE_AUDIO
                val title = if (which == 0) "字幕" else "音轨"
                TrackSelectionDialogBuilder(context, title, player, trackType).build().show()
            }
            .show()
    }
}

@OptIn(UnstableApi::class)
internal fun createPlayerView(context: android.content.Context): PlayerView = PlayerView(context).apply {
    setShowBuffering(PlayerView.SHOW_BUFFERING_WHEN_PLAYING)
    useController = false
    controllerAutoShow = false
}
