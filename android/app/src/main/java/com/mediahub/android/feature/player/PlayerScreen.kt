package com.mediahub.android.feature.player

import android.app.Activity
import android.content.Context
import android.media.AudioManager
import android.os.SystemClock
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.animateDpAsState
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.scaleIn
import androidx.compose.animation.scaleOut
import androidx.compose.animation.slideInHorizontally
import androidx.compose.animation.slideInVertically
import androidx.compose.animation.slideOutHorizontally
import androidx.compose.animation.slideOutVertically
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.gestures.awaitEachGesture
import androidx.compose.foundation.gestures.awaitFirstDown
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Slider
import androidx.compose.material3.SliderDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableLongStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.hapticfeedback.HapticFeedbackType
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalHapticFeedback
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.viewinterop.AndroidView
import androidx.media3.common.C
import androidx.media3.common.Player
import androidx.media3.common.TrackSelectionParameters
import androidx.media3.common.Tracks
import androidx.media3.common.util.UnstableApi
import androidx.media3.session.MediaController
import androidx.media3.ui.AspectRatioFrameLayout
import androidx.media3.ui.PlayerView
import com.composables.icons.lucide.ArrowLeft
import com.composables.icons.lucide.Captions
import com.composables.icons.lucide.Check
import com.composables.icons.lucide.CircleAlert
import com.composables.icons.lucide.Lock
import com.composables.icons.lucide.LockOpen
import com.composables.icons.lucide.Lucide
import com.composables.icons.lucide.Maximize
import com.composables.icons.lucide.Minus
import com.composables.icons.lucide.Pause
import com.composables.icons.lucide.Plus
import com.composables.icons.lucide.PictureInPicture2
import com.composables.icons.lucide.Play
import com.composables.icons.lucide.RotateCcw
import com.composables.icons.lucide.RotateCw
import com.composables.icons.lucide.Settings2
import com.composables.icons.lucide.Sun
import com.composables.icons.lucide.Volume2
import com.composables.icons.lucide.VolumeX
import com.composables.icons.lucide.X
import com.mediahub.android.core.designsystem.MediaHubColors
import com.mediahub.android.core.designsystem.MediaHubIcon
import com.mediahub.android.core.designsystem.MediaHubSegmentedControl
import com.mediahub.android.core.designsystem.MediaHubShapes
import com.mediahub.android.core.designsystem.MediaHubText
import com.mediahub.android.core.network.EmbyRemoteSubtitle
import com.mediahub.android.feature.library.formatPlaybackPosition
import com.mediahub.android.feature.subtitles.RemoteSubtitleUiState
import com.mediahub.android.playback.SubtitleTiming
import com.mediahub.android.playback.ensureDefaultAudioTrack
import com.mediahub.android.playback.formatSubtitleOffset
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import kotlin.math.abs
import kotlin.math.roundToInt

internal sealed interface PlayerUiState {
    data object Loading : PlayerUiState
    data object Ready : PlayerUiState
    data class Error(val message: String, val retryable: Boolean) : PlayerUiState
}

internal data class PlayerSubtitleActions(
    val onSearch: () -> Unit = {},
    val onDownload: (String) -> Unit = {},
    val onClear: () -> Unit = {},
)

internal data class PlayerActions(
    val onRetry: () -> Unit,
    val onBack: () -> Unit,
    val onToggleOrientation: () -> Unit,
    val onEnterPictureInPicture: () -> Unit,
    val onOpenFallback: (() -> Unit)? = null,
)

internal data class SeekGestureState(
    val targetMs: Long,
    val deltaMs: Long,
    val durationMs: Long,
)

private val PlaybackSpeeds = listOf(0.5f, 0.75f, 1.0f, 1.25f, 1.5f, 2.0f)

@androidx.annotation.OptIn(UnstableApi::class)
internal enum class PlayerAspectRatio(
    val label: String,
    val description: String,
    val resizeMode: Int,
) {
    Fit("自适应", "保持比例，完整显示", AspectRatioFrameLayout.RESIZE_MODE_FIT),
    Zoom("铺满", "裁切边缘，填满屏幕", AspectRatioFrameLayout.RESIZE_MODE_ZOOM),
    FixedWidth("宽度优先", "宽度铺满，高度按比例", AspectRatioFrameLayout.RESIZE_MODE_FIXED_WIDTH),
    FixedHeight("高度优先", "高度铺满，宽度按比例", AspectRatioFrameLayout.RESIZE_MODE_FIXED_HEIGHT),
    Fill("拉伸", "拉伸填满，可能变形", AspectRatioFrameLayout.RESIZE_MODE_FILL),
    ;

    companion object {
        val options: List<PlayerAspectRatio> = entries
    }
}

private enum class DragGestureMode {
    NONE,
    HORIZONTAL_SEEK,
    VERTICAL_BRIGHTNESS,
    VERTICAL_VOLUME,
}

@Composable
private fun rememberPlayerTracks(player: Player): Tracks {
    var tracks by remember(player) { mutableStateOf(player.currentTracks) }
    DisposableEffect(player) {
        val listener = object : Player.Listener {
            override fun onTracksChanged(current: Tracks) {
                tracks = current
            }

            override fun onTrackSelectionParametersChanged(parameters: TrackSelectionParameters) {
                tracks = player.currentTracks
            }
        }
        player.addListener(listener)
        onDispose { player.removeListener(listener) }
    }
    return tracks
}

@androidx.annotation.OptIn(UnstableApi::class)
@Composable
internal fun PlayerScreen(
    state: PlayerUiState,
    title: String,
    controller: MediaController?,
    isPictureInPicture: Boolean,
    actions: PlayerActions,
    embyItemId: String? = null,
    subtitleState: RemoteSubtitleUiState = RemoteSubtitleUiState(),
    subtitleActions: PlayerSubtitleActions = PlayerSubtitleActions(),
) {
    val context = LocalContext.current
    val haptic = LocalHapticFeedback.current
    var controlsVisible by remember { mutableStateOf(true) }
    var isLocked by remember { mutableStateOf(false) }
    var lockIndicatorVisible by remember { mutableStateOf(false) }
    var aspectRatio by remember(title, controller?.currentMediaItem?.mediaId) {
        mutableStateOf(PlayerAspectRatio.Fit)
    }
    var speedDialogVisible by remember { mutableStateOf(false) }
    var aspectRatioDialogVisible by remember { mutableStateOf(false) }
    var trackPanelVisible by remember { mutableStateOf(false) }

    // Edge HUD states (Subtitle-Safe)
    var gestureBrightness by remember { mutableStateOf<Float?>(null) }
    var gestureVolume by remember { mutableStateOf<Float?>(null) }
    var doubleTapFlashSide by remember { mutableStateOf<String?>(null) }
    var gestureSeekState by remember { mutableStateOf<SeekGestureState?>(null) }

    // Auto-hide controls timer
    LaunchedEffect(controlsVisible, controller?.isPlaying, isLocked, gestureSeekState, trackPanelVisible, speedDialogVisible, aspectRatioDialogVisible) {
        if (controlsVisible && !isLocked && controller?.isPlaying == true &&
            gestureSeekState == null && !trackPanelVisible && !speedDialogVisible && !aspectRatioDialogVisible
        ) {
            delay(5_000)
            controlsVisible = false
        }
    }

    // Auto-hide lock button when locked
    LaunchedEffect(lockIndicatorVisible) {
        if (lockIndicatorVisible) {
            delay(3_000)
            lockIndicatorVisible = false
        }
    }

    // Auto-hide edge HUDs and flash animations
    LaunchedEffect(gestureBrightness, gestureVolume, doubleTapFlashSide) {
        if (gestureBrightness != null || gestureVolume != null || doubleTapFlashSide != null) {
            delay(1_200)
            gestureBrightness = null
            gestureVolume = null
            doubleTapFlashSide = null
        }
    }

    val showPreparingOverlay = state is PlayerUiState.Loading &&
        (controller == null || controller.currentMediaItem == null)

    LaunchedEffect(controller, state) {
        if (state is PlayerUiState.Ready && controller != null) {
            controller.ensureDefaultAudioTrack()
        }
    }

    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(Color(0xFF07090E)),
    ) {
        // 1. Video Surface Layer
        if (controller != null) {
            AndroidView(
                factory = { ctx -> createPlayerView(ctx) },
                update = { playerView ->
                    if (playerView.player != controller) {
                        playerView.player = controller
                        playerView.useController = false
                    }
                    playerView.resizeMode = aspectRatio.resizeMode
                    playerView.setShowBuffering(
                        if (state is PlayerUiState.Ready) {
                            PlayerView.SHOW_BUFFERING_WHEN_PLAYING
                        } else {
                            PlayerView.SHOW_BUFFERING_NEVER
                        },
                    )
                },
                modifier = Modifier.fillMaxSize(),
            )
        }

        // 2. Fullscreen Gesture Layer (Behind Controls)
        if (!isPictureInPicture && state is PlayerUiState.Ready && controller != null) {
            PlayerGestureLayer(
                controller = controller,
                isLocked = isLocked,
                onSingleTap = {
                    if (isLocked) {
                        lockIndicatorVisible = true
                    } else {
                        controlsVisible = !controlsVisible
                    }
                },
                onDoubleTapLeft = {
                    if (!isLocked) {
                        haptic.performHapticFeedback(HapticFeedbackType.LongPress)
                        controller.seekBack()
                        doubleTapFlashSide = "left"
                        controlsVisible = true
                    }
                },
                onDoubleTapRight = {
                    if (!isLocked) {
                        haptic.performHapticFeedback(HapticFeedbackType.LongPress)
                        controller.seekForward()
                        doubleTapFlashSide = "right"
                        controlsVisible = true
                    }
                },
                onDoubleTapCenter = {
                    if (!isLocked) {
                        haptic.performHapticFeedback(HapticFeedbackType.TextHandleMove)
                        controller.playWhenReady = !controller.playWhenReady
                        doubleTapFlashSide = "center"
                        controlsVisible = true
                    }
                },
                onBrightnessChange = { fraction ->
                    if (!isLocked) {
                        setWindowBrightness(context, fraction)
                        gestureBrightness = fraction
                    }
                },
                onVolumeChange = { fraction ->
                    if (!isLocked) {
                        setDeviceVolume(context, fraction)
                        gestureVolume = fraction
                    }
                },
                onSeekPreview = { seekState ->
                    if (!isLocked) {
                        gestureSeekState = seekState
                        controlsVisible = true
                    }
                },
                onSeekCommit = { targetMs ->
                    if (!isLocked) {
                        haptic.performHapticFeedback(HapticFeedbackType.TextHandleMove)
                        controller.seekTo(targetMs)
                        gestureSeekState = null
                        controlsVisible = true
                    }
                },
                modifier = Modifier.fillMaxSize(),
            )
        }

        // 3. Double-Tap Side Arc Ripple Feedback (Left / Right / Center)
        PlayerDoubleTapFlashOverlay(flashSide = doubleTapFlashSide)

        // 4. Edge-Pinned Slim HUDs (Left Brightness & Right Volume, Subtitle-Safe)
        PlayerEdgeHuds(
            brightness = gestureBrightness,
            volume = gestureVolume,
            modifier = Modifier.fillMaxSize(),
        )

        // 5. Center Seek HUD (Horizontal Drag Preview)
        PlayerCenterSeekHud(
            seekState = gestureSeekState,
            modifier = Modifier.align(Alignment.Center),
        )

        // 6. Floating Screen Lock Button (Left Edge)
        if (!isPictureInPicture && state is PlayerUiState.Ready && controller != null) {
            AnimatedVisibility(
                visible = (controlsVisible && !isLocked) || (isLocked && lockIndicatorVisible),
                enter = slideInHorizontally(initialOffsetX = { -it }) + fadeIn() + scaleIn(),
                exit = slideOutHorizontally(targetOffsetX = { -it }) + fadeOut() + scaleOut(),
                modifier = Modifier
                    .align(Alignment.CenterStart)
                    .padding(start = 28.dp),
            ) {
                PlayerIconButton(
                    imageVector = if (isLocked) Lucide.Lock else Lucide.LockOpen,
                    contentDescription = if (isLocked) "解锁屏幕" else "锁定屏幕",
                    onClick = {
                        haptic.performHapticFeedback(HapticFeedbackType.LongPress)
                        isLocked = !isLocked
                        if (isLocked) {
                            controlsVisible = false
                            lockIndicatorVisible = false
                        } else {
                            controlsVisible = true
                            lockIndicatorVisible = false
                        }
                    },
                    modifier = Modifier.size(48.dp),
                    iconSize = 20.dp,
                )
            }
        }

        // 7. Top Control Bar (Clean, Uncluttered)
        AnimatedVisibility(
            visible = !isPictureInPicture && controlsVisible && !isLocked,
            enter = slideInVertically(initialOffsetY = { -it }, animationSpec = tween(220)) + fadeIn(tween(200)),
            exit = slideOutVertically(targetOffsetY = { -it }, animationSpec = tween(220)) + fadeOut(tween(200)),
            modifier = Modifier
                .fillMaxWidth()
                .align(Alignment.TopCenter),
        ) {
            PlayerTopBar(
                title = title,
                aspectRatio = aspectRatio,
                onOpenAspectRatio = {
                    haptic.performHapticFeedback(HapticFeedbackType.TextHandleMove)
                    aspectRatioDialogVisible = true
                },
                actions = actions,
            )
        }

        // 8. Center Playback Controls
        AnimatedVisibility(
            visible = !isPictureInPicture && controlsVisible && !isLocked && state is PlayerUiState.Ready && controller != null,
            enter = scaleIn(initialScale = 0.85f, animationSpec = tween(200)) + fadeIn(tween(200)),
            exit = scaleOut(targetScale = 0.85f, animationSpec = tween(200)) + fadeOut(tween(200)),
            modifier = Modifier.align(Alignment.Center),
        ) {
            if (controller != null) {
                PlayerCenterControls(
                    controller = controller,
                    onUserInteraction = { controlsVisible = true },
                )
            }
        }

        // 9. Bottom Control Bar (Sleek Streamlined Single Surface)
        AnimatedVisibility(
            visible = !isPictureInPicture && controlsVisible && !isLocked && state is PlayerUiState.Ready && controller != null,
            enter = slideInVertically(initialOffsetY = { it }, animationSpec = tween(220)) + fadeIn(tween(200)),
            exit = slideOutVertically(targetOffsetY = { it }, animationSpec = tween(220)) + fadeOut(tween(200)),
            modifier = Modifier
                .fillMaxWidth()
                .align(Alignment.BottomCenter),
        ) {
            if (controller != null) {
                PlayerBottomControls(
                    controller = controller,
                    onOpenSpeedDialog = {
                        haptic.performHapticFeedback(HapticFeedbackType.TextHandleMove)
                        speedDialogVisible = true
                    },
                    onOpenTracks = {
                        haptic.performHapticFeedback(HapticFeedbackType.TextHandleMove)
                        trackPanelVisible = true
                    },
                    onUserInteraction = { controlsVisible = true },
                )
            }
        }

        // 10. Loading & Error States
        when {
            showPreparingOverlay -> {
                Box(
                    modifier = Modifier
                        .fillMaxSize()
                        .background(Color.Black.copy(alpha = 0.55f)),
                    contentAlignment = Alignment.Center,
                ) {
                    Column(
                        horizontalAlignment = Alignment.CenterHorizontally,
                        verticalArrangement = Arrangement.spacedBy(10.dp),
                        modifier = Modifier
                            .clip(MediaHubShapes.Card)
                            .background(MediaHubColors.Surface.copy(alpha = 0.94f))
                            .padding(horizontal = 18.dp, vertical = 14.dp),
                    ) {
                        CircularProgressIndicator(
                            color = MediaHubColors.Accent,
                            modifier = Modifier.size(22.dp),
                            strokeWidth = 2.5.dp,
                        )
                        MediaHubText(
                            text = "正在准备视频…",
                            color = MediaHubColors.TextPrimary,
                            fontSize = 13.sp,
                            fontWeight = FontWeight.Medium,
                        )
                    }
                }
            }

            state is PlayerUiState.Error -> {
                    Column(
                    modifier = Modifier
                        .align(Alignment.Center)
                        .padding(horizontal = 24.dp)
                        .widthIn(max = 300.dp)
                        .clip(MediaHubShapes.Card)
                        .background(MediaHubColors.Surface.copy(alpha = 0.96f))
                        .padding(16.dp),
                    horizontalAlignment = Alignment.CenterHorizontally,
                    verticalArrangement = Arrangement.spacedBy(10.dp),
                ) {
                    MediaHubIcon(
                        imageVector = Lucide.CircleAlert,
                        contentDescription = null,
                        tint = MediaHubColors.Error,
                        modifier = Modifier.size(28.dp),
                    )
                    MediaHubText(
                        text = state.message,
                        color = MediaHubColors.TextPrimary,
                        fontSize = 15.sp,
                        fontWeight = FontWeight.Medium,
                    )
                    if (state.retryable) {
                        PlayerChipButton(
                            label = "重试",
                            onClick = actions.onRetry,
                            modifier = Modifier.height(48.dp),
                            emphasized = true,
                        )
                    }
                    actions.onOpenFallback?.let { openFallback ->
                        PlayerChipButton(
                            label = "用 Emby 打开",
                            onClick = openFallback,
                            modifier = Modifier.height(48.dp),
                        )
                    }
                }
            }

            else -> Unit
        }

        // 11. Audio and subtitle drawer
        if (trackPanelVisible && controller != null) {
            PlayerTrackSelectionDrawer(
                player = controller,
                embyItemId = embyItemId,
                subtitleState = subtitleState,
                subtitleActions = subtitleActions,
                onDismiss = { trackPanelVisible = false },
            )
        }

        // 12. Speed Selector Dialog
        if (speedDialogVisible && controller != null) {
            PlayerSpeedDialog(
                currentSpeed = controller.playbackParameters.speed,
                onSelectSpeed = { newSpeed ->
                    haptic.performHapticFeedback(HapticFeedbackType.TextHandleMove)
                    controller.setPlaybackSpeed(newSpeed)
                    speedDialogVisible = false
                },
                onDismiss = { speedDialogVisible = false },
            )
        }

        // 13. Aspect Ratio Selector Dialog
        if (aspectRatioDialogVisible) {
            PlayerAspectRatioDialog(
                current = aspectRatio,
                onSelect = { selected ->
                    haptic.performHapticFeedback(HapticFeedbackType.TextHandleMove)
                    aspectRatio = selected
                    aspectRatioDialogVisible = false
                },
                onDismiss = { aspectRatioDialogVisible = false },
            )
        }
    }
}

/**
 * Top control bar with title, subtle stream tag, aspect ratio toggle, orientation toggle and PiP.
 */
@androidx.annotation.OptIn(UnstableApi::class)
@Composable
private fun PlayerTopBar(
    title: String,
    aspectRatio: PlayerAspectRatio,
    onOpenAspectRatio: () -> Unit,
    actions: PlayerActions,
    modifier: Modifier = Modifier,
) {
    Box(
        modifier = modifier
            .background(
                Brush.verticalGradient(
                    colors = listOf(
                        Color.Black.copy(alpha = 0.85f),
                        Color.Black.copy(alpha = 0.40f),
                        Color.Transparent,
                    ),
                ),
            )
            .statusBarsPadding()
            .padding(horizontal = 18.dp, vertical = 10.dp),
    ) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.spacedBy(12.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            PlayerIconButton(
                imageVector = Lucide.ArrowLeft,
                contentDescription = "返回",
                onClick = actions.onBack,
                modifier = Modifier.size(48.dp),
                iconSize = 20.dp,
            )

            Column(
                modifier = Modifier
                    .weight(1f)
                    .padding(horizontal = 4.dp),
                verticalArrangement = Arrangement.spacedBy(3.dp),
            ) {
                MediaHubText(
                    text = title,
                    color = MediaHubColors.TextPrimary,
                    fontSize = 15.sp,
                    fontWeight = FontWeight.SemiBold,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
            }

            PlayerChipButton(
                label = aspectRatio.label,
                onClick = onOpenAspectRatio,
            )

            PlayerIconButton(
                imageVector = Lucide.Maximize,
                contentDescription = "旋转屏幕",
                onClick = actions.onToggleOrientation,
                modifier = Modifier.size(48.dp),
                iconSize = 18.dp,
            )

            PlayerIconButton(
                imageVector = Lucide.PictureInPicture2,
                contentDescription = "进入画中画",
                onClick = actions.onEnterPictureInPicture,
                modifier = Modifier.size(48.dp),
                iconSize = 18.dp,
            )
        }
    }
}

/**
 * Center playback controls.
 */
@Composable
private fun PlayerCenterControls(
    controller: MediaController,
    modifier: Modifier = Modifier,
    onUserInteraction: () -> Unit,
) {
    var isPlaying by remember { mutableStateOf(controller.isPlaying) }
    val haptic = LocalHapticFeedback.current

    DisposableEffect(controller) {
        val listener = object : Player.Listener {
            override fun onIsPlayingChanged(playing: Boolean) {
                isPlaying = playing
            }
        }
        controller.addListener(listener)
        onDispose { controller.removeListener(listener) }
    }

    Row(
        modifier = modifier,
        horizontalArrangement = Arrangement.spacedBy(28.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        PlayerIconButton(
            imageVector = Lucide.RotateCcw,
            contentDescription = "后退 10 秒",
            onClick = {
                haptic.performHapticFeedback(HapticFeedbackType.LongPress)
                onUserInteraction()
                controller.seekBack()
            },
            modifier = Modifier.size(48.dp),
            iconSize = 22.dp,
        )

        Box(
            modifier = Modifier
                .size(58.dp)
                .clip(CircleShape)
                .background(MediaHubColors.Accent)
                .clickable(
                    role = Role.Button,
                    onClick = {
                        haptic.performHapticFeedback(HapticFeedbackType.TextHandleMove)
                        onUserInteraction()
                        controller.playWhenReady = !controller.playWhenReady
                    },
                )
                .semantics { contentDescription = if (isPlaying) "暂停" else "播放" },
            contentAlignment = Alignment.Center,
        ) {
            MediaHubIcon(
                imageVector = if (isPlaying) Lucide.Pause else Lucide.Play,
                contentDescription = if (isPlaying) "暂停" else "播放",
                tint = MediaHubColors.OnAccent,
                modifier = Modifier.size(28.dp),
            )
        }

        PlayerIconButton(
            imageVector = Lucide.RotateCw,
            contentDescription = "前进 10 秒",
            onClick = {
                haptic.performHapticFeedback(HapticFeedbackType.LongPress)
                onUserInteraction()
                controller.seekForward()
            },
            modifier = Modifier.size(48.dp),
            iconSize = 22.dp,
        )
    }
}

/**
 * Bottom controls bar with sleek full-width scrubber and clean right-aligned utility pills.
 */
@androidx.annotation.OptIn(UnstableApi::class)
@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun PlayerBottomControls(
    controller: MediaController,
    onOpenSpeedDialog: () -> Unit,
    onOpenTracks: () -> Unit,
    modifier: Modifier = Modifier,
    onUserInteraction: () -> Unit,
) {
    var positionMs by remember { mutableLongStateOf(0L) }
    var durationMs by remember { mutableLongStateOf(0L) }
    var bufferedPositionMs by remember { mutableLongStateOf(0L) }
    var isPlaying by remember { mutableStateOf(controller.isPlaying) }
    var currentSpeed by remember { mutableFloatStateOf(controller.playbackParameters.speed) }

    // Anti-bounce seek synchronization state
    var pendingSeekTargetMs by remember { mutableStateOf<Long?>(null) }
    var pendingSeekTimestamp by remember { mutableLongStateOf(0L) }
    var isScrubbing by remember { mutableStateOf(false) }
    var scrubFraction by remember { mutableFloatStateOf(0f) }
    val haptic = LocalHapticFeedback.current

    DisposableEffect(controller) {
        val listener = object : Player.Listener {
            override fun onIsPlayingChanged(playing: Boolean) {
                isPlaying = playing
            }

            override fun onPlaybackParametersChanged(playbackParameters: androidx.media3.common.PlaybackParameters) {
                currentSpeed = playbackParameters.speed
            }
        }
        controller.addListener(listener)
        onDispose { controller.removeListener(listener) }
    }

    // Periodic state updater with seek-lock guard
    LaunchedEffect(controller) {
        while (isActive) {
            if (!isScrubbing) {
                val realPosition = controller.currentPosition.coerceAtLeast(0L)
                val duration = controller.duration
                durationMs = if (duration == C.TIME_UNSET || duration < 0L) 0L else duration
                bufferedPositionMs = controller.bufferedPosition.coerceAtLeast(0L)
                isPlaying = controller.isPlaying

                val pending = pendingSeekTargetMs
                if (pending != null) {
                    val elapsed = SystemClock.uptimeMillis() - pendingSeekTimestamp
                    if (abs(realPosition - pending) < 1_500L || elapsed > 2_000L) {
                        pendingSeekTargetMs = null
                        positionMs = realPosition
                    } else {
                        positionMs = pending
                    }
                } else {
                    positionMs = realPosition
                }
            }
            delay(200)
        }
    }

    val displayPositionMs = when {
        isScrubbing && durationMs > 0L -> (scrubFraction * durationMs).toLong().coerceIn(0L, durationMs)
        pendingSeekTargetMs != null -> pendingSeekTargetMs!!
        else -> positionMs
    }

    val playbackFraction = if (durationMs > 0L) {
        (displayPositionMs.toFloat() / durationMs.toFloat()).coerceIn(0f, 1f)
    } else 0f

    val bufferedFraction = if (durationMs > 0L) {
        (bufferedPositionMs.toFloat() / durationMs.toFloat()).coerceIn(0f, 1f)
    } else 0f

    Column(
        modifier = modifier
            .background(
                Brush.verticalGradient(
                    colors = listOf(
                        Color.Transparent,
                        Color.Black.copy(alpha = 0.50f),
                        Color.Black.copy(alpha = 0.90f),
                    ),
                ),
            )
            .navigationBarsPadding()
            .padding(horizontal = 22.dp, vertical = 10.dp),
        verticalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        // Floating Time Preview Bubble (When dragging scrubber)
        if (isScrubbing && durationMs > 0L) {
            Box(
                modifier = Modifier.fillMaxWidth(),
                contentAlignment = Alignment.Center,
            ) {
                Box(
                    modifier = Modifier
                        .clip(MediaHubShapes.Control)
                        .background(MediaHubColors.Surface.copy(alpha = 0.94f))
                        .padding(horizontal = 16.dp, vertical = 6.dp),
                ) {
                    Row(
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(6.dp),
                    ) {
                        MediaHubText(
                            text = formatPlaybackPosition(displayPositionMs),
                            color = MediaHubColors.Accent,
                            fontSize = 15.sp,
                            fontWeight = FontWeight.Bold,
                        )
                        MediaHubText(
                            text = "/",
                            color = MediaHubColors.TextMuted,
                            fontSize = 13.sp,
                        )
                        MediaHubText(
                            text = formatPlaybackPosition(durationMs),
                            color = MediaHubColors.TextSecondary,
                            fontSize = 13.sp,
                            fontWeight = FontWeight.Medium,
                        )
                    }
                }
            }
        }

        // Row 1: Time Readouts & Scrubber Track
        Row(
            modifier = Modifier.fillMaxWidth(),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(14.dp),
        ) {
            MediaHubText(
                text = formatPlaybackPosition(displayPositionMs),
                color = MediaHubColors.TextPrimary,
                fontSize = 13.sp,
                fontWeight = FontWeight.SemiBold,
                modifier = Modifier.widthIn(min = 45.dp),
            )

            Slider(
                value = playbackFraction,
                onValueChange = { fraction ->
                    onUserInteraction()
                    isScrubbing = true
                    scrubFraction = fraction
                },
                onValueChangeFinished = {
                    haptic.performHapticFeedback(HapticFeedbackType.TextHandleMove)
                    onUserInteraction()
                    if (durationMs > 0L) {
                        val targetMs = (scrubFraction * durationMs).toLong().coerceIn(0L, durationMs)
                        pendingSeekTargetMs = targetMs
                        pendingSeekTimestamp = SystemClock.uptimeMillis()
                        positionMs = targetMs
                        controller.seekTo(targetMs)
                    }
                    isScrubbing = false
                },
                enabled = durationMs > 0L,
                thumb = {
                    val thumbRadius by animateDpAsState(
                        targetValue = if (isScrubbing) 8.dp else 5.5.dp,
                        animationSpec = tween(120),
                        label = "thumbRadius",
                    )
                    Box(
                        modifier = Modifier
                            .size(thumbRadius * 2)
                            .clip(CircleShape)
                            .background(MediaHubColors.Accent),
                    )
                },
                track = { sliderState ->
                    val fraction = sliderState.value
                    val trackHeight by animateDpAsState(
                        targetValue = if (isScrubbing) 6.dp else 3.5.dp,
                        animationSpec = tween(120),
                        label = "trackHeight",
                    )
                    Box(
                        modifier = Modifier
                            .fillMaxWidth()
                            .height(trackHeight)
                            .clip(RoundedCornerShape(trackHeight / 2))
                            .background(MediaHubColors.NeutralContainer),
                        contentAlignment = Alignment.CenterStart,
                    ) {
                        if (bufferedFraction > 0f) {
                            Box(
                                modifier = Modifier
                                    .fillMaxWidth(bufferedFraction.coerceIn(0f, 1f))
                                    .fillMaxHeight()
                                    .background(MediaHubColors.TextFaint),
                            )
                        }
                        Box(
                            modifier = Modifier
                                .fillMaxWidth(fraction.coerceIn(0f, 1f))
                                .fillMaxHeight()
                                .background(MediaHubColors.Accent),
                        )
                    }
                },
                colors = SliderDefaults.colors(
                    thumbColor = MediaHubColors.Accent,
                    activeTrackColor = MediaHubColors.Accent,
                    inactiveTrackColor = MediaHubColors.NeutralContainer,
                ),
                modifier = Modifier
                    .weight(1f)
                    .height(36.dp),
            )

            MediaHubText(
                text = if (durationMs > 0L) formatPlaybackPosition(durationMs) else "--:--",
                color = MediaHubColors.TextSecondary,
                fontSize = 13.sp,
                fontWeight = FontWeight.Normal,
                modifier = Modifier.widthIn(min = 45.dp),
            )
        }

        // Row 2: Secondary Quick Actions Row (Streamlined, No duplicate buttons)
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.End,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Row(
                horizontalArrangement = Arrangement.spacedBy(8.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                PlayerChipButton(
                    label = if (currentSpeed == 1.0f) "倍速" else "${currentSpeed}x",
                    onClick = {
                        onUserInteraction()
                        onOpenSpeedDialog()
                    },
                )

                PlayerChipButton(
                    label = "音轨/字幕",
                    icon = Lucide.Settings2,
                    onClick = {
                        onUserInteraction()
                        onOpenTracks()
                    },
                )
            }
        }
    }
}

/**
 * Fullscreen Gesture Handling Layer:
 * - Horizontal drag anywhere: Fast forward / fast rewind video seeking
 * - Left vertical drag: Screen brightness
 * - Right vertical drag: Device volume
 * - Double tap left/right/center: Jump 10s back / Jump 10s forward / Play-Pause
 * - Single tap: Toggle controls
 */
@Composable
private fun PlayerGestureLayer(
    controller: MediaController,
    isLocked: Boolean,
    onSingleTap: () -> Unit,
    onDoubleTapLeft: () -> Unit,
    onDoubleTapRight: () -> Unit,
    onDoubleTapCenter: () -> Unit,
    onBrightnessChange: (Float) -> Unit,
    onVolumeChange: (Float) -> Unit,
    onSeekPreview: (SeekGestureState) -> Unit,
    onSeekCommit: (Long) -> Unit,
    modifier: Modifier = Modifier,
) {
    val context = LocalContext.current

    Box(
        modifier = modifier
            .pointerInput(isLocked) {
                detectTapGestures(
                    onTap = { onSingleTap() },
                    onDoubleTap = { offset ->
                        if (isLocked) {
                            onSingleTap()
                            return@detectTapGestures
                        }
                        val width = size.width
                        when {
                            offset.x < width * 0.35f -> onDoubleTapLeft()
                            offset.x > width * 0.65f -> onDoubleTapRight()
                            else -> onDoubleTapCenter()
                        }
                    },
                )
            }
            .pointerInput(isLocked, controller) {
                if (isLocked) return@pointerInput
                awaitEachGesture {
                    val down = awaitFirstDown(requireUnconsumed = false)
                    val downX = down.position.x
                    val downY = down.position.y
                    val screenWidth = size.width.toFloat()
                    val screenHeight = size.height.toFloat()
                    val isLeftSide = downX < screenWidth * 0.5f
                    val touchSlop = viewConfiguration.touchSlop

                    var dragMode = DragGestureMode.NONE
                    val durationMs = if (controller.duration == C.TIME_UNSET || controller.duration < 0L) 0L else controller.duration
                    var initialSeekMs = controller.currentPosition.coerceAtLeast(0L)
                    var targetSeekMs = initialSeekMs
                    var initialBrightness = getWindowBrightness(context)
                    var initialVolume = getDeviceVolumeFraction(context)

                    while (true) {
                        val event = awaitPointerEvent()
                        val change = event.changes.firstOrNull() ?: break
                        if (!change.pressed) {
                            if (dragMode == DragGestureMode.HORIZONTAL_SEEK) {
                                onSeekCommit(targetSeekMs)
                            }
                            break
                        }

                        val currentX = change.position.x
                        val currentY = change.position.y
                        val totalDx = currentX - downX
                        val totalDy = currentY - downY

                        if (dragMode == DragGestureMode.NONE) {
                            if (abs(totalDx) > touchSlop || abs(totalDy) > touchSlop) {
                                if (abs(totalDx) > abs(totalDy)) {
                                    dragMode = DragGestureMode.HORIZONTAL_SEEK
                                    initialSeekMs = controller.currentPosition.coerceAtLeast(0L)
                                    targetSeekMs = initialSeekMs
                                } else {
                                    if (isLeftSide) {
                                        dragMode = DragGestureMode.VERTICAL_BRIGHTNESS
                                        initialBrightness = getWindowBrightness(context)
                                    } else {
                                        dragMode = DragGestureMode.VERTICAL_VOLUME
                                        initialVolume = getDeviceVolumeFraction(context)
                                    }
                                }
                            }
                        }

                        if (dragMode != DragGestureMode.NONE) {
                            change.consume()
                            when (dragMode) {
                                DragGestureMode.HORIZONTAL_SEEK -> {
                                    if (durationMs > 0L) {
                                        val seekRangeMs = (durationMs.toFloat() * 0.20f).coerceIn(90_000f, 300_000f)
                                        val deltaFraction = totalDx / screenWidth
                                        val deltaMs = (deltaFraction * seekRangeMs).toLong()
                                        targetSeekMs = (initialSeekMs + deltaMs).coerceIn(0L, durationMs)
                                        onSeekPreview(
                                            SeekGestureState(
                                                targetMs = targetSeekMs,
                                                deltaMs = deltaMs,
                                                durationMs = durationMs,
                                            ),
                                        )
                                    }
                                }
                                DragGestureMode.VERTICAL_BRIGHTNESS -> {
                                    val deltaFraction = -totalDy / screenHeight
                                    val newBrightness = (initialBrightness + deltaFraction).coerceIn(0.01f, 1.0f)
                                    onBrightnessChange(newBrightness)
                                }
                                DragGestureMode.VERTICAL_VOLUME -> {
                                    val deltaFraction = -totalDy / screenHeight
                                    val newVolume = (initialVolume + deltaFraction).coerceIn(0.0f, 1.0f)
                                    onVolumeChange(newVolume)
                                }
                                DragGestureMode.NONE -> Unit
                            }
                        }
                    }
                }
            },
    )
}

/**
 * Double Tap Visual Arc Flash Overlay (YouTube / Bilibili style side ripple arcs).
 */
@Composable
private fun PlayerDoubleTapFlashOverlay(flashSide: String?) {
    AnimatedVisibility(
        visible = flashSide != null,
        enter = fadeIn(tween(100)) + scaleIn(initialScale = 0.9f),
        exit = fadeOut(tween(400)) + scaleOut(targetScale = 1.1f),
    ) {
        Box(modifier = Modifier.fillMaxSize()) {
            when (flashSide) {
                "left" -> {
                    Box(
                        modifier = Modifier
                            .fillMaxHeight()
                            .fillMaxWidth(0.32f)
                            .align(Alignment.CenterStart)
                            .background(
                                Brush.horizontalGradient(
                                    colors = listOf(
                                        MediaHubColors.Accent.copy(alpha = 0.28f),
                                        Color.Transparent,
                                    ),
                                ),
                            ),
                        contentAlignment = Alignment.Center,
                    ) {
                        Column(
                            horizontalAlignment = Alignment.CenterHorizontally,
                            verticalArrangement = Arrangement.spacedBy(8.dp),
                        ) {
                            MediaHubIcon(
                                imageVector = Lucide.RotateCcw,
                                contentDescription = null,
                                tint = MediaHubColors.TextPrimary,
                                modifier = Modifier.size(34.dp),
                            )
                            MediaHubText(
                                text = "-10 秒",
                                color = MediaHubColors.TextPrimary,
                                fontSize = 15.sp,
                                fontWeight = FontWeight.Bold,
                            )
                        }
                    }
                }

                "right" -> {
                    Box(
                        modifier = Modifier
                            .fillMaxHeight()
                            .fillMaxWidth(0.32f)
                            .align(Alignment.CenterEnd)
                            .background(
                                Brush.horizontalGradient(
                                    colors = listOf(
                                        Color.Transparent,
                                        MediaHubColors.Accent.copy(alpha = 0.28f),
                                    ),
                                ),
                            ),
                        contentAlignment = Alignment.Center,
                    ) {
                        Column(
                            horizontalAlignment = Alignment.CenterHorizontally,
                            verticalArrangement = Arrangement.spacedBy(8.dp),
                        ) {
                            MediaHubIcon(
                                imageVector = Lucide.RotateCw,
                                contentDescription = null,
                                tint = MediaHubColors.TextPrimary,
                                modifier = Modifier.size(34.dp),
                            )
                            MediaHubText(
                                text = "+10 秒",
                                color = MediaHubColors.TextPrimary,
                                fontSize = 15.sp,
                                fontWeight = FontWeight.Bold,
                            )
                        }
                    }
                }

                "center" -> {
                    Box(
                        modifier = Modifier
                            .size(86.dp)
                            .align(Alignment.Center)
                            .clip(CircleShape)
                            .background(MediaHubColors.Accent.copy(alpha = 0.92f)),
                        contentAlignment = Alignment.Center,
                    ) {
                        MediaHubIcon(
                            imageVector = Lucide.Play,
                            contentDescription = null,
                            tint = MediaHubColors.OnAccent,
                            modifier = Modifier.size(38.dp),
                        )
                    }
                }
            }
        }
    }
}

/**
 * Edge-Pinned Slim Vertical Pill HUDs for Brightness (Left) and Volume (Right).
 * Completely safe from blocking movie subtitles.
 */
@Composable
private fun PlayerEdgeHuds(
    brightness: Float?,
    volume: Float?,
    modifier: Modifier = Modifier,
) {
    Box(modifier = modifier) {
        // Left Edge: Slim Brightness Vertical Pill
        AnimatedVisibility(
            visible = brightness != null,
            enter = fadeIn() + slideInHorizontally(initialOffsetX = { -it }),
            exit = fadeOut() + slideOutHorizontally(targetOffsetX = { -it }),
            modifier = Modifier
                .align(Alignment.CenterStart)
                .padding(start = 24.dp),
        ) {
            if (brightness != null) {
                Column(
                    modifier = Modifier
                        .width(48.dp)
                        .height(150.dp)
                        .clip(MediaHubShapes.Nav)
                        .background(MediaHubColors.Surface.copy(alpha = 0.92f))
                        .padding(vertical = 10.dp),
                    horizontalAlignment = Alignment.CenterHorizontally,
                    verticalArrangement = Arrangement.SpaceBetween,
                ) {
                    MediaHubIcon(
                        imageVector = Lucide.Sun,
                        contentDescription = null,
                        tint = MediaHubColors.Warning,
                        modifier = Modifier.size(18.dp),
                    )

                    Box(
                        modifier = Modifier
                            .width(4.dp)
                            .height(72.dp)
                            .clip(RoundedCornerShape(2.dp))
                            .background(MediaHubColors.NeutralContainer),
                        contentAlignment = Alignment.BottomCenter,
                    ) {
                        Box(
                            modifier = Modifier
                                .fillMaxWidth()
                                .fillMaxHeight(brightness.coerceIn(0f, 1f))
                                .background(MediaHubColors.Warning),
                        )
                    }

                    MediaHubText(
                        text = "${(brightness * 100).roundToInt()}%",
                        color = MediaHubColors.TextPrimary,
                        fontSize = 12.sp,
                        fontWeight = FontWeight.SemiBold,
                    )
                }
            }
        }

        // Right Edge: Slim Volume Vertical Pill
        AnimatedVisibility(
            visible = volume != null,
            enter = fadeIn() + slideInHorizontally(initialOffsetX = { it }),
            exit = fadeOut() + slideOutHorizontally(targetOffsetX = { it }),
            modifier = Modifier
                .align(Alignment.CenterEnd)
                .padding(end = 24.dp),
        ) {
            if (volume != null) {
                Column(
                    modifier = Modifier
                        .width(48.dp)
                        .height(150.dp)
                        .clip(MediaHubShapes.Nav)
                        .background(MediaHubColors.Surface.copy(alpha = 0.92f))
                        .padding(vertical = 10.dp),
                    horizontalAlignment = Alignment.CenterHorizontally,
                    verticalArrangement = Arrangement.SpaceBetween,
                ) {
                    MediaHubIcon(
                        imageVector = if (volume <= 0.01f) Lucide.VolumeX else Lucide.Volume2,
                        contentDescription = null,
                        tint = MediaHubColors.Accent,
                        modifier = Modifier.size(18.dp),
                    )

                    Box(
                        modifier = Modifier
                            .width(4.dp)
                            .height(72.dp)
                            .clip(RoundedCornerShape(2.dp))
                            .background(MediaHubColors.NeutralContainer),
                        contentAlignment = Alignment.BottomCenter,
                    ) {
                        Box(
                            modifier = Modifier
                                .fillMaxWidth()
                                .fillMaxHeight(volume.coerceIn(0f, 1f))
                                .background(MediaHubColors.Accent),
                        )
                    }

                    MediaHubText(
                        text = "${(volume * 100).roundToInt()}%",
                        color = MediaHubColors.TextPrimary,
                        fontSize = 12.sp,
                        fontWeight = FontWeight.SemiBold,
                    )
                }
            }
        }
    }
}

/**
 * Center HUD overlay for Horizontal Swipe Seek Gesture.
 */
@Composable
private fun PlayerCenterSeekHud(
    seekState: SeekGestureState?,
    modifier: Modifier = Modifier,
) {
    AnimatedVisibility(
        visible = seekState != null,
        enter = fadeIn() + scaleIn(initialScale = 0.9f),
        exit = fadeOut() + scaleOut(targetScale = 0.9f),
        modifier = modifier,
    ) {
        if (seekState != null) {
            Box(
                modifier = Modifier
                    .clip(MediaHubShapes.Card)
                    .background(MediaHubColors.Surface.copy(alpha = 0.94f))
                    .padding(horizontal = 26.dp, vertical = 16.dp),
                contentAlignment = Alignment.Center,
            ) {
                Column(
                    horizontalAlignment = Alignment.CenterHorizontally,
                    verticalArrangement = Arrangement.spacedBy(8.dp),
                    modifier = Modifier.widthIn(min = 200.dp),
                ) {
                    Row(
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(8.dp),
                    ) {
                        MediaHubIcon(
                            imageVector = if (seekState.deltaMs >= 0) Lucide.RotateCw else Lucide.RotateCcw,
                            contentDescription = null,
                            tint = MediaHubColors.Accent,
                            modifier = Modifier.size(20.dp),
                        )
                        MediaHubText(
                            text = formatDeltaTime(seekState.deltaMs),
                            color = MediaHubColors.Accent,
                            fontSize = 17.sp,
                            fontWeight = FontWeight.Bold,
                        )
                    }

                    Row(
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(6.dp),
                    ) {
                        MediaHubText(
                            text = formatPlaybackPosition(seekState.targetMs),
                            color = MediaHubColors.TextPrimary,
                            fontSize = 15.sp,
                            fontWeight = FontWeight.SemiBold,
                        )
                        MediaHubText(
                            text = "/",
                            color = MediaHubColors.TextMuted,
                            fontSize = 13.sp,
                        )
                        MediaHubText(
                            text = formatPlaybackPosition(seekState.durationMs),
                            color = MediaHubColors.TextSecondary,
                            fontSize = 13.sp,
                        )
                    }

                    val progressFraction = if (seekState.durationMs > 0L) {
                        (seekState.targetMs.toFloat() / seekState.durationMs.toFloat()).coerceIn(0f, 1f)
                    } else 0f
                    Box(
                        modifier = Modifier
                            .fillMaxWidth()
                            .height(4.dp)
                            .clip(RoundedCornerShape(2.dp))
                            .background(MediaHubColors.NeutralContainer),
                        contentAlignment = Alignment.CenterStart,
                    ) {
                        Box(
                            modifier = Modifier
                                .fillMaxWidth(progressFraction)
                                .fillMaxHeight()
                                .background(MediaHubColors.Accent),
                        )
                    }
                }
            }
        }
    }
}

@androidx.annotation.OptIn(UnstableApi::class)
@Composable
private fun PlayerTrackSelectionDrawer(
    player: Player,
    embyItemId: String?,
    subtitleState: RemoteSubtitleUiState,
    subtitleActions: PlayerSubtitleActions,
    onDismiss: () -> Unit,
) {
    var selectedTab by remember { mutableIntStateOf(0) }
    val haptic = LocalHapticFeedback.current
    val canSearchRemoteSubtitles = !embyItemId.isNullOrBlank()
    val currentTracks = rememberPlayerTracks(player)
    val subtitleOptions = playerTrackOptions(currentTracks, C.TRACK_TYPE_TEXT)
    val audioOptions = playerTrackOptions(currentTracks, C.TRACK_TYPE_AUDIO)
    LaunchedEffect(currentTracks) {
        if (audioOptions.isNotEmpty() && audioOptions.none { it.selected }) {
            player.ensureDefaultAudioTrack()
        }
    }
    val canSelectTracks = player.isCommandAvailable(Player.COMMAND_SET_TRACK_SELECTION_PARAMETERS)
    val isSubtitlesDisabled = player.trackSelectionParameters.disabledTrackTypes.contains(C.TRACK_TYPE_TEXT) ||
        subtitleOptions.none { it.selected }

    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(Color.Black.copy(alpha = 0.35f))
            .clickable(
                indication = null,
                interactionSource = remember { MutableInteractionSource() },
                onClick = onDismiss,
            ),
        contentAlignment = Alignment.CenterEnd,
    ) {
        Column(
            modifier = Modifier
                .padding(end = 12.dp, top = 16.dp, bottom = 16.dp)
                .width(280.dp)
                .heightIn(max = 380.dp)
                .clickable(
                    indication = null,
                    interactionSource = remember { MutableInteractionSource() },
                    onClick = {},
                )
                .clip(MediaHubShapes.Card)
                .background(MediaHubColors.Surface)
                .padding(12.dp),
            verticalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                MediaHubText(
                    text = "音轨与字幕",
                    color = MediaHubColors.TextPrimary,
                    fontSize = 14.sp,
                    fontWeight = FontWeight.SemiBold,
                )
                PlayerIconButton(
                    imageVector = Lucide.X,
                    contentDescription = "关闭",
                    onClick = onDismiss,
                    modifier = Modifier.size(48.dp),
                    iconSize = 16.dp,
                )
            }

            MediaHubSegmentedControl(
                options = listOf(
                    "subtitles" to "字幕 (${subtitleOptions.size})",
                    "audio" to "音轨 (${audioOptions.size})",
                ),
                selected = if (selectedTab == 0) "subtitles" else "audio",
                onSelected = { value ->
                    haptic.performHapticFeedback(HapticFeedbackType.TextHandleMove)
                    selectedTab = if (value == "audio") 1 else 0
                },
            )

            if (!canSelectTracks) {
                MediaHubText(
                    text = "当前无法切换音轨或字幕",
                    color = MediaHubColors.TextMuted,
                    fontSize = 12.sp,
                )
            }

            if (selectedTab == 0) {
                PlayerSubtitleOffsetRow()
            }

            LazyColumn(
                modifier = Modifier.fillMaxWidth().weight(1f, fill = false),
                verticalArrangement = Arrangement.spacedBy(4.dp),
            ) {
                if (selectedTab == 0) {
                    if (canSearchRemoteSubtitles) {
                        item {
                            PlayerRemoteSubtitleSection(
                                subtitleState = subtitleState,
                                onSearch = {
                                    haptic.performHapticFeedback(HapticFeedbackType.TextHandleMove)
                                    subtitleActions.onSearch()
                                },
                                onDownload = subtitleActions.onDownload,
                                onClear = subtitleActions.onClear,
                            )
                        }
                    }

                    item {
                        PlayerTrackItem(
                            title = "关闭字幕",
                            subtitle = "不显示字幕",
                            isSelected = isSubtitlesDisabled,
                            onClick = {
                                haptic.performHapticFeedback(HapticFeedbackType.TextHandleMove)
                                player.disableTrackType(C.TRACK_TYPE_TEXT)
                            },
                        )
                    }

                    items(subtitleOptions) { option ->
                        PlayerTrackItem(
                            title = option.title,
                            subtitle = option.detail,
                            isSelected = option.selected && !isSubtitlesDisabled,
                            enabled = option.supported && canSelectTracks,
                            onClick = {
                                haptic.performHapticFeedback(HapticFeedbackType.TextHandleMove)
                                player.applyTrackOption(option)
                            },
                        )
                    }
                } else if (audioOptions.isEmpty()) {
                    item {
                        MediaHubText(
                            text = "当前片源没有可识别的音轨",
                            color = MediaHubColors.TextMuted,
                            fontSize = 12.sp,
                            modifier = Modifier.padding(12.dp),
                        )
                    }
                } else {
                    if (audioOptions.size == 1) {
                        item {
                            MediaHubText(
                                text = "当前片源只有 1 条音轨",
                                color = MediaHubColors.TextMuted,
                                fontSize = 12.sp,
                                modifier = Modifier.padding(horizontal = 4.dp, vertical = 2.dp),
                            )
                        }
                    }
                    items(audioOptions) { option ->
                        PlayerTrackItem(
                            title = option.title,
                            subtitle = option.detail,
                            isSelected = option.selected,
                            enabled = canSelectTracks,
                            onClick = {
                                haptic.performHapticFeedback(HapticFeedbackType.TextHandleMove)
                                player.applyTrackOption(option)
                            },
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun PlayerSubtitleOffsetRow() {
    val haptic = LocalHapticFeedback.current
    var offsetMs by remember { mutableLongStateOf(SubtitleTiming.offsetMs) }
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .clip(MediaHubShapes.Control)
            .background(MediaHubColors.SurfaceHigh)
            .padding(horizontal = 4.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Box(
            modifier = Modifier
                .size(48.dp)
                .clickable {
                    haptic.performHapticFeedback(HapticFeedbackType.TextHandleMove)
                    SubtitleTiming.shift(-SubtitleTiming.StepMs)
                    offsetMs = SubtitleTiming.offsetMs
                }
                .semantics { contentDescription = "字幕提前 0.1 秒" },
            contentAlignment = Alignment.Center,
        ) {
            MediaHubIcon(
                imageVector = Lucide.Minus,
                contentDescription = null,
                tint = MediaHubColors.TextPrimary,
                modifier = Modifier.size(16.dp),
            )
        }
        Box(
            modifier = Modifier
                .weight(1f)
                .heightIn(min = 48.dp)
                .clickable {
                    haptic.performHapticFeedback(HapticFeedbackType.TextHandleMove)
                    SubtitleTiming.reset()
                    offsetMs = 0L
                }
                .semantics { contentDescription = "重置字幕轴" },
            contentAlignment = Alignment.Center,
        ) {
            MediaHubText(
                text = formatSubtitleOffset(offsetMs),
                color = MediaHubColors.TextPrimary,
                fontSize = 12.sp,
                fontWeight = FontWeight.Medium,
            )
        }
        Box(
            modifier = Modifier
                .size(48.dp)
                .clickable {
                    haptic.performHapticFeedback(HapticFeedbackType.TextHandleMove)
                    SubtitleTiming.shift(SubtitleTiming.StepMs)
                    offsetMs = SubtitleTiming.offsetMs
                }
                .semantics { contentDescription = "字幕延后 0.1 秒" },
            contentAlignment = Alignment.Center,
        ) {
            MediaHubIcon(
                imageVector = Lucide.Plus,
                contentDescription = null,
                tint = MediaHubColors.TextPrimary,
                modifier = Modifier.size(16.dp),
            )
        }
    }
}

@Composable
private fun PlayerRemoteSubtitleSection(
    subtitleState: RemoteSubtitleUiState,
    onSearch: () -> Unit,
    onDownload: (String) -> Unit,
    onClear: () -> Unit,
) {
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .clip(MediaHubShapes.Control)
            .background(MediaHubColors.SurfaceHigh)
            .padding(12.dp),
        verticalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            MediaHubText(
                text = "远程字幕",
                color = MediaHubColors.TextPrimary,
                fontSize = 13.sp,
                fontWeight = FontWeight.SemiBold,
            )
            if (subtitleState.targetId != null || subtitleState.remoteSubtitles.isNotEmpty()) {
                PlayerIconButton(
                    imageVector = Lucide.X,
                    contentDescription = "关闭字幕结果",
                    onClick = onClear,
                    modifier = Modifier.size(48.dp),
                    iconSize = 16.dp,
                )
            }
        }
        Box(
            modifier = Modifier
                .fillMaxWidth()
                .heightIn(min = 48.dp)
                .clip(MediaHubShapes.Control)
                .background(MediaHubColors.AccentLight)
                .clickable(enabled = !subtitleState.searching, onClick = onSearch)
                .padding(horizontal = 12.dp, vertical = 10.dp),
            contentAlignment = Alignment.Center,
        ) {
            Row(
                horizontalArrangement = Arrangement.spacedBy(8.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                MediaHubIcon(
                    imageVector = Lucide.Captions,
                    contentDescription = null,
                    tint = MediaHubColors.Accent,
                    modifier = Modifier.size(16.dp),
                )
                MediaHubText(
                    text = if (subtitleState.searching) "正在搜索中文字幕…" else "搜中文字幕",
                    color = MediaHubColors.Accent,
                    fontSize = 13.sp,
                    fontWeight = FontWeight.Medium,
                )
            }
        }
        subtitleState.message?.let { message ->
            MediaHubText(
                text = message,
                color = MediaHubColors.TextMuted,
                fontSize = 12.sp,
            )
        }
        subtitleState.remoteSubtitles.forEach { subtitle ->
            PlayerRemoteSubtitleRow(
                subtitle = subtitle,
                downloading = subtitleState.downloadingId == subtitle.id,
                enabled = subtitleState.downloadingId == null,
                onDownload = { onDownload(subtitle.id) },
            )
        }
    }
}

@Composable
private fun PlayerRemoteSubtitleRow(
    subtitle: EmbyRemoteSubtitle,
    downloading: Boolean,
    enabled: Boolean,
    onDownload: () -> Unit,
) {
    val meta = listOfNotNull(
        subtitle.format.takeIf { it.isNotBlank() }?.uppercase(),
        subtitle.providerName.takeIf { it.isNotBlank() },
        if (subtitle.isHashMatch) "精确匹配" else null,
    ).joinToString(" · ")
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .clip(MediaHubShapes.Control)
            .background(MediaHubColors.Canvas)
            .padding(horizontal = 10.dp, vertical = 8.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Column(modifier = Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
            MediaHubText(
                text = subtitle.name,
                color = MediaHubColors.TextPrimary,
                fontSize = 12.sp,
                fontWeight = FontWeight.Medium,
                maxLines = 2,
                overflow = TextOverflow.Ellipsis,
            )
            if (meta.isNotBlank()) {
                MediaHubText(
                    text = meta,
                    color = MediaHubColors.TextMuted,
                    fontSize = 12.sp,
                )
            }
        }
        Box(
            modifier = Modifier
                .heightIn(min = 48.dp)
                .clip(MediaHubShapes.Control)
                .background(if (enabled) MediaHubColors.Accent else MediaHubColors.NeutralContainer)
                .clickable(enabled = enabled, onClick = onDownload)
                .padding(horizontal = 12.dp, vertical = 8.dp),
            contentAlignment = Alignment.Center,
        ) {
            MediaHubText(
                text = if (downloading) "下载中…" else "下载",
                color = if (enabled) MediaHubColors.OnAccent else MediaHubColors.TextMuted,
                fontSize = 12.sp,
                fontWeight = FontWeight.Medium,
            )
        }
    }
}

@Composable
private fun PlayerTrackItem(
    title: String,
    subtitle: String,
    isSelected: Boolean,
    onClick: () -> Unit,
    enabled: Boolean = true,
) {
    Box(
        modifier = Modifier
            .fillMaxWidth()
            .heightIn(min = 48.dp)
            .clip(MediaHubShapes.Control)
            .background(if (isSelected) MediaHubColors.SurfaceSelected else Color.Transparent)
            .clickable(enabled = enabled, onClick = onClick)
            .padding(horizontal = 10.dp, vertical = 8.dp),
    ) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Column(verticalArrangement = Arrangement.spacedBy(2.dp)) {
                MediaHubText(
                    text = title,
                    color = if (enabled) MediaHubColors.TextPrimary else MediaHubColors.TextMuted,
                    fontSize = 13.sp,
                    fontWeight = if (isSelected) FontWeight.SemiBold else FontWeight.Medium,
                )
                MediaHubText(
                    text = subtitle,
                    color = MediaHubColors.TextMuted,
                    fontSize = 12.sp,
                )
            }
            if (isSelected) {
                MediaHubIcon(
                    imageVector = Lucide.Check,
                    contentDescription = "已选择",
                    tint = MediaHubColors.Accent,
                    modifier = Modifier.size(16.dp),
                )
            }
        }
    }
}

@Composable
private fun PlayerAspectRatioDialog(
    current: PlayerAspectRatio,
    onSelect: (PlayerAspectRatio) -> Unit,
    onDismiss: () -> Unit,
) {
    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(Color.Black.copy(alpha = 0.28f))
            .clickable(
                indication = null,
                interactionSource = remember { MutableInteractionSource() },
                onClick = onDismiss,
            ),
        contentAlignment = Alignment.TopEnd,
    ) {
        Column(
            modifier = Modifier
                .padding(top = 64.dp, end = 16.dp)
                .width(168.dp)
                .clip(MediaHubShapes.Card)
                .background(MediaHubColors.Surface)
                .clickable(
                    indication = null,
                    interactionSource = remember { MutableInteractionSource() },
                    onClick = {},
                )
                .padding(6.dp),
            verticalArrangement = Arrangement.spacedBy(2.dp),
        ) {
            MediaHubText(
                text = "画面比例",
                color = MediaHubColors.TextMuted,
                fontSize = 12.sp,
                modifier = Modifier.padding(horizontal = 10.dp, vertical = 4.dp),
            )
            PlayerAspectRatio.options.forEach { mode ->
                val isSelected = mode == current
                Box(
                    modifier = Modifier
                        .fillMaxWidth()
                        .heightIn(min = 48.dp)
                        .clip(MediaHubShapes.Control)
                        .background(if (isSelected) MediaHubColors.SurfaceSelected else Color.Transparent)
                        .clickable { onSelect(mode) }
                        .semantics { contentDescription = mode.description }
                        .padding(horizontal = 10.dp),
                    contentAlignment = Alignment.CenterStart,
                ) {
                    MediaHubText(
                        text = mode.label,
                        color = if (isSelected) MediaHubColors.Accent else MediaHubColors.TextPrimary,
                        fontSize = 13.sp,
                        fontWeight = if (isSelected) FontWeight.SemiBold else FontWeight.Medium,
                    )
                }
            }
        }
    }
}

@Composable
private fun PlayerSpeedDialog(
    currentSpeed: Float,
    onSelectSpeed: (Float) -> Unit,
    onDismiss: () -> Unit,
) {
    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(Color.Black.copy(alpha = 0.28f))
            .clickable(
                indication = null,
                interactionSource = remember { MutableInteractionSource() },
                onClick = onDismiss,
            ),
        contentAlignment = Alignment.BottomEnd,
    ) {
        Row(
            modifier = Modifier
                .padding(end = 16.dp, bottom = 72.dp)
                .clip(MediaHubShapes.Card)
                .background(MediaHubColors.Surface)
                .padding(6.dp),
            horizontalArrangement = Arrangement.spacedBy(4.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            PlaybackSpeeds.forEach { speed ->
                val isSelected = abs(speed - currentSpeed) < 0.05f
                Box(
                    modifier = Modifier
                        .heightIn(min = 48.dp)
                        .widthIn(min = 48.dp)
                        .clip(MediaHubShapes.Control)
                        .background(if (isSelected) MediaHubColors.SurfaceSelected else Color.Transparent)
                        .clickable { onSelectSpeed(speed) }
                        .padding(horizontal = 8.dp),
                    contentAlignment = Alignment.Center,
                ) {
                    MediaHubText(
                        text = "${speed}x",
                        color = if (isSelected) MediaHubColors.Accent else MediaHubColors.TextPrimary,
                        fontSize = 12.sp,
                        fontWeight = if (isSelected) FontWeight.SemiBold else FontWeight.Medium,
                    )
                }
            }
        }
    }
}

@Composable
private fun PlayerIconButton(
    imageVector: androidx.compose.ui.graphics.vector.ImageVector,
    contentDescription: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    iconSize: androidx.compose.ui.unit.Dp = 20.dp,
) {
    Box(
        modifier = modifier
            .clip(CircleShape)
            .background(MediaHubColors.SurfaceHigh.copy(alpha = 0.92f))
            .clickable(
                role = Role.Button,
                onClick = onClick,
            )
            .semantics { this.contentDescription = contentDescription },
        contentAlignment = Alignment.Center,
    ) {
        MediaHubIcon(
            imageVector = imageVector,
            contentDescription = contentDescription,
            tint = MediaHubColors.TextPrimary,
            modifier = Modifier.size(iconSize),
        )
    }
}

@Composable
private fun PlayerChipButton(
    label: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    icon: androidx.compose.ui.graphics.vector.ImageVector? = null,
    emphasized: Boolean = false,
) {
    Box(
        modifier = modifier
            .heightIn(min = 48.dp)
            .clip(MediaHubShapes.Chip)
            .background(if (emphasized) MediaHubColors.Accent else MediaHubColors.SurfaceHigh.copy(alpha = 0.92f))
            .clickable(
                role = Role.Button,
                onClick = onClick,
            )
            .padding(horizontal = 14.dp, vertical = 6.dp),
        contentAlignment = Alignment.Center,
    ) {
        Row(
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(6.dp),
        ) {
            if (icon != null) {
                MediaHubIcon(
                    imageVector = icon,
                    contentDescription = null,
                    tint = if (emphasized) MediaHubColors.OnAccent else MediaHubColors.TextPrimary,
                    modifier = Modifier.size(15.dp),
                )
            }
            MediaHubText(
                text = label,
                color = if (emphasized) MediaHubColors.OnAccent else MediaHubColors.TextPrimary,
                fontSize = 12.sp,
                fontWeight = FontWeight.SemiBold,
            )
        }
    }
}

@androidx.annotation.OptIn(UnstableApi::class)
internal fun createPlayerView(context: Context): PlayerView = PlayerView(context).apply {
    setShowBuffering(PlayerView.SHOW_BUFFERING_NEVER)
    useController = false
    controllerAutoShow = false
    resizeMode = AspectRatioFrameLayout.RESIZE_MODE_FIT
}

// Helpers for volume and brightness gestures
private fun getWindowBrightness(context: Context): Float {
    val activity = context as? Activity ?: return 0.5f
    val current = activity.window.attributes.screenBrightness
    return if (current in 0.0f..1.0f) current else 0.5f
}

private fun setWindowBrightness(context: Context, brightness: Float) {
    val activity = context as? Activity ?: return
    val layout = activity.window.attributes
    layout.screenBrightness = brightness.coerceIn(0.01f, 1.0f)
    activity.window.attributes = layout
}

private fun getDeviceVolumeFraction(context: Context): Float {
    val audioManager = context.getSystemService(Context.AUDIO_SERVICE) as? AudioManager ?: return 0.5f
    val max = audioManager.getStreamMaxVolume(AudioManager.STREAM_MUSIC).coerceAtLeast(1)
    val current = audioManager.getStreamVolume(AudioManager.STREAM_MUSIC)
    return (current.toFloat() / max.toFloat()).coerceIn(0f, 1f)
}

private fun setDeviceVolume(context: Context, fraction: Float) {
    val audioManager = context.getSystemService(Context.AUDIO_SERVICE) as? AudioManager ?: return
    val max = audioManager.getStreamMaxVolume(AudioManager.STREAM_MUSIC)
    val target = (fraction * max).roundToInt().coerceIn(0, max)
    audioManager.setStreamVolume(AudioManager.STREAM_MUSIC, target, 0)
}

private fun formatDeltaTime(deltaMs: Long): String {
    val sign = if (deltaMs >= 0) "+" else "-"
    val absMs = abs(deltaMs)
    val totalSeconds = absMs / 1000
    val minutes = totalSeconds / 60
    val seconds = totalSeconds % 60
    return if (minutes > 0) {
        String.format(java.util.Locale.US, "%s%02d:%02d", sign, minutes, seconds)
    } else {
        String.format(java.util.Locale.US, "%s%d 秒", sign, seconds)
    }
}
