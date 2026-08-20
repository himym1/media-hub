package com.mediahub.android.feature.auth

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.composables.icons.lucide.Film
import com.composables.icons.lucide.LogIn
import com.composables.icons.lucide.Lucide
import com.composables.icons.lucide.Server
import com.mediahub.android.core.designsystem.MediaHubButton
import com.mediahub.android.core.designsystem.MediaHubColors
import com.mediahub.android.core.designsystem.MediaHubIcon
import com.mediahub.android.core.designsystem.MediaHubPasswordField
import com.mediahub.android.core.designsystem.MediaHubSecondaryButton
import com.mediahub.android.core.designsystem.MediaHubText

@Composable
internal fun AuthRoute(
    viewModel: AuthViewModel,
    serverUrl: String,
    onAuthenticated: () -> Unit,
    onChangeServer: () -> Unit,
) {
    val uiState by viewModel.uiState.collectAsState()

    LaunchedEffect(viewModel) {
        viewModel.events.collect { event ->
            if (event is AuthEvent.Authenticated) onAuthenticated()
        }
    }

    LoginScreen(
        uiState = uiState,
        serverUrl = serverUrl,
        onPasswordChanged = viewModel::onPasswordChanged,
        onVisibilityChanged = viewModel::togglePasswordVisibility,
        onSubmit = viewModel::login,
        onChangeServer = onChangeServer,
    )
}

@Composable
private fun LoginScreen(
    uiState: AuthUiState,
    serverUrl: String,
    onPasswordChanged: (String) -> Unit,
    onVisibilityChanged: () -> Unit,
    onSubmit: () -> Unit,
    onChangeServer: () -> Unit,
) {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(MediaHubColors.Canvas)
            .statusBarsPadding()
            .navigationBarsPadding()
            .imePadding()
            .padding(horizontal = 20.dp, vertical = 20.dp),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Box(
                modifier = Modifier
                    .size(36.dp)
                    .background(MediaHubColors.SurfaceSelected, RoundedCornerShape(8.dp))
                    .border(width = 1.dp, color = MediaHubColors.Accent, shape = RoundedCornerShape(8.dp)),
                contentAlignment = Alignment.Center,
            ) {
                MediaHubIcon(
                    imageVector = Lucide.Film,
                    contentDescription = null,
                    tint = MediaHubColors.Accent,
                    modifier = Modifier.size(20.dp),
                )
            }
            Spacer(Modifier.width(12.dp))
            Column {
                MediaHubText(text = "MEDIA HUB", fontSize = 14.sp, fontWeight = FontWeight.Bold)
                MediaHubText(text = "媒体自动化控制", color = MediaHubColors.TextMuted, fontSize = 12.sp)
            }
        }

        Spacer(Modifier.height(56.dp))

        Box(
            modifier = Modifier
                .fillMaxWidth()
                .background(MediaHubColors.Surface, RoundedCornerShape(12.dp))
                .border(width = 1.dp, color = MediaHubColors.Border, shape = RoundedCornerShape(12.dp))
                .padding(22.dp),
        ) {
            Column {
                MediaHubText(text = "管理员登录", fontSize = 24.sp, fontWeight = FontWeight.SemiBold)
                MediaHubText(
                    text = "使用你的 Media Hub 管理员密码。",
                    modifier = Modifier.padding(top = 8.dp, bottom = 22.dp),
                    color = MediaHubColors.TextSecondary,
                    fontSize = 13.sp,
                )
                MediaHubText(
                    text = "密码",
                    modifier = Modifier.padding(bottom = 8.dp),
                    color = MediaHubColors.TextStrong,
                    fontSize = 12.sp,
                    fontWeight = FontWeight.Medium,
                )
                MediaHubPasswordField(
                    value = uiState.password,
                    onValueChange = onPasswordChanged,
                    visible = uiState.passwordVisible,
                    onVisibilityChanged = onVisibilityChanged,
                    onSubmit = onSubmit,
                    enabled = !uiState.submitting,
                    modifier = Modifier.fillMaxWidth(),
                )
                uiState.errorMessage?.let { message ->
                    MediaHubText(
                        text = message,
                        modifier = Modifier
                            .padding(top = 12.dp)
                            .semantics { liveRegion = LiveRegionMode.Polite },
                        color = MediaHubColors.Error,
                        fontSize = 12.sp,
                    )
                }
                MediaHubButton(
                    label = if (uiState.submitting) "正在登录…" else "登录",
                    icon = Lucide.LogIn,
                    enabled = uiState.password.isNotEmpty() && !uiState.submitting,
                    onClick = onSubmit,
                    modifier = Modifier.fillMaxWidth().padding(top = 20.dp),
                )
                MediaHubText(
                    text = serverUrl,
                    modifier = Modifier.padding(top = 16.dp),
                    color = MediaHubColors.TextMuted,
                    fontSize = 12.sp,
                )
                MediaHubSecondaryButton(
                    label = "更换服务器",
                    icon = Lucide.Server,
                    enabled = !uiState.submitting,
                    onClick = onChangeServer,
                    modifier = Modifier.fillMaxWidth().padding(top = 8.dp),
                )
            }
        }

        Spacer(Modifier.weight(1f))
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
        ) {
            MediaHubText(text = "单用户控制台", color = MediaHubColors.TextFaint, fontSize = 12.sp)
            MediaHubText(text = "Android 客户端", color = MediaHubColors.TextFaint, fontSize = 12.sp)
        }
    }
}
