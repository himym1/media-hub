package com.mediahub.android.feature.player

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp
import androidx.compose.ui.viewinterop.AndroidView
import androidx.media3.session.MediaController
import androidx.media3.ui.PlayerView
import com.composables.icons.lucide.ArrowLeft
import com.composables.icons.lucide.Lucide
import com.composables.icons.lucide.PictureInPicture2
import com.composables.icons.lucide.RotateCw
import com.mediahub.android.core.designsystem.MediaHubButton
import com.mediahub.android.core.designsystem.MediaHubIconButton
import com.mediahub.android.core.designsystem.MediaHubText

internal sealed interface PlayerUiState {
    data object Loading : PlayerUiState
    data object Ready : PlayerUiState
    data class Error(val message: String) : PlayerUiState
}

internal data class PlayerActions(
    val onRetry: () -> Unit,
    val onBack: () -> Unit,
    val onToggleOrientation: () -> Unit,
    val onEnterPictureInPicture: () -> Unit,
)

@Composable
internal fun PlayerScreen(
    state: PlayerUiState,
    title: String,
    controller: MediaController?,
    isPictureInPicture: Boolean,
    actions: PlayerActions,
) {
    Box(Modifier.fillMaxSize().background(Color.Black)) {
        if (controller != null) {
            AndroidView(
                factory = { context -> PlayerView(context) },
                update = {
                    it.player = controller
                    it.useController = !isPictureInPicture
                },
                modifier = Modifier.fillMaxSize(),
            )
        }
        when (state) {
            PlayerUiState.Loading -> MediaHubText(
                "正在准备 115 视频…",
                color = Color.White,
                modifier = Modifier.align(Alignment.Center),
            )
            is PlayerUiState.Error -> Column(
                modifier = Modifier.align(Alignment.Center),
                horizontalAlignment = Alignment.CenterHorizontally,
                verticalArrangement = Arrangement.spacedBy(16.dp),
            ) {
                MediaHubText(state.message, color = Color.White)
                MediaHubButton("重试", onClick = actions.onRetry)
            }
            PlayerUiState.Ready -> Unit
        }
        if (!isPictureInPicture) {
            Row(
                modifier = Modifier.fillMaxWidth().align(Alignment.TopCenter).padding(16.dp),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                MediaHubIconButton(
                    imageVector = Lucide.ArrowLeft,
                    contentDescription = "返回",
                    onClick = actions.onBack,
                )
                MediaHubText(title, color = Color.White, modifier = Modifier.weight(1f).padding(horizontal = 4.dp))
                MediaHubIconButton(
                    imageVector = Lucide.RotateCw,
                    contentDescription = "旋转屏幕",
                    onClick = actions.onToggleOrientation,
                )
                MediaHubIconButton(
                    imageVector = Lucide.PictureInPicture2,
                    contentDescription = "进入画中画",
                    onClick = actions.onEnterPictureInPicture,
                )
            }
        }
    }
}
