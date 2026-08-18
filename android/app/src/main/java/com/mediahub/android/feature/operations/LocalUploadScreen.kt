package com.mediahub.android.feature.operations

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.composables.icons.lucide.FolderOpen
import com.composables.icons.lucide.Lucide
import com.composables.icons.lucide.Upload
import com.mediahub.android.core.designsystem.MediaHubButton
import com.mediahub.android.core.designsystem.MediaHubColors
import com.mediahub.android.core.designsystem.MediaHubIcon
import com.mediahub.android.core.designsystem.MediaHubText
import com.mediahub.android.core.designsystem.MediaHubTextField
import com.mediahub.android.core.network.LocalUploadEntry
import com.mediahub.android.core.network.LocalUploadJob

internal data class LocalUploadActions(
    val refresh: () -> Unit,
    val rootChanged: (String) -> Unit,
    val pathChanged: (String) -> Unit,
    val entrySelected: (LocalUploadEntry) -> Unit,
    val destinationChanged: (String) -> Unit,
    val create: () -> Unit,
    val retry: (LocalUploadJob) -> Unit,
)

@Composable
internal fun LocalUploadScreen(state: LocalUploadState, actions: LocalUploadActions) {
    Column(
        modifier = Modifier.fillMaxWidth().padding(vertical = 12.dp),
        verticalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            MediaHubIcon(Lucide.Upload, contentDescription = null, tint = MediaHubColors.Accent)
            Spacer(Modifier.padding(horizontal = 4.dp))
            MediaHubText("本地上传", color = MediaHubColors.TextPrimary, fontSize = 18.sp, fontWeight = FontWeight.SemiBold)
        }
        if (state.roots.isEmpty()) {
            MediaHubText(
                "当前服务器未配置本地上传目录",
                modifier = Modifier.padding(vertical = 28.dp),
                color = MediaHubColors.TextMuted,
                fontSize = 13.sp,
            )
            return@Column
        }
        Row(
            modifier = Modifier.fillMaxWidth().horizontalScroll(rememberScrollState()),
            horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            state.roots.forEach { root ->
                MediaHubButton(root.id, onClick = { actions.rootChanged(root.id) })
            }
        }
        MediaHubTextField(state.path, actions.pathChanged, "相对目录", Modifier.fillMaxWidth())
        if (state.path.isNotEmpty()) {
            MediaHubButton("返回上级", onClick = { actions.pathChanged(state.path.substringBeforeLast('/', "")) })
        }
        state.entries.take(50).forEach { entry ->
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .heightIn(min = 48.dp)
                    .clickable(role = Role.Button) { actions.entrySelected(entry) }
                    .background(if (state.selectedFile == entry.path) MediaHubColors.SurfaceSelected else MediaHubColors.Canvas)
                    .padding(vertical = 9.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                MediaHubIcon(
                    Lucide.FolderOpen,
                    contentDescription = null,
                    tint = if (entry.directory) MediaHubColors.Accent else MediaHubColors.TextMuted,
                )
                Spacer(Modifier.padding(horizontal = 5.dp))
                Column(Modifier.weight(1f)) {
                    MediaHubText(entry.name, fontSize = 12.sp)
                    MediaHubText(
                        if (entry.directory) "目录" else formatBytes(entry.size),
                        color = MediaHubColors.TextMuted,
                        fontSize = 12.sp,
                    )
                }
            }
        }
        MediaHubText(state.selectedFile.ifBlank { "请选择一个文件" }, color = MediaHubColors.TextMuted, fontSize = 12.sp)
        MediaHubTextField(
            value = state.destinationId,
            onValueChange = actions.destinationChanged,
            placeholder = "115 目标目录 ID",
            modifier = Modifier.fillMaxWidth(),
            keyboardType = KeyboardType.Number,
        )
        MediaHubButton(
            label = if (state.invoking) "正在创建" else "创建上传任务",
            icon = Lucide.Upload,
            onClick = actions.create,
            enabled = !state.invoking && state.selectedFile.isNotBlank() && state.destinationId.isNotBlank(),
            modifier = Modifier.fillMaxWidth(),
        )
        state.uploads.take(8).forEach { upload ->
            Row(
                modifier = Modifier.fillMaxWidth().padding(vertical = 7.dp),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Column(Modifier.weight(1f)) {
                    MediaHubText(upload.path, fontSize = 12.sp)
                    MediaHubText(
                        if (upload.bytesTotal > 0) "${upload.bytesDone * 100 / upload.bytesTotal}% · ${formatBytes(upload.bytesTotal)}" else upload.id,
                        color = MediaHubColors.TextMuted,
                        fontSize = 12.sp,
                    )
                }
                MediaHubText(localUploadState(upload.state), color = commandStateColor(upload.state), fontSize = 12.sp)
                if (upload.state == "failed" || upload.state == "needs_attention") {
                    MediaHubButton("确认重试", onClick = { actions.retry(upload) })
                }
            }
        }
    }
}
