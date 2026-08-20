package com.mediahub.android.feature.operations

import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
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
import com.composables.icons.lucide.File
import com.composables.icons.lucide.Folder
import com.composables.icons.lucide.Lucide
import com.composables.icons.lucide.Play
import com.mediahub.android.core.designsystem.MediaHubButton
import com.mediahub.android.core.designsystem.MediaHubCard
import com.mediahub.android.core.designsystem.MediaHubColors
import com.mediahub.android.core.designsystem.MediaHubFilterChip
import com.mediahub.android.core.designsystem.MediaHubIcon
import com.mediahub.android.core.designsystem.MediaHubIconButton
import com.mediahub.android.core.designsystem.MediaHubListDivider
import com.mediahub.android.core.designsystem.MediaHubSmallTitle
import com.mediahub.android.core.designsystem.MediaHubText
import com.mediahub.android.core.designsystem.MediaHubTextField
import com.mediahub.android.core.network.Drive115Command
import com.mediahub.android.core.network.Drive115File

internal data class Drive115Actions(
    val refresh: () -> Unit,
    val parentChanged: (String) -> Unit,
    val operationChanged: (String) -> Unit,
    val fileIdsChanged: (String) -> Unit,
    val targetChanged: (String) -> Unit,
    val nameChanged: (String) -> Unit,
    val create: () -> Unit,
    val confirm: (Drive115Command) -> Unit,
    val play: (Drive115File) -> Unit,
)

@Composable
internal fun Drive115Screen(state: Drive115State, actions: Drive115Actions) {
    Column(
        modifier = Modifier.fillMaxWidth().padding(vertical = 4.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        MediaHubSmallTitle(text = "115 文件")
        MediaHubTextField(
            value = state.parentId,
            onValueChange = actions.parentChanged,
            placeholder = "当前目录 ID",
            modifier = Modifier.fillMaxWidth(),
            keyboardType = KeyboardType.Number,
        )
        if (state.files.isNotEmpty()) {
            MediaHubCard {
                state.files.take(50).forEachIndexed { index, file ->
                    if (index > 0) MediaHubListDivider()
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .heightIn(min = 48.dp)
                            .clickable(enabled = file.kind == "folder", role = Role.Button) { actions.parentChanged(file.id) }
                            .padding(horizontal = 12.dp, vertical = 10.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        MediaHubIcon(
                            imageVector = if (file.kind == "folder") Lucide.Folder else Lucide.File,
                            contentDescription = null,
                            tint = if (file.kind == "folder") MediaHubColors.Accent else MediaHubColors.TextMuted,
                            modifier = Modifier.size(18.dp),
                        )
                        Spacer(Modifier.width(10.dp))
                        Column(Modifier.weight(1f)) {
                            MediaHubText(file.name, color = MediaHubColors.TextPrimary, fontSize = 13.sp, fontWeight = FontWeight.Medium)
                            MediaHubText(
                                if (file.kind == "folder") "目录 · ${file.id}" else "${formatBytes(file.size)} · ${file.id}",
                                color = MediaHubColors.TextMuted,
                                fontSize = 12.sp,
                            )
                        }
                        if (file.kind == "file" && isPlayableVideoName(file.name)) {
                            MediaHubIconButton(Lucide.Play, "播放 ${file.name}", { actions.play(file) })
                        }
                    }
                }
            }
        }
        MediaHubSmallTitle(text = "命令")
        Row(
            modifier = Modifier.fillMaxWidth().horizontalScroll(rememberScrollState()),
            horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            listOf("create_folder" to "建目录", "move" to "移动", "rename" to "重命名", "delete" to "删除")
                .forEach { (value, label) ->
                    MediaHubFilterChip(
                        label = label,
                        selected = state.operation == value,
                        onClick = { actions.operationChanged(value) },
                    )
                }
        }
        MediaHubCard(insideMargin = androidx.compose.foundation.layout.PaddingValues(16.dp)) {
            Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
                if (state.operation != "create_folder") {
                    MediaHubTextField(
                        value = state.fileIds,
                        onValueChange = actions.fileIdsChanged,
                        placeholder = if (state.operation == "rename") "文件 ID" else "文件 ID，逗号分隔",
                        modifier = Modifier.fillMaxWidth(),
                    )
                }
                if (state.operation == "create_folder" || state.operation == "move") {
                    MediaHubTextField(
                        value = state.targetParentId,
                        onValueChange = actions.targetChanged,
                        placeholder = if (state.operation == "move") "目标目录 ID" else "父目录 ID",
                        modifier = Modifier.fillMaxWidth(),
                        keyboardType = KeyboardType.Number,
                    )
                }
                if (state.operation == "create_folder" || state.operation == "rename") {
                    MediaHubTextField(state.name, actions.nameChanged, "名称", Modifier.fillMaxWidth())
                }
                MediaHubButton(
                    label = if (state.invoking) "正在持久化" else if (state.operation == "delete") "创建待确认删除命令" else "创建命令",
                    onClick = actions.create,
                    enabled = !state.invoking,
                    modifier = Modifier.fillMaxWidth(),
                )
            }
        }
        if (state.commands.isNotEmpty()) {
            MediaHubCard {
                state.commands.take(8).forEachIndexed { index, command ->
                    if (index > 0) MediaHubListDivider()
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(horizontal = 12.dp, vertical = 10.dp),
                        horizontalArrangement = Arrangement.spacedBy(8.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Column(Modifier.weight(1f)) {
                            MediaHubText(command.operation, color = MediaHubColors.TextPrimary, fontSize = 13.sp, fontWeight = FontWeight.Medium)
                            MediaHubText(command.id, color = MediaHubColors.TextMuted, fontSize = 12.sp)
                        }
                        MediaHubText(commandStateLabel(command.state), color = commandStateColor(command.state), fontSize = 12.sp)
                        if (command.state in setOf("awaiting_confirmation", "failed", "needs_attention")) {
                            MediaHubButton(
                                if (command.state == "awaiting_confirmation") "确认删除" else "确认重试",
                                onClick = { actions.confirm(command) },
                            )
                        }
                    }
                }
            }
        }
    }
}

internal fun isPlayableVideoName(name: String): Boolean = when (name.substringAfterLast('.', "").lowercase()) {
    "mp4", "mkv", "m4v", "mov", "webm", "avi", "ts", "m2ts" -> true
    else -> false
}
