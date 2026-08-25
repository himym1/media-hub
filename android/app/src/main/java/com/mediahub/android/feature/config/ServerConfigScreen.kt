package com.mediahub.android.feature.config

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.composables.icons.lucide.Link
import com.composables.icons.lucide.Lucide
import com.mediahub.android.core.designsystem.MediaHubButton
import com.mediahub.android.core.designsystem.MediaHubCenteredPane
import com.mediahub.android.core.designsystem.MediaHubColors
import com.mediahub.android.core.designsystem.MediaHubText
import com.mediahub.android.core.designsystem.MediaHubTextField

@Composable
fun ServerConfigScreen(
    viewModel: ServerConfigViewModel,
    onConfigured: (String) -> Unit,
) {
    val value by viewModel.serverUrl.collectAsState()
    MediaHubCenteredPane(
        contentPadding = PaddingValues(28.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp, Alignment.CenterVertically),
    ) {
        MediaHubText("Media Hub 服务器", fontSize = 22.sp, fontWeight = FontWeight.SemiBold)
        MediaHubText("填写私有部署的 HTTPS 地址", color = MediaHubColors.TextMuted, fontSize = 12.sp)
        MediaHubTextField(
            value = value,
            onValueChange = viewModel::update,
            placeholder = "https://media.example.com",
        )
        MediaHubButton(
            label = "连接服务器",
            icon = Lucide.Link,
            enabled = value.isNotBlank(),
            onClick = { viewModel.save()?.let(onConfigured) },
            modifier = Modifier.fillMaxWidth(),
        )
    }
}
