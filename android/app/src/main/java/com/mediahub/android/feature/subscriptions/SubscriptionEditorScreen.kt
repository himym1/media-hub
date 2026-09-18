package com.mediahub.android.feature.subscriptions

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.composables.icons.lucide.ArrowLeft
import com.composables.icons.lucide.CircleAlert
import com.composables.icons.lucide.Lucide
import com.composables.icons.lucide.Pause
import com.composables.icons.lucide.Play
import com.composables.icons.lucide.Save
import com.composables.icons.lucide.Trash2
import com.mediahub.android.core.designsystem.BadgeVariant
import com.mediahub.android.core.designsystem.MediaHubBadge
import com.mediahub.android.core.designsystem.MediaHubButton
import com.mediahub.android.core.designsystem.MediaHubCard
import com.mediahub.android.core.designsystem.MediaHubCheckboxRow
import com.mediahub.android.core.designsystem.MediaHubColors
import com.mediahub.android.core.designsystem.MediaHubConfirmDialog
import com.mediahub.android.core.designsystem.MediaHubIcon
import com.mediahub.android.core.designsystem.MediaHubIconButton
import com.mediahub.android.core.designsystem.MediaHubListDivider
import com.mediahub.android.core.designsystem.MediaHubPreferenceRow
import com.mediahub.android.core.designsystem.MediaHubSecondaryButton
import com.mediahub.android.core.designsystem.MediaHubTextButton
import com.mediahub.android.core.designsystem.MediaHubSegmentedControl
import com.mediahub.android.core.designsystem.MediaHubSmallTitle
import com.mediahub.android.core.designsystem.MediaHubSwitchRow
import com.mediahub.android.core.designsystem.MediaHubText
import com.mediahub.android.core.designsystem.MediaHubTextField
import com.mediahub.android.core.designsystem.MediaHubTopAppBar
import com.mediahub.android.core.network.SubscriptionRun

internal data class SubscriptionEditorActions(
    val editorChanged: (SubscriptionEditorState) -> Unit,
    val back: () -> Unit,
    val backEnabled: Boolean = true,
    val save: () -> Unit,
    val toggle: () -> Unit,
    val run: () -> Unit,
    val delete: () -> Unit,
)

@Composable
internal fun SubscriptionEditorScreen(
    state: SubscriptionUiState,
    actions: SubscriptionEditorActions,
) {
    val onEditorChanged = actions.editorChanged
    val onBack = actions.back
    val onSave = actions.save
    val onToggle = actions.toggle
    val onRun = actions.run
    val onDelete = actions.delete
    val editor = state.editor
    val existing = state.selectedId != null
    var confirmingDelete by remember(state.selectedId) { mutableStateOf(false) }
    var sourcesExpanded by remember(state.selectedId) { mutableStateOf(false) }
    var advancedExpanded by remember(state.selectedId) { mutableStateOf(false) }
    Column(modifier = Modifier.fillMaxSize()) {
        MediaHubTopAppBar(
            title = if (existing) "编辑订阅" else "新建订阅",
            subtitle = "身份、版本偏好与调度",
            navigationIcon = {
                MediaHubIconButton(
                    Lucide.ArrowLeft, "返回订阅列表", onBack, enabled = actions.backEnabled,
                )
            },
        )
    LazyColumn(
        modifier = Modifier.fillMaxSize(),
        contentPadding = PaddingValues(start = 16.dp, end = 16.dp, bottom = 18.dp),
        verticalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        state.errorMessage?.let { item { ErrorLine(it) } }
        item { MediaHubSmallTitle(text = "媒体身份") }
        item {
            MediaHubCard(insideMargin = PaddingValues(16.dp), elevated = false) {
                LabeledField("标题", editor.title, { onEditorChanged(editor.copy(title = it.take(300))) }, "媒体标题")
                Spacer(Modifier.size(12.dp))
                LabeledField("TMDB ID", editor.tmdbId, { onEditorChanged(editor.copy(tmdbId = it.take(20))) }, "数字 ID", KeyboardType.Number, enabled = !existing)
                Spacer(Modifier.size(12.dp))
                OptionGroup("类型", listOf("movie" to "电影", "series" to "剧集"), editor.mediaType, enabled = !existing) {
                    onEditorChanged(editor.copy(mediaType = it, season = if (it == "movie") "0" else editor.season))
                }
                if (editor.mediaType == "series") {
                    Spacer(Modifier.size(12.dp))
                    LabeledField("季号", editor.season, { onEditorChanged(editor.copy(season = it.take(3))) }, "0 表示整部剧", KeyboardType.Number, enabled = !existing)
                }
            }
        }
        item { MediaHubSmallTitle(text = "更新与调度") }
        item {
            MediaHubCard(insideMargin = PaddingValues(16.dp), elevated = false) {
                MediaHubSwitchRow(
                    title = "启用自动运行",
                    summary = "按设定的检查频率周期性检索资源",
                    checked = editor.enabled,
                    onCheckedChange = { onEditorChanged(editor.copy(enabled = it)) },
                )
                Spacer(Modifier.size(8.dp))
                MediaHubListDivider()
                Spacer(Modifier.size(12.dp))
                OptionGroup("更新策略", listOf("once" to "首次入库", "upgrade" to "持续升级"), editor.policy) {
                    onEditorChanged(editor.copy(policy = it))
                }
                Spacer(Modifier.size(12.dp))
                MediaHubText(text = "质量预设", color = MediaHubColors.TextMuted, fontSize = 12.sp)
                MediaHubSegmentedControl(
                    options = listOf("standard" to "标准", "space" to "省空间", "balanced" to "均衡", "quality" to "高质量", "custom" to "自定义"),
                    selected = editor.qualityPreset,
                    onSelected = { onEditorChanged(applyQualityPreset(editor, it)) },
                    modifier = Modifier.padding(top = 7.dp),
                    role = Role.RadioButton,
                    raised = false,
                )
                if (editor.qualityPreset != "custom") {
                    MediaHubText(text = qualityPresetSummary(editor.qualityPreset), modifier = Modifier.padding(top = 7.dp), color = MediaHubColors.TextMuted, fontSize = 12.sp)
                }
                Spacer(Modifier.size(12.dp))
                OptionGroup("检查频率", intervalOptions(editor.intervalMinutes), editor.intervalMinutes) {
                    onEditorChanged(editor.copy(intervalMinutes = it))
                }
            }
        }
        item {
            MediaHubCard(elevated = false) {
                MediaHubPreferenceRow(
                    title = "资源来源",
                    summary = if (editor.sourceIds.isEmpty()) "全部可用来源" else "已选择 ${editor.sourceIds.size} 个",
                    onClick = { sourcesExpanded = !sourcesExpanded },
                )
                if (sourcesExpanded) {
                    MediaHubListDivider()
                    if (state.availableSources.isEmpty()) {
                        MediaHubText(
                            text = "来源清单暂不可用；不选择时会搜索全部来源。",
                            modifier = Modifier.padding(horizontal = 16.dp, vertical = 12.dp),
                            color = MediaHubColors.TextMuted,
                            fontSize = 12.sp,
                        )
                    } else {
                        state.availableSources.forEachIndexed { index, source ->
                            if (index > 0) MediaHubListDivider()
                            MediaHubCheckboxRow(
                                title = source.label,
                                checked = editor.sourceIds.contains(source.id),
                                onCheckedChange = { checked ->
                                    val sources = if (checked) (editor.sourceIds + source.id).distinct() else editor.sourceIds - source.id
                                    onEditorChanged(editor.copy(sourceIds = sources))
                                },
                            )
                        }
                    }
                }
            }
        }
        item {
            MediaHubCard(elevated = false) {
                MediaHubPreferenceRow(
                    title = "高级规则与媒体身份",
                    summary = "原始标题、年份与自定义筛选",
                    onClick = { advancedExpanded = !advancedExpanded },
                )
                if (advancedExpanded) {
                    MediaHubListDivider()
                    Column(Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                        LabeledField("原始标题", editor.originalTitle, { onEditorChanged(editor.copy(originalTitle = it.take(300))) }, "可选")
                        LabeledField("年份", editor.year, { onEditorChanged(editor.copy(year = it.take(4))) }, "0 表示未知", KeyboardType.Number)
                        if (editor.qualityPreset == "custom") {
                            LabeledField("偏好来源顺序", editor.preferredSources, { onEditorChanged(editor.copy(preferredSources = it)) }, "framehdr, juying")
                            LabeledField("分辨率", editor.resolutions, { onEditorChanged(editor.copy(resolutions = it)) }, "2160p, 1080p")
                            LabeledField("视频编码", editor.videoCodecs, { onEditorChanged(editor.copy(videoCodecs = it)) }, "HEVC, AVC")
                            LabeledField("动态范围", editor.dynamicRanges, { onEditorChanged(editor.copy(dynamicRanges = it)) }, "Dolby Vision, HDR10")
                            LabeledField("必须包含的音轨", editor.audioContains, { onEditorChanged(editor.copy(audioContains = it)) }, "Atmos, TrueHD")
                            Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                                Column(Modifier.weight(1f)) { LabeledField("最小体积 GiB", editor.minSizeGiB, { onEditorChanged(editor.copy(minSizeGiB = it)) }, "0", KeyboardType.Decimal) }
                                Column(Modifier.weight(1f)) { LabeledField("最大体积 GiB", editor.maxSizeGiB, { onEditorChanged(editor.copy(maxSizeGiB = it)) }, "0", KeyboardType.Decimal) }
                            }
                            MediaHubCheckboxRow(
                                title = "设定体积范围时允许未知体积",
                                checked = editor.allowUnknownSize,
                                onCheckedChange = { onEditorChanged(editor.copy(allowUnknownSize = it)) },
                            )
                            MediaHubCheckboxRow(
                                title = "同分时优先较小版本",
                                checked = editor.preferSmaller,
                                onCheckedChange = { onEditorChanged(editor.copy(preferSmaller = it)) },
                            )
                        }
                    }
                }
            }
        }
        item {
            MediaHubButton(
                label = if (state.saving) "保存中" else "保存订阅",
                icon = Lucide.Save,
                enabled = !state.saving,
                onClick = onSave,
                modifier = Modifier.fillMaxWidth().padding(top = 6.dp),
            )
        }
        if (existing) {
            item {
                Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                    MediaHubSecondaryButton(
                        label = if (editor.enabled) "暂停" else "恢复",
                        icon = if (editor.enabled) Lucide.Pause else Lucide.Play,
                        enabled = !state.saving,
                        onClick = onToggle,
                        modifier = Modifier.weight(1f),
                    )
                    MediaHubSecondaryButton(
                        label = "运行",
                        icon = Lucide.Play,
                        enabled = !state.saving,
                        onClick = onRun,
                        modifier = Modifier.weight(1f),
                    )
                }
            }
            item {
                MediaHubTextButton(
                    label = "删除订阅",
                    icon = Lucide.Trash2,
                    destructive = true,
                    onClick = { confirmingDelete = true },
                    modifier = Modifier.fillMaxWidth(),
                )
                MediaHubConfirmDialog(
                    visible = confirmingDelete,
                    title = "确认删除该订阅？",
                    message = "删除后将停止自动监测和追更「${editor.title}」，已入库的媒体不受影响。",
                    confirmLabel = "确认删除",
                    cancelLabel = "取消",
                    isDestructive = true,
                    onConfirm = {
                        confirmingDelete = false
                        onDelete()
                    },
                    onDismiss = { confirmingDelete = false },
                )
            }
            item { MediaHubSmallTitle(text = "运行历史") }
            if (state.runs.isEmpty()) {
                item { MediaHubText(text = "还没有运行记录", color = MediaHubColors.TextMuted, fontSize = 12.sp) }
            } else {
                item {
                    MediaHubCard {
                        state.runs.forEachIndexed { index, run ->
                            if (index > 0) MediaHubListDivider()
                            RunRow(run)
                        }
                    }
                }
            }
        }
    }
    }
}

private fun intervalOptions(current: String): List<Pair<String, String>> {
    val options = mutableListOf("30" to "30 分", "60" to "1 小时", "360" to "6 小时", "1440" to "每天")
    if (current.isNotBlank() && options.none { it.first == current }) options += current to "$current 分"
    return options
}

private fun qualityPresetSummary(preset: String) = when (preset) {
    "space" -> "1080p HEVC，最大 20 GiB；同等质量优先较小版本。"
    "balanced" -> "优先 2160p / HEVC，允许 1080p 与 AVC，最大 40 GiB。"
    "quality" -> "仅选择 2160p Dolby Vision / HDR10，最小 15 GiB。"
    else -> "不限制清晰度、编码和体积；体积未知的资源也可参与选择。"
}

@Composable
private fun LabeledField(
    label: String,
    value: String,
    onValueChange: (String) -> Unit,
    placeholder: String,
    keyboardType: KeyboardType = KeyboardType.Text,
    enabled: Boolean = true,
) {
    Column {
        MediaHubText(text = label, color = MediaHubColors.TextMuted, fontSize = 12.sp)
        MediaHubTextField(
            value = value,
            onValueChange = onValueChange,
            placeholder = placeholder,
            keyboardType = keyboardType,
            enabled = enabled,
            modifier = Modifier.fillMaxWidth().padding(top = 7.dp),
        )
    }
}

@Composable
private fun OptionGroup(
    label: String,
    values: List<Pair<String, String>>,
    selected: String,
    enabled: Boolean = true,
    onSelected: (String) -> Unit,
) {
    Column {
        MediaHubText(text = label, color = MediaHubColors.TextMuted, fontSize = 12.sp)
        MediaHubSegmentedControl(
            options = values,
            selected = selected,
            onSelected = { if (enabled) onSelected(it) },
            modifier = Modifier.padding(top = 7.dp),
            role = Role.RadioButton,
            raised = false,
        )
    }
}

@Composable
private fun RunRow(run: SubscriptionRun) {
    val variant = when (run.state) {
        "completed" -> BadgeVariant.Success
        "failed", "needs_attention" -> BadgeVariant.Error
        "searching" -> BadgeVariant.Primary
        "duplicate", "no_match" -> BadgeVariant.Neutral
        else -> BadgeVariant.Warning
    }
    MediaHubPreferenceRow(
        title = runStateLabel(run.state),
        summary = run.message ?: "无补充信息",
        end = {
            MediaHubBadge(text = runStateLabel(run.state), variant = variant)
        },
    )
}

@Composable
internal fun ErrorLine(message: String) {
    Row(
        modifier = Modifier.fillMaxWidth().padding(vertical = 12.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        MediaHubIcon(Lucide.CircleAlert, contentDescription = null, tint = MediaHubColors.Error, modifier = Modifier.size(16.dp))
        MediaHubText(text = message, modifier = Modifier.padding(start = 8.dp), color = MediaHubColors.Error, fontSize = 12.sp)
    }
}

private fun runStateLabel(state: String) = when (state) {
    "queued" -> "等待执行"
    "searching" -> "正在搜索"
    "retry_wait" -> "等待重试"
    "no_match" -> "没有匹配"
    "duplicate" -> "已去重"
    "enqueued" -> "已加入任务"
    "completed" -> "已完成"
    "failed" -> "失败"
    "needs_attention" -> "需要确认"
    else -> state
}
