package com.mediahub.android.feature.subscriptions

import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.activity.compose.BackHandler
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.selection.toggleable
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.rememberScrollState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.composables.icons.lucide.ArrowLeft
import com.composables.icons.lucide.ChevronDown
import com.composables.icons.lucide.ChevronUp
import com.composables.icons.lucide.CircleAlert
import com.composables.icons.lucide.Download
import com.composables.icons.lucide.Clock3
import com.composables.icons.lucide.Lucide
import com.composables.icons.lucide.Pause
import com.composables.icons.lucide.Play
import com.composables.icons.lucide.Plus
import com.composables.icons.lucide.Save
import com.composables.icons.lucide.Upload
import com.composables.icons.lucide.Trash2
import com.mediahub.android.core.designsystem.MediaHubButton
import com.mediahub.android.core.designsystem.MediaHubColors
import com.mediahub.android.core.designsystem.MediaHubIcon
import com.mediahub.android.core.designsystem.MediaHubIconButton
import com.mediahub.android.core.designsystem.MediaHubSegmentedControl
import com.mediahub.android.core.designsystem.MediaHubText
import com.mediahub.android.core.designsystem.MediaHubTextField
import com.mediahub.android.core.network.SearchCandidate
import com.mediahub.android.core.network.SubscriptionRun
import java.io.ByteArrayOutputStream
import java.io.InputStream

@Composable
internal fun SubscriptionRoute(
    viewModel: SubscriptionViewModel,
    draft: SearchCandidate?,
    onDraftConsumed: () -> Unit,
) {
    val state by viewModel.uiState.collectAsState()
    DisposableEffect(viewModel) {
        viewModel.startPolling()
        onDispose(viewModel::stopPolling)
    }
    LaunchedEffect(draft) {
        if (draft != null) {
            viewModel.applyDraft(draft)
            onDraftConsumed()
        }
    }
    BackHandler(enabled = state.editing, onBack = viewModel::closeEditor)
    if (state.editing) {
        SubscriptionEditorScreen(
            state = state,
            onEditorChanged = viewModel::updateEditor,
            onBack = viewModel::closeEditor,
            onSave = viewModel::save,
            onToggle = viewModel::toggleEnabled,
            onRun = viewModel::runNow,
            onDelete = viewModel::delete,
        )
    } else {
        SubscriptionListScreen(
            state = state,
            onCreate = viewModel::createNew,
            onSelect = viewModel::select,
            onSetAllEnabled = viewModel::setAllEnabled,
            onExport = viewModel::exportBackup,
            onExportConsumed = viewModel::consumeExport,
            onImport = viewModel::importBackup,
            onFileError = viewModel::reportFileError,
        )
    }
}

@Composable
private fun SubscriptionListScreen(
    state: SubscriptionUiState,
    onCreate: () -> Unit,
    onSelect: (String) -> Unit,
    onSetAllEnabled: (Boolean) -> Unit,
    onExport: () -> Unit,
    onExportConsumed: () -> Unit,
    onImport: (String) -> Unit,
    onFileError: (String) -> Unit,
) {
    val context = LocalContext.current
    val createDocument = rememberLauncherForActivityResult(ActivityResultContracts.CreateDocument("application/json")) { uri ->
        val payload = state.exportPayload
        try {
            if (uri != null && payload != null) {
                context.contentResolver.openOutputStream(uri)?.use { it.write(payload.toByteArray()) }
					?: onFileError("无法写入订阅备份")
            }
        } catch (_: Exception) {
            onFileError("无法写入订阅备份")
        } finally {
            onExportConsumed()
        }
    }
    val openDocument = rememberLauncherForActivityResult(ActivityResultContracts.OpenDocument()) { uri ->
        if (uri == null) return@rememberLauncherForActivityResult
        try {
            val backup = context.contentResolver.openInputStream(uri)?.use { readLimited(it, 2 * 1024 * 1024) }
					?: throw IllegalStateException("missing document")
            onImport(backup)
        } catch (_: Exception) {
            onFileError("无法读取订阅备份")
        }
    }
    LaunchedEffect(state.exportPayload) {
        if (state.exportPayload != null) createDocument.launch("media-hub-subscriptions.json")
    }

    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(MediaHubColors.Canvas)
            .padding(horizontal = 16.dp),
    ) {
        Row(
            modifier = Modifier.fillMaxWidth().padding(top = 8.dp, bottom = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            MediaHubText(text = "${state.subscriptions.size} 个订阅", modifier = Modifier.weight(1f), color = MediaHubColors.TextMuted, fontSize = 13.sp)
            MediaHubIconButton(Lucide.Plus, "新建订阅", onCreate)
        }
        state.errorMessage?.let { ErrorLine(it) }
        state.actionMessage?.let { MediaHubText(text = it, color = MediaHubColors.Source, fontSize = 12.sp) }
        Row(
            modifier = Modifier.fillMaxWidth().horizontalScroll(rememberScrollState()).padding(top = 12.dp),
            horizontalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            MediaHubButton(label = "导入", icon = Lucide.Upload, onClick = { openDocument.launch(arrayOf("application/json", "text/json", "text/plain")) })
            MediaHubButton(label = "导出", icon = Lucide.Download, onClick = onExport)
            MediaHubButton(label = "全部暂停", icon = Lucide.Pause, enabled = state.subscriptions.isNotEmpty() && !state.saving, onClick = { onSetAllEnabled(false) })
            MediaHubButton(label = "全部启用", icon = Lucide.Play, enabled = state.subscriptions.isNotEmpty() && !state.saving, onClick = { onSetAllEnabled(true) })
        }
        LazyColumn(
            modifier = Modifier.fillMaxSize().padding(top = 20.dp),
            contentPadding = PaddingValues(bottom = 24.dp),
        ) {
            items(state.subscriptions, key = { it.id }) { item ->
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .heightIn(min = 48.dp)
                        .clickable(role = Role.Button) { onSelect(item.id) }
                        .padding(vertical = 15.dp),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    Spacer(
                        Modifier
                            .size(7.dp)
                            .background(
                                if (item.enabled) MediaHubColors.Accent else MediaHubColors.TextMuted,
                                RoundedCornerShape(4.dp),
                            ),
                    )
                    Column(Modifier.weight(1f).padding(horizontal = 12.dp)) {
                        MediaHubText(
                            text = item.title + if (item.season > 0) " · S${item.season}" else "",
                            fontSize = 14.sp,
                            fontWeight = FontWeight.Medium,
                        )
                        MediaHubText(
                            text = "${if (item.mediaType == "movie") "电影" else "剧集"} · TMDB ${item.tmdbId}" +
                                if (item.lastEpisode > 0) " · 已入库至 E${item.lastEpisode}" else "",
                            modifier = Modifier.padding(top = 5.dp),
                            color = MediaHubColors.TextMuted,
                            fontSize = 12.sp,
                        )
                    }
                    MediaHubText(
                        text = if (item.enabled) "运行中" else "已暂停",
                        color = if (item.enabled) MediaHubColors.Accent else MediaHubColors.TextMuted,
                        fontSize = 12.sp,
                    )
                }
            }
            if (!state.loading && state.subscriptions.isEmpty()) {
                item {
                    Column(
                        modifier = Modifier.fillMaxWidth().padding(vertical = 64.dp),
                        horizontalAlignment = Alignment.CenterHorizontally,
                    ) {
                        MediaHubIcon(Lucide.Clock3, contentDescription = null, modifier = Modifier.size(28.dp))
                        MediaHubText(
                            text = "还没有订阅",
                            modifier = Modifier.padding(top = 12.dp),
                            color = MediaHubColors.TextMuted,
                            fontSize = 12.sp,
                        )
                    }
                }
            }
        }
    }
}

@Composable
internal fun SubscriptionEditorScreen(
    state: SubscriptionUiState,
    onEditorChanged: (SubscriptionEditorState) -> Unit,
    onBack: () -> Unit,
    onSave: () -> Unit,
    onToggle: () -> Unit,
    onRun: () -> Unit,
    onDelete: () -> Unit,
) {
    val editor = state.editor
    val existing = state.selectedId != null
    var confirmingDelete by remember(state.selectedId) { mutableStateOf(false) }
    var sourcesExpanded by remember(state.selectedId) { mutableStateOf(false) }
    var advancedExpanded by remember(state.selectedId) { mutableStateOf(false) }
    LazyColumn(
        modifier = Modifier.fillMaxSize().background(MediaHubColors.Canvas),
        contentPadding = PaddingValues(start = 16.dp, end = 16.dp, bottom = 18.dp),
        verticalArrangement = Arrangement.spacedBy(14.dp),
    ) {
        item {
            Row(
                modifier = Modifier.fillMaxWidth().padding(top = 8.dp, bottom = 10.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                MediaHubIconButton(Lucide.ArrowLeft, "返回订阅列表", onBack)
                Column(Modifier.weight(1f)) {
                    MediaHubText(text = if (existing) "编辑订阅" else "新建订阅", fontSize = 21.sp, fontWeight = FontWeight.SemiBold)
                    MediaHubText(text = "身份、版本偏好与调度", color = MediaHubColors.TextMuted, fontSize = 12.sp)
                }
            }
            state.errorMessage?.let { ErrorLine(it) }
        }
        item { LabeledField("标题", editor.title, { onEditorChanged(editor.copy(title = it.take(300))) }, "媒体标题") }
        item { LabeledField("TMDB ID", editor.tmdbId, { onEditorChanged(editor.copy(tmdbId = it.take(20))) }, "数字 ID", KeyboardType.Number, enabled = !existing) }
        item {
            OptionGroup("类型", listOf("movie" to "电影", "series" to "剧集"), editor.mediaType, enabled = !existing) {
                onEditorChanged(editor.copy(mediaType = it, season = if (it == "movie") "0" else editor.season))
            }
        }
        if (editor.mediaType == "series") {
            item { LabeledField("季号", editor.season, { onEditorChanged(editor.copy(season = it.take(3))) }, "0 表示整部剧", KeyboardType.Number, enabled = !existing) }
        }
        item {
            OptionGroup("更新策略", listOf("once" to "首次入库", "upgrade" to "持续升级"), editor.policy) {
                onEditorChanged(editor.copy(policy = it))
            }
        }
        item {
            Column {
                MediaHubText(text = "质量预设", color = MediaHubColors.TextMuted, fontSize = 12.sp)
                MediaHubSegmentedControl(
                    options = listOf("standard" to "标准", "space" to "省空间", "balanced" to "均衡", "quality" to "高质量", "custom" to "自定义"),
                    selected = editor.qualityPreset,
                    onSelected = { onEditorChanged(applyQualityPreset(editor, it)) },
                    modifier = Modifier.padding(top = 7.dp),
                    role = Role.RadioButton,
                )
                if (editor.qualityPreset != "custom") {
                    MediaHubText(text = qualityPresetSummary(editor.qualityPreset), modifier = Modifier.padding(top = 7.dp), color = MediaHubColors.TextMuted, fontSize = 12.sp)
                }
            }
        }
        item {
            OptionGroup("检查频率", intervalOptions(editor.intervalMinutes), editor.intervalMinutes) {
                onEditorChanged(editor.copy(intervalMinutes = it))
            }
        }
        item {
            CollapsibleHeader(
                label = "资源来源",
                summary = if (editor.sourceIds.isEmpty()) "全部可用来源" else "已选择 ${editor.sourceIds.size} 个",
                expanded = sourcesExpanded,
                onClick = { sourcesExpanded = !sourcesExpanded },
            )
        }
        if (sourcesExpanded) {
            if (state.availableSources.isEmpty()) {
                item { MediaHubText(text = "来源清单暂不可用；不选择时会搜索全部来源。", color = MediaHubColors.TextMuted, fontSize = 12.sp) }
            } else {
                items(state.availableSources, key = { "source-${it.id}" }) { source ->
                    BooleanOption(source.label, editor.sourceIds.contains(source.id)) { checked ->
                        val sources = if (checked) (editor.sourceIds + source.id).distinct() else editor.sourceIds - source.id
                        onEditorChanged(editor.copy(sourceIds = sources))
                    }
                }
            }
        }
        item { BooleanOption("启用自动运行", editor.enabled) { onEditorChanged(editor.copy(enabled = it)) } }
        item {
            CollapsibleHeader(
                label = "高级规则与媒体身份",
                summary = "原始标题、年份与自定义筛选",
                expanded = advancedExpanded,
                onClick = { advancedExpanded = !advancedExpanded },
            )
        }
        if (advancedExpanded) {
            item { LabeledField("原始标题", editor.originalTitle, { onEditorChanged(editor.copy(originalTitle = it.take(300))) }, "可选") }
            item { LabeledField("年份", editor.year, { onEditorChanged(editor.copy(year = it.take(4))) }, "0 表示未知", KeyboardType.Number) }
            if (editor.qualityPreset == "custom") {
                item { LabeledField("偏好来源顺序", editor.preferredSources, { onEditorChanged(editor.copy(preferredSources = it)) }, "framehdr, juying") }
                item { LabeledField("分辨率", editor.resolutions, { onEditorChanged(editor.copy(resolutions = it)) }, "2160p, 1080p") }
                item { LabeledField("视频编码", editor.videoCodecs, { onEditorChanged(editor.copy(videoCodecs = it)) }, "HEVC, AVC") }
                item { LabeledField("动态范围", editor.dynamicRanges, { onEditorChanged(editor.copy(dynamicRanges = it)) }, "Dolby Vision, HDR10") }
                item { LabeledField("必须包含的音轨", editor.audioContains, { onEditorChanged(editor.copy(audioContains = it)) }, "Atmos, TrueHD") }
                item {
                    Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                        Column(Modifier.weight(1f)) { LabeledField("最小体积 GiB", editor.minSizeGiB, { onEditorChanged(editor.copy(minSizeGiB = it)) }, "0", KeyboardType.Decimal) }
                        Column(Modifier.weight(1f)) { LabeledField("最大体积 GiB", editor.maxSizeGiB, { onEditorChanged(editor.copy(maxSizeGiB = it)) }, "0", KeyboardType.Decimal) }
                    }
                }
                item { BooleanOption("设定体积范围时允许未知体积", editor.allowUnknownSize) { onEditorChanged(editor.copy(allowUnknownSize = it)) } }
                item { BooleanOption("同分时优先较小版本", editor.preferSmaller) { onEditorChanged(editor.copy(preferSmaller = it)) } }
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
                    MediaHubButton(
                        label = if (editor.enabled) "暂停" else "恢复",
                        icon = if (editor.enabled) Lucide.Pause else Lucide.Play,
                        enabled = !state.saving,
                        onClick = onToggle,
                        modifier = Modifier.weight(1f),
                    )
                    MediaHubButton(
                        label = "立即运行",
                        icon = Lucide.Play,
                        enabled = !state.saving,
                        onClick = onRun,
                        modifier = Modifier.weight(1f),
                    )
                }
            }
            item {
                if (confirmingDelete) {
                    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                        MediaHubButton(label = "取消", onClick = { confirmingDelete = false }, modifier = Modifier.weight(1f))
                        MediaHubButton(label = "确认删除", icon = Lucide.Trash2, enabled = !state.saving, onClick = onDelete, modifier = Modifier.weight(1f))
                    }
                } else {
                    MediaHubButton(label = "删除订阅", icon = Lucide.Trash2, onClick = { confirmingDelete = true }, modifier = Modifier.fillMaxWidth())
                }
            }
            item {
                MediaHubText(text = "运行历史", modifier = Modifier.padding(top = 18.dp), fontSize = 17.sp, fontWeight = FontWeight.Medium)
            }
            items(state.runs, key = { it.id }) { run -> RunRow(run) }
            if (state.runs.isEmpty()) item { MediaHubText(text = "还没有运行记录", color = MediaHubColors.TextMuted, fontSize = 12.sp) }
        }
    }
}

@Composable
private fun CollapsibleHeader(label: String, summary: String, expanded: Boolean, onClick: () -> Unit) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .heightIn(min = 48.dp)
            .clickable(role = Role.Button, onClick = onClick)
            .background(MediaHubColors.SurfaceInput, RoundedCornerShape(7.dp))
            .padding(horizontal = 12.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Column(Modifier.weight(1f)) {
            MediaHubText(text = label, fontSize = 13.sp, fontWeight = FontWeight.Medium)
            MediaHubText(text = summary, color = MediaHubColors.TextMuted, fontSize = 12.sp)
        }
        MediaHubIcon(if (expanded) Lucide.ChevronUp else Lucide.ChevronDown, contentDescription = null, modifier = Modifier.size(17.dp))
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
        Row(Modifier.fillMaxWidth().padding(top = 7.dp), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            values.forEach { (value, display) ->
                Row(
                    modifier = Modifier
                        .weight(1f)
                        .heightIn(min = 48.dp)
                        .background(if (selected == value) MediaHubColors.SurfaceSelected else MediaHubColors.SurfaceInput, RoundedCornerShape(7.dp))
                        .selectable(selected = selected == value, enabled = enabled, role = Role.RadioButton) { onSelected(value) }
                        .padding(12.dp),
                    horizontalArrangement = Arrangement.Center,
                ) {
                    MediaHubText(text = display, color = if (selected == value) MediaHubColors.Accent else MediaHubColors.TextSecondary, fontSize = 12.sp)
                }
            }
        }
    }
}

@Composable
private fun BooleanOption(label: String, checked: Boolean, onChanged: (Boolean) -> Unit) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .heightIn(min = 48.dp)
            .toggleable(value = checked, role = Role.Checkbox) { onChanged(it) }
            .background(MediaHubColors.SurfaceInput, RoundedCornerShape(7.dp))
            .padding(12.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Spacer(Modifier.size(16.dp).background(if (checked) MediaHubColors.Accent else MediaHubColors.Border, RoundedCornerShape(4.dp)))
        Spacer(Modifier.width(10.dp))
        MediaHubText(text = label, fontSize = 12.sp)
    }
}

@Composable
private fun RunRow(run: SubscriptionRun) {
    Row(Modifier.fillMaxWidth().padding(vertical = 12.dp), verticalAlignment = Alignment.CenterVertically) {
        Spacer(
            Modifier.size(7.dp).background(
                when (run.state) {
                    "completed" -> MediaHubColors.Source
                    "failed", "needs_attention" -> MediaHubColors.Error
                    else -> MediaHubColors.Accent
                },
                RoundedCornerShape(4.dp),
            ),
        )
        Column(Modifier.padding(start = 11.dp)) {
            MediaHubText(text = runStateLabel(run.state), fontSize = 12.sp, fontWeight = FontWeight.Medium)
            MediaHubText(text = run.message ?: "无补充信息", color = MediaHubColors.TextMuted, fontSize = 12.sp)
        }
    }
}

@Composable
private fun ErrorLine(message: String) {
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

private fun readLimited(input: InputStream, limit: Int): String {
    val output = ByteArrayOutputStream()
    val buffer = ByteArray(8192)
    var total = 0
    while (true) {
        val count = input.read(buffer)
        if (count < 0) break
        total += count
        if (total > limit) throw IllegalArgumentException("backup too large")
        output.write(buffer, 0, count)
    }
    return output.toString(Charsets.UTF_8.name())
}
