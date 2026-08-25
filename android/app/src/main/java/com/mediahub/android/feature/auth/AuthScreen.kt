package com.mediahub.android.feature.auth

import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.navigationBarsPadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.composables.icons.lucide.LogIn
import com.composables.icons.lucide.Lucide
import com.composables.icons.lucide.Server
import com.mediahub.android.app.LocalTwoPane
import com.mediahub.android.core.designsystem.MediaHubButton
import com.mediahub.android.core.designsystem.MediaHubCenteredPane
import com.mediahub.android.core.designsystem.MediaHubColors
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
    MediaHubCenteredPane(
        modifier = Modifier
            .statusBarsPadding()
            .navigationBarsPadding()
            .imePadding(),
        contentPadding = PaddingValues(horizontal = 20.dp, vertical = 20.dp),
    ) {
        MediaHubText(text = "Media Hub", fontSize = 14.sp, fontWeight = FontWeight.Medium, color = MediaHubColors.TextMuted)
        MediaHubText(text = "管理员登录", modifier = Modifier.padding(top = 4.dp), fontSize = 28.sp, fontWeight = FontWeight.SemiBold)
        MediaHubText(
            text = serverUrl,
            modifier = Modifier.padding(top = 8.dp),
            color = MediaHubColors.TextMuted,
            fontSize = 12.sp,
        )

        Spacer(Modifier.height(28.dp))

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
        MediaHubSecondaryButton(
            label = "更换服务器",
            icon = Lucide.Server,
            enabled = !uiState.submitting,
            onClick = onChangeServer,
            modifier = Modifier.fillMaxWidth().padding(top = 10.dp),
        )

        if (LocalTwoPane.current) Spacer(Modifier.height(24.dp)) else Spacer(Modifier.weight(1f))
    }
}
