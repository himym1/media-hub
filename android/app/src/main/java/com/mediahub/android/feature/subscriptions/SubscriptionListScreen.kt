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
import androidx.compose.foundation.lazy.items
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
import com.mediahub.android.core.designsystem.MediaHubColors
import com.mediahub.android.core.designsystem.MediaHubIcon
import com.mediahub.android.core.designsystem.MediaHubIconButton
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
            .background(MediaHubColors.Canvas)
            .padding(horizontal = 16.dp),
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
            Column(
                modifier = Modifier.fillMaxWidth().background(MediaHubColors.SurfaceInput, RoundedCornerShape(7.dp)),
            ) {
                SubscriptionMenuAction(Lucide.Upload, "导入订阅") {
                    toolsExpanded = false
                    openDocument.launch(arrayOf("application/json", "text/json", "text/plain"))
                }
                SubscriptionMenuAction(Lucide.Download, "导出订阅") {
                    toolsExpanded = false
                    actions.export()
                }
                SubscriptionMenuAction(
                    Lucide.Pause, "全部暂停", enabled = state.subscriptions.isNotEmpty() && !state.saving,
                ) {
                    toolsExpanded = false
                    actions.setAllEnabled(false)
                }
                SubscriptionMenuAction(
                    Lucide.Play, "全部启用", enabled = state.subscriptions.isNotEmpty() && !state.saving,
                ) {
                    toolsExpanded = false
                    actions.setAllEnabled(true)
                }
            }
        }
        state.errorMessage?.let { ErrorLine(it) }
        state.actionMessage?.let { MediaHubText(text = it, color = MediaHubColors.Source, fontSize = 12.sp) }
        LazyColumn(modifier = Modifier.fillMaxSize().padding(top = 8.dp)) {
            items(state.subscriptions, key = { it.id }) { item ->
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .heightIn(min = 48.dp)
                        .clickable(role = Role.Button) { actions.select(item.id) }
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
private fun SubscriptionMenuAction(
    icon: androidx.compose.ui.graphics.vector.ImageVector,
    label: String,
    enabled: Boolean = true,
    onClick: () -> Unit,
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .heightIn(min = 48.dp)
            .clickable(enabled = enabled, role = Role.Button, onClick = onClick)
            .padding(horizontal = 14.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        MediaHubIcon(icon, contentDescription = null, modifier = Modifier.size(18.dp))
        MediaHubText(label, modifier = Modifier.padding(start = 11.dp), fontSize = 13.sp)
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
