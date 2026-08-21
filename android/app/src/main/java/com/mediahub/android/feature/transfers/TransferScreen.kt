package com.mediahub.android.feature.transfers
import androidx.activity.compose.BackHandler

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.selection.selectable
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.composables.icons.lucide.ArrowLeft
import com.composables.icons.lucide.Archive
import com.composables.icons.lucide.ArchiveRestore
import com.composables.icons.lucide.CircleAlert
import com.composables.icons.lucide.Clock3
import com.composables.icons.lucide.ListTodo
import com.composables.icons.lucide.Lucide
import com.composables.icons.lucide.RefreshCw
import com.composables.icons.lucide.RotateCcw
import com.composables.icons.lucide.Trash2
import com.mediahub.android.core.designsystem.BadgeVariant
import com.mediahub.android.core.designsystem.MediaHubBadge
import com.mediahub.android.core.designsystem.MediaHubButton
import com.mediahub.android.core.designsystem.MediaHubCard
import com.mediahub.android.core.designsystem.MediaHubColors
import com.mediahub.android.core.designsystem.MediaHubConfirmDialog
import com.mediahub.android.core.designsystem.MediaHubEmptyState
import com.mediahub.android.core.designsystem.MediaHubIcon
import com.mediahub.android.core.designsystem.MediaHubListDetail
import com.mediahub.android.core.designsystem.MediaHubIconButton
import com.mediahub.android.core.designsystem.MediaHubListDivider
import com.mediahub.android.core.designsystem.MediaHubPipelineStepper
import com.mediahub.android.core.designsystem.MediaHubPreferenceRow
import com.mediahub.android.core.designsystem.MediaHubSecondaryButton
import com.mediahub.android.core.designsystem.MediaHubSegmentedControl
import com.mediahub.android.core.designsystem.MediaHubSmallTitle
import com.mediahub.android.core.designsystem.PipelineStepItem
import com.mediahub.android.core.designsystem.MediaHubText
import com.mediahub.android.core.network.TransferJob
import com.mediahub.android.core.network.TransferNotification
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import java.time.Instant
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import java.util.Locale

private val timeFormatter = DateTimeFormatter.ofPattern("MM-dd HH:mm", Locale.CHINA)
    .withZone(ZoneId.systemDefault())

internal fun shouldCloseMissingTransferDetail(detailId: String?, state: TransferUiState): Boolean =
    detailId != null && state.initialized && state.jobs.none { it.id == detailId }

internal fun shouldCloseCompletedTransferDetail(detailId: String?, completedId: String?): Boolean =
    detailId != null && detailId == completedId

internal data class TransferActions(
    val select: (String) -> Unit,
    val refresh: () -> Unit,
    val back: () -> Unit,
    val retry: () -> Unit,
    val retryNotification: (TransferNotification) -> Unit,
    val showArchived: (Boolean) -> Unit,
    val setArchived: () -> Unit,
    val delete: () -> Unit,
)

@Composable
internal fun TransferRoute(
    viewModel: TransferViewModel,
    detailId: String?,
    onDetailChanged: (String?) -> Unit,
) {
    val uiState by viewModel.uiState.collectAsState()
    DisposableEffect(viewModel) {
        viewModel.startPolling()
        onDispose(viewModel::stopPolling)
    }
    LaunchedEffect(detailId, uiState.jobs) {
        if (detailId == null) {
            viewModel.closeDetail()
        } else if (uiState.jobs.any { it.id == detailId } && uiState.selectedId != detailId) {
            viewModel.select(detailId)
        } else if (shouldCloseMissingTransferDetail(detailId, uiState)) {
            viewModel.closeDetail()
            onDetailChanged(null)
        }
    }
    LaunchedEffect(detailId, uiState.archivedCompletedId) {
        val completedId = uiState.archivedCompletedId ?: return@LaunchedEffect
        if (shouldCloseCompletedTransferDetail(detailId, completedId)) {
            viewModel.closeDetail()
            onDetailChanged(null)
        }
        viewModel.consumeArchivedCompletion(completedId)
    }
    LaunchedEffect(detailId, uiState.deletedCompletedId) {
        val deletedId = uiState.deletedCompletedId ?: return@LaunchedEffect
        if (shouldCloseCompletedTransferDetail(detailId, deletedId)) {
            viewModel.closeDetail()
            onDetailChanged(null)
        }
        viewModel.consumeDeletedCompletion(deletedId)
    }
    val actions = TransferActions(
        select = { id ->
            viewModel.select(id)
            onDetailChanged(id)
        },
        refresh = viewModel::refresh,
        back = {
            viewModel.closeDetail()
            onDetailChanged(null)
        },
        retry = viewModel::retry,
        retryNotification = viewModel::retryNotification,
        showArchived = viewModel::showArchived,
        setArchived = viewModel::setSelectedArchived,
        delete = viewModel::deleteSelected,
    )
    TransferScreen(uiState = uiState, detailOpen = detailId != null, actions = actions)
}

@Composable
internal fun TransferScreen(
    uiState: TransferUiState,
    detailOpen: Boolean,
    actions: TransferActions,
) {
    BackHandler(enabled = detailOpen, onBack = actions.back)
    MediaHubListDetail(
        detailOpen = detailOpen,
        emptyTitle = "选择一个任务",
        emptyMessage = "从左侧打开转存进度",
        emptyIcon = Lucide.ListTodo,
        list = { TransferListPage(uiState = uiState, actions = actions) },
        detail = { TransferDetailPage(uiState = uiState, actions = actions) },
    )
}

@Composable
private fun TransferListPage(
    uiState: TransferUiState,
    actions: TransferActions,
) {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .padding(horizontal = 12.dp),
    ) {
        Row(
            modifier = Modifier.fillMaxWidth().padding(top = 8.dp, bottom = 12.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            MediaHubText(
                text = if (uiState.loading) "正在读取任务…" else "${uiState.jobs.size} 个任务",
                modifier = Modifier.weight(1f),
                color = MediaHubColors.TextMuted,
                fontSize = 13.sp,
            )
            MediaHubIconButton(
                imageVector = Lucide.RefreshCw,
                contentDescription = "刷新任务",
                onClick = actions.refresh,
                enabled = !uiState.refreshing,
            )
        }
        MediaHubSegmentedControl(
            options = listOf("current" to "当前", "archived" to "已归档"),
            selected = if (uiState.archived) "archived" else "current",
            onSelected = { actions.showArchived(it == "archived") },
            modifier = Modifier.padding(bottom = 12.dp),
        )

        uiState.errorMessage?.let { message ->
            Row(
                modifier = Modifier.fillMaxWidth().padding(bottom = 12.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                MediaHubIcon(
                    imageVector = Lucide.CircleAlert,
                    contentDescription = null,
                    tint = MediaHubColors.Error,
                    modifier = Modifier.size(17.dp),
                )
                Spacer(Modifier.width(8.dp))
                MediaHubText(text = message, color = MediaHubColors.Error, fontSize = 12.sp)
            }
        }

        if (!uiState.loading && uiState.jobs.isEmpty()) {
            MediaHubEmptyState(
                title = if (uiState.archived) "还没有归档任务" else "还没有转存任务",
                message = if (uiState.archived) "完成的任务可以归档到这里" else "在「发现」页搜索资源并创建转存任务",
                icon = Lucide.ListTodo,
                modifier = Modifier.padding(top = 36.dp),
            )
        } else {
            LazyColumn(
                modifier = Modifier.weight(1f),
                contentPadding = PaddingValues(bottom = 16.dp),
            ) {
                item(key = "jobs") {
                    MediaHubCard {
                        uiState.jobs.forEachIndexed { index, job ->
                            if (index > 0) MediaHubListDivider()
                            TransferRow(
                                job = job,
                                selected = uiState.selectedId == job.id,
                                onClick = { actions.select(job.id) },
                            )
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun TransferDetailPage(
    uiState: TransferUiState,
    actions: TransferActions,
) {
    Column(
        modifier = Modifier.fillMaxSize().padding(horizontal = 12.dp),
    ) {
        Row(
            modifier = Modifier.fillMaxWidth().padding(top = 8.dp, bottom = 10.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            MediaHubIconButton(Lucide.ArrowLeft, "返回任务列表", actions.back)
            Column(Modifier.weight(1f)) {
                MediaHubText("任务详情", fontSize = 19.sp, fontWeight = FontWeight.SemiBold)
                MediaHubText(
                    uiState.selected?.let(::transferTitle) ?: "正在读取任务",
                    color = MediaHubColors.TextMuted,
                    fontSize = 12.sp,
                )
            }
        }
        uiState.errorMessage?.let { MediaHubText(it, color = MediaHubColors.Error, fontSize = 12.sp) }
        LazyColumn(
            modifier = Modifier.weight(1f),
            contentPadding = PaddingValues(bottom = 18.dp),
        ) {
            uiState.selected?.let { selected ->
                item(key = selected.id) {
                    TransferDetail(
                        job = selected,
                        notification = uiState.notifications.firstOrNull { it.jobId == selected.id && it.state == "needs_attention" },
                        retrying = uiState.retrying,
                        notificationRetrying = uiState.notificationRetrying,
                        onRetry = actions.retry,
                        onRetryNotification = actions.retryNotification,
                        archived = uiState.archived,
                        archiving = uiState.archiving,
                        onSetArchived = actions.setArchived,
                        deleting = uiState.deleting,
                        onDelete = actions.delete,
                    )
                }
            }
        }
    }
}

@Composable
private fun TransferRow(job: TransferJob, selected: Boolean, onClick: () -> Unit) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .heightIn(min = 52.dp)
            .background(if (selected) MediaHubColors.SurfaceSelected else androidx.compose.ui.graphics.Color.Transparent)
            .selectable(selected = selected, role = Role.RadioButton, onClick = onClick)
            .padding(14.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Column(Modifier.weight(1f)) {
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                MediaHubText(
                    text = transferTitle(job),
                    fontSize = 14.sp,
                    fontWeight = FontWeight.SemiBold,
                    color = MediaHubColors.TextStrong,
                )
                MediaHubBadge(
                    text = job.source.uppercase(),
                    variant = BadgeVariant.Source,
                )
            }
            MediaHubText(
                text = formatTime(job.updatedAt),
                modifier = Modifier.padding(top = 4.dp),
                color = MediaHubColors.TextMuted,
                fontSize = 12.sp,
            )
            job.errorMessage?.takeIf { it.isNotBlank() }?.let { message ->
                MediaHubText(
                    text = message,
                    modifier = Modifier.padding(top = 4.dp),
                    color = MediaHubColors.Error,
                    fontSize = 12.sp,
                )
            }
        }
        MediaHubBadge(
            text = stateLabel(job.state),
            variant = stateBadgeVariant(job.state),
        )
    }
}

@Composable
private fun TransferDetail(
    job: TransferJob,
    notification: TransferNotification?,
    retrying: Boolean,
    notificationRetrying: Boolean,
    onRetry: () -> Unit,
    onRetryNotification: (TransferNotification) -> Unit,
    archived: Boolean,
    archiving: Boolean,
    onSetArchived: () -> Unit,
    deleting: Boolean,
    onDelete: () -> Unit,
) {
    var confirmingDelete by remember(job.id) { mutableStateOf(false) }
    Column(
        modifier = Modifier.fillMaxWidth().padding(top = 8.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        MediaHubCard(insideMargin = PaddingValues(horizontal = 16.dp, vertical = 12.dp)) {
            MediaHubPipelineStepper(steps = transferPipelineSteps(job))
        }

        MediaHubSmallTitle(text = "状态记录")
        job.errorMessage?.let { MediaHubText(text = it, color = MediaHubColors.Error, fontSize = 12.sp) }
        MediaHubCard {
            job.events.forEachIndexed { index, event ->
                if (index > 0) MediaHubListDivider()
                MediaHubPreferenceRow(
                    title = stateLabel(event.state),
                    summary = "${event.message.ifEmpty { "状态已更新" }} · ${formatTime(event.createdAt)}",
                    start = {
                        MediaHubIcon(
                            imageVector = Lucide.Clock3,
                            contentDescription = null,
                            tint = stateColor(event.state),
                            modifier = Modifier.size(15.dp).padding(end = 12.dp),
                        )
                    },
                )
            }
        }
        notification?.let { item ->
            MediaHubCard(insideMargin = PaddingValues(16.dp)) {
                MediaHubText(
                    text = "企业微信通知结果未知",
                    color = MediaHubColors.Warning,
                    fontSize = 12.sp,
                    fontWeight = FontWeight.SemiBold,
                )
                MediaHubText(
                    text = "再次发送可能产生重复消息。",
                    modifier = Modifier.padding(top = 6.dp),
                    color = MediaHubColors.TextMuted,
                    fontSize = 12.sp,
                )
                MediaHubButton(
                    label = if (notificationRetrying) "正在提交" else "确认并重发通知",
                    icon = Lucide.RotateCcw,
                    enabled = !notificationRetrying,
                    onClick = { onRetryNotification(item) },
                    modifier = Modifier.fillMaxWidth().padding(top = 12.dp),
                )
            }
        }
        if (job.retryable) {
            MediaHubButton(
                label = if (retrying) "正在重试" else "重试任务",
                icon = Lucide.RotateCcw,
                enabled = !retrying && !deleting,
                onClick = onRetry,
                modifier = Modifier.fillMaxWidth(),
            )
        }
        if (canArchiveTransfer(job)) {
            MediaHubSecondaryButton(
                label = if (archiving) "正在处理" else if (archived) "恢复到任务列表" else "归档任务",
                icon = if (archived) Lucide.ArchiveRestore else Lucide.Archive,
                enabled = !archiving && !deleting,
                onClick = onSetArchived,
                modifier = Modifier.fillMaxWidth(),
            )
        }
        if (canDeleteTransfer(job)) {
            MediaHubSecondaryButton(
                label = if (deleting) "正在删除" else "删除任务",
                icon = Lucide.Trash2,
                enabled = !deleting && !archiving && !retrying,
                onClick = { confirmingDelete = true },
                modifier = Modifier.fillMaxWidth(),
            )
            MediaHubConfirmDialog(
                visible = confirmingDelete,
                title = "确认删除该任务？",
                message = "只会删除本地任务记录，不会影响 115 或 Emby 中的媒体。",
                confirmLabel = if (deleting) "正在删除…" else "确认删除",
                cancelLabel = "取消",
                isDestructive = true,
                onConfirm = {
                    confirmingDelete = false
                    onDelete()
                },
                onDismiss = { confirmingDelete = false },
            )
        }
    }
}

private fun transferPipelineSteps(job: TransferJob): List<PipelineStepItem> {
    val state = job.state
    val isFailed = state == "failed" || state == "needs_attention"
    val stageIndex = when (state) {
        "queued" -> 0
        "transferring", "retry_wait" -> 1
        "transferred", "submitting_sync", "syncing" -> 2
        "refreshing_emby", "indexing_emby" -> 3
        "verifying_playback", "completed" -> 4
        else -> if (isFailed) 1 else 0
    }
    return listOf(
        PipelineStepItem("queued", "排队", stageIndex > 0, stageIndex == 0 && !isFailed, isFailed && stageIndex == 0),
        PipelineStepItem("transfer", "转存", stageIndex > 1, stageIndex == 1 && !isFailed, isFailed && stageIndex == 1),
        PipelineStepItem("strm", "生成STRM", stageIndex > 2, stageIndex == 2 && !isFailed, isFailed && stageIndex == 2),
        PipelineStepItem("emby", "刷新Emby", stageIndex > 3, stageIndex == 3 && !isFailed, isFailed && stageIndex == 3),
        PipelineStepItem("completed", "完成", stageIndex >= 4 && !isFailed, stageIndex == 4 && !isFailed, isFailed && stageIndex == 4),
    )
}

private fun stateBadgeVariant(state: String): BadgeVariant = when (state) {
    "completed" -> BadgeVariant.Success
    "failed", "needs_attention" -> BadgeVariant.Error
    "retry_wait" -> BadgeVariant.Warning
    else -> BadgeVariant.Primary
}

private fun stateLabel(state: String): String = when (state) {
    "queued" -> "排队中"
    "transferring" -> "正在转存"
    "retry_wait" -> "等待重试"
    "transferred" -> "已转存"
    "submitting_sync" -> "提交同步"
    "syncing" -> "生成 STRM"
    "refreshing_emby" -> "刷新 Emby"
    "indexing_emby" -> "等待入库"
    "verifying_playback" -> "验证播放"
    "completed" -> "已完成"
    "failed" -> "失败"
    "needs_attention" -> "需要确认"
    else -> "未知"
}

private fun stateColor(state: String) = when (state) {
    "completed" -> MediaHubColors.Source
    "failed", "needs_attention" -> MediaHubColors.Error
    "retry_wait" -> MediaHubColors.Warning
    else -> MediaHubColors.Accent
}

private fun formatTime(value: String): String = runCatching {
    timeFormatter.format(Instant.parse(value))
}.getOrDefault(value)

private fun transferTitle(job: TransferJob): String {
    if (job.season <= 0) return job.title
    if (job.episodeStart <= 0) return "${job.title} · S${job.season}"
    val episodes = if (job.episodeStart == job.episodeEnd) {
        job.episodeStart.toString()
    } else {
        "${job.episodeStart}-${job.episodeEnd}"
    }
    return "${job.title} · S${job.season}E$episodes"
}
