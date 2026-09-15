package com.mediahub.android.app

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.window.Dialog
import com.mediahub.android.core.designsystem.MediaHubButton
import com.mediahub.android.core.designsystem.MediaHubCard
import com.mediahub.android.core.designsystem.MediaHubColors
import com.mediahub.android.core.designsystem.MediaHubLinearProgress
import com.mediahub.android.core.designsystem.MediaHubSecondaryButton
import com.mediahub.android.core.designsystem.MediaHubText

@Composable
internal fun AppUpdateDialog(
    prompt: AppUpdatePrompt,
    onConfirm: () -> Unit,
    onDismiss: () -> Unit,
) {
    val release = prompt.release
    Dialog(onDismissRequest = onDismiss) {
        MediaHubCard(
            modifier = Modifier.fillMaxWidth(),
            insideMargin = PaddingValues(20.dp),
        ) {
            Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                MediaHubText(
                    text = "新版本 ${release.versionName}",
                    fontSize = 18.sp,
                    fontWeight = FontWeight.SemiBold,
                )
                androidReleaseBlurb(release.notes, release.versionName)?.let { blurb ->
                    MediaHubText(
                        text = blurb,
                        color = MediaHubColors.TextMuted,
                        fontSize = 13.sp,
                    )
                }
                MediaHubText(
                    text = androidUpdatePromptBody(prompt),
                    color = MediaHubColors.TextMuted,
                    fontSize = 13.sp,
                )
                if (prompt.downloading) {
                    MediaHubLinearProgress(
                        progress = updateProgressFraction(prompt.downloadedBytes, release.sizeBytes),
                    )
                }
                Row(
                    modifier = Modifier.fillMaxWidth(),
                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                ) {
                    MediaHubSecondaryButton(
                        label = if (prompt.downloading) "后台下载" else "稍后",
                        onClick = onDismiss,
                        modifier = Modifier.weight(1f),
                    )
                    MediaHubButton(
                        label = when {
                            prompt.downloading -> "下载中"
                            prompt.downloadedPath != null -> "安装"
                            else -> "下载"
                        },
                        enabled = !prompt.downloading,
                        onClick = onConfirm,
                        modifier = Modifier.weight(1f),
                    )
                }
            }
        }
    }
}
