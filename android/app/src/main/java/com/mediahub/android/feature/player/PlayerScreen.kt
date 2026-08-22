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
import androidx.compose.foundation.border
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
import androidx.compose.ui.draw.shadow
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
import androidx.media3.common.TrackSelectionOverride
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
import com.composables.icons.lucide.Pause
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
import com.mediahub.android.core.designsystem.MediaHubText
import com.mediahub.android.core.network.EmbyRemoteSubtitle
import com.mediahub.android.feature.library.formatPlaybackPosition
import com.mediahub.android.feature.subtitles.RemoteSubtitleUiState
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
                PlayerFrostedCircleButton(
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
                    modifier = Modifier.size(46.dp),
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

        // 8. Center Playback Controls (The Cinematic Liquid Core)
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
                        verticalArrangement = Arrangement.spacedBy(16.dp),
                        modifier = Modifier
                            .clip(RoundedCornerShape(20.dp))
                            .background(Color(0xE6111827))
                            .border(0.5.dp, Color.White.copy(alpha = 0.15f), RoundedCornerShape(20.dp))
                            .padding(horizontal = 32.dp, vertical = 24.dp),
                    ) {
                        CircularProgressIndicator(
                            color = Color(0xFF38BDF8),
                            modifier = Modifier.size(38.dp),
                            strokeWidth = 3.5.dp,
                        )
                        MediaHubText(
                            text = "正在准备视频…",
                            color = Color.White,
                            fontSize = 14.sp,
                            fontWeight = FontWeight.Medium,
                        )
                    }
                }
            }

            state is PlayerUiState.Error -> {
                Column(
                    modifier = Modifier
                        .align(Alignment.Center)
                        .fillMaxWidth()
                        .padding(horizontal = 24.dp)
                        .widthIn(max = 440.dp)
                        .clip(RoundedCornerShape(24.dp))
                        .background(Color(0xF20F172A))
                        .border(0.5.dp, Color.White.copy(alpha = 0.18f), RoundedCornerShape(24.dp))
                        .padding(28.dp),
                    horizontalAlignment = Alignment.CenterHorizontally,
                    verticalArrangement = Arrangement.spacedBy(16.dp),
                ) {
                    MediaHubIcon(
                        imageVector = Lucide.CircleAlert,
                        contentDescription = null,
                        tint = MediaHubColors.Error,
                        modifier = Modifier.size(44.dp),
                    )
                    MediaHubText(
                        text = state.message,
                        color = Color.White,
                        fontSize = 15.sp,
                        fontWeight = FontWeight.Medium,
                    )
                    if (state.retryable) {
                        PlayerLiquidPillButton(
                            label = "重试",
                            onClick = actions.onRetry,
                            modifier = Modifier.height(48.dp),
                        )
                    }
                }
            }

            else -> Unit
        }

        // 11. Modern Frosted Audio & Subtitle Side Drawer
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
            PlayerFrostedCircleButton(
                imageVector = Lucide.ArrowLeft,
                contentDescription = "返回",
                onClick = actions.onBack,
                modifier = Modifier.size(40.dp),
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
                    color = Color.White,
                    fontSize = 16.sp,
                    fontWeight = FontWeight.Bold,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
                Row(
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(6.dp),
                ) {
                    Box(
                        modifier = Modifier
                            .clip(RoundedCornerShape(4.dp))
                            .background(Color(0x2638BDF8))
                            .border(0.5.dp, Color(0x5538BDF8), RoundedCornerShape(4.dp))
                            .padding(horizontal = 5.dp, vertical = 1.dp),
                    ) {
                        MediaHubText(
                            text = "115 极速直链",
                            color = Color(0xFF7DD3FC),
                            fontSize = 10.sp,
                            fontWeight = FontWeight.SemiBold,
                        )
                    }
                    MediaHubText(
                        text = "·",
                        color = Color.White.copy(alpha = 0.4f),
                        fontSize = 11.sp,
                    )
                    MediaHubText(
                        text = "4K 原画 · 杜比音效",
                        color = Color.White.copy(alpha = 0.60f),
                        fontSize = 11.sp,
                        fontWeight = FontWeight.Normal,
                    )
                }
            }

            // Aspect Ratio Liquid Pill Button
            PlayerLiquidPillButton(
                label = aspectRatio.label,
                onClick = onOpenAspectRatio,
            )

            // Screen Rotation Button
            PlayerFrostedCircleButton(
                imageVector = Lucide.Maximize,
                contentDescription = "旋转屏幕",
                onClick = actions.onToggleOrientation,
                modifier = Modifier.size(40.dp),
                iconSize = 18.dp,
            )

            // Picture in Picture Button
            PlayerFrostedCircleButton(
                imageVector = Lucide.PictureInPicture2,
                contentDescription = "进入画中画",
                onClick = actions.onEnterPictureInPicture,
                modifier = Modifier.size(40.dp),
                iconSize = 18.dp,
            )
        }
    }
}

/**
 * Center screen playback controls (The Cinematic Liquid Core).
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
        horizontalArrangement = Arrangement.spacedBy(40.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        // Rewind 10s Button
        PlayerFrostedCircleButton(
            imageVector = Lucide.RotateCcw,
            contentDescription = "后退 10 秒",
            onClick = {
                haptic.performHapticFeedback(HapticFeedbackType.LongPress)
                onUserInteraction()
                controller.seekBack()
            },
            modifier = Modifier.size(54.dp),
            iconSize = 24.dp,
        )

        // Main Play / Pause Luminous Core
        Box(
            modifier = Modifier
                .size(76.dp)
                .shadow(24.dp, CircleShape, spotColor = Color(0x9038BDF8))
                .clip(CircleShape)
                .background(Color.White.copy(alpha = 0.20f))
                .border(1.5.dp, Color.White.copy(alpha = 0.45f), CircleShape)
                .clickable(
                    role = Role.Button,
                    onClick = {
                        haptic.performHapticFeedback(HapticFeedbackType.TextHandleMove)
                        onUserInteraction()
                        controller.playWhenReady = !controller.playWhenReady
                    },
                ),
            contentAlignment = Alignment.Center,
        ) {
            MediaHubIcon(
                imageVector = if (isPlaying) Lucide.Pause else Lucide.Play,
                contentDescription = if (isPlaying) "暂停" else "播放",
                tint = Color.White,
                modifier = Modifier.size(36.dp),
            )
        }

        // Forward 10s Button
        PlayerFrostedCircleButton(
            imageVector = Lucide.RotateCw,
            contentDescription = "前进 10 秒",
            onClick = {
                haptic.performHapticFeedback(HapticFeedbackType.LongPress)
                onUserInteraction()
                controller.seekForward()
            },
            modifier = Modifier.size(54.dp),
            iconSize = 24.dp,
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
                        .clip(RoundedCornerShape(12.dp))
                        .background(Color(0xF00B101B))
                        .border(0.5.dp, Color(0xFF38BDF8).copy(alpha = 0.5f), RoundedCornerShape(12.dp))
                        .padding(horizontal = 16.dp, vertical = 6.dp),
                ) {
                    Row(
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(6.dp),
                    ) {
                        MediaHubText(
                            text = formatPlaybackPosition(displayPositionMs),
                            color = Color(0xFF38BDF8),
                            fontSize = 15.sp,
                            fontWeight = FontWeight.Bold,
                        )
                        MediaHubText(
                            text = "/",
                            color = Color.White.copy(alpha = 0.4f),
                            fontSize = 13.sp,
                        )
                        MediaHubText(
                            text = formatPlaybackPosition(durationMs),
                            color = Color.White.copy(alpha = 0.8f),
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
                color = Color.White,
                fontSize = 13.sp,
                fontWeight = FontWeight.SemiBold,
                modifier = Modifier.widthIn(min = 45.dp),
            )

            // Precision Material 3 Slider with Custom Buffered Track & Glowing Thumb
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
                            .shadow(6.dp, CircleShape, spotColor = Color(0xFF38BDF8))
                            .clip(CircleShape)
                            .background(Color.White)
                            .border(2.dp, Color(0xFF38BDF8), CircleShape),
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
                            .background(Color.White.copy(alpha = 0.20f)),
                        contentAlignment = Alignment.CenterStart,
                    ) {
                        // Buffered Track
                        if (bufferedFraction > 0f) {
                            Box(
                                modifier = Modifier
                                    .fillMaxWidth(bufferedFraction.coerceIn(0f, 1f))
                                    .fillMaxHeight()
                                    .background(Color.White.copy(alpha = 0.35f)),
                            )
                        }
                        // Played Track (Cyan-Blue Glow Gradient)
                        Box(
                            modifier = Modifier
                                .fillMaxWidth(fraction.coerceIn(0f, 1f))
                                .fillMaxHeight()
                                .background(
                                    Brush.horizontalGradient(
                                        colors = listOf(
                                            Color(0xFF38BDF8),
                                            Color(0xFF2563EB),
                                        ),
                                    ),
                                ),
                        )
                    }
                },
                colors = SliderDefaults.colors(
                    thumbColor = Color.White,
                    activeTrackColor = Color(0xFF38BDF8),
                    inactiveTrackColor = Color.White.copy(alpha = 0.20f),
                ),
                modifier = Modifier
                    .weight(1f)
                    .height(36.dp),
            )

            MediaHubText(
                text = if (durationMs > 0L) formatPlaybackPosition(durationMs) else "--:--",
                color = Color.White.copy(alpha = 0.65f),
                fontSize = 13.sp,
                fontWeight = FontWeight.Normal,
                modifier = Modifier.widthIn(min = 45.dp),
            )
        }

        // Row 2: Secondary Quick Actions Row (Streamlined, No duplicate buttons)
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            // Left Status Pill
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(6.dp),
            ) {
                Box(
                    modifier = Modifier
                        .size(6.dp)
                        .clip(CircleShape)
                        .background(Color(0xFF34D399)),
                )
                MediaHubText(
                    text = "硬件解码 · 极速缓冲",
                    color = Color.White.copy(alpha = 0.45f),
                    fontSize = 11.sp,
                    fontWeight = FontWeight.Medium,
                )
            }

            // Right Utilities: Speed Capsule & Audio/Subtitle Drawer Trigger
            Row(
                horizontalArrangement = Arrangement.spacedBy(10.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                PlayerLiquidPillButton(
                    label = if (currentSpeed == 1.0f) "倍速" else "${currentSpeed}x",
                    onClick = {
                        onUserInteraction()
                        onOpenSpeedDialog()
                    },
                )

                PlayerLiquidPillButton(
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
                                        Color(0x4538BDF8),
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
                                tint = Color.White,
                                modifier = Modifier.size(34.dp),
                            )
                            MediaHubText(
                                text = "-10 秒",
                                color = Color.White,
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
                                        Color(0x4538BDF8),
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
                                tint = Color.White,
                                modifier = Modifier.size(34.dp),
                            )
                            MediaHubText(
                                text = "+10 秒",
                                color = Color.White,
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
                            .background(Color.Black.copy(alpha = 0.60f))
                            .border(0.5.dp, Color.White.copy(alpha = 0.30f), CircleShape),
                        contentAlignment = Alignment.Center,
                    ) {
                        MediaHubIcon(
                            imageVector = Lucide.Play,
                            contentDescription = null,
                            tint = Color.White,
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
                        .width(38.dp)
                        .height(150.dp)
                        .clip(RoundedCornerShape(19.dp))
                        .background(Color(0xE60B101B))
                        .border(0.5.dp, Color.White.copy(alpha = 0.18f), RoundedCornerShape(19.dp))
                        .padding(vertical = 10.dp),
                    horizontalAlignment = Alignment.CenterHorizontally,
                    verticalArrangement = Arrangement.SpaceBetween,
                ) {
                    MediaHubIcon(
                        imageVector = Lucide.Sun,
                        contentDescription = null,
                        tint = Color(0xFFFBBF24),
                        modifier = Modifier.size(18.dp),
                    )

                    // Vertical filling track
                    Box(
                        modifier = Modifier
                            .width(4.5.dp)
                            .height(72.dp)
                            .clip(RoundedCornerShape(2.25.dp))
                            .background(Color.White.copy(alpha = 0.18f)),
                        contentAlignment = Alignment.BottomCenter,
                    ) {
                        Box(
                            modifier = Modifier
                                .fillMaxWidth()
                                .fillMaxHeight(brightness.coerceIn(0f, 1f))
                                .background(
                                    Brush.verticalGradient(
                                        colors = listOf(Color(0xFFFBBF24), Color(0xFFF59E0B)),
                                    ),
                                ),
                        )
                    }

                    MediaHubText(
                        text = "${(brightness * 100).roundToInt()}%",
                        color = Color.White,
                        fontSize = 10.sp,
                        fontWeight = FontWeight.Bold,
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
                        .width(38.dp)
                        .height(150.dp)
                        .clip(RoundedCornerShape(19.dp))
                        .background(Color(0xE60B101B))
                        .border(0.5.dp, Color.White.copy(alpha = 0.18f), RoundedCornerShape(19.dp))
                        .padding(vertical = 10.dp),
                    horizontalAlignment = Alignment.CenterHorizontally,
                    verticalArrangement = Arrangement.SpaceBetween,
                ) {
                    MediaHubIcon(
                        imageVector = if (volume <= 0.01f) Lucide.VolumeX else Lucide.Volume2,
                        contentDescription = null,
                        tint = Color(0xFF38BDF8),
                        modifier = Modifier.size(18.dp),
                    )

                    // Vertical filling track
                    Box(
                        modifier = Modifier
                            .width(4.5.dp)
                            .height(72.dp)
                            .clip(RoundedCornerShape(2.25.dp))
                            .background(Color.White.copy(alpha = 0.18f)),
                        contentAlignment = Alignment.BottomCenter,
                    ) {
                        Box(
                            modifier = Modifier
                                .fillMaxWidth()
                                .fillMaxHeight(volume.coerceIn(0f, 1f))
                                .background(
                                    Brush.verticalGradient(
                                        colors = listOf(Color(0xFF38BDF8), Color(0xFF2563EB)),
                                    ),
                                ),
                        )
                    }

                    MediaHubText(
                        text = "${(volume * 100).roundToInt()}%",
                        color = Color.White,
                        fontSize = 10.sp,
                        fontWeight = FontWeight.Bold,
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
                    .clip(RoundedCornerShape(20.dp))
                    .background(Color(0xF00B101B))
                    .border(0.5.dp, Color.White.copy(alpha = 0.22f), RoundedCornerShape(20.dp))
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
                            tint = if (seekState.deltaMs >= 0) Color(0xFF38BDF8) else Color(0xFF60A5FA),
                            modifier = Modifier.size(20.dp),
                        )
                        MediaHubText(
                            text = formatDeltaTime(seekState.deltaMs),
                            color = if (seekState.deltaMs >= 0) Color(0xFF38BDF8) else Color(0xFF60A5FA),
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
                            color = Color.White,
                            fontSize = 15.sp,
                            fontWeight = FontWeight.SemiBold,
                        )
                        MediaHubText(
                            text = "/",
                            color = Color.White.copy(alpha = 0.45f),
                            fontSize = 13.sp,
                        )
                        MediaHubText(
                            text = formatPlaybackPosition(seekState.durationMs),
                            color = Color.White.copy(alpha = 0.70f),
                            fontSize = 13.sp,
                        )
                    }

                    // Mini progress bar in Seek HUD
                    val progressFraction = if (seekState.durationMs > 0L) {
                        (seekState.targetMs.toFloat() / seekState.durationMs.toFloat()).coerceIn(0f, 1f)
                    } else 0f
                    Box(
                        modifier = Modifier
                            .fillMaxWidth()
                            .height(3.5.dp)
                            .clip(RoundedCornerShape(1.75.dp))
                            .background(Color.White.copy(alpha = 0.20f)),
                        contentAlignment = Alignment.CenterStart,
                    ) {
                        Box(
                            modifier = Modifier
                                .fillMaxWidth(progressFraction)
                                .fillMaxHeight()
                                .background(
                                    Brush.horizontalGradient(
                                        colors = listOf(Color(0xFF38BDF8), Color(0xFF2563EB)),
                                    ),
                                ),
                        )
                    }
                }
            }
        }
    }
}

/**
 * Modern Frosted Glass Side Drawer for Audio & Subtitle Track Selection.
 */
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

    val currentTracks = player.currentTracks
    val subtitleGroups = currentTracks.groups.filter { it.type == C.TRACK_TYPE_TEXT }
    val audioGroups = currentTracks.groups.filter { it.type == C.TRACK_TYPE_AUDIO }

    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(Color.Black.copy(alpha = 0.60f))
            .clickable(
                indication = null,
                interactionSource = remember { MutableInteractionSource() },
                onClick = onDismiss,
            ),
        contentAlignment = Alignment.CenterEnd,
    ) {
        Column(
            modifier = Modifier
                .fillMaxHeight()
                .widthIn(min = 300.dp, max = 350.dp)
                .clickable(
                    indication = null,
                    interactionSource = remember { MutableInteractionSource() },
                    onClick = {},
                )
                .background(Color(0xF20B101B))
                .border(0.5.dp, Color.White.copy(alpha = 0.15f), RoundedCornerShape(topStart = 20.dp, bottomStart = 20.dp))
                .padding(20.dp),
            verticalArrangement = Arrangement.spacedBy(16.dp),
        ) {
            // Header: Title & Close Button
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                MediaHubText(
                    text = "音轨与字幕",
                    color = Color.White,
                    fontSize = 16.sp,
                    fontWeight = FontWeight.Bold,
                )
                PlayerFrostedCircleButton(
                    imageVector = Lucide.X,
                    contentDescription = "关闭",
                    onClick = onDismiss,
                    modifier = Modifier.size(36.dp),
                    iconSize = 18.dp,
                )
            }

            // Tab Switcher (Subtitles vs Audio)
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .clip(RoundedCornerShape(10.dp))
                    .background(Color.White.copy(alpha = 0.08f))
                    .padding(3.dp),
                horizontalArrangement = Arrangement.spacedBy(4.dp),
            ) {
                Box(
                    modifier = Modifier
                        .weight(1f)
                        .height(36.dp)
                        .clip(RoundedCornerShape(8.dp))
                        .background(if (selectedTab == 0) Color(0xFF2563EB) else Color.Transparent)
                        .clickable {
                            haptic.performHapticFeedback(HapticFeedbackType.TextHandleMove)
                            selectedTab = 0
                        },
                    contentAlignment = Alignment.Center,
                ) {
                    MediaHubText(
                        text = "字幕 (${subtitleGroups.size})",
                        color = Color.White,
                        fontSize = 12.sp,
                        fontWeight = if (selectedTab == 0) FontWeight.Bold else FontWeight.Medium,
                    )
                }

                Box(
                    modifier = Modifier
                        .weight(1f)
                        .height(36.dp)
                        .clip(RoundedCornerShape(8.dp))
                        .background(if (selectedTab == 1) Color(0xFF2563EB) else Color.Transparent)
                        .clickable {
                            haptic.performHapticFeedback(HapticFeedbackType.TextHandleMove)
                            selectedTab = 1
                        },
                    contentAlignment = Alignment.Center,
                ) {
                    MediaHubText(
                        text = "音轨 (${audioGroups.size})",
                        color = Color.White,
                        fontSize = 12.sp,
                        fontWeight = if (selectedTab == 1) FontWeight.Bold else FontWeight.Medium,
                    )
                }
            }

            // Track List
            LazyColumn(
                modifier = Modifier.fillMaxWidth().weight(1f),
                verticalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                if (selectedTab == 0) {
                    val isSubtitlesDisabled = player.trackSelectionParameters.disabledTrackTypes.contains(C.TRACK_TYPE_TEXT) ||
                        subtitleGroups.none { it.isSelected }

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
                            subtitle = "不显示任何字幕",
                            isSelected = isSubtitlesDisabled,
                            onClick = {
                                haptic.performHapticFeedback(HapticFeedbackType.TextHandleMove)
                                player.trackSelectionParameters = player.trackSelectionParameters
                                    .buildUpon()
                                    .setTrackTypeDisabled(C.TRACK_TYPE_TEXT, true)
                                    .build()
                            },
                        )
                    }

                    items(subtitleGroups) { group ->
                        val format = group.getTrackFormat(0)
                        val title = format.label ?: format.language ?: "字幕"
                        val subtitle = format.sampleMimeType?.substringAfterLast('/')?.uppercase() ?: "内嵌"
                        val isSelected = group.isSelected && !isSubtitlesDisabled

                        PlayerTrackItem(
                            title = title,
                            subtitle = subtitle,
                            isSelected = isSelected,
                            onClick = {
                                haptic.performHapticFeedback(HapticFeedbackType.TextHandleMove)
                                val override = TrackSelectionOverride(group.mediaTrackGroup, 0)
                                player.trackSelectionParameters = player.trackSelectionParameters
                                    .buildUpon()
                                    .setOverrideForType(override)
                                    .setTrackTypeDisabled(C.TRACK_TYPE_TEXT, false)
                                    .build()
                            },
                        )
                    }
                } else {
                    if (audioGroups.isEmpty()) {
                        item {
                            MediaHubText(
                                text = "暂无多音轨可选",
                                color = Color.White.copy(alpha = 0.5f),
                                fontSize = 13.sp,
                                modifier = Modifier.padding(16.dp),
                            )
                        }
                    } else {
                        items(audioGroups) { group ->
                            val format = group.getTrackFormat(0)
                            val title = format.label ?: format.language ?: "默认音轨"
                            val channels = when (format.channelCount) {
                                6 -> "5.1 环绕声"
                                8 -> "7.1 全景声"
                                2 -> "立体声 2.0"
                                else -> "${format.channelCount} 声道"
                            }
                            val codec = format.sampleMimeType?.substringAfterLast('/')?.uppercase() ?: ""
                            val isSelected = group.isSelected

                            PlayerTrackItem(
                                title = title,
                                subtitle = "$channels · $codec",
                                isSelected = isSelected,
                                onClick = {
                                    haptic.performHapticFeedback(HapticFeedbackType.TextHandleMove)
                                    val override = TrackSelectionOverride(group.mediaTrackGroup, 0)
                                    player.trackSelectionParameters = player.trackSelectionParameters
                                        .buildUpon()
                                        .setOverrideForType(override)
                                        .setTrackTypeDisabled(C.TRACK_TYPE_AUDIO, false)
                                        .build()
                                },
                            )
                        }
                    }
                }
            }
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
            .clip(RoundedCornerShape(12.dp))
            .background(Color.White.copy(alpha = 0.06f))
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
                color = Color.White,
                fontSize = 13.sp,
                fontWeight = FontWeight.SemiBold,
            )
            if (subtitleState.targetId != null || subtitleState.remoteSubtitles.isNotEmpty()) {
                PlayerFrostedCircleButton(
                    imageVector = Lucide.X,
                    contentDescription = "关闭字幕结果",
                    onClick = onClear,
                    modifier = Modifier.size(28.dp),
                    iconSize = 14.dp,
                )
            }
        }
        Box(
            modifier = Modifier
                .fillMaxWidth()
                .heightIn(min = 40.dp)
                .clip(RoundedCornerShape(10.dp))
                .background(Color(0xFF2563EB).copy(alpha = 0.35f))
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
                    tint = Color.White,
                    modifier = Modifier.size(16.dp),
                )
                MediaHubText(
                    text = if (subtitleState.searching) "正在搜索中文字幕…" else "搜中文字幕",
                    color = Color.White,
                    fontSize = 13.sp,
                    fontWeight = FontWeight.Medium,
                )
            }
        }
        subtitleState.message?.let { message ->
            MediaHubText(
                text = message,
                color = Color.White.copy(alpha = 0.65f),
                fontSize = 11.sp,
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
            .clip(RoundedCornerShape(10.dp))
            .background(Color.White.copy(alpha = 0.05f))
            .padding(horizontal = 10.dp, vertical = 8.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Column(modifier = Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
            MediaHubText(
                text = subtitle.name,
                color = Color.White,
                fontSize = 12.sp,
                fontWeight = FontWeight.Medium,
                maxLines = 2,
                overflow = TextOverflow.Ellipsis,
            )
            if (meta.isNotBlank()) {
                MediaHubText(
                    text = meta,
                    color = Color.White.copy(alpha = 0.50f),
                    fontSize = 10.sp,
                )
            }
        }
        Box(
            modifier = Modifier
                .clip(RoundedCornerShape(8.dp))
                .background(if (enabled) Color(0xFF2563EB) else Color.White.copy(alpha = 0.12f))
                .clickable(enabled = enabled, onClick = onDownload)
                .padding(horizontal = 10.dp, vertical = 6.dp),
        ) {
            MediaHubText(
                text = if (downloading) "下载中…" else "下载",
                color = Color.White,
                fontSize = 11.sp,
                fontWeight = FontWeight.Medium,
            )
        }
    }
}

/**
 * Individual track item with checkmark and glowing border.
 */
@Composable
private fun PlayerTrackItem(
    title: String,
    subtitle: String,
    isSelected: Boolean,
    onClick: () -> Unit,
) {
    Box(
        modifier = Modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(12.dp))
            .background(if (isSelected) Color(0xFF2563EB).copy(alpha = 0.30f) else Color.White.copy(alpha = 0.05f))
            .border(
                0.5.dp,
                if (isSelected) Color(0xFF60A5FA) else Color.Transparent,
                RoundedCornerShape(12.dp),
            )
            .clickable(onClick = onClick)
            .padding(horizontal = 14.dp, vertical = 10.dp),
    ) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Column(verticalArrangement = Arrangement.spacedBy(2.dp)) {
                MediaHubText(
                    text = title,
                    color = Color.White,
                    fontSize = 13.sp,
                    fontWeight = if (isSelected) FontWeight.Bold else FontWeight.Medium,
                )
                MediaHubText(
                    text = subtitle,
                    color = Color.White.copy(alpha = 0.50f),
                    fontSize = 11.sp,
                )
            }
            if (isSelected) {
                MediaHubIcon(
                    imageVector = Lucide.Check,
                    contentDescription = "已选择",
                    tint = Color(0xFF38BDF8),
                    modifier = Modifier.size(16.dp),
                )
            }
        }
    }
}

/**
 * Frosted Glass Aspect Ratio Selection Dialog.
 */
@Composable
private fun PlayerAspectRatioDialog(
    current: PlayerAspectRatio,
    onSelect: (PlayerAspectRatio) -> Unit,
    onDismiss: () -> Unit,
) {
    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(Color.Black.copy(alpha = 0.60f))
            .clickable(
                indication = null,
                interactionSource = remember { MutableInteractionSource() },
                onClick = onDismiss,
            ),
        contentAlignment = Alignment.Center,
    ) {
        Column(
            modifier = Modifier
                .widthIn(min = 320.dp, max = 380.dp)
                .clip(RoundedCornerShape(20.dp))
                .background(Color(0xF20B101B))
                .border(0.5.dp, Color.White.copy(alpha = 0.18f), RoundedCornerShape(20.dp))
                .clickable(
                    indication = null,
                    interactionSource = remember { MutableInteractionSource() },
                    onClick = {},
                )
                .padding(20.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            MediaHubText(
                text = "画面比例",
                color = Color.White,
                fontSize = 16.sp,
                fontWeight = FontWeight.Bold,
            )
            PlayerAspectRatio.options.forEach { mode ->
                val isSelected = mode == current
                Box(
                    modifier = Modifier
                        .fillMaxWidth()
                        .clip(RoundedCornerShape(12.dp))
                        .background(if (isSelected) Color(0xFF2563EB).copy(alpha = 0.30f) else Color.White.copy(alpha = 0.07f))
                        .border(
                            0.5.dp,
                            if (isSelected) Color(0xFF60A5FA) else Color.Transparent,
                            RoundedCornerShape(12.dp),
                        )
                        .clickable { onSelect(mode) }
                        .padding(horizontal = 14.dp, vertical = 12.dp),
                ) {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.SpaceBetween,
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Column(modifier = Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
                            MediaHubText(
                                text = mode.label,
                                color = Color.White,
                                fontSize = 14.sp,
                                fontWeight = if (isSelected) FontWeight.Bold else FontWeight.Medium,
                            )
                            MediaHubText(
                                text = mode.description,
                                color = Color.White.copy(alpha = 0.55f),
                                fontSize = 11.sp,
                            )
                        }
                        if (isSelected) {
                            MediaHubIcon(
                                imageVector = Lucide.Check,
                                contentDescription = null,
                                tint = Color(0xFF7DD3FC),
                                modifier = Modifier.size(18.dp),
                            )
                        }
                    }
                }
            }
        }
    }
}

/**
 * Frosted Glass Speed Selection Dialog.
 */
@Composable
private fun PlayerSpeedDialog(
    currentSpeed: Float,
    onSelectSpeed: (Float) -> Unit,
    onDismiss: () -> Unit,
) {
    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(Color.Black.copy(alpha = 0.60f))
            .clickable(
                indication = null,
                interactionSource = remember { MutableInteractionSource() },
                onClick = onDismiss,
            ),
        contentAlignment = Alignment.Center,
    ) {
        Column(
            modifier = Modifier
                .widthIn(min = 320.dp, max = 380.dp)
                .clip(RoundedCornerShape(20.dp))
                .background(Color(0xF20B101B))
                .border(0.5.dp, Color.White.copy(alpha = 0.18f), RoundedCornerShape(20.dp))
                .padding(20.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(14.dp),
        ) {
            MediaHubText(
                text = "播放速度",
                color = Color.White,
                fontSize = 16.sp,
                fontWeight = FontWeight.Bold,
            )

            val chunkedSpeeds = PlaybackSpeeds.chunked(3)
            Column(
                modifier = Modifier.fillMaxWidth(),
                verticalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                chunkedSpeeds.forEach { rowSpeeds ->
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.spacedBy(8.dp),
                    ) {
                        rowSpeeds.forEach { speed ->
                            val isSelected = abs(speed - currentSpeed) < 0.05f
                            Box(
                                modifier = Modifier
                                    .weight(1f)
                                    .height(48.dp)
                                    .clip(RoundedCornerShape(12.dp))
                                    .background(
                                        if (isSelected) Color(0xFF2563EB) else Color.White.copy(alpha = 0.07f),
                                    )
                                    .border(
                                        0.5.dp,
                                        if (isSelected) Color(0xFF60A5FA) else Color.Transparent,
                                        RoundedCornerShape(12.dp),
                                    )
                                    .clickable { onSelectSpeed(speed) },
                                contentAlignment = Alignment.Center,
                            ) {
                                MediaHubText(
                                    text = "${speed}x",
                                    color = Color.White,
                                    fontSize = 14.sp,
                                    fontWeight = if (isSelected) FontWeight.Bold else FontWeight.Medium,
                                )
                            }
                        }
                    }
                }
            }
        }
    }
}

/**
 * Reusable frosted circular icon button (Liquid Glass style).
 */
@Composable
private fun PlayerFrostedCircleButton(
    imageVector: androidx.compose.ui.graphics.vector.ImageVector,
    contentDescription: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    iconSize: androidx.compose.ui.unit.Dp = 20.dp,
) {
    Box(
        modifier = modifier
            .shadow(6.dp, CircleShape)
            .clip(CircleShape)
            .background(Color.White.copy(alpha = 0.12f))
            .border(0.5.dp, Color.White.copy(alpha = 0.22f), CircleShape)
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
            tint = Color.White,
            modifier = Modifier.size(iconSize),
        )
    }
}

/**
 * Reusable liquid glass capsule pill button.
 */
@Composable
private fun PlayerLiquidPillButton(
    label: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    icon: androidx.compose.ui.graphics.vector.ImageVector? = null,
) {
    Box(
        modifier = modifier
            .heightIn(min = 48.dp)
            .clip(RoundedCornerShape(100.dp))
            .background(Color.White.copy(alpha = 0.10f))
            .border(0.5.dp, Color.White.copy(alpha = 0.20f), RoundedCornerShape(100.dp))
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
                    tint = Color.White,
                    modifier = Modifier.size(15.dp),
                )
            }
            MediaHubText(
                text = label,
                color = Color.White,
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
