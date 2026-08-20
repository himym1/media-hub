package com.mediahub.android.feature.operations

import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
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
import com.mediahub.android.core.designsystem.MediaHubCard
import com.mediahub.android.core.designsystem.MediaHubColors
import com.mediahub.android.core.designsystem.MediaHubFilterChip
import com.mediahub.android.core.designsystem.MediaHubIcon
import com.mediahub.android.core.designsystem.MediaHubListDivider
import com.mediahub.android.core.designsystem.MediaHubSmallTitle
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
        modifier = Modifier.fillMaxWidth().padding(vertical = 4.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        MediaHubSmallTitle(text = "本地上传")
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
                MediaHubFilterChip(
                    label = root.id,
                    selected = state.rootId == root.id,
                    onClick = { actions.rootChanged(root.id) },
                )
            }
        }
        MediaHubTextField(state.path, actions.pathChanged, "相对目录", Modifier.fillMaxWidth())
        if (state.path.isNotEmpty()) {
            MediaHubButton("返回上级", onClick = { actions.pathChanged(state.path.substringBeforeLast('/', "")) })
        }
        if (state.entries.isNotEmpty()) {
            MediaHubCard {
                state.entries.take(50).forEachIndexed { index, entry ->
                    if (index > 0) MediaHubListDivider()
                    val isSelected = state.selectedFile == entry.path
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .heightIn(min = 48.dp)
                            .clickable(role = Role.Button) { actions.entrySelected(entry) }
                            .padding(horizontal = 12.dp, vertical = 10.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        MediaHubIcon(
                            imageVector = Lucide.FolderOpen,
                            contentDescription = null,
                            tint = if (isSelected || entry.directory) MediaHubColors.Accent else MediaHubColors.TextMuted,
                            modifier = Modifier.size(18.dp),
                        )
                        Spacer(Modifier.width(10.dp))
                        Column(Modifier.weight(1f)) {
                            MediaHubText(entry.name, color = MediaHubColors.TextPrimary, fontSize = 13.sp, fontWeight = FontWeight.Medium)
                            MediaHubText(
                                if (entry.directory) "目录" else formatBytes(entry.size),
                                color = MediaHubColors.TextMuted,
                                fontSize = 12.sp,
                            )
                        }
                    }
                }
            }
        }
        MediaHubCard(insideMargin = PaddingValues(16.dp)) {
            Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
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
            }
        }
        if (state.uploads.isNotEmpty()) {
            MediaHubCard {
                state.uploads.take(8).forEachIndexed { index, upload ->
                    if (index > 0) MediaHubListDivider()
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(horizontal = 12.dp, vertical = 10.dp),
                        horizontalArrangement = Arrangement.spacedBy(8.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Column(Modifier.weight(1f)) {
                            MediaHubText(upload.path, color = MediaHubColors.TextPrimary, fontSize = 13.sp, fontWeight = FontWeight.Medium)
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
    }
}
