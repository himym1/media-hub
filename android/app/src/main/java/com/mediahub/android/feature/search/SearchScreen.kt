package com.mediahub.android.feature.search

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.selection.selectable
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.testTag
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import com.composables.icons.lucide.BellPlus
import com.composables.icons.lucide.Lucide
import com.composables.icons.lucide.RefreshCw
import com.composables.icons.lucide.Wifi
import com.mediahub.android.app.LocalTwoPane
import com.mediahub.android.core.designsystem.MediaHubButton
import com.mediahub.android.core.designsystem.MediaHubCard
import com.mediahub.android.core.designsystem.MediaHubColors
import com.mediahub.android.core.designsystem.MediaHubIcon
import com.mediahub.android.core.designsystem.MediaHubIconButton
import com.mediahub.android.core.designsystem.MediaHubListDetail
import com.mediahub.android.core.designsystem.MediaHubListDivider
import com.mediahub.android.core.designsystem.MediaHubPreferenceRow
import com.mediahub.android.core.designsystem.MediaHubSearchBar
import com.mediahub.android.core.designsystem.MediaHubSearchField
import com.mediahub.android.core.designsystem.MediaHubSmallTitle
import com.mediahub.android.core.designsystem.MediaHubText
import com.mediahub.android.core.image.RemotePoster
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
internal fun SearchScreen(
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
    if (LocalTwoPane.current) {
        SearchTwoPane(
            uiState = uiState,
            onQueryChanged = onQueryChanged,
            onSearch = onSearch,
            onRefreshOverview = onRefreshOverview,
            onTrendingSelected = onTrendingSelected,
            onCandidateSelected = onCandidateSelected,
            onRecommendationSelected = onRecommendationSelected,
            onTransfer = onTransfer,
            onSubscribe = onSubscribe,
        )
        return
    }
    val selectedCandidate = uiState.selectedCandidate
    var expanded by remember {
        mutableStateOf(uiState.submittedQuery.isNotBlank() || uiState.results.isNotEmpty())
    }
    LaunchedEffect(uiState.submittedQuery, uiState.searching, uiState.results.size) {
        if (uiState.submittedQuery.isNotBlank() || uiState.searching || uiState.results.isNotEmpty()) {
            expanded = true
        }
    }
    Column(modifier = Modifier.fillMaxSize()) {
        MediaHubSearchBar(
            query = uiState.query,
            onQueryChange = onQueryChanged,
            onSearch = onSearch,
            expanded = expanded,
            onExpandedChange = { expanded = it },
            enabled = !uiState.searching,
            modifier = if (expanded) {
                Modifier.weight(1f).fillMaxWidth().padding(top = 4.dp)
            } else {
                Modifier.fillMaxWidth().padding(top = 4.dp)
            },
        ) {
            Column(modifier = Modifier.fillMaxSize().padding(horizontal = 12.dp)) {
                if (uiState.submittedQuery.isNotBlank() || uiState.searching) {
                    MediaHubSmallTitle(text = if (uiState.searching) "正在搜索…" else "搜索结果 ${uiState.results.size}")
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
                    contentPadding = PaddingValues(vertical = 8.dp),
                    verticalArrangement = Arrangement.spacedBy(12.dp),
                ) {
                    if (uiState.results.isNotEmpty()) {
                        item(key = "results") {
                            MediaHubCard {
                                uiState.results.forEachIndexed { index, candidate ->
                                    if (index > 0) MediaHubListDivider()
                                    ReleaseRow(
                                        candidate = candidate,
                                        selected = selectedCandidate?.id == candidate.id,
                                        onClick = { onCandidateSelected(candidate.id) },
                                    )
                                }
                            }
                        }
                    }
                    if (uiState.recommendations.isNotEmpty()) {
                        item(key = "recommendations-heading") {
                            MediaHubSmallTitle(text = "相似内容")
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
                                selectedCandidate.transferToken == null -> "工作流不可用"
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
        if (!expanded) {
            Column(
                modifier = Modifier
                    .weight(1f)
                    .fillMaxWidth()
                    .padding(horizontal = 12.dp),
            ) {
                StatusLine(
                    integrations = uiState.integrations,
                    refreshing = uiState.refreshingOverview,
                    onRefresh = onRefreshOverview,
                )
                if (uiState.trending.isNotEmpty()) {
                    MediaHubSmallTitle(text = "本周热门")
                    LazyRow(
                        horizontalArrangement = Arrangement.spacedBy(10.dp),
                        contentPadding = PaddingValues(bottom = 8.dp),
                    ) {
                        items(uiState.trending, key = { "${it.mediaType}-${it.tmdbId}" }) { item ->
                            TrendingItem(item = item, onClick = { onTrendingSelected(item) })
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun SearchTwoPane(
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
    val showingResults = uiState.submittedQuery.isNotBlank() || uiState.searching || uiState.results.isNotEmpty()
    MediaHubListDetail(
        detailOpen = selectedCandidate != null,
        emptyTitle = "选择一个资源",
        emptyMessage = "从左侧打开详情后转存或订阅",
        emptyIcon = Lucide.Search,
        list = {
            Column(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(horizontal = 12.dp)
                    .testTag("search-list-pane"),
            ) {
                MediaHubSearchField(
                    value = uiState.query,
                    onValueChange = onQueryChanged,
                    onSearch = onSearch,
                    enabled = !uiState.searching,
                    modifier = Modifier.fillMaxWidth().padding(top = 4.dp, bottom = 8.dp),
                )
                if (showingResults) {
                    if (uiState.submittedQuery.isNotBlank() || uiState.searching) {
                        MediaHubSmallTitle(text = if (uiState.searching) "正在搜索…" else "搜索结果 ${uiState.results.size}")
                    }
                    uiState.errorMessage?.let { StatusMessage(it, MediaHubColors.Error) }
                    uiState.sourceMessage?.let { StatusMessage(it, MediaHubColors.Warning) }
                    uiState.transferMessage?.let { StatusMessage(it, MediaHubColors.Error) }
                    LazyColumn(
                        modifier = Modifier.weight(1f),
                        contentPadding = PaddingValues(vertical = 8.dp),
                        verticalArrangement = Arrangement.spacedBy(12.dp),
                    ) {
                        if (uiState.results.isNotEmpty()) {
                            item(key = "results") {
                                MediaHubCard {
                                    uiState.results.forEachIndexed { index, candidate ->
                                        if (index > 0) MediaHubListDivider()
                                        ReleaseRow(
                                            candidate = candidate,
                                            selected = selectedCandidate?.id == candidate.id,
                                            onClick = { onCandidateSelected(candidate.id) },
                                        )
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
                } else {
                    SearchIdleOverview(
                        integrations = uiState.integrations,
                        refreshing = uiState.refreshingOverview,
                        trending = uiState.trending,
                        onRefresh = onRefreshOverview,
                        onTrendingSelected = onTrendingSelected,
                    )
                }
            }
        },
        detail = {
            if (selectedCandidate != null) {
                SearchCandidateDetail(
                    uiState = uiState,
                    candidate = selectedCandidate,
                    onTransfer = onTransfer,
                    onSubscribe = onSubscribe,
                    onRecommendationSelected = onRecommendationSelected,
                )
            }
        },
    )
}

@Composable
private fun SearchIdleOverview(
    integrations: List<IntegrationHealth>,
    refreshing: Boolean,
    trending: List<DiscoveryItem>,
    onRefresh: () -> Unit,
    onTrendingSelected: (DiscoveryItem) -> Unit,
) {
    Column(modifier = Modifier.fillMaxSize()) {
        StatusLine(integrations = integrations, refreshing = refreshing, onRefresh = onRefresh)
        if (trending.isNotEmpty()) {
            MediaHubSmallTitle(text = "本周热门")
            LazyRow(
                horizontalArrangement = Arrangement.spacedBy(10.dp),
                contentPadding = PaddingValues(bottom = 8.dp),
            ) {
                items(trending, key = { "${it.mediaType}-${it.tmdbId}" }) { item ->
                    TrendingItem(item = item, onClick = { onTrendingSelected(item) })
                }
            }
        }
    }
}

@Composable
private fun SearchCandidateDetail(
    uiState: SearchUiState,
    candidate: SearchCandidate,
    onTransfer: (String) -> Unit,
    onSubscribe: (String) -> Unit,
    onRecommendationSelected: (DiscoveryItem) -> Unit,
) {
    Column(
        modifier = Modifier
            .fillMaxSize()
            .padding(horizontal = 16.dp)
            .testTag("search-detail-pane"),
    ) {
        LazyColumn(
            modifier = Modifier.weight(1f),
            contentPadding = PaddingValues(top = 8.dp, bottom = 16.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            item {
                MediaHubText(text = candidateDisplayTitle(candidate), fontSize = 20.sp, fontWeight = FontWeight.SemiBold)
                MediaHubText(
                    text = buildString {
                        append(if (candidate.mediaType == "movie") "电影" else "剧集")
                        if (candidate.year > 0) append(" · ").append(candidate.year)
                        append(" · ").append(candidate.provider ?: candidate.source)
                    },
                    modifier = Modifier.padding(top = 4.dp),
                    color = MediaHubColors.TextMuted,
                    fontSize = 13.sp,
                )
            }
            item {
                MediaHubCard(insideMargin = PaddingValues(16.dp)) {
                    MediaHubText(
                        text = buildString {
                            append(candidate.release.resolution)
                            append(" · ")
                            append(candidate.release.videoCodec)
                            candidate.release.dynamicRange?.let { append(" · ").append(it) }
                            candidate.release.audio?.let { append(" · ").append(it) }
                        },
                        color = MediaHubColors.TextSecondary,
                        fontSize = 13.sp,
                    )
                    MediaHubText(
                        text = formatBytes(candidate.release.sizeBytes),
                        modifier = Modifier.padding(top = 8.dp),
                        color = MediaHubColors.TextMuted,
                        fontSize = 12.sp,
                    )
                    if (candidate.transferState.isNotEmpty() && candidate.transferState != "unknown") {
                        MediaHubText(
                            text = when (candidate.transferState) {
                                "available" -> "可转存"
                                "transferring" -> "转存中"
                                "transferred" -> "已转存"
                                "identity_required" -> "身份待确认"
                                else -> candidate.transferState
                            },
                            modifier = Modifier.padding(top = 8.dp),
                            color = when (candidate.transferState) {
                                "available", "transferred" -> MediaHubColors.Success
                                "identity_required" -> MediaHubColors.Warning
                                else -> MediaHubColors.TextMuted
                            },
                            fontSize = 12.sp,
                        )
                    }
                }
            }
            uiState.transferMessage?.let { message ->
                item { StatusMessage(message, MediaHubColors.Error) }
            }
            if (uiState.recommendations.isNotEmpty()) {
                item { MediaHubSmallTitle(text = "相似内容") }
                item {
                    LazyRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        items(uiState.recommendations, key = { "${it.mediaType}-${it.tmdbId}" }) { item ->
                            TrendingItem(item = item, onClick = { onRecommendationSelected(item) })
                        }
                    }
                }
            }
        }
        Row(
            modifier = Modifier.fillMaxWidth().padding(bottom = 12.dp),
            horizontalArrangement = Arrangement.spacedBy(10.dp),
        ) {
            MediaHubButton(
                label = "订阅",
                icon = Lucide.BellPlus,
                enabled = candidate.tmdbId != null && candidate.transferState != "identity_required",
                modifier = Modifier.weight(0.35f),
                onClick = { onSubscribe(candidate.id) },
            )
            MediaHubButton(
                label = when {
                    uiState.transferringCandidateId == candidate.id -> "正在创建…"
                    candidate.transferState == "identity_required" -> "身份待确认"
                    candidate.transferToken == null -> "工作流不可用"
                    else -> "开始转存"
                },
                enabled = candidate.transferToken != null && uiState.transferringCandidateId == null,
                modifier = Modifier.weight(0.65f),
                onClick = { onTransfer(candidate.id) },
            )
        }
    }
}

@Composable
private fun TrendingItem(item: DiscoveryItem, onClick: () -> Unit) {
    Column(modifier = Modifier.width(132.dp)) {
        MediaHubCard(onClick = onClick) {
            RemotePoster(
                url = item.posterUrl,
                contentDescription = item.title,
                modifier = Modifier.fillMaxWidth().aspectRatio(2f / 3f),
            )
        }
        MediaHubText(
            text = item.title,
            modifier = Modifier.padding(top = 8.dp),
            color = MediaHubColors.TextPrimary,
            fontSize = 13.sp,
            fontWeight = FontWeight.Medium,
            maxLines = 2,
            overflow = androidx.compose.ui.text.style.TextOverflow.Ellipsis,
        )
        MediaHubText(
            text = buildString {
                append(if (item.mediaType == "movie") "电影" else "剧集")
                if (item.year > 0) append(" · ").append(item.year)
            },
            modifier = Modifier.padding(top = 2.dp),
            color = MediaHubColors.TextMuted,
            fontSize = 12.sp,
            maxLines = 1,
        )
    }
}

@Composable
private fun StatusLine(
    integrations: List<IntegrationHealth>,
    refreshing: Boolean,
    onRefresh: () -> Unit,
) {
    MediaHubCard(modifier = Modifier.padding(top = 12.dp)) {
        MediaHubPreferenceRow(
            title = "服务状态",
            summary = integrations.takeIf { it.isNotEmpty() }
                ?.joinToString(" · ") { "${it.label} ${statusLabel(it.status)}" }
                ?: "尚未读取",
            onClick = onRefresh,
            start = {
                MediaHubIcon(
                    imageVector = Lucide.Wifi,
                    contentDescription = null,
                    tint = if (integrations.any { it.status == "healthy" }) MediaHubColors.Accent else MediaHubColors.TextMuted,
                    modifier = Modifier.size(18.dp).padding(end = 12.dp),
                )
            },
            end = {
                MediaHubIconButton(
                    imageVector = Lucide.RefreshCw,
                    contentDescription = "刷新服务状态",
                    onClick = onRefresh,
                    enabled = !refreshing,
                )
            },
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
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .heightIn(min = 52.dp)
            .selectable(
                selected = selected,
                onClick = onClick,
                role = Role.RadioButton,
            )
            .padding(horizontal = 16.dp, vertical = 12.dp),
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
            MediaHubText(
                text = candidate.provider ?: candidate.source,
                color = MediaHubColors.Source,
                fontSize = 12.sp,
                fontWeight = FontWeight.Medium,
            )
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
        Row(
            modifier = Modifier.fillMaxWidth(),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            MediaHubText(
                text = formatBytes(candidate.release.sizeBytes),
                color = MediaHubColors.TextMuted,
                fontSize = 12.sp,
            )
            if (candidate.transferState.isNotEmpty() && candidate.transferState != "unknown") {
                MediaHubText(
                    text = when (candidate.transferState) {
                        "available" -> "可转存"
                        "transferring" -> "转存中"
                        "transferred" -> "已转存"
                        "identity_required" -> "身份待确认"
                        else -> candidate.transferState
                    },
                    color = when (candidate.transferState) {
                        "available", "transferred" -> MediaHubColors.Success
                        "identity_required" -> MediaHubColors.Warning
                        else -> MediaHubColors.TextMuted
                    },
                    fontSize = 12.sp,
                )
            }
        }
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
