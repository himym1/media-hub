package com.mediahub.android.feature.search

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
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
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.composables.icons.lucide.BellPlus
import com.composables.icons.lucide.Film
import com.composables.icons.lucide.Lucide
import com.composables.icons.lucide.RefreshCw
import com.composables.icons.lucide.Wifi
import com.mediahub.android.core.designsystem.MediaHubButton
import com.mediahub.android.core.designsystem.MediaHubColors
import com.mediahub.android.core.designsystem.MediaHubIcon
import com.mediahub.android.core.designsystem.MediaHubIconButton
import com.mediahub.android.core.designsystem.MediaHubSearchField
import com.mediahub.android.core.designsystem.MediaHubText
import com.mediahub.android.core.network.IntegrationHealth
import com.mediahub.android.core.network.DiscoveryItem
import com.mediahub.android.core.network.SearchCandidate
import java.util.Locale

@Composable
internal fun SearchRoute(
    viewModel: SearchViewModel,
    onTransferCreated: () -> Unit,
    onSubscriptionRequested: (SearchCandidate) -> Unit,
) {
    val uiState by viewModel.uiState.collectAsState()

    LaunchedEffect(viewModel) { viewModel.refreshAll() }
    LaunchedEffect(viewModel, onTransferCreated) {
        viewModel.events.collect { event ->
            when (event) {
                SearchEvent.TransferCreated -> onTransferCreated()
                is SearchEvent.SubscriptionRequested -> onSubscriptionRequested(event.candidate)
            }
        }
    }
    SearchScreen(
        uiState = uiState,
        onQueryChanged = viewModel::onQueryChanged,
        onSearch = viewModel::submitSearch,
        onRefreshOverview = viewModel::refreshOverview,
        onTrendingSelected = viewModel::searchTrending,
        onCandidateSelected = viewModel::onCandidateSelected,
        onRecommendationSelected = viewModel::searchRecommendation,
        onTransfer = viewModel::createTransfer,
        onSubscribe = viewModel::requestSubscription,
    )
}

@Composable
private fun SearchScreen(
    uiState: SearchUiState,
    onQueryChanged: (String) -> Unit,
    onSearch: () -> Unit,
    onRefreshOverview: () -> Unit,
    onTrendingSelected: (DiscoveryItem) -> Unit,
    onCandidateSelected: (String) -> Unit,
    onRecommendationSelected: (DiscoveryItem) -> Unit,
    onTransfer: (String) -> Unit,
    onSubscribe: (String) -> Unit,
) {
    val selectedCandidate = uiState.selectedCandidate
    Column(
        modifier = Modifier
            .fillMaxSize()
            .background(MediaHubColors.Canvas)
            .padding(horizontal = 16.dp),
    ) {
        MediaHubText(
            text = "搜索资源并加入自动转存流程",
            modifier = Modifier.padding(top = 14.dp, bottom = 10.dp),
            color = MediaHubColors.TextMuted,
            fontSize = 13.sp,
        )
        MediaHubSearchField(
            value = uiState.query,
            onValueChange = onQueryChanged,
            onSearch = onSearch,
            enabled = !uiState.searching,
            modifier = Modifier.fillMaxWidth(),
        )
        StatusLine(
            integrations = uiState.integrations,
            refreshing = uiState.refreshingOverview,
            onRefresh = onRefreshOverview,
        )
        if (uiState.submittedQuery.isBlank() && uiState.trending.isNotEmpty()) {
            MediaHubText(
                text = "本周热门",
                modifier = Modifier.padding(top = 18.dp, bottom = 8.dp),
                color = MediaHubColors.TextStrong,
                fontSize = 15.sp,
                fontWeight = FontWeight.Medium,
            )
            LazyRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                items(uiState.trending, key = { "${it.mediaType}-${it.tmdbId}" }) { item ->
                    TrendingItem(item = item, onClick = { onTrendingSelected(item) })
                }
            }
        }
        if (uiState.submittedQuery.isNotBlank() || uiState.searching) {
            Row(
                modifier = Modifier.fillMaxWidth().padding(top = 18.dp),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                MediaHubText(
                    text = if (uiState.searching) "正在搜索…" else "搜索结果",
                    color = MediaHubColors.TextStrong,
                    fontSize = 18.sp,
                    fontWeight = FontWeight.Medium,
                )
                MediaHubText(text = if (uiState.searching) "" else "${uiState.results.size}", color = MediaHubColors.Source, fontSize = 13.sp)
            }
        }
        uiState.errorMessage?.let { message ->
            StatusMessage(message, MediaHubColors.Error)
        }
        uiState.sourceMessage?.let { message ->
            StatusMessage(message, MediaHubColors.Warning)
        }
        uiState.transferMessage?.let { message ->
            StatusMessage(message, MediaHubColors.Error)
        }
        LazyColumn(
            modifier = Modifier.weight(1f),
            contentPadding = PaddingValues(vertical = 12.dp),
            verticalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            items(items = uiState.results, key = { it.id }) { candidate ->
                ReleaseRow(
                    candidate = candidate,
                    selected = selectedCandidate?.id == candidate.id,
                    onClick = { onCandidateSelected(candidate.id) },
                )
            }
            if (uiState.recommendations.isNotEmpty()) {
                item(key = "recommendations-heading") {
                    MediaHubText(
                        text = "相似内容",
                        modifier = Modifier.padding(top = 12.dp),
                        color = MediaHubColors.TextStrong,
                        fontSize = 14.sp,
                        fontWeight = FontWeight.Medium,
                    )
                }
                item(key = "recommendations") {
                    LazyRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        items(uiState.recommendations, key = { "${it.mediaType}-${it.tmdbId}" }) { item ->
                            TrendingItem(item = item, onClick = { onRecommendationSelected(item) })
                        }
                    }
                }
            }
            if (!uiState.searching && uiState.submittedQuery.isNotBlank() && uiState.results.isEmpty() && uiState.errorMessage == null) {
                item {
                    MediaHubText(
                        text = "没有找到匹配资源",
                        modifier = Modifier.padding(vertical = 28.dp),
                        color = MediaHubColors.TextMuted,
                        fontSize = 13.sp,
                    )
                }
            }
        }
        if (selectedCandidate != null) {
            Row(
                modifier = Modifier.fillMaxWidth().padding(bottom = 10.dp),
                horizontalArrangement = Arrangement.spacedBy(10.dp),
            ) {
                MediaHubButton(
                    label = "订阅",
                    icon = Lucide.BellPlus,
                    enabled = selectedCandidate.tmdbId != null && selectedCandidate.transferState != "identity_required",
                    modifier = Modifier.weight(0.35f),
                    onClick = { onSubscribe(selectedCandidate.id) },
                )
                MediaHubButton(
                    label = when {
                        uiState.transferringCandidateId == selectedCandidate.id -> "正在创建…"
                        selectedCandidate.transferState == "identity_required" -> "身份待确认"
                        selectedCandidate.transferToken == null -> "不可转存"
                        else -> "开始转存"
                    },
                    enabled = selectedCandidate.transferToken != null && uiState.transferringCandidateId == null,
                    modifier = Modifier.weight(0.65f),
                    onClick = { onTransfer(selectedCandidate.id) },
                )
            }
        }
    }
}

@Composable
private fun TrendingItem(item: DiscoveryItem, onClick: () -> Unit) {
    Row(
        modifier = Modifier
            .width(150.dp)
            .background(MediaHubColors.Surface, RoundedCornerShape(7.dp))
            .heightIn(min = 48.dp)
            .clickable(role = Role.Button, onClick = onClick)
            .padding(10.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        MediaHubIcon(
            imageVector = Lucide.Film,
            contentDescription = null,
            tint = MediaHubColors.Source,
            modifier = Modifier.size(18.dp),
        )
        Spacer(Modifier.width(8.dp))
        Column(Modifier.weight(1f)) {
            MediaHubText(text = item.title, fontSize = 12.sp, fontWeight = FontWeight.Medium)
            MediaHubText(
                text = "${item.year.takeIf { it > 0 } ?: "年份未知"} · ${if (item.mediaType == "movie") "电影" else "剧集"}",
                color = MediaHubColors.TextMuted,
                fontSize = 12.sp,
            )
        }
    }
}


@Composable
private fun StatusLine(
    integrations: List<IntegrationHealth>,
    refreshing: Boolean,
    onRefresh: () -> Unit,
) {
    Row(
        modifier = Modifier.fillMaxWidth().padding(top = 12.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        MediaHubIcon(
            imageVector = Lucide.Wifi,
            contentDescription = null,
            tint = if (integrations.any { it.status == "healthy" }) MediaHubColors.Accent else MediaHubColors.TextMuted,
            modifier = Modifier.size(16.dp),
        )
        Spacer(Modifier.width(8.dp))
        Column(Modifier.weight(1f)) {
            MediaHubText(text = "服务状态", color = MediaHubColors.TextStrong, fontSize = 12.sp)
            MediaHubText(
                text = integrations.takeIf { it.isNotEmpty() }
                    ?.joinToString(" · ") { "${it.label} ${statusLabel(it.status)}" }
                    ?: "尚未读取",
                color = MediaHubColors.TextMuted,
                fontSize = 12.sp,
            )
        }
        MediaHubIconButton(
            imageVector = Lucide.RefreshCw,
            contentDescription = "刷新服务状态",
            onClick = onRefresh,
            enabled = !refreshing,
        )
    }
}

@Composable
private fun StatusMessage(message: String, color: androidx.compose.ui.graphics.Color) {
    MediaHubText(
        text = message,
        modifier = Modifier
            .padding(top = 10.dp)
            .semantics { liveRegion = LiveRegionMode.Polite },
        color = color,
        fontSize = 12.sp,
    )
}

@Composable
private fun ReleaseRow(
    candidate: SearchCandidate,
    selected: Boolean,
    onClick: () -> Unit,
) {
    val background = if (selected) MediaHubColors.SurfaceSelected else MediaHubColors.Surface

    Column(
        modifier = Modifier
            .fillMaxWidth()
            .heightIn(min = 48.dp)
            .background(background, RoundedCornerShape(8.dp))
            .selectable(
                selected = selected,
                onClick = onClick,
                role = Role.RadioButton,
            )
            .padding(14.dp),
        verticalArrangement = Arrangement.spacedBy(6.dp),
    ) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.Top,
        ) {
            MediaHubText(
                text = candidateDisplayTitle(candidate),
                modifier = Modifier.weight(1f).padding(end = 10.dp),
                color = MediaHubColors.TextStrong,
                fontSize = 15.sp,
                fontWeight = FontWeight.SemiBold,
            )
            MediaHubText(text = candidate.provider ?: candidate.source, color = MediaHubColors.Source, fontSize = 12.sp)
        }
        MediaHubText(
            text = buildString {
                append(candidate.release.resolution)
                append(" · ")
                append(candidate.release.videoCodec)
                candidate.release.dynamicRange?.let { append(" · ").append(it) }
                candidate.release.audio?.let { append(" · ").append(it) }
            },
            color = MediaHubColors.TextSecondary,
            fontSize = 12.sp,
        )
        MediaHubText(
            text = formatBytes(candidate.release.sizeBytes),
            color = MediaHubColors.TextFaint,
            fontSize = 12.sp,
        )
    }
}

private fun statusLabel(status: String): String = when (status) {
    "healthy" -> "正常"
    "unconfigured" -> "未配置"
    "unauthorized" -> "鉴权失败"
    "degraded" -> "异常"
    else -> "不可用"
}

private fun formatBytes(size: Long): String {
    if (size <= 0) return "大小未知"
    val units = listOf("B", "KB", "MB", "GB", "TB")
    var value = size.toDouble()
    var unit = 0
    while (value >= 1024 && unit < units.lastIndex) {
        value /= 1024
        unit += 1
    }
    return String.format(Locale.ROOT, if (unit == 0) "%.0f %s" else "%.1f %s", value, units[unit])
}

private fun candidateDisplayTitle(candidate: SearchCandidate): String {
    if (candidate.season <= 0) return candidate.title
    if (candidate.episodeStart <= 0) return "${candidate.title} · S${candidate.season}"
    val episodes = if (candidate.episodeStart == candidate.episodeEnd) {
        candidate.episodeStart.toString()
    } else {
        "${candidate.episodeStart}-${candidate.episodeEnd}"
    }
    return "${candidate.title} · S${candidate.season}E$episodes"
}
