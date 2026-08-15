package com.mediahub.android.feature.auth

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
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
import com.mediahub.android.core.designsystem.MediaHubButton
import com.mediahub.android.core.designsystem.MediaHubColors
import com.mediahub.android.core.designsystem.MediaHubIcon
import com.mediahub.android.core.designsystem.MediaHubPasswordField
import com.mediahub.android.core.designsystem.MediaHubText

@Composable
internal fun AuthRoute(
    viewModel: AuthViewModel,
    onAuthenticated: () -> Unit,
) {
    val uiState by viewModel.uiState.collectAsState()

    LaunchedEffect(viewModel) {
        viewModel.events.collect { event ->
            if (event is AuthEvent.Authenticated) onAuthenticated()
        }
    }

    LoginScreen(
        uiState = uiState,
        onPasswordChanged = viewModel::onPasswordChanged,
        onVisibilityChanged = viewModel::togglePasswordVisibility,
        onSubmit = viewModel::login,
    )
}

@Composable
private fun LoginScreen(
    uiState: AuthUiState,
    onPasswordChanged: (String) -> Unit,
    onVisibilityChanged: () -> Unit,
    onSubmit: () -> Unit,
) {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(MediaHubColors.Canvas)
            .statusBarsPadding()
            .navigationBarsPadding()
            .imePadding()
            .padding(horizontal = 24.dp, vertical = 20.dp),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            MediaHubIcon(
                imageVector = Lucide.Film,
                contentDescription = null,
                tint = MediaHubColors.Canvas,
                modifier = Modifier
                    .size(32.dp)
                    .background(MediaHubColors.Accent, RoundedCornerShape(8.dp))
                    .padding(7.dp),
            )
            Spacer(Modifier.width(11.dp))
            Column {
                MediaHubText(text = "MEDIA HUB", fontSize = 13.sp, fontWeight = FontWeight.Bold)
                MediaHubText(text = "HOME MEDIA CONTROL", color = MediaHubColors.TextMuted, fontSize = 12.sp)
            }
        }

        Spacer(Modifier.height(88.dp))
        MediaHubText(text = "管理员登录", fontSize = 32.sp, fontWeight = FontWeight.SemiBold)
        MediaHubText(
            text = "使用你的 Media Hub 管理员密码。",
            modifier = Modifier.padding(top = 10.dp, bottom = 28.dp),
            color = MediaHubColors.TextSecondary,
            fontSize = 14.sp,
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
            label = if (uiState.submitting) "正在登录" else "登录",
            icon = Lucide.LogIn,
            enabled = uiState.password.isNotEmpty() && !uiState.submitting,
            onClick = onSubmit,
            modifier = Modifier.fillMaxWidth().padding(top = 22.dp),
        )
        Spacer(Modifier.weight(1f))
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
        ) {
            MediaHubText(text = "单用户控制台", color = MediaHubColors.TextFaint, fontSize = 12.sp)
            MediaHubText(text = "ANDROID", color = MediaHubColors.TextFaint, fontSize = 12.sp)
        }
    }
}
