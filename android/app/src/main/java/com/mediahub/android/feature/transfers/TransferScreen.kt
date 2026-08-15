package com.mediahub.android.feature.transfers

import androidx.compose.foundation.background
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
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.composables.icons.lucide.Archive
import com.composables.icons.lucide.ArchiveRestore
import com.composables.icons.lucide.CircleAlert
import com.composables.icons.lucide.Clock3
import com.composables.icons.lucide.ListTodo
import com.composables.icons.lucide.Lucide
import com.composables.icons.lucide.RefreshCw
import com.composables.icons.lucide.RotateCcw
import com.mediahub.android.core.designsystem.MediaHubButton
import com.mediahub.android.core.designsystem.MediaHubColors
import com.mediahub.android.core.designsystem.MediaHubIcon
import com.mediahub.android.core.designsystem.MediaHubIconButton
import com.mediahub.android.core.designsystem.MediaHubSegmentedControl
import com.mediahub.android.core.designsystem.MediaHubText
import com.mediahub.android.core.network.TransferJob
import com.mediahub.android.core.network.TransferNotification
import java.time.Instant
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import java.util.Locale

private val timeFormatter = DateTimeFormatter.ofPattern("MM-dd HH:mm", Locale.CHINA)
    .withZone(ZoneId.systemDefault())

@Composable
internal fun TransferRoute(viewModel: TransferViewModel) {
    val uiState by viewModel.uiState.collectAsState()
    DisposableEffect(viewModel) {
        viewModel.startPolling()
        onDispose(viewModel::stopPolling)
    }
    TransferScreen(
        uiState = uiState,
        onSelect = viewModel::select,
        onRefresh = viewModel::refresh,
        onRetry = viewModel::retry,
        onRetryNotification = viewModel::retryNotification,
        onShowArchived = viewModel::showArchived,
        onSetArchived = viewModel::setSelectedArchived,
    )
}

@Composable
private fun TransferScreen(
    uiState: TransferUiState,
    onSelect: (String) -> Unit,
    onRefresh: () -> Unit,
    onRetry: () -> Unit,
    onRetryNotification: (TransferNotification) -> Unit,
    onShowArchived: (Boolean) -> Unit,
    onSetArchived: () -> Unit,
) {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(MediaHubColors.Canvas)
            .padding(horizontal = 16.dp),
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
                onClick = onRefresh,
                enabled = !uiState.refreshing,
            )
        }
        MediaHubSegmentedControl(
            options = listOf("current" to "当前", "archived" to "已归档"),
            selected = if (uiState.archived) "archived" else "current",
            onSelected = { onShowArchived(it == "archived") },
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
            Column(
                modifier = Modifier.fillMaxSize(),
                verticalArrangement = Arrangement.Center,
                horizontalAlignment = Alignment.CenterHorizontally,
            ) {
                MediaHubIcon(imageVector = Lucide.ListTodo, contentDescription = null, modifier = Modifier.size(30.dp))
                MediaHubText(
                    text = if (uiState.archived) "还没有归档任务" else "还没有转存任务",
                    modifier = Modifier.padding(top = 12.dp),
                    color = MediaHubColors.TextMuted,
                    fontSize = 13.sp,
                )
            }
        } else {
            LazyColumn(
                modifier = Modifier.weight(1f),
                contentPadding = PaddingValues(bottom = 16.dp),
                verticalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                items(uiState.jobs, key = { it.id }) { job ->
                    TransferRow(
                        job = job,
                        selected = uiState.selectedId == job.id,
                        onClick = { onSelect(job.id) },
                    )
                }
                uiState.selected?.let { selected ->
                    item(key = "detail-${selected.id}") {
                        TransferDetail(
                            job = selected,
                            notification = uiState.notifications.firstOrNull { it.jobId == selected.id && it.state == "needs_attention" },
                            retrying = uiState.retrying,
                            notificationRetrying = uiState.notificationRetrying,
                            onRetry = onRetry,
                            onRetryNotification = onRetryNotification,
                            archived = uiState.archived,
                            archiving = uiState.archiving,
                            onSetArchived = onSetArchived,
                        )
                    }
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
            .heightIn(min = 48.dp)
            .background(if (selected) MediaHubColors.SurfaceSelected else MediaHubColors.Surface, RoundedCornerShape(8.dp))
            .selectable(selected = selected, role = Role.RadioButton, onClick = onClick)
            .padding(14.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        MediaHubIcon(
            imageVector = if (job.state == "failed" || job.state == "needs_attention") Lucide.CircleAlert else Lucide.Clock3,
            contentDescription = null,
            tint = stateColor(job.state),
            modifier = Modifier.size(17.dp),
        )
        Spacer(Modifier.width(11.dp))
        Column(Modifier.weight(1f)) {
            MediaHubText(text = transferTitle(job), fontSize = 14.sp, fontWeight = FontWeight.SemiBold)
            MediaHubText(
                text = "${job.source} · ${formatTime(job.updatedAt)}",
                modifier = Modifier.padding(top = 5.dp),
                color = MediaHubColors.TextMuted,
                fontSize = 12.sp,
            )
        }
        MediaHubText(text = stateLabel(job.state), color = stateColor(job.state), fontSize = 12.sp)
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
) {
    Column(
        modifier = Modifier.fillMaxWidth().padding(top = 18.dp),
        verticalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        MediaHubText(text = "状态记录", color = MediaHubColors.TextStrong, fontSize = 16.sp, fontWeight = FontWeight.Medium)
        job.errorMessage?.let { MediaHubText(text = it, color = MediaHubColors.Error, fontSize = 12.sp) }
        job.events.forEach { event ->
            Row(verticalAlignment = Alignment.Top) {
                MediaHubIcon(
                    imageVector = Lucide.Clock3,
                    contentDescription = null,
                    tint = stateColor(event.state),
                    modifier = Modifier.size(15.dp),
                )
                Spacer(Modifier.width(9.dp))
                Column {
                    MediaHubText(text = stateLabel(event.state), color = MediaHubColors.TextStrong, fontSize = 12.sp)
                    MediaHubText(
                        text = "${event.message.ifEmpty { "状态已更新" }} · ${formatTime(event.createdAt)}",
                        modifier = Modifier.padding(top = 3.dp),
                        color = MediaHubColors.TextMuted,
                        fontSize = 12.sp,
                    )
                }
            }
        }
        notification?.let { item ->
            Column(
                modifier = Modifier.fillMaxWidth().padding(vertical = 8.dp),
                verticalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                MediaHubText(
                    text = "企业微信通知结果未知",
                    color = MediaHubColors.Warning,
                    fontSize = 12.sp,
                    fontWeight = FontWeight.SemiBold,
                )
                MediaHubText(text = "再次发送可能产生重复消息。", color = MediaHubColors.TextMuted, fontSize = 12.sp)
                MediaHubButton(
                    label = if (notificationRetrying) "正在提交" else "确认并重发通知",
                    icon = Lucide.RotateCcw,
                    enabled = !notificationRetrying,
                    onClick = { onRetryNotification(item) },
                    modifier = Modifier.fillMaxWidth(),
                )
            }
        }
        if (job.retryable) {
            MediaHubButton(
                label = if (retrying) "正在重试" else "重试任务",
                icon = Lucide.RotateCcw,
                enabled = !retrying,
                onClick = onRetry,
                modifier = Modifier.fillMaxWidth(),
            )
        }
        if (canArchiveTransfer(job)) {
            MediaHubButton(
                label = if (archiving) "正在处理" else if (archived) "恢复到任务列表" else "归档任务",
                icon = if (archived) Lucide.ArchiveRestore else Lucide.Archive,
                enabled = !archiving,
                onClick = onSetArchived,
                modifier = Modifier.fillMaxWidth(),
            )
        }
    }
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
