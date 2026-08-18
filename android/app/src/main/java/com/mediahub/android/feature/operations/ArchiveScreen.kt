package com.mediahub.android.feature.operations

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.composables.icons.lucide.FolderOpen
import com.composables.icons.lucide.Lucide
import com.mediahub.android.core.designsystem.MediaHubButton
import com.mediahub.android.core.designsystem.MediaHubColors
import com.mediahub.android.core.designsystem.MediaHubIcon
import com.mediahub.android.core.designsystem.MediaHubText
import com.mediahub.android.core.designsystem.MediaHubTextField
import com.mediahub.android.core.network.ArchivePlan

internal data class ArchiveActions(
    val refresh: () -> Unit,
    val parentChanged: (String) -> Unit,
    val targetChanged: (String) -> Unit,
    val preview: () -> Unit,
    val toggle: (String) -> Unit,
    val nameChanged: (String, String) -> Unit,
    val create: () -> Unit,
    val confirm: (ArchivePlan) -> Unit,
    val retry: (ArchivePlan) -> Unit,
)

@Composable
internal fun ArchiveScreen(state: ArchiveState, actions: ArchiveActions) {
    Column(
        modifier = Modifier.fillMaxWidth().padding(vertical = 12.dp),
        verticalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            MediaHubIcon(Lucide.FolderOpen, contentDescription = null, tint = MediaHubColors.Accent)
            Spacer(Modifier.padding(horizontal = 4.dp))
            Column {
                MediaHubText("原生归档整理", fontSize = 18.sp, fontWeight = FontWeight.SemiBold)
                MediaHubText(
                    "预览只生成建议；选中的步骤落库并以计划 ID 二次确认后执行",
                    color = MediaHubColors.TextMuted,
                    fontSize = 12.sp,
                )
            }
        }
        MediaHubTextField(state.parentId, actions.parentChanged, "来源目录 ID", keyboardType = KeyboardType.Number)
        MediaHubTextField(state.targetId, actions.targetChanged, "移动目标目录 ID（可选）", keyboardType = KeyboardType.Number)
        MediaHubButton("预览建议", onClick = actions.preview, enabled = state.parentId.isNotBlank())
        state.suggestions.forEach { item ->
            Column(Modifier.fillMaxWidth().background(MediaHubColors.Surface).padding(10.dp)) {
                MediaHubText(item.currentName, fontSize = 13.sp, fontWeight = FontWeight.SemiBold)
                MediaHubText(
                    "${if (item.kind == "directory") "目录" else "文件"} · 需人工复核",
                    color = MediaHubColors.TextSecondary,
                    fontSize = 12.sp,
                )
                Spacer(Modifier.height(6.dp))
                MediaHubButton(
                    if (item.fileId in state.selected) "取消" else "选择",
                    onClick = { actions.toggle(item.fileId) },
                )
                Spacer(Modifier.height(6.dp))
                MediaHubTextField(
                    state.names[item.fileId] ?: item.suggestedName,
                    { actions.nameChanged(item.fileId, it) },
                    "新名称",
                )
            }
        }
        if (state.suggestions.isNotEmpty()) {
            MediaHubButton("保存待确认计划", onClick = actions.create, enabled = state.selected.isNotEmpty())
        }
        if (state.plans.isNotEmpty()) {
            MediaHubText("归档计划", fontSize = 15.sp, fontWeight = FontWeight.Bold)
        }
        state.plans.forEach { plan ->
            Row(
                modifier = Modifier.fillMaxWidth().padding(vertical = 8.dp),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Column(Modifier.weight(1f)) {
                    MediaHubText(plan.state, color = commandStateColor(plan.state), fontSize = 12.sp, fontWeight = FontWeight.Bold)
                    MediaHubText(
                        "${plan.stepIndex} / ${plan.stepTotal} 步 · ${plan.id}",
                        color = MediaHubColors.TextSecondary,
                        fontSize = 12.sp,
                    )
                    if (plan.errorMessage.isNotBlank()) {
                        MediaHubText(plan.errorMessage, color = MediaHubColors.Error, fontSize = 12.sp)
                    }
                }
                when (plan.state) {
                    "awaiting_confirmation" -> MediaHubButton("确认计划 ID", onClick = { actions.confirm(plan) })
                    "failed", "needs_attention" -> MediaHubButton("核对后继续", onClick = { actions.retry(plan) })
                }
            }
        }
    }
}
