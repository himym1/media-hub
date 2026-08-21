package com.mediahub.android.feature.subscriptions

import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.composables.icons.lucide.Clock3
import com.composables.icons.lucide.Download
import com.composables.icons.lucide.EllipsisVertical
import com.composables.icons.lucide.Lucide
import com.composables.icons.lucide.Pause
import com.composables.icons.lucide.Play
import com.composables.icons.lucide.Plus
import com.composables.icons.lucide.Upload
import androidx.compose.foundation.layout.Arrangement
import com.mediahub.android.core.designsystem.BadgeVariant
import com.mediahub.android.core.designsystem.MediaHubBadge
import com.mediahub.android.core.designsystem.MediaHubCard
import com.mediahub.android.core.designsystem.MediaHubColors
import com.mediahub.android.core.designsystem.MediaHubEmptyState
import com.mediahub.android.core.designsystem.MediaHubIcon
import com.mediahub.android.core.designsystem.MediaHubIconButton
import com.mediahub.android.core.designsystem.MediaHubListDivider
import com.mediahub.android.core.designsystem.MediaHubPreferenceRow
import com.mediahub.android.core.designsystem.MediaHubText
import java.io.ByteArrayOutputStream
import java.io.InputStream

internal data class SubscriptionListActions(
    val create: () -> Unit,
    val select: (String) -> Unit,
    val setAllEnabled: (Boolean) -> Unit,
    val export: () -> Unit,
    val exportConsumed: () -> Unit,
    val import: (String) -> Unit,
    val fileError: (String) -> Unit,
)

@Composable
internal fun SubscriptionListScreen(
    state: SubscriptionUiState,
    actions: SubscriptionListActions,
) {
    val context = LocalContext.current
    val createDocument = rememberLauncherForActivityResult(ActivityResultContracts.CreateDocument("application/json")) { uri ->
        val payload = state.exportPayload
        try {
            if (uri != null && payload != null) {
                context.contentResolver.openOutputStream(uri)?.use { it.write(payload.toByteArray()) }
                    ?: actions.fileError("无法写入订阅备份")
            }
        } catch (_: Exception) {
            actions.fileError("无法写入订阅备份")
        } finally {
            actions.exportConsumed()
        }
    }
    val openDocument = rememberLauncherForActivityResult(ActivityResultContracts.OpenDocument()) { uri ->
        if (uri == null) return@rememberLauncherForActivityResult
        try {
            val backup = context.contentResolver.openInputStream(uri)?.use { readLimited(it, 2 * 1024 * 1024) }
                ?: throw IllegalStateException("missing document")
            actions.import(backup)
        } catch (_: Exception) {
            actions.fileError("无法读取订阅备份")
        }
    }
    LaunchedEffect(state.exportPayload) {
        if (state.exportPayload != null) createDocument.launch("media-hub-subscriptions.json")
    }

    var toolsExpanded by remember { mutableStateOf(false) }
    Column(
        modifier = Modifier
            .fillMaxSize()
            .padding(horizontal = 12.dp),
    ) {
        Row(
            modifier = Modifier.fillMaxWidth().padding(top = 8.dp, bottom = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            MediaHubText(
                text = "${state.subscriptions.size} 个订阅",
                modifier = Modifier.weight(1f),
                color = MediaHubColors.TextMuted,
                fontSize = 13.sp,
            )
            MediaHubIconButton(Lucide.EllipsisVertical, "更多订阅操作", { toolsExpanded = !toolsExpanded })
            MediaHubIconButton(Lucide.Plus, "新建订阅", actions.create)
        }
        if (toolsExpanded) {
            MediaHubCard(modifier = Modifier.padding(bottom = 8.dp)) {
                MediaHubPreferenceRow(
                    title = "导入订阅",
                    onClick = {
                        toolsExpanded = false
                        openDocument.launch(arrayOf("application/json", "text/json", "text/plain"))
                    },
                    start = { MediaHubIcon(Lucide.Upload, contentDescription = null, modifier = Modifier.size(18.dp).padding(end = 12.dp)) },
                )
                MediaHubListDivider()
                MediaHubPreferenceRow(
                    title = "导出订阅",
                    onClick = {
                        toolsExpanded = false
                        actions.export()
                    },
                    start = { MediaHubIcon(Lucide.Download, contentDescription = null, modifier = Modifier.size(18.dp).padding(end = 12.dp)) },
                )
                MediaHubListDivider()
                MediaHubPreferenceRow(
                    title = "全部暂停",
                    enabled = state.subscriptions.isNotEmpty() && !state.saving,
                    onClick = {
                        toolsExpanded = false
                        actions.setAllEnabled(false)
                    },
                    start = { MediaHubIcon(Lucide.Pause, contentDescription = null, modifier = Modifier.size(18.dp).padding(end = 12.dp)) },
                )
                MediaHubListDivider()
                MediaHubPreferenceRow(
                    title = "全部启用",
                    enabled = state.subscriptions.isNotEmpty() && !state.saving,
                    onClick = {
                        toolsExpanded = false
                        actions.setAllEnabled(true)
                    },
                    start = { MediaHubIcon(Lucide.Play, contentDescription = null, modifier = Modifier.size(18.dp).padding(end = 12.dp)) },
                )
            }
        }
        state.errorMessage?.let { ErrorLine(it) }
        state.actionMessage?.let { MediaHubText(text = it, color = MediaHubColors.Source, fontSize = 12.sp) }
        LazyColumn(
            modifier = Modifier.fillMaxSize().padding(top = 8.dp),
        ) {
            if (state.subscriptions.isNotEmpty()) {
                item(key = "subscriptions") {
                    MediaHubCard {
                        state.subscriptions.forEachIndexed { index, item ->
                            if (index > 0) MediaHubListDivider()
                            Row(
                                modifier = Modifier
                                    .fillMaxWidth()
                                    .heightIn(min = 52.dp)
                                    .clickable(role = Role.Button) { actions.select(item.id) }
                                    .semantics { contentDescription = "打开订阅 ${item.title}" }
                                    .padding(horizontal = 14.dp, vertical = 12.dp),
                                verticalAlignment = Alignment.CenterVertically,
                            ) {
                                Column(Modifier.weight(1f)) {
                                    Row(
                                        verticalAlignment = Alignment.CenterVertically,
                                        horizontalArrangement = Arrangement.spacedBy(8.dp),
                                    ) {
                                        MediaHubText(
                                            text = item.title + if (item.season > 0) " · S${item.season}" else "",
                                            fontSize = 14.sp,
                                            fontWeight = FontWeight.SemiBold,
                                            color = MediaHubColors.TextStrong,
                                        )
                                        MediaHubBadge(
                                            text = if (item.mediaType == "movie") "电影" else "剧集",
                                            variant = BadgeVariant.Neutral,
                                        )
                                    }
                                    Row(
                                        modifier = Modifier.padding(top = 4.dp),
                                        verticalAlignment = Alignment.CenterVertically,
                                        horizontalArrangement = Arrangement.spacedBy(6.dp),
                                    ) {
                                        MediaHubText(
                                            text = "TMDB ${item.tmdbId}",
                                            color = MediaHubColors.TextMuted,
                                            fontSize = 12.sp,
                                        )
                                        if (item.lastEpisode > 0) {
                                            MediaHubBadge(
                                                text = "已入库至 E${item.lastEpisode}",
                                                variant = BadgeVariant.Primary,
                                            )
                                        }
                                    }
                                }
                                MediaHubBadge(
                                    text = if (item.enabled) "运行中" else "已暂停",
                                    variant = if (item.enabled) BadgeVariant.Success else BadgeVariant.Neutral,
                                )
                            }
                        }
                    }
                }
            }
            if (!state.loading && state.subscriptions.isEmpty()) {
                item {
                    MediaHubEmptyState(
                        title = "暂无自动追剧订阅",
                        message = "可在「发现」搜索页为剧集创建订阅，或点击右上角「+」手动新建",
                        icon = Lucide.Clock3,
                        modifier = Modifier.padding(vertical = 36.dp),
                    )
                }
            }
        }
    }
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
